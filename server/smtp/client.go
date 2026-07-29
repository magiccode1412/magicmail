// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package smtp

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"magicmail/models"
	"magicmail/proxy"
)

// SendRequest 发送邮件请求
type SendRequest struct {
	AccountID uint     `json:"account_id"`
	To        []string `json:"to" validate:"required,min=1"`
	Cc        []string `json:"cc,omitempty"`
	Bcc       []string `json:"bcc,omitempty"`
	Subject   string   `json:"subject" validate:"required"`
	Body      string   `json:"body"` // 纯文本正文
	HTMLBody  string   `json:"html_body,omitempty"` // HTML 正文（可选）
}

// SendResult 发送结果
type SendResult struct {
	MessageID string `json:"message_id,omitempty"`
}

// SendMail 通过指定账号的 SMTP 服务器发送邮件
func SendMail(account *models.MailAccount, req *SendRequest) (*SendResult, error) {
	smtpHost := getSMTPHost(account)
	smtpPort := getSMTPPort(account)

	addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)

	// 构建认证信息
	auth := smtp.PlainAuth("", account.Username, account.Password, smtpHost)

	// 构建邮件内容（RFC822 格式）
	from := account.Email
	msgID := generateMessageID(from)
	msgBytes := buildMessage(from, req.To, req.Cc, req.Bcc, req.Subject, req.Body, req.HTMLBody, msgID)

	// 合并所有收件人用于 MAIL FROM 验证
	recipients := append(req.To, append(req.Cc, req.Bcc...)...)

	// 获取代理 Dialer
	customDialer, err := proxy.Dialer(account.ProxyEnabled, account.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理配置错误: %w", err)
	}

	// TLS 连接（大多数现代 SMTP 使用 STARTTLS 或直连 TLS）
	if smtpPort == 465 {
		// SSL/TLS 直连模式（端口 465）
		err = sendViaTLSWithDialer(addr, from, recipients, msgBytes, auth, customDialer)
	} else if smtpPort == 25 || smtpPort == 587 {
		// STARTTLS 模式（端口 25/587）
		err = sendViaSTARTTLSWithDialer(addr, from, recipients, msgBytes, auth, customDialer)
	} else {
		// 默认尝试 STARTTLS
		err = sendViaSTARTTLSWithDialer(addr, from, recipients, msgBytes, auth, customDialer)
		if err != nil {
			err = sendViaTLSWithDialer(addr, from, recipients, msgBytes, auth, customDialer)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("发送邮件失败 (%s): %w", addr, err)
	}

	return &SendResult{
		MessageID: msgID,
	}, nil
}

// sendViaSTARTTLS 通过 STARTTLS 方式发送邮件（端口 25/587）
func sendViaSTARTTLS(addr, from string, to []string, msg []byte, auth smtp.Auth) error {
	return sendViaSTARTTLSWithDialer(addr, from, to, msg, auth, nil)
}

// sendViaSTARTTLSWithDialer 通过 STARTTLS 方式发送邮件，支持自定义 dialer（代理）
func sendViaSTARTTLSWithDialer(addr, from string, to []string, msg []byte, auth smtp.Auth, customDialer func(network, addr string) (net.Conn, error)) error {
	var client *smtp.Client
	var err error

	if customDialer != nil {
		conn, err := customDialer("tcp", addr)
		if err != nil {
			return fmt.Errorf("通过代理连接失败: %w", err)
		}
		host := strings.Split(addr, ":")[0]
		client, err = smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
		}
	} else {
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("连接失败: %w", err)
		}
	}
	defer client.Close()

	// 如果服务器支持，启动 TLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: strings.Split(addr, ":")[0],
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS 失败: %w", err)
		}
	}

	// 认证
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); !ok {
			return fmt.Errorf("服务器不支持 AUTH")
		}
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("认证失败: %w", err)
		}
	}

	// 发送
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM 失败: %w", err)
	}
	for _, addr := range to {
		if err = client.Rcpt(addr); err != nil {
			return fmt.Errorf("RCPT TO <%s> 失败: %w", addr, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA 命令失败: %w", err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("写入数据失败: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("关闭 DATA 流失败: %w", err)
	}
	return nil
}

// sendViaTLS 通过 SSL/TLS 直连方式发送邮件（端口 465）
func sendViaTLS(addr, from string, to []string, msg []byte, auth smtp.Auth) error {
	return sendViaTLSWithDialer(addr, from, to, msg, auth, nil)
}

// sendViaTLSWithDialer 通过 SSL/TLS 直连方式发送邮件，支持自定义 dialer（代理）
func sendViaTLSWithDialer(addr, from string, to []string, msg []byte, auth smtp.Auth, customDialer func(network, addr string) (net.Conn, error)) error {
	host := strings.Split(addr, ":")[0]
	tlsConfig := &tls.Config{
		ServerName: host,
	}

	var conn net.Conn
	var err error

	if customDialer != nil {
		conn, err = customDialer("tcp", addr)
		if err != nil {
			return fmt.Errorf("通过代理连接失败: %w", err)
		}
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return fmt.Errorf("TLS 握手失败: %w", err)
		}
		conn = tlsConn
	} else {
		conn, err = tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("TLS 连接失败: %w", err)
		}
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("创建客户端失败: %w", err)
	}
	defer client.Close()

	// 认证
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("认证失败: %w", err)
		}
	}

	// 发送
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM 失败: %w", err)
	}
	for _, addr := range to {
		if err = client.Rcpt(addr); err != nil {
			return fmt.Errorf("RCPT TO <%s> 失败: %w", addr, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA 命令失败: %w", err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("写入数据失败: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("关闭 DATA 流失败: %w", err)
	}
	return nil
}

// sanitizeHeaderValue 清洗邮件头字段值，移除可能导致 CRLF 注入的控制字符
// 防止攻击者通过 \r\n 注入额外的 SMTP 头部（CWE-93）
func sanitizeHeaderValue(value string) string {
	// 移除 \r (回车)、\n (换行)、空字节等控制字符
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\x00", "")
	return value
}

// sanitizeEmailAddr 清洗邮箱地址，移除控制字符并去除首尾空白
func sanitizeEmailAddr(addr string) string {
	addr = sanitizeHeaderValue(addr)
	addr = strings.TrimSpace(addr)
	return addr
}

// buildMessage 构建 RFC822 格式的原始邮件内容
func buildMessage(from string, to, cc, bcc []string, subject, textBody, htmlBody string, msgID string) []byte {
	var sb strings.Builder
	now := time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700") // RFC1123 格式

	// 清洗所有用户输入，防止 CRLF 注入
	from = sanitizeEmailAddr(from)
	subject = sanitizeHeaderValue(subject)

	// --- 头部 ---
	sb.WriteString(fmt.Sprintf("From: %s\r\n", from))

	sb.WriteString("To: ")
	for i, addr := range to {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(sanitizeEmailAddr(addr))
	}
	sb.WriteString("\r\n")

	if len(cc) > 0 {
		sb.WriteString("Cc: ")
		for i, addr := range cc {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(sanitizeEmailAddr(addr))
		}
		sb.WriteString("\r\n")
	}

	sb.WriteString(fmt.Sprintf("Subject: =?UTF-8?B?%s?=\r\n", encodeBase64(subject)))
	sb.WriteString(fmt.Sprintf("Date: %s\r\n", now))
	sb.WriteString(fmt.Sprintf("Message-ID: %s\r\n", msgID))
	sb.WriteString("MIME-Version: 1.0\r\n")

	// --- 正文 ---
	if htmlBody != "" {
		// Multipart alternative: 纯文本 + HTML
		boundary := "=_Magicmail_" + randomBoundary()
		sb.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
		sb.WriteString("\r\n")

		sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		sb.WriteString("Content-Transfer-Encoding: base64\r\n")
		sb.WriteString("\r\n")
		sb.WriteString(encodeBase64(textBody))
		sb.WriteString("\r\n")

		sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		sb.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		sb.WriteString("Content-Transfer-Encoding: base64\r\n")
		sb.WriteString("\r\n")
		sb.WriteString(encodeBase64(htmlBody))
		sb.WriteString("\r\n")

		sb.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		// 纯文本
		sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		sb.WriteString("Content-Transfer-Encoding: base64\r\n")
		sb.WriteString("\r\n")
		sb.WriteString(encodeBase64(textBody))
		sb.WriteString("\r\n")
	}

	return []byte(sb.String())
}

// getSMTPHost 获取 SMTP 服务器地址
func getSMTPHost(account *models.MailAccount) string {
	if account.SmtpHost != "" {
		return account.SmtpHost
	}
	// 回退到收信主机名
	return account.ImapHost
}

// getSMTPPort 获取 SMTP 端口
func getSMTPPort(account *models.MailAccount) int {
	if account.SmtpPort > 0 {
		return account.SmtpPort
	}
	return 587 // 默认 STARTTLS 端口
}
