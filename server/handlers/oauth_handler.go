// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"context"
	"time"

	"magicmail/oauth2"

	"github.com/gofiber/fiber/v2"
)

const timeFormatRFC3339 = "2006-01-02T15:04:05Z07:00"

// OAuth2Handler OAuth2 设备码授权 API 处理器
type OAuth2Handler struct{}

// NewOAuth2Handler 创建 OAuth2 Handler
func NewOAuth2Handler() *OAuth2Handler {
	return &OAuth2Handler{}
}

// DeviceCodeRequest 发起设备码授权请求
// @Summary 请求设备码（开始 OAuth2 授权流程）
// @Tags OAuth2
// @Accept json
// @Produce json
// @Param body body DeviceCodeRequest true "设备码请求参数"
// @Success 200 {object} oauth2.DeviceCodeResponse
// @Router /api/v1/oauth/{provider}/device-code [post]
func (h *OAuth2Handler) DeviceCode(c *fiber.Ctx) error {
	providerName := c.Params("provider")
	if providerName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "缺少 provider 参数"})
	}

	type DeviceCodeRequest struct {
		Email         string `json:"email"`          // 用户邮箱地址
		CustomClientId string `json:"custom_client_id"` // 可选：用户自定义 Client ID
	}
	var req DeviceCodeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}

	provider, clientID, err := oauth2.ResolveProviderAndClientID(providerName, req.CustomClientId)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	resp, err := provider.GetDeviceCode(context.Background(), clientID, req.Email)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  "获取设备码失败",
			"detail": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"device_code":      resp.DeviceCode,
			"user_code":        resp.UserCode,
			"verification_uri": resp.VerificationURI,
			"expires_in":       int(resp.ExpiresIn.Seconds()),
			"interval":         int(resp.Interval.Seconds()),
		},
	})
}

// PollToken 轮询 Token 授权状态
// @Summary 轮询 Token（等待用户完成浏览器授权）
// @Tags OAuth2
// @Accept json
// @Produce json
// @Param provider path string true "服务商名称 (microsoft)"
// @Param body body PollTokenRequest true "轮询参数"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/oauth/{provider}/poll [post]
func (h *OAuth2Handler) PollToken(c *fiber.Ctx) error {
	providerName := c.Params("provider")
	if providerName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "缺少 provider 参数"})
	}

	type PollTokenRequest struct {
		DeviceCode     string `json:"device_code" validate:"required"`
		CustomClientId string `json:"custom_client_id"`
	}
	var req PollTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}

	provider, clientID, err := oauth2.ResolveProviderAndClientID(providerName, req.CustomClientId)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	tokenResp, err := provider.PollToken(context.Background(), clientID, req.DeviceCode)
	if err != nil {
		// 授权待处理状态 — 返回 202 让前端继续轮询
		if oauth2.IsPending(err) {
			return c.Status(202).JSON(fiber.Map{
				"pending": true,
				"message": "等待用户在浏览器中完成授权...",
			})
		}
		return c.Status(400).JSON(fiber.Map{
			"error":  "Token 获取失败",
			"detail": err.Error(),
		})
	}

	// 授权成功，返回 Token 数据
	now := timeNow()
	expiresAt := now.Add(tokenResp.ExpiresIn)

	// 从 id_token 解析用户邮箱（OAuth2 授权成功后即可自动获取，无需用户预填）
	email := oauth2.ExtractEmailFromIDToken(tokenResp.IDToken)

	return c.JSON(fiber.Map{
		"success": true,
		"pending": false,
		"data": fiber.Map{
			"provider":         providerName,
			"refresh_token":    tokenResp.RefreshToken,
			"expires_in":       int(tokenResp.ExpiresIn.Seconds()),
			"token_expires_at": expiresAt.Format(timeFormatRFC3339),
			"email":            email,
		},
	})
}

// timeNow 当前时间（方便测试时替换）
func timeNow() time.Time {
	return time.Now()
}
