// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"magicmail/models"
	"magicmail/notifier"

	"gorm.io/gorm"
)

// WebhookService Webhook 业务逻辑
type WebhookService struct {
	db *gorm.DB
	httpClient *http.Client
}

// NewWebhookService 创建 Webhook Service
func NewWebhookService(db *gorm.DB) *WebhookService {
	return &WebhookService{
		db: db,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// TriggerByEvent 根据事件名触发匹配的 Webhook（按用户隔离）
func (s *WebhookService) TriggerByEvent(event string, data map[string]interface{}, userID uint) {
	notifier.TriggerByEvent(s.db, event, data, userID)
}

// --- CRUD ---

// List 获取指定用户的 Webhook 列表
func (s *WebhookService) List(userID uint) ([]models.WebhookResponse, error) {
	var hooks []models.Webhook
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&hooks).Error; err != nil {
		return nil, err
	}
	responses := make([]models.WebhookResponse, len(hooks))
	for i, h := range hooks {
		responses[i] = s.toResponse(h)
	}
	return responses, nil
}

// GetByID 获取单个 Webhook 详情（校验归属用户）
func (s *WebhookService) GetByID(id, userID uint) (*models.WebhookResponse, error) {
	var hook models.Webhook
	if err := s.db.Where("user_id = ?", userID).First(&hook, id).Error; err != nil {
		return nil, err
	}
	resp := s.toResponse(hook)
	return &resp, nil
}

// Create 创建 Webhook（归属当前用户）
func (s *WebhookService) Create(req models.WebhookRequest, userID uint) (*models.WebhookResponse, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.Events == "" {
		req.Events = "mail.received"
	}

	hook := models.Webhook{
		UserID:  userID,
		Name:    req.Name,
		URL:     req.URL,
		Events:  req.Events,
		Secret:  req.Secret,
		Headers: req.Headers,
		Body:    req.Body,
		Enabled: enabled,
	}

	if err := s.db.Create(&hook).Error; err != nil {
		return nil, err
	}

	resp := s.toResponse(hook)
	return &resp, nil
}

// Update 更新 Webhook（校验归属用户）
func (s *WebhookService) Update(id uint, req models.WebhookRequest, userID uint) (*models.WebhookResponse, error) {
	var hook models.Webhook
	if err := s.db.Where("user_id = ?", userID).First(&hook, id).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"name":    req.Name,
		"url":     req.URL,
		"events":  req.Events,
		"headers": req.Headers,
		"body":    req.Body,
	}

	if req.Secret != "" {
		updates["secret"] = req.Secret
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if err := s.db.Model(&hook).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.db.First(&hook, id)
	resp := s.toResponse(hook)
	return &resp, nil
}

// Delete 删除 Webhook（校验归属用户）
func (s *WebhookService) Delete(id, userID uint) error {
	result := s.db.Where("user_id = ?", userID).Delete(&models.Webhook{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	// 同时清理日志
	s.db.Where("webhook_id = ?", id).Delete(&models.WebhookLog{})
	return nil
}

// Test 发送测试请求（校验归属用户）
func (s *WebhookService) Test(id, userID uint) (*TestResult, error) {
	var hook models.Webhook
	if err := s.db.Where("user_id = ?", userID).First(&hook, id).Error; err != nil {
		return nil, err
	}

	payload := models.WebhookPayload{
		Event:     "test",
		Timestamp: time.Now().Unix(),
		Data:      map[string]interface{}{"message": "Magicmail Webhook 测试"},
	}

	result := s.dispatch(&hook, payload)
	s.updateHookAfterDispatch(&hook, result)
	return result, nil
}

// dispatch 执行 HTTP 推送（用于测试）
func (s *WebhookService) dispatch(hook *models.Webhook, payload models.WebhookPayload) *TestResult {
	start := time.Now()
	result := &TestResult{}

	var bodyBytes []byte
	var err error
	if hook.Body != "" {
		bodyStr := notifier.RenderBodyTemplate(hook.Body, payload.Event, payload.Timestamp, payload.Data)
		bodyBytes = []byte(bodyStr)
	} else {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			result.Success = false
			result.ErrorMsg = "JSON 序列化失败: " + err.Error()
			return result
		}
	}

	req, err := http.NewRequest("POST", hook.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		result.Success = false
		result.ErrorMsg = "创建请求失败: " + err.Error()
		return result
	}

	// 设置标准请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Magicmail-Webhook/1.0")
	req.Header.Set("X-Magicmail-Event", payload.Event)
	req.Header.Set("X-Magicmail-Timestamp", fmt.Sprintf("%d", payload.Timestamp))

	// 签名
	if hook.Secret != "" {
		sig := generateSignature(hook.Secret, bodyBytes)
		req.Header.Set("X-Magicmail-Signature", "sha256="+sig)
	}

	// 自定义 Headers
	if hook.Headers != "" {
		customHeaders := make(map[string]string)
		if err := json.Unmarshal([]byte(hook.Headers), &customHeaders); err == nil {
			for k, v := range customHeaders {
				req.Header.Set(k, v)
			}
		}
	}

	resp, err := s.httpClient.Do(req)
	result.Duration = time.Since(start).Milliseconds()

	if err != nil {
		result.Success = false
		result.ErrorMsg = "请求发送失败: " + err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	respBody, _ := io.ReadAll(resp.Body)
	result.Response = string(respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true
	} else {
		result.Success = false
		result.ErrorMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	return result
}

// updateHookAfterDispatch 更新 Webhook 的状态信息
func (s *WebhookService) updateHookAfterDispatch(hook *models.Webhook, result *TestResult) {
	now := time.Now()
	status := "success"
	errMsg := ""
	if !result.Success {
		status = "error"
		errMsg = result.ErrorMsg
	}

	s.db.Model(hook).Updates(map[string]interface{}{
		"last_status":    status,
		"last_trigger_at": now,
		"error_msg":      errMsg,
	})
}

// saveLog 保存推送日志
func (s *WebhookService) saveLog(webhookID uint, event string, result *TestResult) {
	logEntry := models.WebhookLog{
		WebhookID:    webhookID,
		Event:       event,
		Status:      map[bool]string{true: "success", false: "error"}[result.Success],
		ResponseCode: result.StatusCode,
		ResponseBody: result.Response,
		Duration:    result.Duration,
		ErrorMsg:    result.ErrorMsg,
	}
	// 需要获取 URL
	var hook models.Webhook
	if s.db.First(&hook, webhookID).Error == nil {
		logEntry.RequestURL = hook.URL
	}

	if err := s.db.Create(&logEntry).Error; err != nil {
		log.Printf("[Webhook] 保存日志失败: %v", err)
	}
}

// GetLogs 获取指定 Webhook 的最近日志（校验归属用户）
func (s *WebhookService) GetLogs(webhookID, userID uint, limit int) ([]models.WebhookLog, error) {
	var logs []models.WebhookLog
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	// 先确认 webhook 归属当前用户
	var hook models.Webhook
	if err := s.db.Where("user_id = ?", userID).First(&hook, webhookID).Error; err != nil {
		return nil, err
	}
	if err := s.db.Where("webhook_id = ?", webhookID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// --- 工具函数 ---

// toResponse 转换为响应 DTO
func (s *WebhookService) toResponse(hook models.Webhook) models.WebhookResponse {
	return models.WebhookResponse{
		ID:           hook.ID,
		Name:         hook.Name,
		URL:          hook.URL,
		Events:       hook.Events,
		HasSecret:    hook.Secret != "",
		Headers:      hook.Headers,
		Body:         hook.Body,
		Enabled:      hook.Enabled,
		LastStatus:   hook.LastStatus,
		LastTriggerAt: hook.LastTriggerAt,
		ErrorMsg:     hook.ErrorMsg,
		CreatedAt:    hook.CreatedAt,
		UpdatedAt:    hook.UpdatedAt,
	}
}

// generateSignature 生成 HMAC-SHA256 签名
func generateSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TestResult 测试结果
type TestResult struct {
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	Response   string `json:"response,omitempty"`
	ErrorMsg   string `json:"error,omitempty"`
	Duration   int64  `json:"duration_ms"`
}
