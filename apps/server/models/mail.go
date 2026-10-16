// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package models

import (
	"database/sql"
	"time"
)

// Mail 邮件模型 - 存储从 IMAP 同步的邮件数据
type Mail struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	UserID        uint           `json:"user_id" gorm:"index;not null;default:0;comment:所属用户ID"`
	AccountID     uint           `json:"account_id" gorm:"index;not null;comment:所属邮箱账号ID"`
	MessageID     string         `json:"message_id" gorm:"type:varchar(512);index;not null;comment:原始Message-ID(去重用)"`
	MessageUID    uint32         `json:"message_uid" gorm:"index;comment:IMAP UID(用于增量同步)"`
	Folder        string         `json:"folder" gorm:"type:varchar(32);default:'inbox';index;comment:邮件文件夹(inbox/sent)"`
	From          string         `json:"from" gorm:"type:varchar(255);not null;default:'';comment:发件人"`
	To            string         `json:"to" gorm:"type:text;not null;default:'';comment:收件人"`
	Cc            string         `json:"cc,omitempty" gorm:"type:text;comment:抄送"`
	Bcc           string         `json:"-" gorm:"type:text;comment:密送(不暴露)"`
	Subject       string         `json:"subject" gorm:"type:varchar(500);not null;default:'';comment:邮件主题"`
	TextBody      sql.NullString `json:"text_body,omitempty" gorm:"type:text;comment:纯文本正文"`
	HTMLBody      sql.NullString `json:"html_body,omitempty" gorm:"type:text;comment:HTML正文"`
	SentAt        time.Time      `json:"sent_at" gorm:"index;comment:邮件发送时间"`
	IsRead        bool           `json:"is_read" gorm:"default:false;index;comment:是否已读"`
	IsStarred     bool           `json:"is_starred" gorm:"default:false;comment:是否标星"`
	HasAttachment bool           `json:"has_attachment" gorm:"default:false;index;comment:是否有附件"`
	Flags         int32          `json:"flags" gorm:"default:0;comment:IMAP标志位"`
	Size          int64          `json:"size" gorm:"comment:邮件大小(字节)"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`

	// 关联
	Account    *MailAccount  `json:"account,omitempty" gorm:"foreignKey:AccountID"`
	Attachments []Attachment `json:"attachments,omitempty" gorm:"foreignKey:MailID"`
}

// TableName 指定表名
func (Mail) TableName() string {
	return "mails"
}

// MailListFilter 邮件列表筛选条件
type MailListFilter struct {
	Page         int    `form:"page" validate:"min=1"`              // 页码，默认1
	PageSize     int    `form:"page_size" validate:"max=100"`         // 每页条数，默认20
	AccountID    uint   `form:"account_id"`                           // 筛选指定邮箱
	Folder       string `form:"folder"`                               // 文件夹筛选（inbox/sent）
	Keyword      string `form:"keyword"`                             // 搜索关键词（发件人/主题）
	IsRead       *bool  `form:"is_read"`                             // 筛选已读/未读
	HasAttachment *bool  `form:"has_attachment"`                     // 筛选有/无附件
	SortBy       string `form:"sort_by"`                             // 排序字段（sent_at, created_at）
	SortOrder    string `form:"sort_order"`                          // 排序方向（asc, desc）
}

// MailResponse 邮件详情 API 响应
type MailResponse struct {
	ID            uint             `json:"id"`
	AccountID     uint             `json:"account_id"`
	AccountName   string           `json:"account_name,omitempty"`
	Folder        string           `json:"folder"`
	MessageID     string           `json:"message_id"`
	From          string           `json:"from"`
	To            string           `json:"to"`
	Cc            string           `json:"cc,omitempty"`
	Subject       string           `json:"subject"`
	TextBody      *string          `json:"text_body,omitempty"`
	HTMLBody      *string          `json:"html_body,omitempty"`
	SentAt        time.Time        `json:"sent_at"`
	IsRead        bool             `json:"is_read"`
	IsStarred     bool             `json:"is_starred"`
	HasAttachment bool             `json:"has_attachment"`
	Size          int64            `json:"size"`
	Attachments   []AttachmentResp `json:"attachments,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
}

// MailListItem 邮件列表项（轻量，不含正文）
type MailListItem struct {
	ID            uint      `json:"id"`
	AccountID     uint      `json:"account_id"`
	AccountName   string    `json:"account_name,omitempty"`
	Folder        string    `json:"folder"`
	From          string    `json:"from"`
	To            string    `json:"to"`
	Subject       string    `json:"subject"`
	Preview       string    `json:"preview"`        // 正文预览（截取前150字符）
	SentAt        time.Time `json:"sent_at"`
	IsRead        bool      `json:"is_read"`
	IsStarred     bool      `json:"is_starred"`
	HasAttachment bool      `json:"has_attachment"`
	Size          int64     `json:"size"`
	CreatedAt     time.Time `json:"created_at"`
}
