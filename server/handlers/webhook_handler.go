// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"fmt"
	"strconv"
	"time"

	"magicmail/models"
	"magicmail/notifier"
	"magicmail/services"

	"github.com/gofiber/fiber/v2"
)

// WebhookHandler Webhook API 处理器
type WebhookHandler struct {
	service *services.WebhookService
}

// NewWebhookHandler 创建 Webhook Handler
func NewWebhookHandler(svc *services.WebhookService) *WebhookHandler {
	return &WebhookHandler{service: svc}
}

// List 获取所有 Webhook
func (h *WebhookHandler) List(c *fiber.Ctx) error {
	hooks, err := h.service.List(getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "获取 Webhook 列表失败"})
	}
	return c.JSON(fiber.Map{"data": hooks})
}

// Get 获取单个 Webhook 详情
func (h *WebhookHandler) Get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	hook, err := h.service.GetByID(uint(id), getUserID(c))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Webhook 不存在"})
	}

	return c.JSON(hook)
}

// Create 创建 Webhook
func (h *WebhookHandler) Create(c *fiber.Ctx) error {
	var req models.WebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}

	if req.Name == "" || req.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "名称和 URL 不能为空"})
	}

	hook, err := h.service.Create(req, getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "创建失败", "detail": err.Error()})
	}

	return c.Status(201).JSON(hook)
}

// Update 更新 Webhook
func (h *WebhookHandler) Update(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	var req models.WebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}

	if req.Name == "" || req.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "名称和 URL 不能为空"})
	}

	hook, err := h.service.Update(uint(id), req, getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "更新失败", "detail": err.Error()})
	}

	return c.JSON(hook)
}

// Delete 删除 Webhook
func (h *WebhookHandler) Delete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	if err := h.service.Delete(uint(id), getUserID(c)); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "删除失败", "detail": err.Error()})
	}

	return c.SendStatus(204)
}

// Test 测试 Webhook
func (h *WebhookHandler) Test(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	result, err := h.service.Test(uint(id), getUserID(c))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Webhook 不存在"})
	}

	return c.JSON(result)
}

// GetLogs 获取 Webhook 日志
func (h *WebhookHandler) GetLogs(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	logs, err := h.service.GetLogs(uint(id), getUserID(c), limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "获取日志失败"})
	}

	return c.JSON(fiber.Map{"data": logs})
}

// SimulateMailReceived 模拟邮件接收，每封邮件独立触发一次通知
func (h *WebhookHandler) SimulateMailReceived(c *fiber.Ctx) error {
	var req struct {
		AccountID    uint                     `json:"account_id"`
		AccountEmail string                   `json:"account_email"`
		AccountName  string                   `json:"account_name"`
		MailCount    int                      `json:"mail_count"`
		Mails        []map[string]interface{} `json:"mails"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败: " + err.Error()})
	}

	if req.AccountEmail == "" {
		req.AccountEmail = "simulate@test.com"
	}
	if req.AccountName == "" {
		req.AccountName = "模拟测试账号"
	}

	// 默认构造一封测试邮件（如果未提供）
	if len(req.Mails) == 0 || req.MailCount == 0 {
		req.Mails = []map[string]interface{}{
			{
				"subject": "\U0001f9ea 测试邮件",
				"from":    "tester@example.com",
				"sent_at": time.Now().Format("2006-01-02 15:04:05"),
				"preview": "这是一封模拟的测试邮件，用于验证 webhook 通知配置。",
			},
		}
		req.MailCount = 1
	}

	nowTs := fmt.Sprintf("%d", time.Now().Unix())
	triggeredCount := 0

	for _, mail := range req.Mails {
		h.service.TriggerByEvent("mail.received", map[string]interface{}{
			"account_id":    req.AccountID,
			"account_email": req.AccountEmail,
			"account_name":  req.AccountName,
			"protocol":      "imap",
			"subject":       mail["subject"],
			"from":          mail["from"],
			"sent_at":       mail["sent_at"],
			"preview":       mail["preview"],
			"text_body":     mail["text_body"],
			"html_body":     mail["html_body"],
			"timestamp":     nowTs,
		}, getUserID(c))
		triggeredCount++
	}

	// 触发 Web Push 离线推送
	notifier.SendPushNotification(
		getUserID(c),
		fmt.Sprintf("📧 模拟收到 %d 封新邮件", triggeredCount),
		fmt.Sprintf("来自 %s", req.AccountEmail),
		map[string]interface{}{"event": "simulate"},
	)

	return c.JSON(fiber.Map{
		"success":     true,
		"message":     fmt.Sprintf("已触发 %d 条邮件通知", triggeredCount),
		"total_mails": triggeredCount,
		"event":       "mail.received",
	})
}
