// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package models

import (
	"log"
	"strings"
	"time"

	"magicmail/crypto"

	"gorm.io/gorm"
)

// MailAccount 邮箱账号模型 - 存储用户配置的 IMAP/POP3/SMTP 邮箱连接信息
//
// Password / RefreshToken / CustomClientId 字段在数据库中以 AES-256-GCM 密文存储（ENC: 前缀标识），
// 通过 GORM 钩子实现写入时自动加密、读取时自动解密，业务层无需关心。
type MailAccount struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	Name         string     `json:"name" gorm:"type:varchar(100);not null;comment:显示名称"`
	Email        string     `json:"email" gorm:"type:varchar(255);not null;index:idx_user_email,unique;comment:邮箱地址(同一用户不可重复,不同用户可各自绑定)"`
	Protocol     string     `json:"protocol" gorm:"type:varchar(10);not null;default:'imap';comment:邮件协议(imap/pop3)"`
	ImapHost     string     `json:"host" gorm:"type:varchar(255);not null;comment:收信服务器地址"`
	Port         int        `json:"port" gorm:"default:993;comment:收信端口"`
	SmtpHost     string     `json:"smtp_host" gorm:"type:varchar(255);comment:SMTP发信服务器地址(为空则使用收信地址)"`
	SmtpPort     int        `json:"smtp_port" gorm:"comment:SMTP端口(默认587)"`
	Username     string     `json:"username" gorm:"type:varchar(255);not null;comment:登录用户名"`
	UserID       uint       `json:"user_id" gorm:"index:idx_user_email,unique;not null;default:0;comment:所属用户ID"`
	Password     string     `json:"-" gorm:"type:text;not null;comment:密码(AES-256-GCM加密存储)"`

	// --- OAuth2 字段（可选，为空时走传统密码认证）---
	AuthType        string     `json:"auth_type" gorm:"type:varchar(20);default:'';comment:认证类型(password/oauth2_microsoft/oauth2_google)"`
	OAuthProvider    string     `json:"oauth_provider" gorm:"type:varchar(30);default:'';comment:OAuth2服务商(microsoft/google)"`
	RefreshToken    string     `json:"-" gorm:"type:text;comment:OAuth2 Refresh Token(AES-256-GCM加密存储)"`
	CustomClientId  string     `json:"-" gorm:"type:text;comment:用户自定义OAuth2 Client ID(AES-256-GCM加密存储,为空则使用默认值)"`
	TokenExpiresAt  *time.Time `json:"token_expires_at" gorm:"comment:Access Token 过期时间"`

	// --- 同步与代理 ---
	ProxyEnabled    bool       `json:"proxy_enabled" gorm:"default:false;comment:是否启用HTTP代理"`
	ProxyURL        string     `json:"proxy_url,omitempty" gorm:"type:varchar(512);comment:HTTP代理地址(http://user:pass@host:port)"`
	SyncMode        string     `json:"sync_mode" gorm:"type:varchar(20);default:'unread';comment:同步模式(unread/all/recent)"`
	SyncDays        int        `json:"sync_days" gorm:"default:30;comment:同步最近天数(仅sync_mode=recent时有效)"`
	DeleteOnServer  bool       `json:"delete_on_server" gorm:"default:false;comment:删除时是否同步删除源服务器上的邮件"`
	LastSyncAt      *time.Time `json:"last_sync_at" gorm:"comment:最后同步时间"`
	Status          string     `json:"status" gorm:"type:varchar(20);default:'active';index;comment:状态(active/error/disabled)"`
	ErrorMsg        string     `json:"error_msg,omitempty" gorm:"type:text;comment:错误信息"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// 关联
	Mails []Mail `json:"mails,omitempty" gorm:"foreignKey:AccountID"`
}

// TableName 指定表名
func (MailAccount) TableName() string {
	return "mail_accounts"
}

// BeforeCreate 创建前钩子：设置默认值 + 加密敏感字段
func (a *MailAccount) BeforeCreate(tx *gorm.DB) error {
	a.setDefaultValues()
	if err := a.encryptSensitiveFields(); err != nil {
		return err
	}
	return nil
}

// BeforeUpdate 更新前钩子：加密敏感字段（仅当字段被更新时）
func (a *MailAccount) BeforeUpdate(tx *gorm.DB) error {
	return a.encryptSensitiveFields()
}

// AfterFind 查询后钩子：自动解密所有敏感字段供运行时使用
func (a *MailAccount) AfterFind(tx *gorm.DB) error {
	// Password
	if a.Password != "" {
		decrypted, err := crypto.Decrypt(a.Password)
		if err != nil {
			log.Printf("[WARN] MailAccount#%d 密码解密失败: %v", a.ID, err)
			a.Password = ""
		} else {
			a.Password = decrypted
		}
	}
	// RefreshToken (OAuth2)
	if a.RefreshToken != "" {
		decrypted, err := crypto.Decrypt(a.RefreshToken)
		if err != nil {
			log.Printf("[WARN] MailAccount#%d RefreshToken 解密失败: %v", a.ID, err)
			a.RefreshToken = ""
		} else {
			a.RefreshToken = decrypted
		}
	}
	// CustomClientId (OAuth2)
	if a.CustomClientId != "" {
		decrypted, err := crypto.Decrypt(a.CustomClientId)
		if err != nil {
			log.Printf("[WARN] MailAccount#%d CustomClientId 解密失败: %v", a.ID, err)
			a.CustomClientId = ""
		} else {
			a.CustomClientId = decrypted
		}
	}
	return nil
}

// setDefaultValues 设置默认值
func (a *MailAccount) setDefaultValues() {
	if a.Status == "" {
		a.Status = "active"
	}
	if a.Protocol == "" {
		a.Protocol = "imap"
	}
	if a.AuthType == "" {
		a.AuthType = "password" // 默认使用密码认证
	}
	if a.Port == 0 {
		a.Port = DefaultPort(a.Protocol)
	}
	if a.SmtpPort == 0 {
		a.SmtpPort = DefaultSmtpPort(a.ImapHost)
	}
	if a.SmtpHost == "" && a.ImapHost != "" {
		a.SmtpHost = inferSMTPHost(a.ImapHost)
	}
	if a.SyncMode == "" {
		a.SyncMode = "unread"
	}
	if a.SyncDays == 0 {
		a.SyncDays = 30
	}
}

// encryptSensitiveFields 加密所有敏感字段（Password / RefreshToken / CustomClientId）
func (a *MailAccount) encryptSensitiveFields() error {
	// Password
	if a.Password != "" && !crypto.IsEncrypted(a.Password) {
		encrypted, err := crypto.Encrypt(a.Password)
		if err != nil {
			return err
		}
		a.Password = encrypted
	}
	// RefreshToken (OAuth2)
	if a.RefreshToken != "" && !crypto.IsEncrypted(a.RefreshToken) {
		encrypted, err := crypto.Encrypt(a.RefreshToken)
		if err != nil {
			return err
		}
		a.RefreshToken = encrypted
	}
	// CustomClientId (OAuth2)
	if a.CustomClientId != "" && !crypto.IsEncrypted(a.CustomClientId) {
		encrypted, err := crypto.Encrypt(a.CustomClientId)
		if err != nil {
			return err
		}
		a.CustomClientId = encrypted
	}
	return nil
}

// DefaultPort 根据协议返回默认端口
func DefaultPort(protocol string) int {
	switch protocol {
	case "pop3":
		return 995 // POP3S (SSL/TLS)
	case "pop3-no-ssl":
		return 110
	default:
		return 993 // IMAPS (SSL/TLS)
	}
}

// smtpHostMap 收信服务器到SMTP服务器的映射（常见邮箱服务商）
var smtpHostMap = map[string]string{
	"imap.163.com":         "smtp.163.com",
	"imap.126.com":         "smtp.126.com",
	"imap.qq.com":          "smtp.qq.com",
	"imap.sina.com":        "smtp.sina.com",
	"imap.aliyun.com":      "smtp.aliyun.com",
	"imap.gmail.com":       "smtp.gmail.com",
	"imap.mail.yahoo.com":  "smtp.mail.yahoo.com",
	"outlook.office365.com": "smtp.office365.com",
}

// smtpPortMap SMTP服务器到默认端口的映射
var smtpPortMap = map[string]int{
	"smtp.163.com":           465, // 163邮箱使用SSL/TLS
	"smtp.126.com":           465,
	"smtp.qq.com":            465,
	"smtp.sina.com":          465,
	"smtp.aliyun.com":        465,
	"smtp.mail.yahoo.com":    465,
	"smtp.office365.com":     587, // Outlook使用STARTTLS
	"smtp.gmail.com":         587, // Gmail使用STARTTLS
}

// DefaultSmtpPort 根据收信服务器或SMTP服务器地址推断默认SMTP端口
func DefaultSmtpPort(imapHost string) int {
	// 先检查是否有直接的SMTP端口映射
	if port, ok := smtpPortMap[imapHost]; ok {
		return port
	}
	// 检查是否在已知SMTP主机映射中
	if smtpHost, ok := smtpHostMap[imapHost]; ok {
		if port, ok := smtpPortMap[smtpHost]; ok {
			return port
		}
	}
	return 587 // 默认使用STARTTLS端口
}

// inferSMTPHost 根据收信服务器地址推断SMTP服务器地址
func inferSMTPHost(imapHost string) string {
	if smtpHost, ok := smtpHostMap[imapHost]; ok {
		return smtpHost
	}
	// 通用规则：将 imap 替换为 smtp
	return strings.Replace(imapHost, "imap.", "smtp.", 1)
}
type AccountRequest struct {
	Name           string `json:"name" validate:"required,min=1,max=100"`
	Email          string `json:"email" validate:"required,email"`
	Protocol       string `json:"protocol" validate:"omitempty,oneof=imap pop3 pop3-no-ssl"`
	Host           string `json:"host" validate:"required"`
	Port           int    `json:"port" validate:"min=1,max=65535"`
	SmtpHost       string `json:"smtp_host"`
	SmtpPort       int    `json:"smtp_port" validate:"omitempty,min=1,max=65535"`
	Username       string `json:"username" validate:"required"`
	Password       string `json:"password"` // 为空时不更新密码
	// OAuth2 字段（OAuth2 认证时使用）
	AuthType        string     `json:"auth_type" validate:"omitempty,oneof=password oauth2_microsoft oauth2_google"` // 认证类型
	OAuthProvider    string     `json:"oauth_provider" validate:"omitempty,oneof=microsoft google"`                // OAuth2 服务商
	RefreshToken    string     `json:"refresh_token"`   // OAuth2 Refresh Token（授权完成后由后端设置）
	CustomClientId  string     `json:"custom_client_id"` // 用户自定义 Client ID（可选）
	TokenExpiresAt  *time.Time `json:"token_expires_at"` // Access Token 过期时间
	ProxyEnabled   bool   `json:"proxy_enabled"`    // 是否启用HTTP代理
	ProxyURL       string `json:"proxy_url,omitempty"` // HTTP代理地址
	SyncMode       string `json:"sync_mode" validate:"omitempty,oneof=unread all recent"` // 同步模式
	SyncDays       int    `json:"sync_days" validate:"omitempty,min=1,max=365"`           // 最近天数
	DeleteOnServer bool  `json:"delete_on_server"`                                      // 删除时同步到源服务器
}

// AccountResponse 邮箱 API 响应（脱敏）
type AccountResponse struct {
	ID             uint        `json:"id"`
	Name           string      `json:"name"`
	Email          string      `json:"email"`
	Protocol       string      `json:"protocol"`
	ImapHost       string      `json:"host"`
	Port           int         `json:"port"`
	SmtpHost       string      `json:"smtp_host,omitempty"`
	SmtpPort       int         `json:"smtp_port,omitempty"`
	Username       string      `json:"username"`
	HasPassword    bool        `json:"has_password"`     // 是否已设置密码/凭证
	AuthType       string      `json:"auth_type"`         // 认证类型
	OAuthProvider   string      `json:"oauth_provider"`    // OAuth2 服务商（前端用于显示标签）
	TokenExpiresAt *time.Time  `json:"token_expires_at"`  // Token 过期时间（前端判断是否需要重新授权）
	ProxyEnabled   bool        `json:"proxy_enabled"`     // 是否启用HTTP代理
	ProxyURL       string      `json:"proxy_url,omitempty"` // HTTP代理地址
	SyncMode       string      `json:"sync_mode"`            // 同步模式
	SyncDays       int         `json:"sync_days"`            // 最近天数
	DeleteOnServer bool        `json:"delete_on_server"`     // 删除时同步到源服务器
	LastSyncAt     *time.Time  `json:"last_sync_at"`
	Status         string      `json:"status"`
	ErrorMsg       string      `json:"error_msg,omitempty"`
	MailCount      int64       `json:"mail_count"`
	UnreadCount    int64       `json:"unread_count"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// AccountListDTO 列表查询专用 DTO（不含敏感字段，避免触发 AfterFind 解密）
// 用于列表场景：只需展示基本信息，不需要密码/Token 明文
type AccountListDTO struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `json:"name"`
	Email          string     `json:"email"`
	Protocol       string     `json:"protocol"`
	ImapHost       string     `gorm:"column:imap_host" json:"host"`
	Port           int        `json:"port"`
	SmtpHost       string     `gorm:"column:smtp_host" json:"smtp_host,omitempty"`
	SmtpPort       int        `gorm:"column:smtp_port" json:"smtp_port,omitempty"`
	Username       string     `json:"username"`
	UserID         uint       `gorm:"column:user_id" json:"user_id"`
	PasswordRaw    string     `gorm:"column:password" json:"-"`
	AuthType       string     `gorm:"column:auth_type" json:"auth_type"`
	OAuthProvider   string    `gorm:"column:oauth_provider" json:"oauth_provider"`
	TokenExpiresAt *time.Time `gorm:"column:token_expires_at" json:"token_expires_at"`
	ProxyEnabled   bool       `gorm:"column:proxy_enabled" json:"proxy_enabled"`
	ProxyURL       string     `gorm:"column:proxy_url" json:"proxy_url,omitempty"`
	SyncMode       string     `gorm:"column:sync_mode" json:"sync_mode"`
	SyncDays       int        `gorm:"column:sync_days" json:"sync_days"`
	DeleteOnServer bool        `gorm:"column:delete_on_server" json:"delete_on_server"`
	LastSyncAt     *time.Time  `gorm:"column:last_sync_at" json:"last_sync_at"`
	Status         string      `gorm:"column:status" json:"status"`
	ErrorMsg       string      `gorm:"column:error_msg" json:"error_msg,omitempty"`
	CreatedAt      time.Time   `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time   `gorm:"column:updated_at" json:"updated_at"`
}

// HasPassword 判断是否设置了密码（PasswordRaw 非空即有密码）
func (dto *AccountListDTO) HasPassword() bool {
	return dto.PasswordRaw != ""
}

// TableName 指定 DTO 的表名（与 MailAccount 共享同一张表）
func (AccountListDTO) TableName() string {
	return "mail_accounts"
}

// DefaultPort 根据协议返回默认端口
