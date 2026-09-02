// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package oauth2

import (
	"context"
	"fmt"
	"os"
)

// OAuth2Provider OAuth2 服务商接口
// 每个邮箱服务商（Microsoft/Google/Yahoo等）需实现此接口
type OAuth2Provider interface {
	// Name 返回服务商唯一标识，如 "microsoft", "google"
	Name() string

	// Scopes 返回所需的 OAuth2 权限范围（IMAP + SMTP + offline_access）
	Scopes() []string

	// DefaultClientID 返回开发者内置的默认 Client ID（兜底值）
	DefaultClientID() string

	// EnvVarName 返回用于环境变量覆盖的环境变量名
	EnvVarName() string

	// GetDeviceCode 发起设备码授权请求
	// loginHint 为可选的用户邮箱，用于预填微软登录页（可传空字符串）
	GetDeviceCode(ctx context.Context, clientID, loginHint string) (*DeviceCodeResponse, error)

	// PollToken 轮询 Token（使用 device_code换取 token）
	PollToken(ctx context.Context, clientID string, deviceCode string) (*TokenResponse, error)

	// RefreshAccessToken 使用 RefreshToken 获取新的 AccessToken
	RefreshAccessToken(ctx context.Context, clientID string, refreshToken string) (*TokenResponse, error)

	// BuildXOAUTH2String 构建 IMAP XOAUTH2 认证字符串
	BuildXOAUTH2String(email, accessToken string) string
}

// ProviderRegistry OAuth2 Provider 注册中心
type ProviderRegistry struct {
	providers map[string]OAuth2Provider
}

// NewProviderRegistry 创建注册中心实例
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]OAuth2Provider),
	}
}

// Register 注册一个 OAuth2 Provider
func (r *ProviderRegistry) Register(provider OAuth2Provider) {
	r.providers[provider.Name()] = provider
}

// Get 按 name 获取已注册的 Provider
func (r *ProviderRegistry) Get(name string) (OAuth2Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// ResolveClientID 解析最终使用的 Client ID（三级优先级）
// 优先级：用户自定义 > 环境变量 > 开发者默认值
func (r *ProviderRegistry) ResolveClientID(providerName, userOverride string) string {
	// 1. 用户自定义（最高优先级）
	if userOverride != "" {
		return userOverride
	}

	provider, ok := r.providers[providerName]
	if !ok {
		return ""
	}

	// 2. 环境变量覆盖（中等优先级）
	if envVal := os.Getenv(provider.EnvVarName()); envVal != "" {
		return envVal
	}

	// 3. 开发者默认值（兜底）
	return provider.DefaultClientID()
}

// Providers 返回所有已注册的 Provider 名称列表
func (r *ProviderRegistry) Providers() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// globalRegistry 全局单例
var globalRegistry *ProviderRegistry

// InitGlobalRegistry 初始化全局注册中心并注册所有内置 Provider
func InitGlobalRegistry() *ProviderRegistry {
	if globalRegistry == nil {
		globalRegistry = NewProviderRegistry()
		globalRegistry.Register(NewMicrosoftProvider())
	}
	return globalRegistry
}

// GlobalRegistry 获取全局注册中心实例
func GlobalRegistry() *ProviderRegistry {
	if globalRegistry == nil {
		panic("oauth2: global registry not initialized, call InitGlobalRegistry first")
	}
	return globalRegistry
}

// ResolveProviderAndClientID 便捷方法：同时解析 Provider 和 Client ID
func ResolveProviderAndClientID(providerName, userOverride string) (OAuth2Provider, string, error) {
	reg := GlobalRegistry()

	provider, ok := reg.Get(providerName)
	if !ok {
		return nil, "", fmt.Errorf("unsupported oauth2 provider: %s", providerName)
	}

	clientID := reg.ResolveClientID(providerName, userOverride)
	if clientID == "" {
		return nil, "", fmt.Errorf("no client_id configured for provider %s", providerName)
	}

	return provider, clientID, nil
}
