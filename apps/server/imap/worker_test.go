// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package imap

import (
	"errors"
	"testing"
	"time"

	"magicmail/config"
	"magicmail/models"
)

// TestIdleBackoff 验证 IDLE 失败退避按 30s → 60s → 120s 翻倍，且不超过上限。
// 回归背景：旧实现固定 30 秒重试，导致瞬时故障下每 30 秒重连一次。
func TestIdleBackoff(t *testing.T) {
	cases := []struct {
		failures int
		want     time.Duration
	}{
		{0, idleBackoffBase},
		{1, 30 * time.Second},
		{2, 60 * time.Second},
		{3, 120 * time.Second},
		{4, 240 * time.Second},
		{20, idleBackoffMax}, // 达到上限后不再增长
	}

	for _, c := range cases {
		if got := idleBackoff(c.failures); got != c.want {
			t.Errorf("idleBackoff(%d) = %v, want %v", c.failures, got, c.want)
		}
	}
}

// TestIsIDLEUnsupportedErr 区分"该服务器无法使用 IDLE"与"网络抖动等瞬时故障"。
// 只有前者才会写入全局黑名单，后者应走退避重试。
func TestIsIDLEUnsupportedErr(t *testing.T) {
	unsupported := []string{
		// 服务器明确声明不支持
		"IDLE 命令失败: BAD Command not supported",
		"IDLE not supported by server",
		"IDLE not allowed on this mailbox",
		// 协议解析失败（确定性失败，重试无意义）
		`IDLE 命令失败: in response: cannot read tag: imapwire: expected atom, got "("`,
		"in response: imapwire: malformed response",
	}
	for _, msg := range unsupported {
		if !isIDLEUnsupportedErr(errors.New(msg)) {
			t.Errorf("isIDLEUnsupportedErr(%q) = false, want true", msg)
		}
	}

	transient := []string{
		"连接 imap.example.com:993 失败: dial tcp: i/o timeout",
		"EOF",
		"connection closed",
		"use of closed network connection",
	}
	for _, msg := range transient {
		if isIDLEUnsupportedErr(errors.New(msg)) {
			t.Errorf("isIDLEUnsupportedErr(%q) = true, want false", msg)
		}
	}

	if isIDLEUnsupportedErr(nil) {
		t.Error("isIDLEUnsupportedErr(nil) = true, want false")
	}
}

// TestIsKnownBrokenServer 验证静态黑名单按域名后缀匹配：
// 服务商在不同时期给出的主机别名较多，精确匹配容易漏网。
func TestIsKnownBrokenServer(t *testing.T) {
	broken := []string{
		"imap.189.cn",
		"imap.mail.189.cn", // 别名也要命中
		"IMAP.189.CN",      // 大小写不敏感
		" imap.189.cn ",    // 容忍手填时带入的空白
		"imap.139.com",
	}
	for _, host := range broken {
		if !isKnownBrokenServer(host) {
			t.Errorf("isKnownBrokenServer(%q) = false, want true", host)
		}
	}

	normal := []string{
		"",
		"imap.gmail.com",
		"imap.example.com",
		"not189.cn", // 不能只做子串包含
	}
	for _, host := range normal {
		if isKnownBrokenServer(host) {
			t.Errorf("isKnownBrokenServer(%q) = true, want false", host)
		}
	}
}

// TestIsIDLEUnsupportedStaticBlacklist 验证静态黑名单命中时直接跳过 IDLE，
// 连"首次尝试"都不做（189.cn 的 IDLE 响应不合规，每次尝试都是一次无效登录）。
func TestIsIDLEUnsupportedStaticBlacklist(t *testing.T) {
	w := &AccountWorker{account: &models.MailAccount{ImapHost: "imap.189.cn"}}
	if !w.isIDLEUnsupported() {
		t.Error("isIDLEUnsupported() = false, want true（189.cn 在静态黑名单中）")
	}

	// 白名单/未知服务器不应被静态黑名单拦截，仍需尝试 IDLE
	w2 := &AccountWorker{account: &models.MailAccount{ImapHost: "imap.example.com"}}
	if w2.isIDLEUnsupported() {
		t.Error("isIDLEUnsupported() = true, want false（未知服务器应允许尝试 IDLE）")
	}
}

// TestIdleHeartbeatInterval 验证 IDLE 兜底同步间隔：
// 未配置时跟随 PollInterval，显式配置优先生效，两者都受 60 秒下限保护，
// 避免把 IDLE 退化成高频轮询。
func TestIdleHeartbeatInterval(t *testing.T) {
	cases := []struct {
		name      string
		heartbeat int
		poll      int
		want      time.Duration
	}{
		{"未配置时跟随轮询间隔", 0, 300, 300 * time.Second},
		{"显式配置优先", 120, 300, 120 * time.Second},
		{"显式配置低于下限则取下限", 10, 300, idleHeartbeatMin},
		{"轮询间隔低于下限则取下限", 0, 15, idleHeartbeatMin},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := &AccountWorker{config: &config.Config{
				IMAP: config.IMAPConfig{IdleHeartbeat: c.heartbeat, PollInterval: c.poll},
			}}
			if got := w.idleHeartbeatInterval(); got != c.want {
				t.Errorf("idleHeartbeatInterval() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestWakeMergesDuplicateSignals 验证连点"立即同步"只会合并为一次唤醒信号，
// 避免堆积成同步风暴。
func TestWakeMergesDuplicateSignals(t *testing.T) {
	w := &AccountWorker{wakeCh: make(chan struct{}, 1)}

	w.Wake()
	if !w.manualSync.Load() {
		t.Fatal("Wake() 未置 manualSync 标志")
	}
	w.Wake()
	w.Wake()

	if got := len(w.wakeCh); got != 1 {
		t.Errorf("连续唤醒 3 次后待处理信号数 = %d, want 1", got)
	}
}
