// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package pop3

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"magicmail/config"
	"magicmail/models"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"gorm.io/gorm"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

// POP3Fetcher POP3 邮件拉取器 - 从 POP3 服务器拉取并解析存储邮件
type POP3Fetcher struct {
	db            *gorm.DB
	config        *config.Config
	SyncedMailIDs []uint // 本次同步成功入库的邮件ID列表（精确追踪，用于webhook）
}

// NewPOP3Fetcher 创建 POP3 拉取器实例
func NewPOP3Fetcher(db *gorm.DB, cfg *config.Config) *POP3Fetcher {
	return &POP3Fetcher{
		db:     db,
		config: cfg,
	}
}

// SyncMailbox 同步指定 POP3 邮箱账号的所有邮件，返回新增/更新的邮件数量
func (f *POP3Fetcher) SyncMailbox(client *POP3Client) (int, error) {
	count, err := client.MessageCount()
	if err != nil {
		return 0, err
	}

	if count == 0 {
		log.Printf("📭 收件箱为空: %s", client.Account.Email)
		return 0, nil
	}

	log.Printf("📬 POP3 发现 %d 封邮件: %s (模式=%s)", count, client.Account.Email, client.Account.SyncMode)

	newCount := 0
	syncMode := client.Account.SyncMode
	syncDays := client.Account.SyncDays
	if syncDays <= 0 {
		syncDays = 30 // 默认30天
	}
	cutoffTime := time.Now().AddDate(0, 0, -syncDays)

	for seq := 1; seq <= count; seq++ {
		rawData, size, err := client.RetrieveMessage(seq)
		if err != nil {
			log.Printf("⚠️  获取第 %d 封邮件失败: %v", seq, err)
			continue
		}

		parsed, err := f.parseMessage(client, rawData, uint32(seq), size)
		if err != nil {
			log.Printf("⚠️  解析第 %d 封邮件失败: %v", seq, err)
			continue
		}
		if parsed == nil {
			continue
		}

		// 根据 SyncMode 过滤（POP3 不支持未读判断）
		if syncMode == "recent" && !parsed.SentAt.IsZero() && parsed.SentAt.Before(cutoffTime) {
			continue // 超出最近N天范围，跳过
		}
		// POP3 协议不支持已读/未读状态，syncMode=unread 时降级为同步全部
		// （POP3 每次拉取都是全量，去重由 Message-ID 控制）

		// 去重 + 入库
		var existing int64
		f.db.Model(&models.Mail{}).
			Where("message_id = ? AND account_id = ? AND folder = ?", parsed.MessageID, client.Account.ID, "inbox").
			Count(&existing)
		if existing > 0 {
			continue
		}
		parsed.UserID = client.Account.UserID
		if err := f.db.Create(parsed).Error; err != nil {
			log.Printf("⚠️  保存邮件失败 (seq=%d): %v", seq, err)
			continue
		}

		// ⭐ 记录本次成功入库的邮件ID（用于精确触发 webhook）
		f.SyncedMailIDs = append(f.SyncedMailIDs, parsed.ID)
		log.Printf("✅ [POP3] 入库邮件 ID=%d, seq=%d, subject=%q", parsed.ID, seq, parsed.Subject)

		// ⭐ 入库后补全所有附件的 MailID（解析时 mailObj.ID 为 0）
		for i := range parsed.Attachments {
			parsed.Attachments[i].MailID = parsed.ID
			if attErr := f.db.Create(&parsed.Attachments[i]).Error; attErr != nil {
				log.Printf("⚠️  [POP3] 保存附件失败: %v", attErr)
				if parsed.Attachments[i].FilePath != "" {
					os.Remove(parsed.Attachments[i].FilePath)
				}
			}
		}

		newCount++
	}

	log.Printf("📬 POP3 同步完成 %s: 新增/更新 %d 封邮件, IDs=%v", client.Account.Email, newCount, f.SyncedMailIDs)
	return newCount, nil
}

// parseMessage 解析单封原始 RFC822 邮件，返回 Mail 对象
func (f *POP3Fetcher) parseMessage(client *POP3Client, rawData []byte, seq uint32, size int64) (*models.Mail, error) {
	entity, err := message.Read(bytes.NewReader(rawData))
	if err != nil {
		return nil, fmt.Errorf("MIME 解析失败: %w", err)
	}

	messageID := entity.Header.Get("message-id")
	if messageID == "" {
		// 无 Message-ID 时使用稳定键（account_id + seq），
		// 避免使用 time.Now() 导致每次同步生成不同 ID 从而去重失败
		messageID = fmt.Sprintf("<pop3-account-%d-seq-%d@magicmail>", client.Account.ID, seq)
	}

	fromAddr := extractAddrHeader(&entity.Header, "from")
	toAddr := extractAddrHeader(&entity.Header, "to")
	ccAddr := extractAddrHeader(&entity.Header, "cc")

	subject, _ := entity.Header.Text("subject")
	if subject == "" {
		subject = decodeRFC2047(entity.Header.Get("subject"))
	}

	sentAt := time.Now()
	if dateStr := entity.Header.Get("date"); dateStr != "" {
		if parsedDate, err := parseDate(dateStr); err == nil && !parsedDate.IsZero() {
			sentAt = parsedDate
		}
	}

	mailObj := &models.Mail{
		AccountID:  client.Account.ID,
		Folder:     "inbox", // POP3 始终是收件箱
		MessageID:  messageID,
		MessageUID: seq,
		From:       fromAddr,
		To:         toAddr,
		Cc:         ccAddr,
		Subject:    subject,
		SentAt:     sentAt,
		IsRead:     false,
		IsStarred:  false,
		Size:       size,
		CreatedAt:  time.Now(),
	}

	// ⭐ POP3 全量下载模式：准备附件目录
	baseDir := filepath.Join(".", "data", "attachments")
	os.MkdirAll(baseDir, 0755)

	f.parseEntityRecursive(entity, mailObj, baseDir, seq)

	return mailObj, nil
}

// parseEntityRecursive 递归解析 MIME 实体（处理 multipart 和单部分）
// POP3 模式：全量下载所有附件，不支持懒加载（无 PartID 概念）
func (f *POP3Fetcher) parseEntityRecursive(entity *message.Entity, mailObj *models.Mail, baseDir string, pop3Seq uint32) {
	mediaType, params, _ := entity.Header.ContentType()

	if mr := entity.MultipartReader(); mr != nil {
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("⚠️  读取 MIME 部分失败: %v", err)
				break
			}
			f.parseEntityRecursive(part, mailObj, baseDir, pop3Seq)
		}
		return
	}

	contentDisposition, dispParams, _ := entity.Header.ContentDisposition()
	isAttachment := contentDisposition == "attachment" ||
		(contentDisposition == "inline" && params["name"] != "")

	if isAttachment {
		filename := dispParams["filename"]
		if filename == "" {
			filename = params["name"]
		}
		decodedFilename := decodeRFC2047Filename(filename)

		// 尝试从 Content-Length 获取大小
		contentLength := entity.Header.Get("Content-Length")
		var estimatedSize int64
		if contentLength != "" {
			fmt.Sscanf(contentLength, "%d", &estimatedSize)
		}

		maxSize := f.config.IMAP.GetMaxAttachmentSize()

		// 超过最大限制的附件跳过
		if estimatedSize > maxSize && maxSize > 0 {
			log.Printf("⚠️  [POP3] 附件超过最大限制 (%d > %d)，跳过: %s", estimatedSize, maxSize, decodedFilename)
			return
		}

		// ⭐ 大附件流式写入磁盘（>5MB 或无法确定大小）
		shouldStream := estimatedSize > models.MaxDBSize || (estimatedSize == 0 && baseDir != "")

		if shouldStream && baseDir != "" {
			fileName := fmt.Sprintf("pop3_%d_%s", pop3Seq, decodedFilename)
			filePath := filepath.Join(baseDir, fileName)

			if outFile, err := os.Create(filePath); err == nil {
				written, copyErr := io.Copy(outFile, entity.Body)
				outFile.Close()

				if copyErr != nil {
					log.Printf("⚠️  [POP3] 写入附件文件失败: %v", copyErr)
					os.Remove(filePath)
					return
				}

				cacheExpire := time.Now().Add(f.config.IMAP.GetCacheExpireDuration())
				att := models.Attachment{
					MailID:      mailObj.ID,
					Filename:    decodedFilename,
					ContentType: mediaType,
					Size:        written,
					FilePath:    filePath,
					IMAPUID:     pop3Seq,       // POP3 序号作为标识
					PartID:      "",            // ⭐ POP3 无 PartID，留空
					IsCached:    true,           // ⭐ 全量下载，标记已缓存
					CacheExpire: &cacheExpire,
					CreatedAt:   time.Now(),
				}
				mailObj.HasAttachment = true
				mailObj.Attachments = append(mailObj.Attachments, att)
				log.Printf("📎 [POP3] 大附件已缓存到本地: %s (%d bytes)", decodedFilename, written)
			} else {
				log.Printf("⚠️  [POP3] 创建附件文件失败: %v", err)
			}
		} else {
			// 小附件：读入内存存 DB BLOB
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, entity.Body); err == nil {
				cacheExpire := time.Now().Add(f.config.IMAP.GetCacheExpireDuration())
				att := models.Attachment{
					MailID:      mailObj.ID,
					Filename:    decodedFilename,
					ContentType: mediaType,
					Size:        int64(buf.Len()),
					Content:     buf.Bytes(),
					IMAPUID:     pop3Seq,       // POP3 序号作为标识
					PartID:      "",            // ⭐ POP3 无 PartID，留空
					IsCached:    true,           // ⭐ 全量下载，标记已缓存
					CacheExpire: &cacheExpire,
					CreatedAt:   time.Now(),
				}
				mailObj.HasAttachment = true
				mailObj.Attachments = append(mailObj.Attachments, att)
			}
		}
		return
	}

	switch {
	case strings.HasPrefix(mediaType, "text/plain"):
		textData, _ := io.ReadAll(entity.Body)
		charset := strings.ToLower(strings.Trim(params["charset"], "\"' \t"))
		decoded := decodeTextBodyWithCharset(textData, charset)
		mailObj.TextBody.String = decoded
		mailObj.TextBody.Valid = true
	case strings.HasPrefix(mediaType, "text/html"):
		htmlData, _ := io.ReadAll(entity.Body)
		charset := strings.ToLower(strings.Trim(params["charset"], "\"' \t"))
		decoded := decodeTextBodyWithCharset(htmlData, charset)
		mailObj.HTMLBody.String = decoded
		mailObj.HTMLBody.Valid = true
	}
}

// --- 工具函数 ---

// extractAddrHeader 从 message.Header 中提取地址字段的格式化字符串
func extractAddrHeader(h *message.Header, key string) string {
	v := h.Get(key)
	if v == "" {
		return ""
	}
	addrs, err := mail.ParseAddressList(v)
	if err != nil || len(addrs) == 0 {
		return v // 解析失败则返回原始值
	}
	parts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if addr.Name != "" {
			parts = append(parts, fmt.Sprintf("%s <%s>", decodeRFC2047(addr.Name), addr.Address))
		} else {
			parts = append(parts, addr.Address)
		}
	}
	return strings.Join(parts, ", ")
}

// decodeRFC2047 解码 RFC 2047 编码的头部字段值
func decodeRFC2047(raw string) string {
	if raw == "" {
		return raw
	}
	dec := &mime.WordDecoder{
		CharsetReader: charsetReader,
	}
	decoded, err := dec.DecodeHeader(raw)
	if err != nil {
		log.Printf("[WARN] decodeRFC2047 failed: %v, raw: %s", err, truncateStringPOP3(raw, 80))
		return raw
	}
	return decoded
}

// decodeRFC2047Filename 解码 RFC 2047 编码的文件名
func decodeRFC2047Filename(raw string) string {
	if raw == "" {
		return raw
	}
	dec := &mime.WordDecoder{
		CharsetReader: charsetReader,
	}
	decoded, err := dec.DecodeHeader(raw)
	if err != nil {
		return raw
	}
	return decoded
}

// charsetReader 为 mime.WordDecoder 提供字符集解码支持
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
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
}

// truncateStringPOP3 截断字符串用于日志输出
func truncateStringPOP3(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// decodeTextBodyWithCharset 根据 MIME 声明的 charset 解码邮件正文
func decodeTextBodyWithCharset(data []byte, charset string) string {
	if len(data) == 0 {
		return ""
	}
	// 规范化：去除引号和空白
	charset = strings.ToLower(strings.Trim(charset, "\"' \t"))
	switch {
	case charset == "", charset == "utf-8", charset == "utf8", charset == "us-ascii":
		return string(data)
	case charset == "gb18030", charset == "gbk", charset == "gb2312":
		if decoded, err := simplifiedchinese.GBK.NewDecoder().String(string(data)); err == nil {
			return decoded
		}
		log.Printf("[POP3] GBK/GB18030 解码失败，回退到原始数据")
		return string(data)
	case charset == "iso-8859-1", charset == "latin-1", charset == "latin1":
		if decoded, err := unicode.UTF8.NewDecoder().String(string(data)); err == nil {
			return decoded
		}
		return string(data)
	default:
		str := string(data)
		if isUTF8(str) {
			return str
		}
		if decoded, err := simplifiedchinese.GBK.NewDecoder().String(str); err == nil {
			return decoded
		}
		return str
	}
}

// isUTF8 简单检查字符串是否为有效 UTF-8
func isUTF8(s string) bool {
	for _, r := range s {
		if r == '\ufffd' {
			return false
		}
	}
	return true
}

// parseDate 解析邮件日期字符串（兼容多种 RFC5322 格式）
var dateFormats = []string{
	time.RFC1123Z,
	"Mon, 02 Jan 2006 15:04:05 -0700",
	time.RFC850,
	"Mon, 02 Jan 2006 15:04:05 MST",
	"02 Jan 2006 15:04:05 MST",
	time.ANSIC,
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05.000Z07:00",
}

func parseDate(s string) (time.Time, error) {
	for _, layout := range dateFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析日期: %s", s)
}
