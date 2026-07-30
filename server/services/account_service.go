// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package services

import (
	"fmt"
	"magicmail/config"
	"magicmail/crypto"
	"magicmail/imap"
	"magicmail/models"

	"gorm.io/gorm"
)

// AccountService 邮箱账号业务逻辑
type AccountService struct {
	db     *gorm.DB
	config *config.Config
}

// NewAccountService 创建账号 Service 实例
func NewAccountService(db *gorm.DB, cfg *config.Config) *AccountService {
	return &AccountService{db: db, config: cfg}
}

// List 获取指定用户的邮箱账号列表（分页）
// 使用 AccountListDTO 避免 AfterFind 钩子触发密码解密，提升列表查询性能和容错性
func (s *AccountService) List(page, pageSize int, userID uint) ([]models.AccountResponse, int64, error) {
	var dtos []models.AccountListDTO
	var total int64

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	query := s.db.Model(&models.MailAccount{}).Where("user_id = ?", userID).Order("created_at DESC")
	query.Count(&total)

	// 使用 Select 明确指定字段 + Scan 绕过 MailAccount 的 AfterFind 钩子
	// password 字段仅用于判断是否为空（has_password），不进行解密
	selectFields := []string{
		"id", "name", "email", "protocol", "imap_host", "port",
		"smtp_host", "smtp_port", "username",
		"password", // 仅检查是否为空，不解密
		"proxy_enabled", "proxy_url", "sync_mode", "sync_days",
		"delete_on_server", "last_sync_at", "status", "error_msg",
		"created_at", "updated_at",
	}

	if err := query.Offset(offset).Limit(pageSize).
		Select(selectFields).
		Scan(&dtos).Error; err != nil { // Scan 不触发 GORM 钩子
		return nil, 0, err
	}

	responses := make([]models.AccountResponse, len(dtos))
	for i, dto := range dtos {
		responses[i] = s.dtoToResponse(dto)
	}

	// 批量统计每个邮箱账号的邮件总数与未读数，回填到列表响应。
	// 注意：列表走 AccountListDTO（不含计数），必须在查出账号后单独统计，
	// 否则邮箱管理页会始终显示 0/0（已同步却看不到数量）。
	if len(responses) > 0 {
		ids := make([]uint, len(responses))
		for i, r := range responses {
			ids[i] = r.ID
		}
		type acCount struct {
			AccountID uint
			Cnt       int64
		}
		var mailCounts, unreadCounts []acCount
		s.db.Model(&models.Mail{}).
			Select("account_id, count(*) as cnt").
			Where("account_id IN ?", ids).
			Group("account_id").
			Scan(&mailCounts)
		s.db.Model(&models.Mail{}).
			Select("account_id, count(*) as cnt").
			Where("account_id IN ?", ids).
			Where("is_read = ?", false).
			Group("account_id").
			Scan(&unreadCounts)

		mailMap := make(map[uint]int64, len(mailCounts))
		for _, m := range mailCounts {
			mailMap[m.AccountID] = m.Cnt
		}
		unreadMap := make(map[uint]int64, len(unreadCounts))
		for _, u := range unreadCounts {
			unreadMap[u.AccountID] = u.Cnt
		}
		for i := range responses {
			responses[i].MailCount = mailMap[responses[i].ID]
			responses[i].UnreadCount = unreadMap[responses[i].ID]
		}
	}

	return responses, total, nil
}

// GetByID 根据 ID 获取单个邮箱详情（校验归属用户）
func (s *AccountService) GetByID(id, userID uint) (*models.AccountResponse, error) {
	var account models.MailAccount
	if err := s.db.Where("user_id = ?", userID).First(&account, id).Error; err != nil {
		return nil, err
	}
	resp := s.toResponse(account)
	return &resp, nil
}

// Create 创建新邮箱账号（归属当前用户）
func (s *AccountService) Create(req models.AccountRequest, userID uint) (*models.AccountResponse, error) {
	account := models.MailAccount{
		Name:           req.Name,
		Email:          req.Email,
		Protocol:       req.Protocol,
		ImapHost:       req.Host,
		Port:           req.Port,
		SmtpHost:       req.SmtpHost,
		SmtpPort:       req.SmtpPort,
		Username:       req.Username,
		UserID:         userID,
		Password:       req.Password,
		// OAuth2 字段
		AuthType:        req.AuthType,
		OAuthProvider:    req.OAuthProvider,
		RefreshToken:    req.RefreshToken,
		CustomClientId:  req.CustomClientId,
		TokenExpiresAt:  req.TokenExpiresAt,
		// 同步与代理
		ProxyEnabled:   req.ProxyEnabled,
		ProxyURL:       req.ProxyURL,
		SyncMode:       req.SyncMode,
		SyncDays:       req.SyncDays,
		DeleteOnServer: req.DeleteOnServer,
		Status:         "active",
	}

	// 邮箱地址为可选项：未填写时复用登录用户名，保证唯一索引与展示、发信 From 均有值
	if account.Email == "" {
		account.Email = account.Username
	}

	if account.Port == 0 {
		account.Port = models.DefaultPort(account.Protocol)
	}

	// 同一用户不可重复绑定同一邮箱（不同用户可各自绑定同一邮箱，由复合唯一索引保证）
	var dup int64
	if err := s.db.Model(&models.MailAccount{}).
		Where("user_id = ? AND email = ?", userID, account.Email).
		Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, fmt.Errorf("该邮箱已被您绑定，请勿重复添加")
	}

	if err := s.db.Create(&account).Error; err != nil {
		return nil, err
	}

	resp := s.toResponse(account)

	// 启动该账号的同步 Worker
	go func() {
		if pool := imap.GlobalPool(); pool != nil {
			pool.RestartWorker(&account)
		}
	}()

	return &resp, nil
}

// Update 更新邮箱账号信息（校验归属用户）
func (s *AccountService) Update(id uint, req models.AccountRequest, userID uint) (*models.AccountResponse, error) {
	var account models.MailAccount
	if err := s.db.Where("user_id = ?", userID).First(&account, id).Error; err != nil {
		return nil, err
	}

	// 邮箱地址为可选项：未填写时复用登录用户名
	email := req.Email
	if email == "" {
		email = req.Username
	}

	updates := map[string]interface{}{
		"name":            req.Name,
		"email":           email,
		"protocol":        req.Protocol,
		"imap_host":       req.Host,
		"port":            req.Port,
		"smtp_host":       req.SmtpHost,
		"smtp_port":       req.SmtpPort,
		"username":        req.Username,
		"proxy_enabled":   req.ProxyEnabled,
		"proxy_url":       req.ProxyURL,
		"sync_mode":       req.SyncMode,
		"sync_days":       req.SyncDays,
		"delete_on_server": req.DeleteOnServer,
	}

	// 仅在提供了新密码时更新密码字段。
	// 注意：此处是 map 更新，GORM 的 BeforeUpdate 钩子对结构体字段的加密不会作用到 map，
	// 因此必须显式加密后再写入，否则密码会以明文入库，导致后续读取解密失败、密码被置空。
	if req.Password != "" {
		encrypted, err := crypto.Encrypt(req.Password)
		if err != nil {
			return nil, err
		}
		updates["password"] = encrypted
	}

	if err := s.db.Model(&account).Updates(updates).Error; err != nil {
		return nil, err
	}

	// 重新加载更新后的数据
	s.db.First(&account, id)
	resp := s.toResponse(account)

	// 重启该账号的 Worker 使配置生效
	if pool := imap.GlobalPool(); pool != nil {
		pool.RestartWorker(&account)
	}

	return &resp, nil
}

// Delete 删除邮箱账号及其所有邮件数据（校验归属用户）
func (s *AccountService) Delete(id, userID uint) error {
	// 先停掉该账号的 Worker
	if pool := imap.GlobalPool(); pool != nil {
		var acc models.MailAccount
		if s.db.Where("user_id = ?", userID).First(&acc, id).Error != nil {
			return gorm.ErrRecordNotFound
		}
		pool.StopWorker(id)
	}

	// 级联删除：先删附件，再删邮件，最后删账号
	tx := s.db.Begin()

	// 查找该账号下所有邮件 ID
	var mailIDs []uint
	tx.Model(&models.Mail{}).Where("account_id = ?", id).Pluck("id", &mailIDs)

	// 删除附件
	if len(mailIDs) > 0 {
		tx.Where("mail_id IN ?", mailIDs).Delete(&models.Attachment{})
	}

	// 删除邮件
	tx.Where("account_id = ?", id).Delete(&models.Mail{})

	// 删除账号
	result := tx.Delete(&models.MailAccount{}, id)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}

	return tx.Commit().Error
}

// TestConnection 测试连接是否可用
func (s *AccountService) TestConnection(req models.AccountRequest) error {
	account := &models.MailAccount{
		Name:     req.Name,
		Email:    req.Email,
		Protocol: req.Protocol,
		ImapHost: req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Status:   "active",
	}
	if account.Port == 0 {
		account.Port = models.DefaultPort(account.Protocol)
	}
	return imap.TestConnection(account, s.config)
}

// SetStatus 设置账号状态（停用/启用）（校验归属用户）
func (s *AccountService) SetStatus(id uint, status string, userID uint) error {
	var account models.MailAccount
	if err := s.db.Where("user_id = ?", userID).First(&account, id).Error; err != nil {
		return err
	}

	// 验证状态值
	if status != "active" && status != "disabled" && status != "error" {
		return fmt.Errorf("无效的状态值: %s", status)
	}

	// 停用时停止 Worker，启用时重启 Worker
	pool := imap.GlobalPool()
	if status == "disabled" && pool != nil {
		pool.StopWorker(id)
	}

	if err := s.db.Model(&account).Update("status", status).Error; err != nil {
		return err
	}

	// 启用时重启 Worker
	if status == "active" && pool != nil {
		s.db.First(&account, id) // 刷新最新数据
		go pool.RestartWorker(&account)
	}

	return nil
}

// TriggerSync 手动触发指定账号的同步（校验归属用户）
func (s *AccountService) TriggerSync(id, userID uint) error {
	pool := imap.GlobalPool()
	if pool == nil {
		return ErrWorkerNotRunning
	}

	var account models.MailAccount
	if err := s.db.Where("user_id = ?", userID).First(&account, id).Error; err != nil {
		return err
	}

	go pool.RestartWorker(&account)
	return nil
}

// dtoToResponse 将 AccountListDTO 转换为 API 响应格式
func (s *AccountService) dtoToResponse(dto models.AccountListDTO) models.AccountResponse {
	return models.AccountResponse{
		ID:             dto.ID,
		Name:           dto.Name,
		Email:          dto.Email,
		Protocol:       dto.Protocol,
		ImapHost:       dto.ImapHost,
		Port:           dto.Port,
		Mode:           imap.GlobalWorkerMode(dto.ID),
		SmtpHost:       dto.SmtpHost,
		SmtpPort:       dto.SmtpPort,
		Username:       dto.Username,
		HasPassword:    dto.HasPassword(),
		ProxyEnabled:   dto.ProxyEnabled,
		ProxyURL:       dto.ProxyURL,
		SyncMode:       dto.SyncMode,
		SyncDays:       dto.SyncDays,
		DeleteOnServer: dto.DeleteOnServer,
		LastSyncAt:     dto.LastSyncAt,
		Status:         dto.Status,
		ErrorMsg:       dto.ErrorMsg,
		CreatedAt:      dto.CreatedAt,
		UpdatedAt:      dto.UpdatedAt,
	}
}

// toResponse 将模型转换为 API 响应格式（脱敏）
func (s *AccountService) toResponse(account models.MailAccount) models.AccountResponse {
	var mailCount, unreadCount int64
	s.db.Model(&models.Mail{}).Where("account_id = ?", account.ID).Count(&mailCount)
	s.db.Model(&models.Mail{}).Where("account_id = ? AND is_read = false", account.ID).Count(&unreadCount)

	return models.AccountResponse{
		ID:           account.ID,
		Name:         account.Name,
		Email:        account.Email,
		Protocol:     account.Protocol,
		ImapHost:     account.ImapHost,
		Port:         account.Port,
		Mode:         imap.GlobalWorkerMode(account.ID),
		SmtpHost:     account.SmtpHost,
		SmtpPort:     account.SmtpPort,
		Username:     account.Username,
		HasPassword:  account.Password != "",
		ProxyEnabled: account.ProxyEnabled,
		ProxyURL:     account.ProxyURL,
		SyncMode:     account.SyncMode,
		SyncDays:     account.SyncDays,
		DeleteOnServer: account.DeleteOnServer,
		LastSyncAt:   account.LastSyncAt,
		Status:       account.Status,
		ErrorMsg:     account.ErrorMsg,
		MailCount:    mailCount,
		UnreadCount:  unreadCount,
		CreatedAt:    account.CreatedAt,
		UpdatedAt:    account.UpdatedAt,
	}
}

// 错误定义
var ErrWorkerNotRunning = fmt.Errorf("worker pool 未运行")
