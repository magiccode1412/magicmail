// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package models

import (
	"time"
)

// Draft 邮件草稿模型
type Draft struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	AccountID uint      `json:"account_id" gorm:"index;comment:发件邮箱账号ID"`
	To        string    `json:"to" gorm:"type:text;not null;default:'';comment:收件人(JSON数组字符串)"`
	Cc        string    `json:"cc,omitempty" gorm:"type:text;comment:抄送(JSON数组字符串)"`
	Bcc       string    `json:"-" gorm:"type:text;comment:密送(JSON数组字符串)"`
	Subject   string    `json:"subject" gorm:"type:varchar(500);not null;default:'';comment:邮件主题"`
	Body      string    `json:"body,omitempty" gorm:"type:text;comment:纯文本正文"`
	HTMLBody  string    `json:"html_body,omitempty" gorm:"type:text;comment:HTML正文"`
	UserID    uint      `json:"user_id" gorm:"index;comment:所属用户ID"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 关联
	Account *MailAccount `json:"account,omitempty" gorm:"foreignKey:AccountID"`
}

// TableName 指定表名
func (Draft) TableName() string {
	return "drafts"
}

// DraftListItem 草稿列表项（轻量，不含正文）
type DraftListItem struct {
	ID          uint      `json:"id"`
	AccountID   uint      `json:"account_id"`
	AccountName string    `json:"account_name,omitempty"`
	To          string    `json:"to"`
	Subject     string    `json:"subject"`
	Preview     string    `json:"preview"` // 正文预览（截取前150字符）
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DraftResponse 草稿详情 API 响应
type DraftResponse struct {
	ID          uint      `json:"id"`
	AccountID   uint      `json:"account_id"`
	AccountName string    `json:"account_name,omitempty"`
	To          string    `json:"to"`
	Cc          string    `json:"cc,omitempty"`
	Subject     string    `json:"subject"`
	Body        *string   `json:"body,omitempty"`
	HTMLBody    *string   `json:"html_body,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
