// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package pop3

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"magicmail/config"
	"magicmail/models"
	"magicmail/proxy"
)

// POP3 各阶段超时。
//
// 背景：POP3 客户端此前完全裸奔——net.Dial 没有 Timeout、textproto 读取没有 deadline。
// 一旦服务端建立 TCP 后不响应（半开连接、SSL 端口被当明文连、服务器静默踢人），
// 读取会永久阻塞，Worker 卡死在 syncOnce 里：既不报错也不退出，
// 表现为"点击同步后终端彻底没动静"，且手动同步信号永远排不上。
// 这里给建连、命令响应、邮件下载分别设上限，保证任何挂起都能在可预期时间内变成一条错误日志。
const (
	pop3DialTimeout = 30 * time.Second // 建连 + TLS 握手
	pop3CmdTimeout  = 60 * time.Second // 普通命令（USER/PASS/STAT/DELE 等）响应
	pop3ListTimeout = 2 * time.Minute  // LIST 全量列表（大邮箱行数多）
	pop3RetrTimeout = 5 * time.Minute  // 单封邮件下载
)

// POP3Client 封装 POP3 客户端连接
type POP3Client struct {
	conn    net.Conn        // 底层 TCP/TLS 连接
	tp      *textproto.Conn // 文本协议读写封装
	Account *models.MailAccount
	config  *config.Config
}

// NewPOP3Client 创建新的 POP3 邮件连接实例
func NewPOP3Client(account *models.MailAccount, cfg *config.Config) (*POP3Client, error) {
	host := account.ImapHost
	port := account.Port
	// 用 JoinHostPort 而非 Sprintf("%s:%d")：后者在 IPv6 主机下会拼出非法地址
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// 获取自定义 Dialer（代理）
	customDialer, err := proxy.Dialer(account.ProxyEnabled, account.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理配置错误: %w", err)
	}

	dialer := &net.Dialer{Timeout: pop3DialTimeout}

	var conn net.Conn

	if customDialer != nil {
		// 通过代理建立 TCP 连接
		conn, err = customDialer("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("通过代理连接 POP3 服务器 %s 失败: %w", addr, err)
		}
	} else {
		conn, err = dialer.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("连接 POP3 服务器 %s 失败: %w", addr, err)
		}
	}

	if account.Protocol != "pop3-no-ssl" {
		// 升级为 TLS（握手同样要设上限，否则对端不发 ServerHello 时会一直等）
		tlsConfig := &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}
		tlsConn := tls.Client(conn, tlsConfig)
		_ = conn.SetDeadline(time.Now().Add(pop3DialTimeout))
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("POP3 TLS 握手失败 (%s): %w", addr, err)
		}
		conn = tlsConn
	}

	client := &POP3Client{
		conn:    conn,
		tp:      textproto.NewConn(conn),
		Account: account,
		config:  cfg,
	}

	// 读取服务器欢迎消息（应包含 +OK）
	client.setDeadline(pop3CmdTimeout)
	_, err = client.tp.ReadLine()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("读取服务器欢迎消息失败: %w", err)
	}
	// 欢迎消息之后进入长会话：清除 deadline，由各命令按需设置
	client.setDeadline(0)

	return client, nil
}

// setDeadline 设置底层连接的读写 deadline，d <= 0 表示清除 deadline。
//
// 必须在每条命令交互前设置、结束后清除：POP3 会话是长连接，
// 若一直挂着短 deadline，空闲时段（如逐封下载大邮件的间隔）会被误判超时。
func (c *POP3Client) setDeadline(d time.Duration) {
	if c.conn == nil {
		return
	}
	if d <= 0 {
		_ = c.conn.SetDeadline(time.Time{})
		return
	}
	_ = c.conn.SetDeadline(time.Now().Add(d))
}

// Authenticate 使用 USER/PASS 命令认证
func (c *POP3Client) Authenticate() error {
	if c.Account.Password == "" {
		return fmt.Errorf("密码为空，无法认证")
	}

	// 发送 USER 命令
	c.setDeadline(pop3CmdTimeout)
	id, err := c.tp.Cmd("USER %s", c.Account.Username)
	if err != nil {
		c.setDeadline(0)
		return fmt.Errorf("发送 USER 命令失败: %w", err)
	}
	c.tp.StartResponse(id)
	_, _, err = c.tp.ReadResponse(200) // 期望 +OK (2xx)
	c.tp.EndResponse(id)
	if err != nil {
		c.setDeadline(0)
		return fmt.Errorf("POP3 USER 命令失败 (%s): %w", c.Account.Username, err)
	}

	// 发送 PASS 命令（服务端校验密码可能较慢，同样需要上限）
	c.setDeadline(pop3CmdTimeout)
	id, err = c.tp.Cmd("PASS %s", c.Account.Password)
	if err != nil {
		c.setDeadline(0)
		return fmt.Errorf("发送 PASS 命令失败: %w", err)
	}
	c.tp.StartResponse(id)
	_, _, err = c.tp.ReadResponse(200)
	c.tp.EndResponse(id)
	c.setDeadline(0)
	if err != nil {
		return fmt.Errorf("POP3 登录失败 (%s@%s): %w", c.Account.Username, c.Account.Email, err)
	}

	log.Printf("✅ POP3 认证成功: %s", c.Account.Email)
	return nil
}

// MessageCount 获取邮箱中的邮件数量
func (c *POP3Client) MessageCount() (int, error) {
	c.setDeadline(pop3ListTimeout)
	defer c.setDeadline(0)

	id, err := c.tp.Cmd("LIST")
	if err != nil {
		return 0, err
	}
	c.tp.StartResponse(id)

	lineCount := 0
	count := 0
	for {
		line, err := c.tp.ReadLine()
		if err != nil {
			break
		}
		if line == "." {
			break
		}
		if lineCount == 0 {
			// 第一行格式：count total_size
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				count, _ = strconv.Atoi(parts[0])
			}
		}
		lineCount++
	}

	c.tp.EndResponse(id)
	return count, nil
}

// RetrieveMessage 按序号获取单封邮件的原始内容（RFC822 格式）
func (c *POP3Client) RetrieveMessage(seq int) ([]byte, int64, error) {
	// 先获取邮件大小
	size, err := c.getMsgSize(seq)
	if err != nil {
		return nil, 0, err
	}

	// 发送 RETR 命令（响应含整封邮件，用更长的下载超时）
	c.setDeadline(pop3RetrTimeout)
	id, err := c.tp.Cmd("RETR %d", seq)
	if err != nil {
		c.setDeadline(0)
		return nil, 0, fmt.Errorf("发送 RETR 命令失败: %w", err)
	}
	c.tp.StartResponse(id)

	// 使用 dotReader 读取多行数据（处理行首 "." 的 byte-stuffing）
	dr := c.tp.DotReader()
	data, err := io.ReadAll(dr)
	if err != nil {
		c.tp.EndResponse(id)
		c.setDeadline(0)
		return nil, 0, fmt.Errorf("读取邮件内容失败: %w", err)
	}

	c.tp.EndResponse(id)
	c.setDeadline(0)
	return data, size, nil
}

// getMsgSize 获取指定序号邮件的大小
func (c *POP3Client) getMsgSize(seq int) (int64, error) {
	c.setDeadline(pop3CmdTimeout)
	defer c.setDeadline(0)

	id, err := c.tp.Cmd("LIST %d", seq)
	if err != nil {
		return 0, err
	}
	c.tp.StartResponse(id)
	_, line, err := c.tp.ReadResponse(200)
	c.tp.EndResponse(id)
	if err != nil {
		return 0, err
	}

	// 格式：seq size
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		size, e := strconv.ParseInt(parts[1], 10, 64)
		if e == nil {
			return size, nil
		}
	}
	return 0, fmt.Errorf("解析邮件大小失败")
}

// Close 关闭连接（发送 QUIT 后关闭 socket）
func (c *POP3Client) Close() {
	if c.conn != nil {
		// 退出前设短 deadline：对端若已失联，QUIT 的等待不应拖住 Worker 的下一次同步
		c.setDeadline(pop3CmdTimeout)

		// 发送 QUIT 命令
		_, err := c.tp.Cmd("QUIT")
		if err == nil {
			bufReader := bufio.NewReader(c.conn)
			// 读取 QUIT 响应
			for {
				line, rerr := bufReader.ReadString('\n')
				if rerr != nil || strings.HasPrefix(line, "+OK") || strings.HasPrefix(line, "-ERR") {
					break
				}
			}
		}
		c.conn.Close()
	}
}

// DeleteMessage 通过序号删除服务器上的邮件（DELE 命令）
func (c *POP3Client) DeleteMessage(seq int) error {
	line, err := c.sendCmd("DELE %d", seq)
	if err != nil {
		return fmt.Errorf("POP3 DELE 失败 (seq=%d): %w", seq, err)
	}
	log.Printf("🗑️  已从源服务器标记删除 (seq=%d, %s): %s", seq, c.Account.Email, line)
	return nil
}

// --- 内部工具函数 ---

// sendCmd 发送命令并检查 +OK 响应
func (c *POP3Client) sendCmd(format string, args ...interface{}) (string, error) {
	c.setDeadline(pop3CmdTimeout)
	defer c.setDeadline(0)

	id, err := c.tp.Cmd(format, args...)
	if err != nil {
		return "", err
	}
	c.tp.StartResponse(id)
	_, line, err := c.tp.ReadResponse(200)
	c.tp.EndResponse(id)
	return line, err
}
