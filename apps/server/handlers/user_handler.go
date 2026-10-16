// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"magicmail/models"
	"magicmail/services"
	"magicmail/sse"

	"github.com/gofiber/fiber/v2"
)

// UserHandler 用户管理 Handler（仅管理员可用）
type UserHandler struct {
	service *services.AuthService
}

// NewUserHandler 创建用户管理 Handler
func NewUserHandler(svc *services.AuthService) *UserHandler {
	return &UserHandler{service: svc}
}

// List 获取用户列表
func (h *UserHandler) List(c *fiber.Ctx) error {
	users, err := h.service.ListUsers()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "获取用户列表失败", "detail": err.Error()})
	}
	return c.JSON(fiber.Map{"data": users})
}

// Create 管理员后台创建用户（不受开放注册开关限制）
func (h *UserHandler) Create(c *fiber.Ctx) error {
	var req models.AdminCreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}
	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "用户名和密码不能为空"})
	}
	if len(req.Password) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "密码长度至少 6 位"})
	}

	user, err := h.service.AdminCreateUser(req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "创建用户失败", "detail": err.Error()})
	}
	// 广播给所有在线管理员，实现多管理员用户列表实时一致
	sse.PublishUserCreated(user.ID, user.Username)
	return c.Status(201).JSON(user)
}

// Delete 删除用户（含其全部关联数据），禁止删除自身
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	currentUserID := getUserID(c)
	if err := h.service.DeleteUser(uint(id), currentUserID); err != nil {
		statusCode := 500
		msg := "删除失败"
		switch err {
		case services.ErrCannotDeleteSelf:
			statusCode = 400
			msg = "不能删除当前登录的管理员账号"
		case services.ErrUserNotFound:
			statusCode = 404
			msg = "用户不存在"
		}
		return c.Status(statusCode).JSON(fiber.Map{"error": msg, "detail": err.Error()})
	}

	// 广播给所有在线管理员，实现多管理员用户列表实时一致
	sse.PublishUserDeleted(uint(id))

	return c.SendStatus(204)
}
