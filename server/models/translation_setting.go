// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package models

import "time"

// UserTranslationSetting 用户级翻译设置（per-user MyMemory 凭据 + 目标语言）
//
// 设计要点（与需求一致）：
//   - email / key 均为「按用户」维度配置，各用户使用自己的 MyMemory 额度，互不挤占；
//   - key 以 AES-256-GCM 密文存储（mm_key_enc），明文字段永不入库、也绝不回传给前端；
//   - target_lang 让用户各自选择翻译后的目标语言。
type UserTranslationSetting struct {
	ID         uint      `gorm:"primaryKey" json:"-"`
	UserID     uint      `gorm:"uniqueIndex;not null;column:user_id" json:"-"`
	TargetLang string    `gorm:"size:16;not null;default:'zh-CN';column:target_lang" json:"-"`
	MMEmail    string    `gorm:"size:255;default:'';column:mm_email" json:"-"`  // MyMemory 邮箱（提额到 5000 词/天）
	MMKeyEnc   string    `gorm:"type:text;default:'';column:mm_key_enc" json:"-"` // 加密后的 MyMemory key（无限额度）
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`
}
