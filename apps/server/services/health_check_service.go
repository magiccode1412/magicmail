// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package services

import (
	"log"
	"magicmail/crypto"
	"magicmail/models"

	"gorm.io/gorm"
)

// HealthCheckService 账号健康检查服务
// 用于定期扫描账号数据，检测并报告密码解密等潜在问题
type HealthCheckService struct {
	db *gorm.DB
}

// NewHealthCheckService 创建健康检查服务实例
func NewHealthCheckService(db *gorm.DB) *HealthCheckService {
	return &HealthCheckService{db: db}
}

// AccountHealthResult 单个账号的健康状态
type AccountHealthResult struct {
	ID       uint   `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Healthy  bool   `json:"healthy"`
	Issue    string `json:"issue,omitempty"`
	Severity string `json:"severity,omitempty"` // warning / error
}

// HealthCheckSummary 健康检查汇总结果
type HealthCheckSummary struct {
	Total      int                   `json:"total"`
	Healthy    int                   `json:"healthy"`
	Warning    int                   `json:"warning"`
	Error      int                   `json:"error"`
	Results    []AccountHealthResult `json:"results,omitempty"`
	CheckedAt  string                `json:"checked_at"`
}

// CheckAllAccounts 检查所有账号的健康状态
func (s *HealthCheckService) CheckAllAccounts(includeDetails bool) (*HealthCheckSummary, error) {
	var accounts []models.MailAccount

	if err := s.db.Find(&accounts).Error; err != nil {
		return nil, err
	}

	summary := &HealthCheckSummary{
		Total:   len(accounts),
		Results: make([]AccountHealthResult, 0, len(accounts)),
	}

	for _, acc := range accounts {
		result := s.checkSingleAccount(&acc)
		if result.Healthy {
			summary.Healthy++
		} else if result.Severity == "warning" {
			summary.Warning++
		} else {
			summary.Error++
		}

		if includeDetails || !result.Healthy {
			summary.Results = append(summary.Results, result)
		}
	}

	return summary, nil
}

// checkSingleAccount 检查单个账号的健康状态
func (s *HealthCheckService) checkSingleAccount(acc *models.MailAccount) AccountHealthResult {
	result := AccountHealthResult{
		ID:      acc.ID,
		Email:   acc.Email,
		Name:    acc.Name,
		Healthy: true,
	}

	// 检查 1：密码是否可以正常解密
	if acc.Password != "" && crypto.IsEncrypted(acc.Password) {
		if _, err := crypto.Decrypt(acc.Password); err != nil {
			result.Healthy = false
			result.Issue = "密码解密失败，可能是加密密钥变更或数据损坏"
			result.Severity = "error"
			log.Printf("[HEALTH] 账号 #%d (%s) 密码解密失败: %v", acc.ID, acc.Email, err)
			return result // 密码问题是最严重的，直接返回
		}
	} else if acc.Password == "" {
		result.Healthy = false
		result.Issue = "密码为空"
		result.Severity = "warning"
	} else if !crypto.IsEncrypted(acc.Password) {
		// 未加密的明文密码（可能需要迁移）
		result.Healthy = false
		result.Issue = "密码未加密（需要执行数据迁移）"
		result.Severity = "warning"
		log.Printf("[HEALTH] 账号 #%d (%s) 密码未加密", acc.ID, acc.Email)
	}

	// 检查 2：必要的配置字段是否完整
	if acc.ImapHost == "" || acc.Username == "" {
		result.Healthy = false
		if result.Issue == "" {
			result.Issue = "缺少必要配置（服务器地址或用户名）"
		}
		result.Severity = "error"
	}

	// 检查 3：长时间处于错误状态
	// TODO: 可根据业务需求添加更多检查项

	return result
}

// FixBrokenPassword 尝试修复损坏的密码记录（需要用户提供新密码）
func (s *HealthCheckService) FixBrokenPassword(id uint, newPassword string) error {
	var account models.MailAccount
	if err := s.db.First(&account, id).Error; err != nil {
		return err
	}

	// 加密新密码并更新
	encrypted, err := crypto.Encrypt(newPassword)
	if err != nil {
		return err
	}

	return s.db.Model(&account).Update("password", encrypted).Error
}

// GetUnhealthyAccounts 获取所有不健康的账号列表
func (s *HealthCheckService) GetUnhealthyAccounts() ([]AccountHealthResult, error) {
	var accounts []models.MailAccount
	if err := s.db.Find(&accounts).Error; err != nil {
		return nil, err
	}

	var unhealthy []AccountHealthResult
	for _, acc := range accounts {
		result := s.checkSingleAccount(&acc)
		if !result.Healthy {
			unhealthy = append(unhealthy, result)
		}
	}

	return unhealthy, nil
}
