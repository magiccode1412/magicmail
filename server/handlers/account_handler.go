// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"magicmail/models"
	"magicmail/oauth2"
	"magicmail/services"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AccountHandler 邮箱账号 API 处理器
type AccountHandler struct {
	service           *services.AccountService
	healthCheckService *services.HealthCheckService
}

// NewAccountHandler 创建邮箱 Handler 实例
func NewAccountHandler(svc *services.AccountService, healthSvc ...*services.HealthCheckService) *AccountHandler {
	h := &AccountHandler{service: svc}
	if len(healthSvc) > 0 && healthSvc[0] != nil {
		h.healthCheckService = healthSvc[0]
	}
	return h
}

// List 获取邮箱列表
// @Summary 获取所有邮箱账号
// @Tags 邮箱管理
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/accounts [get]
func (h *AccountHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	accounts, total, err := h.service.List(page, pageSize, getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  "获取邮箱列表失败",
			"detail": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data":  accounts,
		"total": total,
		"page":  page,
		"page_size": pageSize,
	})
}

// Get 获取单个邮箱详情
// @Summary 获取邮箱详情
// @Tags 邮箱管理
// @Produce json
// @Param id path int true "邮箱ID"
// @Success 200 {object} models.AccountResponse
// @Router /api/v1/accounts/{id} [get]
func (h *AccountHandler) Get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	account, err := h.service.GetByID(uint(id), getUserID(c))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "邮箱不存在"})
	}

	return c.JSON(account)
}

// Create 创建邮箱账号
// @Summary 创建新邮箱账号
// @Tags 邮箱管理
// @Accept json
// @Produce json
// @Param account body models.AccountRequest true "邮箱信息"
// @Success 201 {object} models.AccountResponse
// @Router /api/v1/accounts [post]
func (h *AccountHandler) Create(c *fiber.Ctx) error {
	var req models.AccountRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}

	// 基本校验
	if req.Name == "" || req.Email == "" || req.Host == "" || req.Username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "必填字段不能为空"})
	}

	// OAuth2 认证：允许 password 为空，但必须有 refresh_token
	// 传统密码认证：password 必填
	isOAuth2 := oauth2.IsOAuth2Account(req.AuthType)
	if isOAuth2 {
		if req.RefreshToken == "" {
			return c.Status(400).JSON(fiber.Map{"error": "OAuth2 授权未完成或 Refresh Token 缺失"})
		}
	} else if req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "密码不能为空"})
	}

	account, err := h.service.Create(req, getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  "创建失败",
			"detail": err.Error(),
		})
	}

	return c.Status(201).JSON(account)
}

// Update 更新邮箱账号
// @Summary 更新邮箱信息
// @Tags 邮箱管理
// @Accept json
// @Produce json
// @Param id path int true "邮箱ID"
// @Param account body models.AccountRequest true "邮箱信息"
// @Success 200 {object} models.AccountResponse
// @Router /api/v1/accounts/{id} [put]
func (h *AccountHandler) Update(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	var req models.AccountRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}
	
	if req.Name == "" || req.Email == "" || req.Host == "" || req.Username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "必填字段不能为空"})
	}

	account, err := h.service.Update(uint(id), req, getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  "更新失败",
			"detail": err.Error(),
		})
	}

	return c.JSON(account)
}

// Delete 删除邮箱账号
// @Summary 删除邮箱账号
// @Tags 邮箱管理
// @Param id path int true "邮箱ID"
// @Success 204
// @Router /api/v1/accounts/{id} [delete]
func (h *AccountHandler) Delete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	if err := h.service.Delete(uint(id), getUserID(c)); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  "删除失败",
			"detail": err.Error(),
		})
	}

	return c.SendStatus(204)
}

// TestConnection 测试 IMAP 连接
// @Summary 测试邮箱连接
// @Tags 邮箱管理
// @Accept json
// @Produce json
// @Param body body models.AccountRequest true "邮箱配置（用于测试）"
// @Success 200 {object} map[string]string
// @Router /api/v1/accounts/test-connection [post]
func (h *AccountHandler) TestConnection(c *fiber.Ctx) error {
	var req models.AccountRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}

	if req.Host == "" || req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "邮件服务器/用户名/密码不能为空"})
	}

	err := h.service.TestConnection(req)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "连接测试失败",
			"detail":  err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "连接成功",
	})
}

// TriggerSync 手动触发同步
// @Summary 手动触发邮件同步
// @Tags 邮箱管理
// @Param id path int true "邮箱ID"
// @Success 200 {object} map[string]string
// @Router /api/v1/accounts/{id}/sync [post]
func (h *AccountHandler) TriggerSync(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	err = h.service.TriggerSync(uint(id), getUserID(c))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "已触发同步任务，正在后台执行...",
	})
}

// ToggleStatus 切换账号状态（停用/启用）
// @Summary 停用/启用邮箱账号
// @Tags 邮箱管理
// @Param id path int true "邮箱ID"
// @Success 200 {object} map[string]string
// @Router /api/v1/accounts/{id}/status [put]
func (h *AccountHandler) ToggleStatus(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	type StatusRequest struct {
		Status string `json:"status"`
	}
	var req StatusRequest
	if err := c.BodyParser(&req); err != nil || req.Status == "" {
		return c.Status(400).JSON(fiber.Map{"error": "请指定状态 (active/disabled)"})
	}

	if req.Status != "active" && req.Status != "disabled" {
		return c.Status(400).JSON(fiber.Map{"error": "无效状态值，仅支持 active 或 disabled"})
	}

	if err := h.service.SetStatus(uint(id), req.Status, getUserID(c)); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	actionMsg := map[string]string{
		"active":   "已启用",
		"disabled": "已停用",
	}[req.Status]

	return c.JSON(fiber.Map{
		"success": true,
		"message": actionMsg,
	})
}

// HealthCheck 检查所有账号的健康状态
// @Summary 账号健康检查
// @Tags 邮箱管理
// @Produce json
// @Param details query bool false "是否包含详细信息（默认只返回不健康的账号）"
// @Success 200 {object} services.HealthCheckSummary
// @Router /api/v1/accounts/health [get]
func (h *AccountHandler) HealthCheck(c *fiber.Ctx) error {
	if h.healthCheckService == nil {
		return c.Status(503).JSON(fiber.Map{
			"error": "健康检查服务未启用",
		})
	}

	details := c.Query("details", "false") == "true"

	summary, err := h.healthCheckService.CheckAllAccounts(details)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  "健康检查失败",
			"detail": err.Error(),
		})
	}

	summary.CheckedAt = time.Now().Format(time.RFC3339)

	return c.JSON(summary)
}
