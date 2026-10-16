// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package oauth2

import (
	"errors"
	"time"
)

// ErrRefreshTokenRevoked 表示 OAuth2 Refresh Token 已被吊销/失效
// （如微软返回 invalid_grant）。此时无法通过刷新恢复，必须引导用户重新授权。
// 上层调用方可用 errors.Is 识别该错误，与"网络/服务端临时错误"区分。
var ErrRefreshTokenRevoked = errors.New("refresh token 已失效/被吊销")

// DeviceCodeResponse 设备码授权响应（返回给前端）
type DeviceCodeResponse struct {
	DeviceCode      string        `json:"device_code"`
	UserCode        string        `json:"user_code"`         // 用户输入的短验证码，如 "ABCD-EFGH"
	VerificationURI string        `json:"verification_uri"`  // 用户打开的链接
	ExpiresIn       time.Duration `json:"expires_in"`       // 设备码有效期
	Interval        time.Duration `json:"interval"`          // 轮询间隔（秒）
}

// TokenResponse OAuth2 Token 响应
type TokenResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresIn    time.Duration `json:"expires_in"`
	TokenType    string        `json:"token_type"` // 通常为 "Bearer"
	Scope        string        `json:"scope"`
	IDToken      string        `json:"id_token"`  // OIDC id_token（Microsoft 等 IdP 返回）
}

// PendingError 授权待处理错误（用户尚未完成授权）
type PendingError struct {
	Message string `json:"error"`
}

func (e *PendingError) Error() string {
	return e.Message
}

// IsPending 判断是否为"授权中"状态
func IsPending(err error) bool {
	if e, ok := err.(*PendingError); ok {
		return e.Message == "authorization_pending" || e.Message == "slow_down"
	}
	return false
}
