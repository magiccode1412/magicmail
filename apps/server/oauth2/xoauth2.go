// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package oauth2

import (
	"encoding/base64"
	"fmt"
)

// BuildXOAUTH2String 构建 IMAP/SMTP XOAUTH2 SASL 认证字符串
// 格式: base64("user=" + email + "\x01auth=Bearer " + accessToken + "\x01\x01")
func BuildXOAUTH2String(email, accessToken string) string {
	raw := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", email, accessToken)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// ParseAuthType 从 AuthType 字段提取 provider 名称
// "oauth2_microsoft" -> "microsoft"
// "oauth2_google" -> "google"
// "" / "password" -> ""（非 OAuth2）
func ParseAuthType(authType string) string {
	if authType == "" || authType == "password" {
		return ""
	}
	if len(authType) > 7 && authType[:7] == "oauth2_" {
		return authType[7:]
	}
	return ""
}

// IsOAuth2Account 判断账号是否使用 OAuth2 认证
func IsOAuth2Account(authType string) bool {
	return len(authType) > 7 && authType[:7] == "oauth2_"
}

// MakeAuthType 构建 AuthType 字段值
// MakeAuthType("microsoft") -> "oauth2_microsoft"
func MakeAuthType(providerName string) string {
	return "oauth2_" + providerName
}
