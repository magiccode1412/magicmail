// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// StreamHandler 处理 SSE 连接请求
// GET /api/v1/mails/stream
func StreamHandler(c *fiber.Ctx) error {
	// 设置 SSE 必需的响应头
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache, no-transform")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no") // 禁用 Nginx 缓冲

	// 获取全局 Broker（如果未初始化则返回错误）
	broker := GlobalBroker()
	if broker == nil {
		return c.Status(503).SendString("SSE service not available")
	}

	// 注册新的客户端连接（按用户隔离推送）
	var userID uint
	if v, ok := c.Locals("user_id").(float64); ok {
		userID = uint(v)
	}
	client, clientID := broker.Register(userID)
	defer broker.Unregister(clientID)

	// ⭐ 使用 SetBodyStreamWriter 保持长连接（Fiber SSE 标准写法）
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// 发送初始连接成功事件
		sendEventWriter(w, "connected", map[string]interface{}{
			"client_id":    clientID,
			"server_time":  time.Now().Format(time.RFC3339),
			"online_count": broker.GetOnlineCount(),
		})
		w.Flush()

		// 保持连接活跃，发送心跳和推送事件
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case event, ok := <-client.Events:
				if !ok {
					return
				}

				// 发送事件到客户端
				if err := sendEventWriter(w, event.Event, event.Data); err != nil {
					log.Printf("[SSE] send event error: %v", err)
					return
				}
				w.Flush() // 立即刷新缓冲区

			case <-ticker.C:
				// 发送心跳包保持连接活跃
				if err := sendEventWriter(w, "heartbeat", map[string]interface{}{
					"time": time.Now().Format(time.RFC3339),
				}); err != nil {
					return
				}
				w.Flush()
			}
		}
	})

	return nil
}

// sendEventWriter 发送 SSE 格式的事件数据（写入 bufio.Writer，用于长连接）
func sendEventWriter(w *bufio.Writer, event string, data interface{}) error {
	// 序列化数据
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	// 构建 SSE 格式消息:
	// event: <event_name>\n
	// data: <json_data>\n\n
	message := fmt.Sprintf("event: %s\ndata: %s\n\n", event, jsonData)

	// 写入 bufio.Writer
	if _, err := w.WriteString(message); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	return nil
}

// sendEvent 发送 SSE 格式的事件数据（兼容旧接口）
func sendEvent(c *fiber.Ctx, event string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	message := fmt.Sprintf("event: %s\ndata: %s\n\n", event, jsonData)

	if _, err := c.Write([]byte(message)); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	return nil
}

// HealthCheckHandler SSE 服务健康检查
// GET /api/v1/mails/stream/health
func HealthCheckHandler(c *fiber.Ctx) error {
	broker := GlobalBroker()
	if broker == nil {
		return c.JSON(fiber.Map{
			"status":  "error",
			"message": "SSE broker not initialized",
		})
	}

	return c.JSON(fiber.Map{
		"status":       "ok",
		"online_count": broker.GetOnlineCount(),
		"service":      "sse-stream",
	})
}

// PublishMailReceived 发布新邮件到达事件（供 Worker 调用，按用户隔离）
func PublishMailReceived(userID, accountID uint, accountEmail string, count int, mails []map[string]interface{}) {
	if GlobalBroker() == nil {
		return
	}

	GlobalBroker().PublishToUser(userID, "mail.received", fiber.Map{
		"account_id":    accountID,
		"account_email": accountEmail,
		"mail_count":    count,
		"mails":         mails,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
}

// PublishMailSynced 发布邮件同步完成事件（供 Worker 调用，按用户隔离）
func PublishMailSynced(userID, accountID uint, accountEmail string) {
	if GlobalBroker() == nil {
		return
	}

	GlobalBroker().PublishToUser(userID, "mail.synced", fiber.Map{
		"account_id":    accountID,
		"account_email": accountEmail,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
}
