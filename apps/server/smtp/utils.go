// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package smtp

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// encodeBase64 对字符串进行 Base64 编码（每行 76 字符换行）
func encodeBase64(s string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	// 按 76 字符换行
	var result strings.Builder
	lineLen := 0
	for _, r := range encoded {
		result.WriteRune(r)
		lineLen++
		if lineLen == 76 {
			result.WriteString("\r\n")
			lineLen = 0
		}
	}
	return result.String()
}

// generateMessageID 生成唯一 Message-ID
func generateMessageID(from string) string {
	return fmt.Sprintf("<%s@%s>", generateMessageIDShort(), extractDomain(from))
}

// generateMessageIDShort 生成短随机 ID
func generateMessageIDShort() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:24]
}

// extractDomain 从邮箱地址提取域名
func extractDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at >= 0 && at+1 < len(email) {
		return email[at+1:]
	}
	return "magicmail.local"
}

// randomBoundary 生成分隔边界
func randomBoundary() string {
	b := make([]byte, 12)
	rand.Read(b)
	return fmt.Sprintf("%d_%x", time.Now().UnixNano(), b)
}

// validateAddresses 验证邮箱地址格式
func validateAddresses(addrs []string) error {
	for _, addr := range addrs {
		if !strings.Contains(addr, "@") {
			return fmt.Errorf("无效的邮箱地址: %s", addr)
		}
	}
	return nil
}
