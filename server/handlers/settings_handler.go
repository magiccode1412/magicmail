// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"magicmail/services"

	"github.com/gofiber/fiber/v2"
)

// SettingsHandler 系统设置 Handler（仅管理员可用）
type SettingsHandler struct {
	service *services.AuthService
}

// NewSettingsHandler 创建设置 Handler
func NewSettingsHandler(svc *services.AuthService) *SettingsHandler {
	return &SettingsHandler{service: svc}
}

// GetOpenRegistration 获取开放注册开关状态
func (h *SettingsHandler) GetOpenRegistration(c *fiber.Ctx) error {
	open, err := h.service.GetOpenRegistration()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "获取配置失败", "detail": err.Error()})
	}
	return c.JSON(fiber.Map{"open_registration": open})
}

// SetOpenRegistration 设置开放注册开关状态
func (h *SettingsHandler) SetOpenRegistration(c *fiber.Ctx) error {
	var req struct {
		OpenRegistration bool `json:"open_registration"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}
	if err := h.service.SetOpenRegistration(req.OpenRegistration); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "保存配置失败", "detail": err.Error()})
	}
	return c.JSON(fiber.Map{"open_registration": req.OpenRegistration, "message": "设置已保存"})
}
