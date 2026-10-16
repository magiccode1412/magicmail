// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package services

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"magicmail/config"
	"magicmail/models"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"gorm.io/gorm"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message"
)

// AttachmentService 附件业务逻辑（支持混合缓存模式）
type AttachmentService struct {
	db     *gorm.DB
	config *config.Config
}

// NewAttachmentService 创建附件 Service 实例
func NewAttachmentService(db *gorm.DB) *AttachmentService {
	return &AttachmentService{db: db, config: config.Load()}
}

// GetByID 获取单个附件（含内容/缓存检查/懒加载触发），校验所属邮件归属用户
func (s *AttachmentService) GetByID(id, userID uint) (*models.Attachment, error) {
	var att models.Attachment
	if err := s.db.
		Joins("JOIN mails ON mails.id = attachments.mail_id").
		Where("attachments.id = ? AND mails.user_id = ?", id, userID).
		First(&att).Error; err != nil {
		return nil, err
	}

	// ========== 模式1：已缓存 → 直接返回 ==========
	if att.IsCached {
		return &att, nil
	}

	// ========== 模式2：POP3 兼容 — 有实际内容但未标记 IsCached ==========
	// （旧版 POP3 数据或迁移数据可能缺失 IsCached 标记）
	if att.FilePath != "" || len(att.Content) > 0 {
		log.Printf("📎 [POP3 兼容] 附件有本地内容但未标记已缓存 (ID=%d)，自动修复", id)
		s.db.Model(&att).Update("is_cached", true)
		att.IsCached = true
		return &att, nil
	}

	// ========== 模式3：IMAP 懒加载 — 有 PartID 可按需获取 ==========
	if att.IMAPUID > 0 && att.PartID != "" {
		log.Printf("📎 请求懒加载附件 (ID=%d, UID=%d, Part=%s)", att.ID, att.IMAPUID, att.PartID)
		return &att, nil
	}

	// ========== 模式4：POP3 无 PartID 且无本地内容 ==========
	// POP3 协议不支持部分获取，无法按需下载，返回明确错误
	log.Printf("❌ [POP3] 附件不可用：无本地内容且无法从源服务器按需获取 (ID=%d)", id)
	return &att, fmt.Errorf("POP3 附件暂不可用（无缓存且不支持按需下载）")
}

// GetByMailID 获取邮件的所有附件列表（不含内容），校验所属邮件归属用户
func (s *AttachmentService) GetByMailID(mailID, userID uint) ([]models.AttachmentResp, error) {
	// 先确认邮件归属当前用户
	var mailCount int64
	s.db.Model(&models.Mail{}).Where("id = ? AND user_id = ?", mailID, userID).Count(&mailCount)
	if mailCount == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var attachments []models.Attachment
	err := s.db.Where("mail_id = ?", mailID).Find(&attachments).Error
	if err != nil {
		return nil, err
	}

	resps := make([]models.AttachmentResp, len(attachments))
	for i, att := range attachments {
		resps[i] = models.AttachmentResp{
			ID:          att.ID,
			MailID:      att.MailID,
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Size:        att.Size,
			SizeHuman:   formatAttachmentSize(att.Size),
			IsCached:    att.IsCached, // ⭐ 新增字段
			CreatedAt:   att.CreatedAt,
		}
	}
	return resps, nil
}

// GetMailAndAccount 获取邮件及其关联的邮箱账号（用于 IMAP 按需下载）
func (s *AttachmentService) GetMailAndAccount(mailID uint) (*models.Mail, *models.MailAccount, error) {
	var mail models.Mail
	if err := s.db.Preload("Account").First(&mail, mailID).Error; err != nil {
		return nil, nil, err
	}
	if mail.Account == nil {
		return nil, nil, fmt.Errorf("邮件 %d 无关联账号", mailID)
	}
	return &mail, mail.Account, nil
}

// StreamFromIMAP 从 IMAP 服务器流式获取指定 MIME 部分
// 返回 io.Reader 用于直接写入 HTTP 响应（零内存拷贝）
func (s *AttachmentService) StreamFromIMAP(client *imapclient.Client, imapUID uint32, partID string) (io.ReadCloser, string, int64, error) {
	// 构建 BODY[section] 请求（如 BODY[1.2]）
	// Part 字段要求 []int 类型，需将字符串 "1.2" 转换为 []int{1, 2}
	parts := parsePartIDs(partID)
	if len(parts) == 0 {
		return nil, "", 0, fmt.Errorf("无效的 PartID: %q", partID)
	}
	log.Printf("📎 [DEBUG] parsePartIDs: %q -> %v", partID, parts)

	bodySection := &imap.FetchItemBodySection{
		Part: parts,
	}

	fetchOptions := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}

	uidSet := imap.UIDSetNum(imap.UID(imapUID))
	log.Printf("📎 [DEBUG] 发起 IMAP FETCH (UID=%d, Part=%v)", imapUID, parts)
	fetchCmd := client.Fetch(uidSet, fetchOptions)

	msgs, err := fetchCmd.Collect()
	if err != nil {
		return nil, "", 0, fmt.Errorf("IMAP FETCH 失败: %w", err)
	}
	log.Printf("📎 [DEBUG] IMAP FETCH 完成，收到 %d 条消息", len(msgs))

	if len(msgs) == 0 {
		return nil, "", 0, fmt.Errorf("IMAP 无消息返回")
	}

	msg := msgs[0]
	var bodyBytes []byte
	for _, data := range msg.BodySection {
		bodyBytes = data
		break
	}
	if len(bodyBytes) == 0 {
		return nil, "", 0, fmt.Errorf("IMAP 返回空 body")
	}

	// 解析 MIME entity 以获取 Content-Type 和真实数据
	entity, err := message.Read(bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("📎 [DEBUG] MIME 解析失败 (%v)，尝试自动检测编码...", err)

		// ⭐ 关键修复：IMAP 返回的可能是 base64 或 quoted-printable 编码
		// 当 go-message 无法解析时，需要手动检测并解码
		finalData := bodyBytes
		var decoded bool

		// 检测并尝试 base64 解码
		if looksLikeBase64(string(bodyBytes)) {
			if decData, decErr := tryManualBase64Decode(bodyBytes); decErr == nil && len(decData) > 0 {
				log.Printf("✅ [DEBUG] 自动 base64 解码成功: %d -> %d bytes", len(bodyBytes), len(decData))
				finalData = decData
				decoded = true
			}
		}

		if !decoded {
			// 尝试 quoted-printable 解码
			qpReader := mime.WordDecoder{}
			if qpDecoded, qpErr := qpReader.DecodeHeader(string(bodyBytes)); qpErr == nil && qpDecoded != string(bodyBytes) {
				log.Printf("✅ [DEBUG] 自动 QP 解码成功")
				finalData = []byte(qpDecoded)
				decoded = true
			}
		}

		if !decoded && len(bodyBytes) < len(finalData) {
			log.Printf("⚠️ [DEBUG] 无法自动解码，使用原始数据 (%d bytes)", len(finalData))
		}

		contentType := "application/octet-stream"
		// 尝试从解码后的头部推断 Content-Type
		if decoded && len(finalData) >= 8 {
			contentType = detectContentTypeFromMagic(finalData)
			log.Printf("📎 [DEBUG] 从文件头推断类型: %s", contentType)
		}

		reader := io.NopCloser(bytes.NewReader(finalData))
		return reader, contentType, int64(len(finalData)), nil
	}

	mediaType, _, _ := entity.Header.ContentType()
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	// 检查 Content-Transfer-Encoding
	cte := entity.Header.Get("Content-Transfer-Encoding")
	log.Printf("📎 [DEBUG] MIME 信息: ContentType=%s, CTE=%q, 原始大小=%d", mediaType, cte, len(bodyBytes))

	// ⭐ 读取解码后的数据以验证完整性
	decodedData, readErr := io.ReadAll(entity.Body)
	if readErr != nil {
		log.Printf("❌ [DEBUG] 读取 entity.Body 失败: %v", readErr)
		// 读取失败时回退到原始数据
		reader := io.NopCloser(bytes.NewReader(bodyBytes))
		return reader, mediaType, int64(len(bodyBytes)), nil
	}
	log.Printf("📎 [DEBUG] 解码后大小: %d (CTE=%q)", len(decodedData), cte)

	// ⭐ 验证：如果解码后大小异常（如为0或远小于原始），回退到原始数据
	if len(decodedData) == 0 && len(bodyBytes) > 0 {
		log.Printf("⚠️ [DEBUG] 解码结果为空！回退到原始数据 (原始=%d bytes)", len(bodyBytes))
		reader := io.NopCloser(bytes.NewReader(bodyBytes))
		return reader, mediaType, int64(len(bodyBytes)), nil
	}

	// 检查是否有 WAV/媒体文件的魔数来验证完整性
	if len(decodedData) >= 4 {
		magic := string(decodedData[:4])
		log.Printf("📎 [DEBUG] 文件头部 (hex): %x, 文本: %s", decodedData[:8], magic)

		// 如果看起来不像有效媒体文件，且原始数据像 base64 尝试手动解码
		if !isValidMediaHeader(decodedData) && looksLikeBase64(string(bodyBytes)) {
			log.Printf("⚠️ [DEBUG] 解码结果非标准媒体格式，尝试手动 base64 解码")
			if manualDecoded, decodeErr := tryManualBase64Decode(bodyBytes); decodeErr == nil && len(manualDecoded) > len(decodedData) {
				log.Printf("✅ [DEBUG] 手动 base64 解码成功: %d bytes", len(manualDecoded))
				decodedData = manualDecoded
			}
		}
	}

	reader := io.NopCloser(bytes.NewReader(decodedData))
	return reader, mediaType, int64(len(decodedData)), nil
}

// CacheAttachment 将从 IMAP 获取的附件内容缓存到本地
// 用于用户首次下载后异步缓存（可选优化）
func (s *AttachmentService) CacheAttachment(att *models.Attachment, content io.Reader) error {
	baseDir := filepath.Join(".", "data", "attachments")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("创建附件目录失败: %w", err)
	}

	fileName := fmt.Sprintf("%d_%s", att.MailID, att.Filename)
	filePath := filepath.Join(baseDir, fileName)

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建缓存文件失败: %w", err)
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, content)
	if err != nil {
		os.Remove(filePath)
		return fmt.Errorf("写入缓存失败: %w", err)
	}

	cacheExpire := time.Now().Add(s.config.IMAP.GetCacheExpireDuration())

	// 更新数据库记录
	now := time.Now()
	s.db.Model(att).Updates(map[string]interface{}{
		"file_path":    filePath,
		"is_cached":    true,
		"cache_expire": cacheExpire,
		"size":         written,
		"updated_at":   now,
	})

	log.Printf("✅ 附件已缓存到本地 (ID=%d): %s (%d bytes)", att.ID, att.Filename, written)
	return nil
}

// CleanupExpiredCache 清理过期的附件缓存（建议通过定时任务调用）
func (s *AttachmentService) CleanupExpiredCache() (int64, error) {
	result := s.db.Model(&models.Attachment{}).
		Where("is_cached = ? AND file_path != '' AND cache_expire < ?", true, time.Now()).
		Find(&[]models.Attachment{})

	if result.Error != nil || result.RowsAffected == 0 {
		return 0, result.Error
	}

	var expiredAtt []models.Attachment
	s.db.Where("is_cached = ? AND file_path != '' AND cache_expire < ?", true, time.Now()).
		Find(&expiredAtt)

	cleaned := int64(0)
	for _, att := range expiredAtt {
		if att.FilePath != "" {
			if err := os.Remove(att.FilePath); err == nil || os.IsNotExist(err) {
				cleaned++
				s.db.Model(&att).Updates(map[string]interface{}{
					"is_cached":   false,
					"file_path":   "",
					"cache_expire": nil,
				})
				log.Printf("🗑️  清理过期缓存: ID=%d, File=%s", att.ID, att.Filename)
			}
		}
	}

	return cleaned, nil
}

// --- 工具函数 ---

// CheckDiskSpace 检查磁盘剩余空间是否足够
// 返回 true 表示有足够空间，false 表示空间不足
func (s *AttachmentService) CheckDiskSpace(requiredBytes int64) bool {
	freeBytes, err := getDiskFreeSpaceForServices()
	if err != nil {
		log.Printf("⚠️ 无法获取磁盘信息: %v", err)
		return true // 无法获取时默认允许（避免阻塞）
	}

	minFree := s.config.IMAP.GetMinDiskFree()

	log.Printf("💾 磁盘检查: 需要 %d, 可用 %s, 最低要求 %s",
		requiredBytes, formatAttachmentSize(freeBytes), formatAttachmentSize(minFree))

	return freeBytes >= minFree+requiredBytes
}

// ShouldAutoCache 判断附件是否应该自动缓存
func (s *AttachmentService) ShouldAutoCache(att *models.Attachment) bool {
	// 1. 检查功能是否开启
	if !s.config.IMAP.IsAutoCacheEnabled() {
		return false
	}

	// 2. 检查是否已缓存
	if att.IsCached {
		return false
	}

	// 3. 检查大小是否超过阈值（0=不限制）
	threshold := s.config.IMAP.GetCacheThreshold()
	if threshold > 0 && att.Size > threshold {
		log.Printf("⏭️ 附件 %d (%s) 超过缓存阈值 %s，跳过自动缓存",
			att.ID, att.Filename, formatAttachmentSize(threshold))
		return false
	}

	// 4. 检查最大附件限制
	maxSize := s.config.IMAP.GetMaxAttachmentSize()
	if maxSize > 0 && att.Size > maxSize {
		return false
	}

	// 5. 检查磁盘空间
	if !s.CheckDiskSpace(att.Size) {
		log.Printf("⚠️ 磁盘空间不足，跳过自动缓存 (ID=%d)", att.ID)
		return false
	}

	return true
}

// formatAttachmentSize 格式化附件文件大小为人类可读格式
func formatAttachmentSize(size int64) string {
	if size < 0 {
		return "未知"
	}
	if size <= 0 {
		return "0 B"
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
	return fmt.Sprintf("%.1f %cB",
		float64(size)/float64(div), "KMGTPE"[exp])
}

// decodeRFC2047Filename 解码 RFC 2047 编码的文件名
func decodeRFC2047Filename(raw string) string {
	if raw == "" {
		return raw
	}
	dec := &mime.WordDecoder{
		CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
			switch strings.ToLower(charset) {
			case "gbk", "gb2312", "gb18030":
				return transform.NewReader(input, simplifiedchinese.GBK.NewDecoder()), nil
			case "utf-8", "utf8":
				return input, nil
			case "iso-8859-1", "latin-1", "latin1":
				return transform.NewReader(input, unicode.UTF8.NewDecoder()), nil
			default:
				return input, nil
			}
		},
	}
	decoded, err := dec.DecodeHeader(raw)
	if err != nil {
		return raw
	}
	return decoded
}

// parsePartIDs 将 MIME Part ID 字符串（如 "1.2"）转换为 []int（如 {1, 2}）
func parsePartIDs(partID string) []int {
	var parts []int
	for _, s := range strings.Split(partID, ".") {
		n, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		parts = append(parts, n)
	}
	return parts
}

// --- 媒体文件验证辅助函数 ---

// isValidMediaHeader 检查数据头部是否为有效的媒体文件魔数
func isValidMediaHeader(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	// WAV: RIFF
	if string(data[:4]) == "RIFF" {
		return true
	}
	// MP3: FF FB / FF F3 / 49 44 33 (ID3)
	if data[0] == 0xFF && (data[1]&0xE0) == 0xE0 {
		return true
	}
	if string(data[:3]) == "ID3" {
		return true
	}
	// MP4: ftyp
	if len(data) >= 8 && string(data[4:8]) == "ftyp" {
		return true
	}
	// WebM: 1A 45 DF A3
	if data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3 {
		return true
	}
	// OGG: OggS
	if string(data[:4]) == "OggS" {
		return true
	}
	// PNG, JPEG, GIF 等图片
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 { // PNG
		return true
	}
	if data[0] == 0xFF && data[1] == 0xD8 { // JPEG
		return true
	}
	if string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a" { // GIF
		return true
	}
	// PDF
	if string(data[:5]) == "%PDF-" {
		return true
	}
	// ZIP (包括 docx, xlsx, etc.)
	if len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04 {
		return true
	}
	// 如果不是已知格式，也返回 true（可能是其他有效格式）
	return true // 默认认为有效，避免误判
}

// looksLikeBase64 检查数据是否看起来像 base64 编码
func looksLikeBase64(s string) bool {
	s = strings.TrimSpace(s)
	// 移除可能的换行符
	s = strings.ReplaceAll(s, "\r\n", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	// 检查是否只包含 base64 字符
	base64Chars := 0
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=' {
			base64Chars++
		}
	}
	ratio := float64(base64Chars) / float64(len(s))
	return ratio > 0.95 && len(s) > 100
}

// tryManualBase64Decode 尝试手动 base64 解码（处理可能的 MIME 头部）
func tryManualBase64Decode(rawData []byte) ([]byte, error) {
	s := string(rawData)

	// 尝试移除可能的 MIME 头部
	if idx := strings.Index(s, "\r\n\r\n"); idx != -1 {
		s = s[idx+4:]
	}
	if idx := strings.Index(s, "\n\n"); idx != -1 {
		s = s[idx+2:]
	}

	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")

	return base64.StdEncoding.DecodeString(s)
}

// detectContentTypeFromMagic 从文件头魔数检测 Content-Type
func detectContentTypeFromMagic(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}

	// WAV: RIFF....WAVE
	if string(data[:4]) == "RIFF" && len(data) >= 12 && string(data[8:12]) == "WAVE" {
		return "audio/wav"
	}

	// MP3 / ID3
	if data[0] == 0xFF && (data[1]&0xE0) == 0xE0 {
		return "audio/mpeg"
	}
	if string(data[:3]) == "ID3" {
		return "audio/mpeg"
	}

	// MP4 (包括 M4A, M4V): ftyp
	if len(data) >= 8 && string(data[4:8]) == "ftyp" {
		ftyp := string(data[8:12])
		switch ftyp {
		case "isom", "mp42", "mp41", "M4V ", "M4A ":
			return "video/mp4"
		default:
			return "video/mp4"
		}
	}

	// WebM: EBML
	if data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3 {
		return "video/webm"
	}

	// OGG: OggS
	if string(data[:4]) == "OggS" {
		return "audio/ogg"
	}

	// PNG
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}

	// JPEG
	if data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}

	// GIF
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "image/gif"
	}

	// PDF
	if len(data) >= 5 && string(data[:5]) == "%PDF-" {
		return "application/pdf"
	}

	// ZIP (docx, xlsx, etc.)
	if len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04 {
		return "application/zip"
	}

	// 默认
	return "application/octet-stream"
}
