// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package oauth2

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"magicmail/crypto"
	"magicmail/models"

	"gorm.io/gorm"
)

// EnsureAccessToken 获取 OAuth2 账号当前可用的 Access Token。
// 若 Token 仍有效（提前 2 分钟容差）则直接返回缓存（OAuth2 模式下 account.Password 复用存储 AT）；
// 否则使用 RefreshToken 刷新，并把刷新后的 Token 写回 account（可选持久化到数据库）。
//
// 逻辑与 IMAP 路径的 token 解析保持一致，供 SMTP 发送等非 IMAP 场景复用，
// 避免重复实现导致行为分叉。
func EnsureAccessToken(account *models.MailAccount, db *gorm.DB) (string, error) {
	if !IsOAuth2Account(account.AuthType) {
		return "", fmt.Errorf("账号不是 OAuth2 类型: %s", account.AuthType)
	}

	providerName := ParseAuthType(account.AuthType)
	if providerName == "" {
		return "", fmt.Errorf("无效的 OAuth2 认证类型: %s", account.AuthType)
	}

	provider, clientID, err := ResolveProviderAndClientID(providerName, account.CustomClientId)
	if err != nil {
		return "", fmt.Errorf("OAuth2 Provider 解析失败: %w", err)
	}

	// 缓存命中：Token 仍有效（提前 2 分钟刷新，避免临界过期）
	if account.TokenExpiresAt != nil {
		expiresAt := account.TokenExpiresAt.Add(-2 * time.Minute)
		if time.Now().Before(expiresAt) && account.Password != "" {
			return account.Password, nil
		}
	}

	// 需要刷新
	if account.RefreshToken == "" {
		if account.RefreshTokenDecryptFailed {
			log.Printf("❌ [OAuth2] 账号 %s 的 RefreshToken 解密失败（密钥不匹配或密文损坏），无法刷新 AccessToken，需用户重新授权",
				account.Email)
			return "", fmt.Errorf("RefreshToken 解密失败（密钥不匹配或密文损坏），需用户重新授权")
		}
		log.Printf("[WARN] [OAuth2] 账号 %s 无 RefreshToken，需要用户重新授权", account.Email)
		return "", fmt.Errorf("账号未授权（无 RefreshToken），需要用户重新授权")
	}

	log.Printf("🔄 [SMTP] 正在刷新 AccessToken (%s@%s)...", account.Email, provider.Name())
	tokenResp, err := provider.RefreshAccessToken(context.Background(), clientID, account.RefreshToken)
	if err != nil {
		// 区分"RefreshToken 已被吊销"（需重新授权）与"网络/服务端临时错误"（可重试）
		if errors.Is(err, ErrRefreshTokenRevoked) {
			log.Printf("❌ [OAuth2] AccessToken 刷新失败（%s）：RefreshToken 已被吊销/失效，需用户重新授权: %v",
				account.Email, err)
		} else {
			log.Printf("❌ [OAuth2] AccessToken 刷新失败（%s）：网络或服务端错误，可稍后重试: %v",
				account.Email, err)
		}
		return "", err
	}

	// 更新运行时缓存（account.Password 复用存储 Access Token）
	account.Password = tokenResp.AccessToken
	now := time.Now()
	expiresAt := now.Add(tokenResp.ExpiresIn)
	account.TokenExpiresAt = &expiresAt
	if tokenResp.RefreshToken != "" {
		account.RefreshToken = tokenResp.RefreshToken
	}

	// 可选持久化：把滚动后的 Token 写回数据库，保证重启后仍可用
	if db != nil {
		if err := persistTokens(db, account); err != nil {
			log.Printf("[WARN] [SMTP] 持久化 OAuth2 Token 失败 (账号 %s): %v", account.Email, err)
		}
	}

	log.Printf("✅ [SMTP] AccessToken 刷新成功: %s (有效期 %v)", account.Email, tokenResp.ExpiresIn)
	return tokenResp.AccessToken, nil
}

// persistTokens 将刷新后的 OAuth2 Token 显式加密后写回数据库。
// 必须与 imap/client.go 的 persistOAuthTokens 一样"显式加密"，不能依赖 GORM 的 BeforeUpdate 钩子
// （钩子会作用在 Model() 传入的空模型上，导致明文凭据被原样写入）。
func persistTokens(db *gorm.DB, account *models.MailAccount) error {
	if !IsOAuth2Account(account.AuthType) {
		return nil
	}

	// 显式加密敏感字段（防双重加密：已是密文则原样保留）
	refreshToken := account.RefreshToken
	if refreshToken != "" && !crypto.IsEncrypted(refreshToken) {
		enc, err := crypto.Encrypt(refreshToken)
		if err != nil {
			return fmt.Errorf("加密 RefreshToken 失败: %w", err)
		}
		refreshToken = enc
	}
	// OAuth2 模式下 Password 字段复用存储 Access Token
	accessToken := account.Password
	if accessToken != "" && !crypto.IsEncrypted(accessToken) {
		enc, err := crypto.Encrypt(accessToken)
		if err != nil {
			return fmt.Errorf("加密 AccessToken 失败: %w", err)
		}
		accessToken = enc
	}

	// 使用 map 更新仅写入指定列，不触发不可靠的加密钩子，避免覆盖其它列
	updates := map[string]interface{}{
		"password":         accessToken,
		"refresh_token":    refreshToken,
		"token_expires_at": account.TokenExpiresAt,
	}
	if err := db.Model(&models.MailAccount{}).
		Where("id = ?", account.ID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("持久化 OAuth2 Token 失败: %w", err)
	}
	log.Printf("💾 已持久化刷新后的 OAuth2 Token (%s)", account.Email)
	return nil
}
