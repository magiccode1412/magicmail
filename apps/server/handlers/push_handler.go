// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"magicmail/services"

	"github.com/gofiber/fiber/v2"
)

// PushHandler Web Push API 处理器
type PushHandler struct {
	service *services.PushService
}

// NewPushHandler 创建 Push Handler
func NewPushHandler(svc *services.PushService) *PushHandler {
	return &PushHandler{service: svc}
}

// GetVAPIDPublicKey 返回 VAPID base64url 编码的公钥（公开接口）
func (h *PushHandler) GetVAPIDPublicKey(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"public_key": h.service.VAPIDPublicKey(),
	})
}

// Subscribe 注册/更新 Push Subscription（需认证）
func (h *PushHandler) Subscribe(c *fiber.Ctx) error {
	if c.Locals("user_id") == nil {
		return c.Status(401).JSON(fiber.Map{"error": "未登录"})
	}
	userID := getUserID(c)

	var body struct {
		Endpoint  string `json:"endpoint"`
		Keys      struct {
			P256DH string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
		UserAgent string `json:"user_agent,omitempty"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}
	if body.Endpoint == "" || body.Keys.P256DH == "" || body.Keys.Auth == "" {
		return c.Status(400).JSON(fiber.Map{"error": "endpoint 和 keys 不能为空"})
	}

	req := &services.SubReq{
		Endpoint:  body.Endpoint,
		P256DH:    body.Keys.P256DH,
		Auth:      body.Keys.Auth,
		UserAgent: body.UserAgent,
	}
	if err := h.service.Subscribe(userID, req); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "订阅保存失败", "detail": err.Error()})
	}
	return c.Status(201).JSON(fiber.Map{"message": "订阅成功"})
}

// Unsubscribe 取消 Push Subscription（需认证）
func (h *PushHandler) Unsubscribe(c *fiber.Ctx) error {
	if c.Locals("user_id") == nil {
		return c.Status(401).JSON(fiber.Map{"error": "未登录"})
	}
	userID := getUserID(c)

	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}
	if body.Endpoint == "" {
		return c.Status(400).JSON(fiber.Map{"error": "endpoint 不能为空"})
	}

	if err := h.service.Unsubscribe(userID, body.Endpoint); err != nil {
		if err.Error() == "record not found" {
			return c.Status(404).JSON(fiber.Map{"error": "订阅不存在"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "取消订阅失败", "detail": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "已取消订阅"})
}

// ListSubscriptions 列出当前用户的订阅（需认证）
func (h *PushHandler) ListSubscriptions(c *fiber.Ctx) error {
	if c.Locals("user_id") == nil {
		return c.Status(401).JSON(fiber.Map{"error": "未登录"})
	}
	userID := getUserID(c)
	subs, err := h.service.ListSubscriptions(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "获取订阅列表失败"})
	}
	return c.JSON(fiber.Map{"data": subs})
}

// SendTest 发送测试推送（需认证）
func (h *PushHandler) SendTest(c *fiber.Ctx) error {
	if c.Locals("user_id") == nil {
		return c.Status(401).JSON(fiber.Map{"error": "未登录"})
	}
	userID := getUserID(c)
	if err := h.service.SendTest(userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "测试推送发送失败"})
	}
	return c.JSON(fiber.Map{"message": "测试推送已发送"})
}
