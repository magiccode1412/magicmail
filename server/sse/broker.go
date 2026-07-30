// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package sse

import (
	"sync"
	"time"
)

// SSEEvent 表示一个服务器推送事件
type SSEEvent struct {
	Event  string      `json:"event"` // 事件类型: mail.received, mail.synced, etc.
	Data   interface{} `json:"data"`  // 事件负载数据
	UserID uint        `json:"-"`     // 目标用户（0 表示广播给所有连接）
}

// Client 表示一个 SSE 客户端连接
type Client struct {
	ID        string
	UserID    uint // 关联用户 ID（按用户隔离推送）
	Events    chan *SSEEvent // 事件通道
	CreatedAt time.Time
}

// Broker 管理 SSE 客户端连接和事件广播
type Broker struct {
	clients map[string]*Client // clientID -> client
	mu      sync.RWMutex

	// 新客户端注册通道（可选优化）
	register chan *Client
	// 客户端离开通道
	unregister chan string
	// 全局广播通道
	broadcast chan *SSEEvent

	// history 保存近期可重放事件，供新连接回放，避免错过已发生的状态变更（P0 第4点）
	// key 为 userID：0 表示全局广播事件（如 user.*），非 0 为对应用户的私有事件
	history map[uint][]*SSEEvent
}

// replayableEvents 标记哪些事件类型值得重放给新连接。
// 仅包含轻量的"状态变更/控制"类事件；高频且负载大的 mail.received / mail.sent 不重放。
//
// 注意：以下一次性结果/通知类事件已从重放列表移除，因为它们只应在"实时发生"时提示一次，
// 不应在每次新建 SSE 连接（刷新页面/重连）时被历史重放，否则会造成误报：
//   - account.health / account.sync_error：账号连接异常/同步失败的 toast，重放会反复骚扰（尤其账号已删除后仍在弹）
//   - account.sync_started / account.sync_done：同步进度/“同步完成” toast，重放会显示虚假进度或重复提示
// 这些事件的"当前状态"已由 accountStore.fetchAccounts() 的 status 字段反映，无需靠重放还原。
var replayableEvents = map[string]bool{
	"mail.synced":        true,
	"oauth.authorized":   true,
	"oauth.expired":      true,
	"account.created":    true,
	"account.updated":    true,
	"account.deleted":    true,
	"account.status_changed": true,
	"webhook.delivered":  true,
	"webhook.failed":     true,
	"user.created":       true,
	"user.deleted":       true,
	"stats.updated":      true,
}

const historyLimit = 32

// NewBroker 创建新的 SSE Broker
func NewBroker() *Broker {
	return &Broker{
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 64),
		unregister: make(chan string, 64),
		broadcast:  make(chan *SSEEvent, 256),
		history:    make(map[uint][]*SSEEvent),
	}
}

// 全局单例 Broker 实例
var globalBroker *Broker

// InitBroker 初始化全局 Broker 并启动后台处理协程
func InitBroker() {
	globalBroker = NewBroker()
	go globalBroker.run()
}

// GlobalBroker 获取全局 Broker 实例
func GlobalBroker() *Broker {
	return globalBroker
}

// run 是 Broker 的主循环，处理客户端注册/注销和事件广播
func (b *Broker) run() {
	for {
		select {
		case client := <-b.register:
			b.mu.Lock()
			b.clients[client.ID] = client
			b.mu.Unlock()

		case clientID := <-b.unregister:
			b.mu.Lock()
			if client, ok := b.clients[clientID]; ok {
				close(client.Events)
				delete(b.clients, clientID)
			}
			b.mu.Unlock()

		case event := <-b.broadcast:
			b.mu.RLock()
			for id, client := range b.clients {
				// 按用户隔离：仅投递给目标用户（UserID=0 表示广播）
				if event.UserID != 0 && client.UserID != event.UserID {
					continue
				}
				select {
				case client.Events <- event:
					// 发送成功
				default:
					// 客户端通道已满或阻塞，移除该客户端避免内存泄漏
					go func(clientID string) {
						b.unregister <- clientID
					}(id)
				}
			}
			b.mu.RUnlock()

			// 记录到历史事件（供新连接重放），仅在释放读锁后再取写锁
			if replayableEvents[event.Event] {
				b.appendHistory(event)
			}
		}
	}
}

// Register 注册新的 SSE 客户端，返回客户端 ID 和事件通道
func (b *Broker) Register(userID uint) (*Client, string) {
	clientID := generateClientID()
	client := &Client{
		ID:        clientID,
		UserID:    userID,
		Events:    make(chan *SSEEvent, 32),
		CreatedAt: time.Now(),
	}

	b.register <- client
	return client, clientID
}

// Unregister 注销 SSE 客户端
func (b *Broker) Unregister(clientID string) {
	b.unregister <- clientID
}

// Publish 发布事件给所有连接的客户端（全局广播）
func (b *Broker) Publish(eventType string, data interface{}) {
	event := &SSEEvent{
		Event: eventType,
		Data:  data,
	}

	select {
	case b.broadcast <- event:
	default:
	}
}

// appendHistory 将事件追加到对应用户（或全局）的历史缓冲，超出上限丢弃最旧
func (b *Broker) appendHistory(event *SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := event.UserID // 0 = 全局广播事件
	h := b.history[key]
	h = append(h, event)
	if len(h) > historyLimit {
		h = h[len(h)-historyLimit:]
	}
	b.history[key] = h
}

// ReplayHistory 返回某用户（含全局）近期可重放事件快照，供新连接初始化状态
func (b *Broker) ReplayHistory(userID uint) []*SSEEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []*SSEEvent
	out = append(out, b.history[0]...)     // 全局广播事件（如 user.*）
	out = append(out, b.history[userID]...) // 该用户私有事件
	return out
}

// PublishToUser 发布事件给指定用户的客户端（按用户隔离）
func (b *Broker) PublishToUser(userID uint, eventType string, data interface{}) {
	event := &SSEEvent{
		Event:  eventType,
		Data:   data,
		UserID: userID,
	}

	select {
	case b.broadcast <- event:
	default:
	}
}

// GetOnlineCount 获取当前在线客户端数量
func (b *Broker) GetOnlineCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// generateClientID 生成唯一客户端 ID
func generateClientID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond) // 避免重复（仅用于 ID 生成，性能影响可忽略）
	}
	return string(b)
}
