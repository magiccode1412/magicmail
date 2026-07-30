// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package services

import (
	"encoding/json"
	"fmt"
	"time"

	"magicmail/models"

	"gorm.io/gorm"
)

// DraftService 草稿业务逻辑
type DraftService struct {
	db *gorm.DB
}

// NewDraftService 创建草稿 Service 实例
func NewDraftService(db *gorm.DB) *DraftService {
	return &DraftService{db: db}
}

// SaveDraft 保存草稿（新建或更新）
func (s *DraftService) SaveDraft(userID uint, req *SaveDraftRequest) (*models.DraftResponse, error) {
	var draft models.Draft

	// 校验草稿所引用的邮箱账号归属当前用户
	if req.AccountID != 0 {
		var accCount int64
		s.db.Model(&models.MailAccount{}).Where("id = ? AND user_id = ?", req.AccountID, userID).Count(&accCount)
		if accCount == 0 {
			return nil, fmt.Errorf("邮箱账号不存在或无权使用")
		}
	}

	if req.ID != nil && *req.ID > 0 {
		// 更新已有草稿
		if err := s.db.Where("id = ? AND user_id = ?", *req.ID, userID).First(&draft).Error; err != nil {
			return nil, fmt.Errorf("草稿不存在")
		}
		draft.AccountID = req.AccountID
		draft.To = toJSONString(req.To)
		draft.Cc = toJSONString(req.Cc)
		draft.Bcc = toJSONString(req.Bcc)
		draft.Subject = req.Subject
		draft.Body = req.Body
		draft.HTMLBody = req.HTMLBody

		if err := s.db.Save(&draft).Error; err != nil {
			return nil, fmt.Errorf("保存草稿失败: %w", err)
		}
	} else {
		// 新建草稿
		toJSON, _ := json.Marshal(req.To)
		ccJSON, _ := json.Marshal(req.Cc)
		bccJSON, _ := json.Marshal(req.Bcc)

		draft = models.Draft{
			UserID:    userID,
			AccountID: req.AccountID,
			To:        string(toJSON),
			Cc:        string(ccJSON),
			Bcc:       string(bccJSON),
			Subject:   req.Subject,
			Body:      req.Body,
			HTMLBody:  req.HTMLBody,
		}

		if err := s.db.Create(&draft).Error; err != nil {
			return nil, fmt.Errorf("创建草稿失败: %w", err)
		}
	}

	return s.buildDraftResponse(&draft), nil
}

// ListDrafts 获取草稿列表
func (s *DraftService) ListDrafts(userID uint, page, pageSize int, keyword string) ([]models.DraftListItem, int64, error) {
	var total int64

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	baseQuery := s.db.Model(&models.Draft{}).Where("user_id = ?", userID)
	if keyword != "" {
		kw := "%" + keyword + "%"
		baseQuery = baseQuery.Where("drafts.subject LIKE ? OR drafts.body LIKE ? OR drafts.`to` LIKE ?", kw, kw, kw)
	}

	countQuery := baseQuery.Session(&gorm.Session{})
	countQuery.Count(&total)

	offset := (page - 1) * pageSize

	type draftRow struct {
		ID          uint
		AccountID   uint
		AccountName string
		To          string
		Subject     string
		Body        string
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}

	var rows []draftRow
	fields := "drafts.id, drafts.account_id, mail_accounts.name AS account_name," +
		" drafts.`to`, drafts.subject, drafts.body," +
		" drafts.created_at, drafts.updated_at"

	err := baseQuery.
		Joins("LEFT JOIN mail_accounts ON drafts.account_id = mail_accounts.id").
		Select(fields).
		Order("drafts.updated_at DESC").
		Offset(offset).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	items := make([]models.DraftListItem, len(rows))
	for i, d := range rows {
		items[i] = models.DraftListItem{
			ID:          d.ID,
			AccountID:   d.AccountID,
			AccountName: d.AccountName,
			To:          d.To,
			Subject:     d.Subject,
			Preview:     truncatePreview(d.Body, 150),
			CreatedAt:   d.CreatedAt,
			UpdatedAt:   d.UpdatedAt,
		}
	}

	return items, total, nil
}

// GetDraftByID 获取草稿详情
func (s *DraftService) GetDraftByID(id, userID uint) (*models.DraftResponse, error) {
	var draft models.Draft
	if err := s.db.Preload("Account").Where("id = ? AND user_id = ?", id, userID).First(&draft).Error; err != nil {
		return nil, fmt.Errorf("草稿不存在")
	}
	return s.buildDraftResponse(&draft), nil
}

// DeleteDraft 删除草稿
func (s *DraftService) DeleteDraft(id, userID uint) error {
	result := s.db.Delete(&models.Draft{}, "id = ? AND user_id = ?", id, userID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("草稿不存在或无权删除")
	}
	return nil
}

// BatchDelete 批量删除草稿
func (s *DraftService) BatchDelete(ids []uint, userID uint) (int64, []uint, error) {
	var deleted int64
	var failed []uint

	for _, id := range ids {
		if err := s.DeleteDraft(id, userID); err != nil {
			failed = append(failed, id)
			continue
		}
		deleted++
	}

	return deleted, failed, nil
}

// buildDraftResponse 构建草稿详情响应
func (s *DraftService) buildDraftResponse(draft *models.Draft) *models.DraftResponse {
	resp := &models.DraftResponse{
		ID:        draft.ID,
		AccountID: draft.AccountID,
		To:        draft.To,
		Cc:        draft.Cc,
		Subject:   draft.Subject,
		CreatedAt: draft.CreatedAt,
		UpdatedAt: draft.UpdatedAt,
	}

	if draft.Account != nil {
		resp.AccountName = draft.Account.Name
	}
	if draft.Body != "" {
		resp.Body = &draft.Body
	}
	if draft.HTMLBody != "" {
		resp.HTMLBody = &draft.HTMLBody
	}

	return resp
}

// SaveDraftRequest 保存草稿请求体
type SaveDraftRequest struct {
	ID        *uint    `json:"id,omitempty"`
	AccountID uint     `json:"account_id"`
	To        []string `json:"to"`
	Cc        []string `json:"cc,omitempty"`
	Bcc       []string `json:"bcc,omitempty"`
	Subject   string   `json:"subject"`
	Body      string   `json:"body"`
	HTMLBody  string   `json:"html_body,omitempty"`
}

// toJSONString 将字符串数组转为 JSON 字符串
func toJSONString(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(arr)
	return string(data)
}
