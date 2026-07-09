// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"magicmail/services"
	"magicmail/models"
	"magicmail/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// MailHandler 邮件 API 处理器
type MailHandler struct {
	service *services.MailService
}

// NewMailHandler 创建邮件 Handler 实例
func NewMailHandler(svc *services.MailService) *MailHandler {
	return &MailHandler{service: svc}
}

// List 获取邮件列表（分页、搜索、筛选）
// @Summary 获取邮件列表
// @Tags 邮件管理
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param account_id query int false "邮箱ID"
// @Param keyword query string false "搜索关键词"
// @Param is_read query bool false "已读筛选"
// @Param has_attachment query bool false "附件筛选"
// @Param sort_by query string false "排序字段"
// @Param sort_order query string false "排序方向"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/mails [get]
func (h *MailHandler) List(c *fiber.Ctx) error {
	filter := models.MailListFilter{
		Page:      1,
		PageSize: 20,
		SortBy:   "sent_at",
		SortOrder: "desc",
	}

	if v := c.Query("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			filter.Page = p
		}
	}
	if v := c.Query("page_size"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 100 {
			filter.PageSize = p
		}
	}
	if v := c.Query("account_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			filter.AccountID = uint(id)
		}
	}
	filter.Keyword = c.Query("keyword")
	filter.Folder = c.Query("folder")

	if v := c.Query("is_read"); v != "" {
		b := v == "true"
		filter.IsRead = &b
	}
	if v := c.Query("has_attachment"); v != "" {
		b := v == "true"
		filter.HasAttachment = &b
	}
	filter.SortBy = c.Query("sort_by")
	if filter.SortBy == "" {
		filter.SortBy = "sent_at"
	}
	filter.SortOrder = c.Query("sort_order")
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}

	mails, total, err := h.service.List(filter, getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  "获取邮件列表失败",
			"detail": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data":       mails,
		"total":      total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
	})
}

// Get 获取邮件详情
// @Summary 获取邮件详情
// @Tags 邮件管理
// @Produce json
// @Param id path int true "邮件ID"
// @Success 200 {object} models.MailResponse
// @Router /api/v1/mails/{id} [get]
func (h *MailHandler) Get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	mail, err := h.service.GetByID(uint(id), getUserID(c))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "邮件不存在"})
	}

	return c.JSON(mail)
}

// MarkAsRead 标记已读/未读
// @Summary 标记邮件已读状态
// @Tags 邮件管理
// @Accept json
// @Param id path int true "邮件ID"
// @Param body body map[string]bool true "{ is_read: true/false }"
// @Success 200 {object} map[string]string
// @Router /api/v1/mails/{id}/read [put]
func (h *MailHandler) MarkAsRead(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	var body struct {
		IsRead bool `json:"is_read"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}

	err = h.service.MarkAsRead(uint(id), body.IsRead, getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "操作失败"})
	}

	status := "未读"
	if body.IsRead {
		status = "已读"
	}
	return c.JSON(fiber.Map{
		"success": true,
		"message": "已标记为" + status,
	})
}

// MarkAsStarred 标记星标
// @Summary 标记邮件星标
// @Tags 邮件管理
// @Accept json
// @Param id path int true "邮件ID"
// @Param body body map[string]bool true "{ starred: true/false }"
// @Success 200 {object} map[string]string
// @Router /api/v1/mails/{id}/star [put]
func (h *MailHandler) MarkAsStarred(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	var body struct {
		Starred bool `json:"starred"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}

	err = h.service.MarkAsStarred(uint(id), body.Starred, getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "操作失败"})
	}

	action := "取消标星"
	if body.Starred {
		action = "已标星"
	}
	return c.JSON(fiber.Map{
		"success": true,
		"message": action,
	})
}

// Delete 删除邮件
// @Summary 删除邮件
// @Tags 邮件管理
// @Param id path int true "邮件ID"
// @Success 200 {object} services.DeleteResult
// @Router /api/v1/mails/{id} [delete]
func (h *MailHandler) Delete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	result := h.service.Delete(uint(id), getUserID(c))
	if !result.Success {
		return c.Status(500).JSON(fiber.Map{"error": "删除失败"})
	}

	return c.JSON(result)
}

// BatchDelete 批量删除邮件
func (h *MailHandler) BatchDelete(c *fiber.Ctx) error {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}
	if len(req.IDs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "请选择要删除的邮件"})
	}

	result := h.service.BatchDelete(req.IDs, getUserID(c))

	return c.JSON(fiber.Map{
		"success":           result.Success,
		"deleted":           result.Deleted,
		"failed":            result.Failed,
		"server_sync_result": result.ServerSyncResult,
		"message":           fmt.Sprintf("已删除 %d 封邮件", result.Deleted),
	})
}

// BatchMarkAsRead 批量标记邮件已读/未读状态
func (h *MailHandler) BatchMarkAsRead(c *fiber.Ctx) error {
	var req struct {
		IDs    []uint `json:"ids"`
		IsRead bool   `json:"is_read"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}
	if len(req.IDs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "请选择要操作的邮件"})
	}

	result := h.service.BatchMarkAsRead(req.IDs, req.IsRead, getUserID(c))

	status := "未读"
	if req.IsRead {
		status = "已读"
	}

	return c.JSON(fiber.Map{
		"success": result.Success,
		"updated": result.Updated,
		"failed":  result.Failed,
		"message": fmt.Sprintf("已将 %d 封邮件标记为%s", result.Updated, status),
	})
}

// MarkAllAsRead 一键将符合筛选条件的邮件全部标记为已读
// @Summary 一键标记所有邮件为已读
// @Tags 邮件管理
// @Accept json
// @Param body body object false "筛选条件（与列表查询一致，可选）"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/mails/mark-all-read [post]
func (h *MailHandler) MarkAllAsRead(c *fiber.Ctx) error {
	var body struct {
		AccountID     uint   `json:"account_id"`
		Folder        string `json:"folder"`
		Keyword       string `json:"keyword"`
		IsRead        *bool  `json:"is_read"`
		HasAttachment *bool  `json:"has_attachment"`
	}
	// 容忍空 body
	_ = c.BodyParser(&body)

	filter := models.MailListFilter{
		AccountID:     body.AccountID,
		Folder:        body.Folder,
		Keyword:       body.Keyword,
		IsRead:        body.IsRead,
		HasAttachment: body.HasAttachment,
	}

	updated, err := h.service.MarkAllAsRead(filter, getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "操作失败"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"updated": updated,
		"message": fmt.Sprintf("已将 %d 封邮件标记为已读", updated),
	})
}

// GetStats 获取邮件统计信息
// @Summary 邮件统计
// @Tags 邮件管理
// @Produce json
// @Param account_id query int false "指定邮箱"
// @Success 200 {object} map[string]int64
// @Router /api/v1/mails/stats [get]
func (h *MailHandler) GetStats(c *fiber.Ctx) error {
	var accountID *uint
	if v := c.Query("account_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			aID := uint(id)
			accountID = &aID
		}
	}

	stats := h.service.GetStats(accountID, getUserID(c))
	return c.JSON(stats)
}

// Send 发送邮件
// @Summary 发送邮件
// @Tags 邮件管理
// @Accept json
// @Produce json
// @Param body body smtp.SendRequest true "发送邮件参数"
// @Success 200 {object} smtp.SendResult
// @Router /api/v1/mails/send [post]
func (h *MailHandler) Send(c *fiber.Ctx) error {
	var req smtp.SendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}

	// 基本校验
	if req.AccountID == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "请选择发件邮箱账号"})
	}
	if len(req.To) == 0 || (len(req.To) == 1 && req.To[0] == "") {
		return c.Status(400).JSON(fiber.Map{"error": "请填写至少一个收件人"})
	}
	if req.Subject == "" {
		return c.Status(400).JSON(fiber.Map{"error": "请填写邮件主题"})
	}

	// 安全校验：拒绝包含控制字符的输入（防止 CRLF 注入，CWE-93）
	allAddrs := append(append(req.To, req.Cc...), req.Bcc...)
	for _, addr := range allAddrs {
		if strings.ContainsAny(addr, "\r\n") {
			return c.Status(400).JSON(fiber.Map{"error": "邮箱地址包含非法字符"})
		}
	}
	if strings.ContainsAny(req.Subject, "\r\n") {
		return c.Status(400).JSON(fiber.Map{"error": "邮件主题包含非法字符"})
	}

	result, err := h.service.SendMail(req, getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "发送失败",
			"detail":  err.Error(),
		})
	}

	// 发送成功后，保存邮件记录到数据库（folder=sent）
	senderEmail, _ := h.service.GetAccountEmail(req.AccountID, getUserID(c))
	toJSON, _ := json.Marshal(req.To)
	ccJSON, _ := json.Marshal(req.Cc)

	sentMail := models.Mail{
		AccountID:  req.AccountID,
		MessageID:  result.MessageID,
		Folder:     "sent",
		From:       senderEmail,
		To:         string(toJSON),
		Cc:         string(ccJSON),
		Subject:    req.Subject,
		TextBody:   sql.NullString{String: req.Body, Valid: req.Body != ""},
		SentAt:     time.Now(),
		IsRead:     true,
	}
	if req.HTMLBody != "" {
		sentMail.HTMLBody = sql.NullString{String: req.HTMLBody, Valid: true}
	}
	if err := h.service.SaveSentMail(&sentMail); err != nil {
		// 保存失败不回滚发送结果，仅记录日志
		log.Printf("[WARN] 保存已发送邮件记录失败: %v", err)
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"message":   "邮件已成功发送",
		"messageId": result.MessageID,
	})
}
