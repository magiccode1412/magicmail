// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"fmt"
	"magicmail/sse"
	"magicmail/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// DraftHandler 草稿 API 处理器
type DraftHandler struct {
	service *services.DraftService
}

// NewDraftHandler 创建草稿 Handler 实例
func NewDraftHandler(svc *services.DraftService) *DraftHandler {
	return &DraftHandler{service: svc}
}

// getUserID 从 context 中提取用户 ID
func getUserID(c *fiber.Ctx) uint {
	uid, _ := c.Locals("user_id").(float64)
	return uint(uid)
}

// List 获取草稿列表
func (h *DraftHandler) List(c *fiber.Ctx) error {
	userID := getUserID(c)

	page := 1
	pageSize := 20

	if v := c.Query("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := c.Query("page_size"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 100 {
			pageSize = p
		}
	}

	keyword := c.Query("keyword")

	drafts, total, err := h.service.ListDrafts(userID, page, pageSize, keyword)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  "获取草稿列表失败",
			"detail": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data":       drafts,
		"total":      total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Get 获取草稿详情
func (h *DraftHandler) Get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	userID := getUserID(c)
	draft, err := h.service.GetDraftByID(uint(id), userID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "草稿不存在"})
	}

	return c.JSON(draft)
}

// Save 保存草稿（新建或更新）
func (h *DraftHandler) Save(c *fiber.Ctx) error {
	var req services.SaveDraftRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}

	userID := getUserID(c)

	draft, err := h.service.SaveDraft(userID, &req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "保存草稿失败",
			"detail":  err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "草稿已保存",
		"data":    draft,
	})
}

// Delete 删除草稿
func (h *DraftHandler) Delete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	userID := getUserID(c)

	if err := h.service.DeleteDraft(uint(id), userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// 推送实时事件：携带被删草稿 ID，供其它标签页/客户端即时从本地列表移除（无需整列表刷新）
	sse.PublishDraftDeleted(userID, []uint{uint(id)})

	return c.JSON(fiber.Map{
		"success": true,
		"message": "草稿已删除",
	})
}

// BatchDelete 批量删除草稿
func (h *DraftHandler) BatchDelete(c *fiber.Ctx) error {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}
	if len(req.IDs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "请选择要删除的草稿"})
	}

	userID := getUserID(c)

	deleted, failed, err := h.service.BatchDelete(req.IDs, userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "批量删除失败", "detail": err.Error()})
	}

	// 推送实时事件：携带被删草稿 ID 列表，供其它标签页/客户端即时从本地列表移除
	sse.PublishDraftDeleted(userID, req.IDs)

	return c.JSON(fiber.Map{
		"success": true,
		"deleted": deleted,
		"failed":  failed,
		"message": fmt.Sprintf("已删除 %d 个草稿", deleted),
	})
}
