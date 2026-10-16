// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package proxy

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Dialer 根据代理配置创建自定义 Dialer，支持通过 HTTP 代理建立 TCP 连接
// 如果 proxyURL 为空或 enabled 为 false，返回 nil（调用方使用默认直连）
func Dialer(enabled bool, proxyURL string) (func(network, addr string) (net.Conn, error), error) {
	if !enabled || proxyURL == "" {
		return nil, nil
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理地址解析失败: %w", err)
	}

	switch u.Scheme {
	case "http", "https":
		return httpProxyDialer(u), nil
	case "socks5":
		return socks5Dialer(u)
	default:
		return nil, fmt.Errorf("不支持的代理协议: %s（支持 http/https/socks5）", u.Scheme)
	}
}

// NewHTTPClient 创建一个使用指定代理的 http.Client
func NewHTTPClient(enabled bool, proxyURL string) (*http.Client, error) {
	if !enabled || proxyURL == "" {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理地址解析失败: %w", err)
	}

	transport := &http.Transport{
		Proxy:           http.ProxyURL(u),
		IdleConnTimeout: 90 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}

// httpProxyDialer 通过 HTTP CONNECT 代理建立 TCP 连接
func httpProxyDialer(proxyURL *url.URL) func(network, addr string) (net.Conn, error) {
	return func(network, addr string) (net.Conn, error) {
		// 先连接到代理服务器
		proxyAddr := proxyURL.Host
		if proxyURL.Port() == "" {
			proxyAddr += ":80"
		}

		conn, err := net.DialTimeout("tcp", proxyAddr, 15*time.Second)
		if err != nil {
			return nil, fmt.Errorf("连接代理 %s 失败: %w", proxyAddr, err)
		}

		// 发送 CONNECT 请求
		connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
		if proxyURL.User != nil {
			pass, _ := proxyURL.User.Password()
			connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n",
				basicAuth(proxyURL.User.Username(), pass))
		}
		connectReq += "\r\n"

		if _, err := conn.Write([]byte(connectReq)); err != nil {
			conn.Close()
			return nil, fmt.Errorf("发送 CONNECT 请求失败: %w", err)
		}

		// 读取响应
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("读取代理响应失败: %w", err)
		}

		resp := string(buf[:n])
		if len(resp) < 12 || resp[9:12] != "200" {
			conn.Close()
			return nil, fmt.Errorf("代理 CONNECT 失败: %s", resp)
		}

		return conn, nil
	}
}

// socks5Dialer 通过 SOCKS5 代理建立连接
func socks5Dialer(u *url.URL) (func(network, addr string) (net.Conn, error), error) {
	return func(network, addr string) (net.Conn, error) {
		proxyAddr := u.Host
		if u.Port() == "" {
			proxyAddr += ":1080"
		}

		conn, err := net.DialTimeout("tcp", proxyAddr, 15*time.Second)
		if err != nil {
			return nil, fmt.Errorf("连接 SOCKS5 代理 %s 失败: %w", proxyAddr, err)
		}

		// SOCKS5 握手
		// 1. 认证协商 (无认证)
		conn.Write([]byte{0x05, 0x01, 0x00})
		resp := make([]byte, 2)
		if _, err := conn.Read(resp); err != nil {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5 握手失败: %w", err)
		}

		// 2. 连接请求
		host, port, err := parseHostPort(addr)
		if err != nil {
			conn.Close()
			return nil, err
		}

		req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
		req = append(req, []byte(host)...)
		req = append(req, byte(port>>8), byte(port&0xff))
		conn.Write(req)

		resp2 := make([]byte, 10)
		if _, err := conn.Read(resp2); err != nil {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5 连接请求失败: %w", err)
		}
		if resp2[1] != 0x00 {
			conn.Close()
			return nil, fmt.Errorf("SOCKS5 连接被拒绝 (code=%d)", resp2[1])
		}

		return conn, nil
	}, nil
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func parseHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	return host, port, nil
}
