// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package oauth2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// Microsoft OAuth2 端点（使用 common tenant，同时支持个人和工作账户）
	// consumers tenant 不支持 outlook.office365.com IMAP scope，必须用 common
	microsoftDeviceCodeEndpoint = "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode"
	microsoftTokenEndpoint      = "https://login.microsoftonline.com/common/oauth2/v2.0/token"

	// Microsoft 默认 Client ID（MagicMail 开发者应用）
	microsoftDefaultClientID = "0051d6e3-655d-4b1f-ab63-6695bb65d3f1"

	// Microsoft 授权范围（用户委托流程必须使用 outlook.office.com，不能用 outlook.office365.com）
	// 参考: https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth
	microsoftIMAPScope    = "https://outlook.office.com/IMAP.AccessAsUser.All"
	microsoftSMTPScope    = "https://outlook.office.com/SMTP.Send"
	microsoftOfflineScope = "offline_access" // 获取 Refresh Token
	microsoftOpenIDScope = "openid"          // 获取 id_token（含用户标识）
	microsoftEmailScope  = "email"           // 在 id_token 中包含 email 声明
)

// MicrosoftProvider Microsoft/Outlook/Hotmail OAuth2 Provider 实现
type MicrosoftProvider struct {
	deviceEndpoint string
	tokenEndpoint  string
}

// NewMicrosoftProvider 创建 Microsoft Provider 实例
func NewMicrosoftProvider() *MicrosoftProvider {
	return &MicrosoftProvider{
		deviceEndpoint: microsoftDeviceCodeEndpoint,
		tokenEndpoint:  microsoftTokenEndpoint,
	}
}

func (p *MicrosoftProvider) Name() string {
	return "microsoft"
}

func (p *MicrosoftProvider) Scopes() []string {
	return []string{
		microsoftIMAPScope,
		microsoftSMTPScope,
		microsoftOfflineScope,
		microsoftOpenIDScope,
		microsoftEmailScope,
	}
}

func (p *MicrosoftProvider) DefaultClientID() string {
	return microsoftDefaultClientID
}

func (p *MicrosoftProvider) EnvVarName() string {
	return "MAGICMAIL_OAUTH_MICROSOFT_CLIENT_ID"
}

func (p *MicrosoftProvider) GetDeviceCode(ctx context.Context, clientID, loginHint string) (*DeviceCodeResponse, error) {
	scopes := strings.Join(p.Scopes(), " ")

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", scopes)
	if loginHint != "" {
		data.Set("login_hint", loginHint)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.deviceEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建设备码请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求设备码失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("设备码请求失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("解析设备码响应失败: %w", err)
	}

	interval := time.Duration(raw.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second // 微软建议最小 5 秒
	}

	return &DeviceCodeResponse{
		DeviceCode:      raw.DeviceCode,
		UserCode:        raw.UserCode,
		VerificationURI: raw.VerificationURI,
		ExpiresIn:       time.Duration(raw.ExpiresIn) * time.Second,
		Interval:        interval,
	}, nil
}

func (p *MicrosoftProvider) PollToken(ctx context.Context, clientID string, deviceCode string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("device_code", deviceCode)

	req, err := http.NewRequestWithContext(ctx, "POST", p.tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建 Token 轮询请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("轮询 Token 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Token 响应失败: %w", err)
	}

	// 检查是否为错误响应（授权中 / 慢下来）
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil {
			if errResp.Error == "authorization_pending" || errResp.Error == "slow_down" {
				return nil, &PendingError{Message: errResp.Error}
			}
			if errResp.Error == "expired_token" {
				return nil, fmt.Errorf("设备码已过期，请重新发起授权")
			}
			return nil, fmt.Errorf("OAuth2 错误 [%s]: %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("Token 轮询失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		IDToken      string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析 Token 响应失败: %w", err)
	}

	return &TokenResponse{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    time.Duration(tokenResp.ExpiresIn) * time.Second,
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
		IDToken:      tokenResp.IDToken,
	}, nil
}

func (p *MicrosoftProvider) RefreshAccessToken(ctx context.Context, clientID string, refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("scope", strings.Join(p.Scopes(), " "))

	req, err := http.NewRequestWithContext(ctx, "POST", p.tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建刷新 Token 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("刷新 Token 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取刷新响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil {
			if errResp.Error == "invalid_grant" {
				return nil, fmt.Errorf("%w: 微软返回 invalid_grant，需要用户重新授权", ErrRefreshTokenRevoked)
			}
			return nil, fmt.Errorf("刷新 Token 错误 [%s]: %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("刷新 Token 失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"` // 微软可能返回新的 RT
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析刷新 Token 响应失败: %w", err)
	}

	return &TokenResponse{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken, // 可能为空（微软不总是返回新 RT）
		ExpiresIn:    time.Duration(tokenResp.ExpiresIn) * time.Second,
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
	}, nil
}

func (p *MicrosoftProvider) BuildXOAUTH2String(email, accessToken string) string {
	return BuildXOAUTH2String(email, accessToken)
}

// ExtractEmailFromIDToken 从 JWT 的 id_token 中解析出用户邮箱地址（仅解码 payload，不做签名校验）。
// 返回优先级：email > preferred_username > upn > unique_name（取第一个非空）。
// 用于在 OAuth2 授权成功后自动获取邮箱，免去用户预填。
func ExtractEmailFromIDToken(idToken string) string {
	if idToken == "" {
		return ""
	}
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return ""
	}
	payload := parts[1]

	// JWT 使用 base64url 且无填充
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		// 兜底：尝试标准 url-safe（带填充）
		if raw, err = base64.URLEncoding.DecodeString(payload); err != nil {
			return ""
		}
	}

	var claims struct {
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
		UPN               string `json:"upn"`
		UniqueName        string `json:"unique_name"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return ""
	}

	if claims.Email != "" {
		return claims.Email
	}
	if claims.PreferredUsername != "" {
		return claims.PreferredUsername
	}
	if claims.UPN != "" {
		return claims.UPN
	}
	return claims.UniqueName
}
