// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package models

import (
	"time"
)

// Attachment 附件模型 - 存储邮件附件（支持懒加载模式）
type Attachment struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	MailID      uint       `json:"mail_id" gorm:"index;not null;comment:所属邮件ID"`
	Filename    string     `json:"filename" gorm:"type:varchar(500);not null;comment:文件名"`
	ContentType string     `json:"content_type" gorm:"type:varchar(255);not null;comment:MIME类型"`
	Size        int64      `json:"size" gorm:"not null;comment:大小(字节)"`
	Content     []byte     `json:"-" gorm:"type:blob;comment:附件内容(小文件存DB)"` // 小附件存 DB BLOB
	FilePath    string     `json:"-" gorm:"type:text;comment:大附件存储路径"`          // 缓存到本地文件系统的路径
	IMAPUID     uint32     `json:"-" gorm:"comment:IMAP UID(用于按需从服务器下载)"`    // ⭐ 新增：IMAP 消息 UID
	PartID      string     `json:"-" gorm:"type:varchar(64);comment:MIME Part ID(如1.2)"` // ⭐ 新增：MIME 部分 ID
	IsCached    bool       `json:"is_cached" gorm:"default:false;comment:是否已缓存到本地"` // ⭐ 新增
	CacheExpire *time.Time `json:"-" gorm:"comment:缓存过期时间"`                        // ⭐ 新增
	CreatedAt   time.Time  `json:"created_at"`
}

// TableName 指定表名
func (Attachment) TableName() string {
	return "attachments"
}

// AttachmentResp 附件 API 响应（不含内容字段）
type AttachmentResp struct {
	ID          uint       `json:"id"`
	MailID      uint       `json:"mail_id"`
	Filename    string     `json:"filename"`
	ContentType string     `json:"content_type"`
	Size        int64      `json:"size"`
	SizeHuman   string     `json:"size_human"` // 人类可读大小
	IsCached    bool       `json:"is_cached"` // ⭐ 新增：前端可据此显示状态
	CreatedAt   time.Time `json:"created_at"`
}

// IsFileBased 判断是否存储在文件系统
func (a *Attachment) IsFileBased() bool {
	return a.FilePath != ""
}

// IsLazyLoaded 判断是否为懒加载模式（未缓存，需从 IMAP 按需获取）
func (a *Attachment) IsLazyLoaded() bool {
	return !a.IsCached && a.IMAPUID > 0 && a.PartID != ""
}

// MaxDBSize 数据库 BLOB 存储阈值：5MB
const MaxDBSize = 5 * 1024 * 1024
