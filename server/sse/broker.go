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
}

// NewBroker 创建新的 SSE Broker
func NewBroker() *Broker {
	return &Broker{
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 64),
		unregister: make(chan string, 64),
		broadcast:  make(chan *SSEEvent, 256),
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
