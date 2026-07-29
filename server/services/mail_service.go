// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package services

import (
	"fmt"
	"log"
	"strings"

	"magicmail/config"
	"magicmail/imap"
	"magicmail/models"
	pop3pkg "magicmail/pop3"
	"magicmail/smtp"

	"gorm.io/gorm"
)

// MailService 邮件业务逻辑
type MailService struct {
	db     *gorm.DB
	config *config.Config
}

// NewMailService 创建邮件 Service 实例
func NewMailService(db *gorm.DB, cfg *config.Config) *MailService {
	return &MailService{db: db, config: cfg}
}

// GetDB 暴露底层 *gorm.DB（供 handler 层触发 webhook/SSE 等使用）
func (s *MailService) GetDB() *gorm.DB {
	return s.db
}

// List 获取邮件列表（分页、搜索、筛选），按所属用户隔离
func (s *MailService) List(filter models.MailListFilter, userID uint) ([]models.MailListItem, int64, error) {
	var mails []models.Mail
	var total int64

	// 默认参数
	if filter.Page < 1 {
		filter.Page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := s.db.Model(&models.Mail{}).
		Joins("LEFT JOIN mail_accounts ON mails.account_id = mail_accounts.id").
		Where("mails.user_id = ?", userID)

	// 筛选：指定邮箱账号
	if filter.AccountID > 0 {
		query = query.Where("mails.account_id = ?", filter.AccountID)
	}

	// 筛选：文件夹（inbox/sent）
	if filter.Folder != "" {
		query = query.Where("mails.folder = ?", filter.Folder)
	}

	// 筛选：已读/未读
	if filter.IsRead != nil {
		query = query.Where("mails.is_read = ?", *filter.IsRead)
	}

	// 筛选：有/无附件
	if filter.HasAttachment != nil {
		query = query.Where("mails.has_attachment = ?", *filter.HasAttachment)
	}

	// 搜索：发件人或主题关键词
	if filter.Keyword != "" {
		keyword := "%" + filter.Keyword + "%"
		query = query.Where("mails.from LIKE ? OR mails.subject LIKE ?", keyword, keyword)
	}

	// 统计总数
	countQuery := s.db.Model(&models.Mail{}).Where("user_id = ?", userID)
	if filter.AccountID > 0 {
		countQuery = countQuery.Where("account_id = ?", filter.AccountID)
	}
	if filter.Folder != "" {
		countQuery = countQuery.Where("folder = ?", filter.Folder)
	}
	if filter.IsRead != nil {
		countQuery = countQuery.Where("is_read = ?", *filter.IsRead)
	}
	if filter.HasAttachment != nil {
		countQuery = countQuery.Where("has_attachment = ?", *filter.HasAttachment)
	}
	if filter.Keyword != "" {
		keyword := "%" + filter.Keyword + "%"
		countQuery = countQuery.Where("`from` LIKE ? OR subject LIKE ?", keyword, keyword)
	}
	countQuery.Count(&total)

	// 排序
	sortBy := "sent_at"
	if filter.SortBy != "" {
		sortBy = strings.ToLower(filter.SortBy)
	}
	switch sortBy {
	case "created_at":
		sortBy = "mails.created_at"
	case "from", "subject":
		sortBy = "mails." + sortBy
	default:
		sortBy = "mails.sent_at"
	}

 sortOrder := "DESC"
	if strings.ToUpper(filter.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}

	// 分页查询（只取必要字段，不含正文）
	offset := (filter.Page - 1) * pageSize
	fields := "mails.id, mails.account_id, mail_accounts.name AS account_name," +
		" mails.folder, mails.`from`, mails.`to`, mails.subject, mails.text_body," +
		" mails.sent_at, mails.is_read, mails.is_starred, mails.has_attachment," +
		" mails.size, mails.created_at"

	err := query.Select(fields).
		Offset(offset).Limit(pageSize).
		Order(sortBy + " " + sortOrder).
		Scan(&mails).Error

	if err != nil {
		return nil, 0, err
	}

	// 构建预览文本
	items := make([]models.MailListItem, len(mails))
	for i, m := range mails {
		item := models.MailListItem{
			ID:            m.ID,
			AccountID:     m.AccountID,
			Folder:        m.Folder,
			From:          m.From,
			To:            m.To,
			Subject:       m.Subject,
			SentAt:        m.SentAt,
			IsRead:        m.IsRead,
			IsStarred:     m.IsStarred,
			HasAttachment: m.HasAttachment,
			Size:          m.Size,
			CreatedAt:     m.CreatedAt,
		}
		// 预览文本（优先纯文本，其次 HTML 去标签）
		if m.TextBody.Valid {
			item.Preview = truncatePreview(m.TextBody.String, 150)
		} else if m.HTMLBody.Valid {
			item.Preview = stripHTML(m.HTMLBody.String)
			item.Preview = truncatePreview(item.Preview, 150)
		}
		items[i] = item
	}

	return items, total, nil
}

// GetByID 获取邮件详情（含正文和附件），校验归属用户
func (s *MailService) GetByID(id, userID uint) (*models.MailResponse, error) {
	var mail models.Mail

	if err := s.db.Preload("Attachments").Preload("Account").Where("user_id = ?", userID).First(&mail, id).Error; err != nil {
		return nil, err
	}

	resp := models.MailResponse{
		ID:            mail.ID,
		AccountID:     mail.AccountID,
		Folder:        mail.Folder,
		MessageID:     mail.MessageID,
		From:         mail.From,
		To:           mail.To,
		Cc:           mail.Cc,
		Subject:       mail.Subject,
		SentAt:        mail.SentAt,
		IsRead:        mail.IsRead,
		IsStarred:     mail.IsStarred,
		HasAttachment: mail.HasAttachment,
		Size:          mail.Size,
		CreatedAt:     mail.CreatedAt,
	}

	if mail.Account != nil {
		resp.AccountName = mail.Account.Name
	}

	if mail.TextBody.Valid {
		resp.TextBody = &mail.TextBody.String
	}
	if mail.HTMLBody.Valid {
		resp.HTMLBody = &mail.HTMLBody.String
	}

	// ⭐ 调试日志：记录邮件详情获取情况
	log.Printf("🔍 [DEBUG] GetByID (id=%d): text_valid=%v, text_len=%d, html_valid=%v, html_len=%d, attachments=%d, has_attachment=%v",
		id, mail.TextBody.Valid, len(mail.TextBody.String), mail.HTMLBody.Valid, len(mail.HTMLBody.String),
		len(mail.Attachments), mail.HasAttachment)

	// 转换附件响应
	resp.Attachments = make([]models.AttachmentResp, len(mail.Attachments))
	for i, att := range mail.Attachments {
		resp.Attachments[i] = models.AttachmentResp{
			ID:          att.ID,
			MailID:      att.MailID,
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Size:        att.Size,
			SizeHuman:   formatSize(att.Size),
			IsCached:    att.IsCached,
			CreatedAt:   att.CreatedAt,
		}
	}

	return &resp, nil
}

// MarkAsRead 标记邮件为已读/未读（校验归属用户）
func (s *MailService) MarkAsRead(id uint, isRead bool, userID uint) error {
	return s.db.Model(&models.Mail{}).Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", isRead).Error
}

// GetMailAccountID 获取指定邮件所属账号 ID，用于 SSE 事件携带账号维度（按用户隔离）
func (s *MailService) GetMailAccountID(id, userID uint) (uint, error) {
	var accountID uint
	err := s.db.Model(&models.Mail{}).
		Where("id = ? AND user_id = ?", id, userID).
		Select("account_id").Scan(&accountID).Error
	return accountID, err
}

// BatchMarkAsReadResult 批量标记已读/未读操作结果
type BatchMarkAsReadResult struct {
	Success bool   `json:"success"`
	Updated int64  `json:"updated"`
	Failed  []uint `json:"failed"`
}

// BatchMarkAsRead 批量标记邮件已读/未读状态（仅当前用户数据）
func (s *MailService) BatchMarkAsRead(ids []uint, isRead bool, userID uint) *BatchMarkAsReadResult {
	result := &BatchMarkAsReadResult{}

	// 批量更新数据库（使用 IN 查询，一次 SQL 完成）
	dbResult := s.db.Model(&models.Mail{}).Where("id IN ? AND user_id = ?", ids, userID).
		Update("is_read", isRead)

	if dbResult.Error != nil {
		result.Failed = ids
		return result
	}

	result.Updated = dbResult.RowsAffected
	result.Success = true
	return result
}

// MarkAllAsRead 将符合筛选条件的所有邮件标记为已读（仅当前用户数据）
func (s *MailService) MarkAllAsRead(filter models.MailListFilter, userID uint) (int64, error) {
	query := s.db.Model(&models.Mail{}).Where("user_id = ?", userID)

	if filter.AccountID > 0 {
		query = query.Where("account_id = ?", filter.AccountID)
	}
	if filter.Folder != "" {
		query = query.Where("folder = ?", filter.Folder)
	}
	if filter.IsRead != nil {
		query = query.Where("is_read = ?", *filter.IsRead)
	}
	if filter.HasAttachment != nil {
		query = query.Where("has_attachment = ?", *filter.HasAttachment)
	}
	if filter.Keyword != "" {
		keyword := "%" + filter.Keyword + "%"
		query = query.Where("`from` LIKE ? OR subject LIKE ?", keyword, keyword)
	}

	result := query.Update("is_read", true)
	return result.RowsAffected, result.Error
}

// MarkAsStarred 标记邮件星标（校验归属用户）
func (s *MailService) MarkAsStarred(id uint, starred bool, userID uint) error {
	return s.db.Model(&models.Mail{}).Where("id = ? AND user_id = ?", id, userID).
		Update("is_starred", starred).Error
}

// DeleteResult 删除操作结果
type DeleteResult struct {
	Success        bool   `json:"success"`
	DeletedFromServer bool `json:"deleted_from_server"` // 是否成功从源服务器删除
	ServerDeleteError string `json:"server_delete_error,omitempty"` // 源服务器删除错误信息（如有）
}

// Delete 删除邮件及其附件，可选同步删除源服务器上的邮件（校验归属用户）
func (s *MailService) Delete(id, userID uint) *DeleteResult {
	result := &DeleteResult{Success: false}

	// 先获取邮件信息（需要 account_id 和 message_uid）
	var mail models.Mail
	if err := s.db.Where("user_id = ?", userID).First(&mail, id).Error; err != nil {
		return result // 邮件不存在
	}

	// 查询账号配置
	var account models.MailAccount
	if err := s.db.First(&account, mail.AccountID).Error; err != nil {
		return result // 邮箱账号不存在
	}

	// 如果开启了源服务器删除，先删除远程邮件
	if account.DeleteOnServer && s.config != nil {
		if err := s.deleteFromServer(&account, &mail); err != nil {
			log.Printf("[WARN] 源服务器删除失败 (mail_id=%d): %v", id, err)
			result.ServerDeleteError = err.Error()
			// 远程删除失败不阻止本地删除，仅记录错误
		} else {
			result.DeletedFromServer = true
		}
	}

	tx := s.db.Begin()

	// 删除附件
	tx.Where("mail_id = ?", id).Delete(&models.Attachment{})

	// 删除邮件
	if dbErr := tx.Delete(&models.Mail{}, id).Error; dbErr != nil {
		tx.Rollback()
		return result
	}

	if commitErr := tx.Commit().Error; commitErr != nil {
		return result
	}

	result.Success = true
	return result
}

// BatchDeleteResult 批量删除操作结果
type BatchDeleteResult struct {
	Success           bool   `json:"success"`
	Deleted           int64  `json:"deleted"`
	Failed            []uint `json:"failed"`
	ServerSyncResult  map[uint]*DeleteResult `json:"server_sync_result,omitempty"` // 每封邮件的同步删除状态
}

// BatchDelete 批量删除邮件（仅当前用户数据）
func (s *MailService) BatchDelete(ids []uint, userID uint) *BatchDeleteResult {
	result := &BatchDeleteResult{
		ServerSyncResult: make(map[uint]*DeleteResult),
	}

	for _, id := range ids {
		delResult := s.Delete(id, userID)
		result.ServerSyncResult[id] = delResult

		if delResult.Success {
			result.Deleted++
		} else {
			result.Failed = append(result.Failed, id)
			log.Printf("[MailService] 批量删除失败 (id=%d)", id)
		}
	}

	result.Success = len(result.Failed) == 0
	return result
}

// deleteFromServer 在源服务器上删除对应邮件
func (s *MailService) deleteFromServer(account *models.MailAccount, mail *models.Mail) error {
	client, err := imap.NewMailClient(account, s.config)
	if err != nil {
		return fmt.Errorf("创建客户端失败: %w", err)
	}
	defer client.Close()

	if err := client.Authenticate(); err != nil {
		return fmt.Errorf("认证失败: %w", err)
	}

	switch account.Protocol {
	case "pop3", "pop3-no-ssl":
		pc, ok := client.(*pop3pkg.POP3Client)
		if !ok {
			return fmt.Errorf("类型断言失败")
		}
		// POP3 使用序号（message_uid 存储的即 POP3 序号）
		return pc.DeleteMessage(int(mail.MessageUID))
	default:
		ic, ok := client.(*imap.IMAPClient)
		if !ok {
			return fmt.Errorf("类型断言失败")
		}
		return ic.DeleteMessage(mail.MessageUID)
	}
}

// GetStats 获取邮件统计信息（仅当前用户数据）
func (s *MailService) GetStats(accountID *uint, userID uint) map[string]int64 {
	stats := make(map[string]int64)

	query := s.db.Model(&models.Mail{}).Where("user_id = ?", userID)
	if accountID != nil {
		query = query.Where("account_id = ?", *accountID)
	}

	var total, unread, withAttachment, starred, sent int64

	// 总数
	query.Count(&total)
	stats["total"] = total

	// 已发送
	sentQuery := s.db.Model(&models.Mail{}).Where("user_id = ?", userID)
	if accountID != nil {
		sentQuery = sentQuery.Where("account_id = ?", *accountID)
	}
	sentQuery.Where("folder = 'sent'").Count(&sent)
	stats["sent"] = sent

	// 未读数
	unreadQuery := s.db.Model(&models.Mail{}).Where("user_id = ?", userID)
	if accountID != nil {
		unreadQuery = unreadQuery.Where("account_id = ?", *accountID)
	}
	unreadQuery.Where("is_read = false").Count(&unread)
	stats["unread"] = unread

	// 有附件
	q := s.db.Model(&models.Mail{}).Where("user_id = ?", userID)
	if accountID != nil {
		q = q.Where("account_id = ?", *accountID)
	}
	q.Where("has_attachment = true").Count(&withAttachment)
	stats["with_attachment"] = withAttachment

	// 已标星
	q2 := s.db.Model(&models.Mail{}).Where("user_id = ?", userID)
	if accountID != nil {
		q2 = q2.Where("account_id = ?", *accountID)
	}
	q2.Where("is_starred = true").Count(&starred)
	stats["starred"] = starred

	return stats
}

// truncatePreview 截取预览文本
func truncatePreview(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) <= maxLen {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxLen]) + "..."
}

// formatSize 格式化文件大小为人类可读格式
func formatSize(size int64) string {
	if size < 0 {
		return "未知"
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// stripHTML 移除 HTML 标签，返回纯文本
// 会完整移除 <style> 和 <script> 标签及其内部内容，避免 CSS/JS 代码泄漏
func stripHTML(html string) string {
	s := html

	// 1. 先移除 <style>...</style> 和 <script>...</script> 块（大小写不敏感）
	for _, tag := range []string{"style", "script"} {
		for {
			lower := strings.ToLower(s)
			startTag := "<" + tag
			endTag := "</" + tag + ">"

			startIdx := strings.Index(lower, startTag)
			if startIdx == -1 {
				break
			}
			startClose := strings.Index(s[startIdx:], ">")
			if startClose == -1 {
				break
			}
			contentStart := startIdx + startClose + 1
			endIdx := strings.Index(strings.ToLower(s[contentStart:]), endTag)
			if endIdx == -1 {
				s = s[:startIdx]
				break
			}
			s = s[:startIdx] + s[contentStart+endIdx+len(endTag):]
		}
	}

	// 2. 移除剩余 HTML 标签
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// 3. 清理多余空白
	raw := result.String()
	var cleaned strings.Builder
	prevSpace := false
	for _, r := range raw {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				cleaned.WriteRune(' ')
				prevSpace = true
			}
		} else {
			cleaned.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(cleaned.String())
}

// SendMail 发送邮件（校验账号归属用户）
func (s *MailService) SendMail(req smtp.SendRequest, userID uint) (*smtp.SendResult, error) {
	var account models.MailAccount
	if err := s.db.Where("user_id = ?", userID).First(&account, req.AccountID).Error; err != nil {
		return nil, fmt.Errorf("邮箱账号不存在 (ID=%d)", req.AccountID)
	}

	if account.Password == "" {
		return nil, fmt.Errorf("该邮箱账号未设置密码，无法发送邮件")
	}

	return smtp.SendMail(&account, &req)
}

// SaveSentMail 保存已发送邮件到数据库（自动归属到所属账号的用户）
func (s *MailService) SaveSentMail(mail *models.Mail) error {
	if mail.UserID == 0 && mail.AccountID != 0 {
		var acc models.MailAccount
		if err := s.db.Select("user_id").First(&acc, mail.AccountID).Error; err == nil {
			mail.UserID = acc.UserID
		}
	}
	return s.db.Create(mail).Error
}

// GetAccountEmail 获取指定账号的邮箱地址（校验归属用户）
func (s *MailService) GetAccountEmail(accountID, userID uint) (string, error) {
	var account models.MailAccount
	if err := s.db.Select("email").Where("user_id = ?", userID).First(&account, accountID).Error; err != nil {
		return "", err
	}
	return account.Email, nil
}
