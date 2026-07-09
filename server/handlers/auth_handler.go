// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"magicmail/models"
	"magicmail/services"

	"github.com/gofiber/fiber/v2"
)

// AuthHandler 认证相关 Handler
type AuthHandler struct {
	service *services.AuthService
}

// NewAuthHandler 创建认证 Handler
func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{service: svc}
}

// Login 登录
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数无效: " + err.Error()})
	}
	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "用户名和密码不能为空"})
	}

	result, err := h.service.Login(req)
	if err != nil {
		msg := "登录失败"
		if err == services.ErrInvalidCredentials {
			msg = "用户名或密码错误"
		}
		return c.Status(401).JSON(fiber.Map{"error": msg, "detail": err.Error()})
	}
	return c.JSON(result)
}

// Register 注册（首次初始化，单用户模式）
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req models.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数无效: " + err.Error()})
	}
	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "用户名和密码不能为空"})
	}
	if len(req.Password) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "密码长度至少 6 位"})
	}

	err := h.service.Register(req)
	if err != nil {
		statusCode := 500
		msg := "注册失败"
		switch err {
		case services.ErrRegistrationClosed:
			statusCode = 403
			msg = "公开注册已关闭，仅管理员可创建账号"
		case services.ErrUserExists:
			statusCode = 409
			msg = "账号已存在，请直接登录"
		}
		return c.Status(statusCode).JSON(fiber.Map{"error": msg, "detail": err.Error()})
	}

	// 注册成功后自动登录，返回 Token
	loginReq := models.LoginRequest{Username: req.Username, Password: req.Password}
	result, loginErr := h.service.Login(loginReq)
	if loginErr != nil {
		return c.Status(201).JSON(fiber.Map{"message": "注册成功，请手动登录", "detail": loginErr.Error()})
	}

	c.Status(201)
	return c.JSON(result)
}

// Status 查询认证状态（是否需要初始化）
func (h *AuthHandler) Status(c *fiber.Ctx) error {
	status := h.service.GetAuthStatus()
	return c.JSON(status)
}
