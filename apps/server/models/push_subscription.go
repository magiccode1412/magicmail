// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package models

import (
	"time"
)

// PushSubscription Web Push 订阅记录（每个设备/浏览器一条）
type PushSubscription struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index;not null;comment:关联用户 ID"`
	Endpoint  string    `json:"endpoint" gorm:"type:text;not null;comment:Push endpoint URL"`
	P256DH    string    `json:"p256dh" gorm:"type:text;not null;comment:ECDH 公钥(base64url)"`
	Auth      string    `json:"auth" gorm:"type:text;not null;comment:Auth secret(base64url)"`
	UserAgent string    `json:"user_agent,omitempty" gorm:"type:text;comment:浏览器 UA(调试用)"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PushSubscription) TableName() string {
	return "push_subscriptions"
}

// SubscribeRequest 前端提交的订阅请求体
type SubscribeRequest struct {
	Endpoint string `json:"endpoint" validate:"required"`
	Keys     struct {
		P256DH string `json:"p256dh" validate:"required"`
		Auth   string `json:"auth" validate:"required"`
	} `json:"keys" validate:"required"`
	UserAgent string `json:"user_agent,omitempty"`
}

// ToPushSubscription 将请求转换为模型
func (r *SubscribeRequest) ToPushSubscription(userID uint) *PushSubscription {
	return &PushSubscription{
		UserID:    userID,
		Endpoint:  r.Endpoint,
		P256DH:    r.Keys.P256DH,
		Auth:      r.Keys.Auth,
		UserAgent: r.UserAgent,
	}
}
