// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package models

import (
	"time"
)

// Webhook 自定义 Webhook 通知配置
type Webhook struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index;not null;default:0;comment:所属用户ID"`
	Name      string    `json:"name" gorm:"type:varchar(100);not null;comment:Webhook 名称"`
	URL       string    `json:"url" gorm:"type:varchar(500);not null;comment:Webhook 回调地址"`
	Events    string    `json:"events" gorm:"type:varchar(200);not null;default:'mail.received';comment:触发事件(逗号分隔)"`
	Secret    string    `json:"-" gorm:"type:varchar(255);comment:签名密钥(可选)"`
	Headers   string    `json:"headers,omitempty" gorm:"type:text;comment:自定义请求头(JSON格式)"`
	Body      string    `json:"body,omitempty" gorm:"type:text;comment:自定义请求体(JSON模板,留空则使用默认)"`
	Enabled   bool      `json:"enabled" gorm:"default:true;index;comment:是否启用"`
	LastStatus string   `json:"last_status" gorm:"type:varchar(20);default:'';comment:上次推送状态(success/error/pending)"`
	LastTriggerAt *time.Time `json:"last_trigger_at" gorm:"comment:最后触发时间"`
	ErrorMsg  string    `json:"error_msg,omitempty" gorm:"type:text;comment:错误信息"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Webhook) TableName() string {
	return "webhooks"
}

// WebhookRequest 创建/编辑 Webhook 的请求体
type WebhookRequest struct {
	Name    string `json:"name" validate:"required,min=1,max=100"`
	URL     string `json:"url" validate:"required,url"`
	Events  string `json:"events"` // 逗号分隔的事件名, 如 "mail.received,mail.sent"
	Secret  string `json:"secret"`  // 签名密钥（可选）
	Headers string `json:"headers"` // 自定义请求头 JSON（可选）
	Body    string `json:"body"`    // 自定义请求体 JSON 模板（可选，留空使用默认结构）
	Enabled *bool  `json:"enabled"` // 为 nil 时不更新
}

// WebhookResponse Webhook API 响应（不暴露 Secret）
type WebhookResponse struct {
	ID           uint       `json:"id"`
	Name         string     `json:"name"`
	URL          string     `json:"url"`
	Events       string     `json:"events"`
	HasSecret    bool       `json:"has_secret"`
	Headers      string     `json:"headers,omitempty"`
	Body         string     `json:"body,omitempty"`
	Enabled      bool       `json:"enabled"`
	LastStatus   string     `json:"last_status"`
	LastTriggerAt *time.Time `json:"last_trigger_at,omitempty"`
	ErrorMsg     string     `json:"error_msg,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// WebhookPayload 推送给 Webhook 的数据结构
type WebhookPayload struct {
	Event     string                 `json:"event"`               // 事件类型
	Timestamp int64                  `json:"timestamp"`           // Unix 时间戳
	Data      map[string]interface{} `json:"data"`                // 事件数据
}

// WebhookLog Webhook 推送日志
type WebhookLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	WebhookID uint      `json:"webhook_id" gorm:"index;not null;comment:Webhook ID"`
	Event     string    `json:"event" gorm:"type:varchar(50);not null;comment:触发事件"`
	Status    string    `json:"status" gorm:"type:varchar(20);not null;comment:推送状态(success/error)"`
	RequestURL string  `json:"request_url" gorm:"type:varchar(500);not null;comment:请求地址"`
	ResponseCode int   `json:"response_code" gorm:"comment:HTTP 状态码"`
	ResponseBody string `json:"response_body,omitempty" gorm:"type:text;comment:响应内容"`
	Duration  int64     `json:"duration" gorm:"comment:耗时(ms)"`
	ErrorMsg  string    `json:"error_msg,omitempty" gorm:"type:text;comment:错误信息"`
	CreatedAt time.Time `json:"created_at"`
}

func (WebhookLog) TableName() string { return "webhook_logs" }
