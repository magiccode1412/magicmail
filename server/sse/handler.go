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

	// ⭐ 使用 SetBodyStreamWriter 保持长连接（Fiber SSE 标准写法）
	// 注意：writer 是在 handler 返回后由 fasthttp 异步执行的，
	// 因此 Unregister 必须放在 writer 内部，而不能 defer 在 handler 里
	// （否则 handler 一返回就会关闭 Events 通道，导致连接刚建立就断开）
	ctx := c.Context()
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer broker.Unregister(clientID)

		// 发送初始连接成功事件
		sendEventWriter(w, "connected", map[string]interface{}{
			"client_id":    clientID,
			"server_time":  time.Now().Format(time.RFC3339),
			"online_count": broker.GetOnlineCount(),
		})
		w.Flush()

		// 重放近期事件，避免新连接错过已发生的状态变更（P0 第4点：Broker 无重放 → 现已补齐）
		for _, ev := range broker.ReplayHistory(userID) {
			if err := sendEventWriter(w, ev.Event, ev.Data); err != nil {
				return
			}
			w.Flush()
		}

		// 保持连接活跃，发送心跳和推送事件
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// 客户端断开连接，及时退出并清理
				return

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

// PublishMailSent 发布邮件已发送事件（供 handler 调用，按用户隔离）
func PublishMailSent(userID, accountID uint, accountEmail string, data map[string]interface{}) {
	if GlobalBroker() == nil {
		return
	}

	GlobalBroker().PublishToUser(userID, "mail.sent", fiber.Map{
		"account_id":    accountID,
		"account_email": accountEmail,
		"mail":          data,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
}

// PublishMailDeleted 发布邮件删除事件（供 handler 在邮件删除成功后调用，按用户隔离）
// 携带被删除的邮件 ID 列表：其它标签页/客户端据此从本地列表即时移除对应项，
// 无需整列表刷新（避免删除操作的“复活”与陈旧数据回显），实现跨标签页实时一致。
func PublishMailDeleted(userID, accountID uint, ids []uint) {
	if GlobalBroker() == nil {
		return
	}
	GlobalBroker().PublishToUser(userID, "mail.deleted", fiber.Map{
		"account_id": accountID,
		"ids":        ids,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

// PublishDraftDeleted 发布草稿删除事件（供 handler 在草稿删除成功后调用，按用户隔离）
// 携带被删除的草稿 ID 列表：其它标签页/客户端据此从本地草稿列表即时移除对应项，
// 无需整列表刷新，实现跨标签页实时一致。
func PublishDraftDeleted(userID uint, ids []uint) {
	if GlobalBroker() == nil {
		return
	}
	GlobalBroker().PublishToUser(userID, "draft.deleted", fiber.Map{
		"ids":       ids,
		"timestamp": time.Now().Format(time.RFC3339),
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
	PublishStatsUpdated(userID, accountID, accountEmail)
}

// PublishOAuthAuthorized 发布 OAuth2 设备码授权成功事件（供后端轮询 goroutine 调用，按用户隔离）
func PublishOAuthAuthorized(userID uint, data fiber.Map) {
	if GlobalBroker() == nil {
		return
	}

	GlobalBroker().PublishToUser(userID, "oauth.authorized", data)
}

// PublishOAuthExpired 发布 OAuth2 设备码授权过期/失败事件（按用户隔离）
func PublishOAuthExpired(userID uint) {
	if GlobalBroker() == nil {
		return
	}

	GlobalBroker().PublishToUser(userID, "oauth.expired", fiber.Map{
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// PublishAccountSyncStarted 发布账号同步开始事件（供 handler 在用户手动触发同步时调用）
func PublishAccountSyncStarted(userID, accountID uint, accountEmail string) {
	if GlobalBroker() == nil {
		return
	}

	GlobalBroker().PublishToUser(userID, "account.sync_started", fiber.Map{
		"account_id":    accountID,
		"account_email": accountEmail,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
}

// PublishAccountSyncDone 发布账号同步完成事件（供 Worker 调用）
func PublishAccountSyncDone(userID, accountID uint, accountEmail string, mailCount int) {
	if GlobalBroker() == nil {
		return
	}

	GlobalBroker().PublishToUser(userID, "account.sync_done", fiber.Map{
		"account_id":    accountID,
		"account_email": accountEmail,
		"mail_count":    mailCount,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
}

// PublishAccountSyncError 发布账号同步失败事件（供 Worker 调用）
func PublishAccountSyncError(userID, accountID uint, accountEmail, errMsg string) {
	if GlobalBroker() == nil {
		return
	}

	GlobalBroker().PublishToUser(userID, "account.sync_error", fiber.Map{
		"account_id":    accountID,
		"account_email": accountEmail,
		"error":         errMsg,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
}

// PublishAccountCreated 发布账号创建事件（按用户隔离）
func PublishAccountCreated(userID, accountID uint, accountEmail string) {
	publishAccountEvent(userID, "account.created", accountID, accountEmail, "")
}

// PublishAccountUpdated 发布账号更新事件（按用户隔离）
func PublishAccountUpdated(userID, accountID uint, accountEmail string) {
	publishAccountEvent(userID, "account.updated", accountID, accountEmail, "")
}

// PublishAccountDeleted 发布账号删除事件（按用户隔离）
func PublishAccountDeleted(userID, accountID uint) {
	publishAccountEvent(userID, "account.deleted", accountID, "", "")
}

// PublishAccountStatusChanged 发布账号状态变更事件（按用户隔离）
func PublishAccountStatusChanged(userID, accountID uint, status string) {
	publishAccountEvent(userID, "account.status_changed", accountID, "", status)
}

// publishAccountEvent 账号事件的统一发布实现
func publishAccountEvent(userID uint, event string, accountID uint, accountEmail, status string) {
	if GlobalBroker() == nil {
		return
	}
	GlobalBroker().PublishToUser(userID, event, fiber.Map{
		"account_id":    accountID,
		"account_email": accountEmail,
		"status":        status,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
}

// PublishWebhookDelivered 发布 Webhook 投递成功事件（供 notifier 调用，按用户隔离）
func PublishWebhookDelivered(userID, webhookID uint) {
	if GlobalBroker() == nil {
		return
	}
	GlobalBroker().PublishToUser(userID, "webhook.delivered", fiber.Map{
		"webhook_id": webhookID,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

// PublishWebhookFailed 发布 Webhook 投递失败事件（供 notifier 调用，按用户隔离）
func PublishWebhookFailed(userID, webhookID uint, errMsg string) {
	if GlobalBroker() == nil {
		return
	}
	GlobalBroker().PublishToUser(userID, "webhook.failed", fiber.Map{
		"webhook_id": webhookID,
		"error":      errMsg,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

// PublishStatsUpdated 发布统计变更事件（供 Worker/Handler 在邮件数据变化时调用，按用户隔离）
// 前端侧边栏角标等消费此轻量信号以刷新未读计数，避免整列表刷新
func PublishStatsUpdated(userID, accountID uint, accountEmail string) {
	if GlobalBroker() == nil {
		return
	}
	GlobalBroker().PublishToUser(userID, "stats.updated", fiber.Map{
		"account_id":    accountID,
		"account_email": accountEmail,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
}

// PublishAccountHealth 发布账号连接健康状态变化事件（供 Worker 在状态切换时调用，按用户隔离）
// status 取值："active"（连接正常）/ "error"（同步失败）；前端据此实时提示账号连接异常
func PublishAccountHealth(userID, accountID uint, accountEmail, status, errMsg string) {
	if GlobalBroker() == nil {
		return
	}
	GlobalBroker().PublishToUser(userID, "account.health", fiber.Map{
		"account_id":    accountID,
		"account_email": accountEmail,
		"status":        status,
		"error":         errMsg,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
}

// PublishUserCreated 发布用户创建事件（广播给所有在线管理员，用于多管理员一致性）
func PublishUserCreated(userID uint, username string) {
	if GlobalBroker() == nil {
		return
	}
	// 使用 Publish（UserID=0）广播给全部连接
	GlobalBroker().Publish("user.created", fiber.Map{
		"user_id":   userID,
		"username":  username,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// PublishUserDeleted 发布用户删除事件（广播给所有在线管理员）
func PublishUserDeleted(userID uint) {
	if GlobalBroker() == nil {
		return
	}
	GlobalBroker().Publish("user.deleted", fiber.Map{
		"user_id":   userID,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
