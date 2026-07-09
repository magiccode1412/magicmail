// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package notifier

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
	"strings"
	"time"

	"gorm.io/gorm"
)

// WebhookPayload 推送数据结构
type Payload struct {
	Event     string                 `json:"event"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// hookInfo 从 DB 查询出的最小化 webhook 数据
type hookInfo struct {
	ID      uint
	URL     string
	Secret  string
	Headers string
	Body    string
}

// TriggerByEvent 查询匹配的 Webhook 并异步推送（供 Worker 调用，避免循环依赖）
// userID 用于按用户隔离：仅触发该用户创建的 Webhook；userID=0 表示触发全部（兼容旧行为）
func TriggerByEvent(db *gorm.DB, event string, data map[string]interface{}, userID uint) {
	query := db.Table("webhooks").
		Select("id, url, secret, headers, body").
		Where("enabled = ?", true)
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	var hooks []hookInfo
	if err := query.Find(&hooks).Error; err != nil {
		log.Printf("[Webhook] 查询失败: %v", err)
		return
	}

	for _, h := range hooks {
		// 事件过滤
		eventsRaw := ""
		db.Table("webhooks").Select("events").Where("id = ?", h.ID).Scan(&eventsRaw)
		if !matchEvent(eventsRaw, event) {
			continue
		}

		go func(hook hookInfo) {
			payload := Payload{
				Event:     event,
				Timestamp: time.Now().Unix(),
				Data:      data,
			}
			result := dispatch(&hook, payload, event)

			now := time.Now()
			status := "success"
			errMsg := ""
			if !result.Success {
				status = "error"
				errMsg = result.ErrorMsg
			}
			db.Table("webhooks").Where("id = ?", hook.ID).Updates(map[string]interface{}{
				"last_status":    status,
				"last_trigger_at": now,
				"error_msg":      errMsg,
			})
			log.Printf("[Webhook] %s -> %s (%dms) %s%s", event, hook.URL, result.Duration, status, func() string {
			if status == "error" && result.ErrorMsg != "" {
				return " | " + result.ErrorMsg
			}
			return ""
		}())
		}(h)
	}
}

// matchEvent 检查事件是否匹配
func matchEvent(eventsRaw, event string) bool {
	for _, e := range strings.Split(eventsRaw, ",") {
		e = strings.TrimSpace(e)
		if e == "*" || e == event || (strings.HasSuffix(e, ".*") && strings.HasPrefix(event, strings.TrimSuffix(e, "."))) {
			return true
		}
	}
	return false
}

// dispatch 执行 HTTP POST（带重试：网络错误和 5xx 最多重试 3 次）
func dispatch(h *hookInfo, payload Payload, event string) *dispatchResult {
	const maxRetries = 3
	start := time.Now()
	var lastResult *dispatchResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s, 4s
			log.Printf("[Webhook] 第 %d 次重试 %s (等待 %v)...", attempt, h.URL, backoff)
			time.Sleep(backoff)
		}

		result := doDispatch(h, payload, event)
		lastResult = result

		// 成功 → 直接返回
		if result.Success {
			result.Duration = time.Since(start).Milliseconds()
			return result
		}

		// 4xx 客户端错误不重试（请求本身有问题）
		if result.StatusCode >= 400 && result.StatusCode < 500 {
			result.Duration = time.Since(start).Milliseconds()
			return result
		}

		// 网络错误或 5xx → 继续重试
		if attempt < maxRetries {
			log.Printf("[Webhook] ⚠️ %s 失败 (尝试 %d/%d): %s", h.URL, attempt+1, maxRetries+1, result.ErrorMsg)
		}
	}

	lastResult.Duration = time.Since(start).Milliseconds()
	lastResult.ErrorMsg = fmt.Sprintf("重试 %d 次后仍失败: %s", maxRetries, lastResult.ErrorMsg)
	return lastResult
}

// doDispatch 单次 HTTP POST 请求（无重试）
func doDispatch(h *hookInfo, payload Payload, event string) *dispatchResult {
	result := &dispatchResult{}

	var bodyBytes []byte
	var err error
	if h.Body != "" {
		bodyStr := RenderBodyTemplate(h.Body, event, payload.Timestamp, payload.Data)
		bodyBytes = []byte(bodyStr)
	} else {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			result.ErrorMsg = "JSON 序列化失败: " + err.Error()
			return result
		}
	}

	req, err := http.NewRequest("POST", h.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		result.ErrorMsg = "创建请求失败: " + err.Error()
		return result
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Magicmail-Webhook/1.0")
	req.Header.Set("X-Magicmail-Event", payload.Event)
	req.Header.Set("X-Magicmail-Timestamp", fmt.Sprintf("%d", payload.Timestamp))

	if h.Secret != "" {
		mac := hmac.New(sha256.New, []byte(h.Secret))
		mac.Write(bodyBytes)
		req.Header.Set("X-Magicmail-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	if h.Headers != "" {
		var custom map[string]string
		if json.Unmarshal([]byte(h.Headers), &custom) == nil {
			for k, v := range custom {
				req.Header.Set(k, v)
			}
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.ErrorMsg = "请求失败: " + err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	respBody, _ := io.ReadAll(resp.Body)
	result.Response = string(respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true
	} else {
		bodyPreview := string(respBody)
		if len(bodyPreview) > 300 {
			bodyPreview = bodyPreview[:300] + "..."
		}
		result.ErrorMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, bodyPreview)
	}
	return result
}

type dispatchResult struct {
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	Response   string `json:"response,omitempty"`
	ErrorMsg   string `json:"error,omitempty"`
	Duration   int64  `json:"duration_ms"`
}

// RenderBodyTemplate 渲染自定义 body 模板，支持 {{event}} {{timestamp}} {{data.xxx}} 占位符（导出供 service 复用）
func RenderBodyTemplate(tmpl string, event string, timestamp int64, data map[string]interface{}) string {
	result := tmpl
	result = strings.ReplaceAll(result, "{{event}}", event)
	result = strings.ReplaceAll(result, "{{timestamp}}", fmt.Sprintf("%d", timestamp))
	// 替换 data.xxx 形式的变量
	for k, v := range data {
		placeholder := "{{data." + k + "}}"
		var valStr string
		switch val := v.(type) {
		case string:
			valStr = val
		default:
			b, _ := json.Marshal(val)
			valStr = string(b)
		}
		result = strings.ReplaceAll(result, placeholder, valStr)
	}
	return result
}

// --- Web Push 通知桥接（避免循环依赖）---

var pushNotifier func(userID uint, title, body string, data any)

// RegisterPushNotifier 注册 Web Push 发送回调（由 routes.Register 调用）
func RegisterPushNotifier(fn func(userID uint, title, body string, data any)) {
	pushNotifier = fn
}

// SendPushNotification 发送 Web Push 推送（供 IMAP Worker 调用）
func SendPushNotification(userID uint, title, body string, data any) {
	log.Printf("[Push-Bridge] SendPushNotification called (userID=%d, title=%q, body=%q)", userID, title, body)
	if pushNotifier == nil {
		log.Printf("[Push-Bridge] ⚠️ pushNotifier is NULL — RegisterPushNotifier 可能未被调用！")
		return
	}
	pushNotifier(userID, title, body, data)
	log.Printf("[Push-Bridge] ✓ pushNotifier 已调用完成")
}
