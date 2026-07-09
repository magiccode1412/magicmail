// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package models

import (
	"crypto/rand"
	"encoding/hex"
)

// AppConfig 系统级安全配置（单行记录，存储于数据库）
type AppConfig struct {
	ID              uint   `gorm:"primaryKey"`
	JWTSecret       string `gorm:"type:text;not null"`       // JWT 签名密钥（随机生成）
	EncryptionKey   string `gorm:"type:text;not null"`        // 邮箱密码加密密钥（随机生成）
	VAPIDPublicKey  string `gorm:"type:text;default:''"`     // Web Push VAPID 公钥 (base64url)
	VAPIDPrivateKey string `gorm:"type:text;default:''"`     // Web Push VAPID 私钥 (base64url DER)
	OpenRegistration bool  `gorm:"default:false;comment:是否开放公开注册(默认关闭)"` // 开放注册开关（仅管理员可见）
}

// GenerateRandomKey 生成指定长度的随机 hex 字符串作为密钥
func GenerateRandomKey(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// TableName 指定表名
func (AppConfig) TableName() string {
	return "app_configs"
}
