// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package imap

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

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

// Fetcher 邮件拉取器 - 负责从 IMAP 服务器拉取邮件并解析存储
type Fetcher struct {
	db            *gorm.DB
	config        *config.Config
	folder        string // 当前同步的文件夹名（inbox/sent）
	SyncedMailIDs []uint // 本次同步成功入库的邮件ID列表（精确追踪，用于webhook）
}

// NewFetcher 创建拉取器实例
func NewFetcher(db *gorm.DB, cfg *config.Config) *Fetcher {
	return &Fetcher{
		db:     db,
		config: cfg,
	}
}

// SyncMailbox 同步指定邮箱账号的 INBOX，返回新增/更新的邮件数量
func (f *Fetcher) SyncMailbox(client *IMAPClient) (int, error) {
	return f.syncMailbox(client, "INBOX", "inbox")
}

// SyncSentMailbox 同步已发送文件夹（Sent），返回新增/更新的邮件数量
func (f *Fetcher) SyncSentMailbox(client *IMAPClient) (int, error) {
	return f.syncMailbox(client, "Sent", "sent")
}

// syncMailbox 同步指定邮箱账号的指定 IMAP 文件夹
func (f *Fetcher) syncMailbox(client *IMAPClient, mailboxName, folder string) (int, error) {
	f.folder = folder
	// 注意：不在此处重置 SyncedMailIDs，由调用方（worker）在创建 Fetcher 后统一管理
	// 因为同一 Fetcher 可能被多次调用（INBOX + Sent），需要累积所有ID

	mbox, err := client.SelectMailbox(mailboxName)
	if err != nil {
		// Sent 文件夹可能不存在或无权限，静默跳过不报错
		if folder == "sent" {
			log.Printf("⚠️  %s 的 %s 文件夹不可用，跳过同步: %v", client.Account.Email, mailboxName, err)
			return 0, nil
		}
		return 0, err
	}

	if mbox.NumMessages == 0 {
		log.Printf("📭 收件箱为空: %s", client.Account.Email)
		return 0, nil
	}

	// 构建序列集：获取所有邮件（使用显式范围而非 1:* 避免部分 IMAP 兼容性问题）
	seqSet := imap.SeqSet{}
	seqSet.AddRange(1, mbox.NumMessages) // 显式指定 1 到 NumMessages

	// 定义获取选项：信封 + UID + 标志 + 大小 + 内部日期（用于日期过滤）
	fetchOptions := &imap.FetchOptions{
		Envelope:    true,
		UID:         true,
		Flags:       true,
		RFC822Size:  true,
		InternalDate: true,
	}

	fetchCmd := client.Client.Fetch(seqSet, fetchOptions)
	defer fetchCmd.Close()

	newCount := 0
	syncMode := client.Account.SyncMode
	syncDays := client.Account.SyncDays
	if syncDays <= 0 {
		syncDays = 30 // 默认30天
	}
	cutoffTime := time.Now().AddDate(0, 0, -syncDays)

	log.Printf("📬 开始同步 %s: 模式=%s, 天数=%d, 收件箱共 %d 封邮件",
		client.Account.Email, syncMode, syncDays, mbox.NumMessages)

	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		// 使用 Collect() 获取完整消息数据（包含 Flags、InternalDate 等字段）
		buf, err := msg.Collect()
		if err != nil {
			log.Printf("⚠️  收集消息数据失败: %v", err)
			continue
		}

	// 根据 SyncMode 判断是否需要同步这封邮件
	if !shouldSync(buf, syncMode, cutoffTime) {
		continue
	}

		parsed, _, err := f.parseMessage(client, buf)
		if err != nil {
			log.Printf("⚠️  解析邮件失败 (UID=%d): %v", buf.UID, err)
			continue
		}
		if parsed == nil {
			continue // 已存在（去重跳过）
		}

		// ⭐ 记录本次成功入库的邮件ID（用于精确触发 webhook）
		f.SyncedMailIDs = append(f.SyncedMailIDs, parsed.ID)
		log.Printf("✅ [%s] 入库邮件 ID=%d, UID=%d, subject=%q", f.folder, parsed.ID, buf.UID, parsed.Subject)

		// 邮件和附件已在 parseMessage 中统一入库，无需额外操作
		newCount++
	}

	log.Printf("📬 同步完成 %s (模式=%s): 新增/更新 %d 封邮件, IDs=%v", client.Account.Email, syncMode, newCount, f.SyncedMailIDs)
	return newCount, nil
}

// parseMessage 解析单封邮件，返回解析后的 Mail 对象；若已存在则返回 nil（去重）
func (f *Fetcher) parseMessage(client *IMAPClient, buf *imapclient.FetchMessageBuffer) (*models.Mail, *BodyResult, error) {
	envelope := buf.Envelope
	if envelope == nil {
		return nil, nil, fmt.Errorf("邮件信封为空")
	}

	messageID := envelope.MessageID
	if messageID == "" {
		// 无 Message-ID 时使用稳定键（account_id + folder + uid），
		// 避免使用 time.Now() 导致每次同步生成不同 ID 从而去重失败
		messageID = fmt.Sprintf("<auto-account-%d-folder-%s-uid-%d@magicmail>", client.Account.ID, f.folder, buf.UID)
	}

	// 去重：根据 Message-ID + AccountID + Folder 判断是否已存在
	// 加入 folder 避免同一 Message-ID 在不同文件夹（inbox/sent）中冲突
	var existing int64
	f.db.Model(&models.Mail{}).
		Where("message_id = ? AND account_id = ? AND folder = ?", messageID, client.Account.ID, f.folder).
		Count(&existing)
	if existing > 0 {
		return nil, nil, nil // 已存在，跳过
	}

	// 获取发件人（v2 的 Address 是值类型）
	fromAddr := extractIMAPAddressList(envelope.From)
	toAddr := extractIMAPAddressList(envelope.To)
	ccAddr := extractIMAPAddressList(envelope.Cc)

	subject := decodeHeader(envelope.Subject)

	sentAt := envelope.Date
	if sentAt.IsZero() {
		sentAt = time.Now()
	}

	// 判断已读/标星状态
	isRead := false
	isStarred := false
	for _, flag := range buf.Flags {
		if flag == imap.FlagSeen {
			isRead = true
		}
		if flag == imap.FlagFlagged {
			isStarred = true
		}
	}

	// 构建邮件对象（先不包含正文和附件信息）
	mailObj := &models.Mail{
		UserID:     client.Account.UserID,
		AccountID:  client.Account.ID,
		Folder:     f.folder,
		MessageID:  messageID,
		MessageUID: uint32(buf.UID),
		From:       fromAddr,
		To:         toAddr,
		Cc:         ccAddr,
		Subject:    subject,
		SentAt:     sentAt,
		IsRead:     isRead,
		IsStarred:  isStarred,
		Size:       buf.RFC822Size,
		CreatedAt:  time.Now(),
	}

	// ⭐ 先入库以获取 mailID（大附件流式写入需要）
	if err := f.db.Create(mailObj).Error; err != nil {
		return nil, nil, fmt.Errorf("创建邮件记录失败: %w", err)
	}

	// 拉取完整邮件体（正文 + 附件），传入 mailID 支持大附件流式写入
	bodySection, err := f.fetchBody(client, buf.UID, mailObj.ID)
	if err != nil {
		log.Printf("⚠️  拉取邮件体失败 (UID=%d), 删除不完整邮件记录 (mail_id=%d): %v", buf.UID, mailObj.ID, err)
		// ⭐ 关键修复：删除没有正文/附件的不完整邮件记录，避免后续因去重而无法重新同步
		f.db.Delete(mailObj)
		return nil, nil, err
	}
	if bodySection == nil {
		log.Printf("⚠️  邮件体返回空 (UID=%d), 删除不完整邮件记录 (mail_id=%d)", buf.UID, mailObj.ID)
		f.db.Delete(mailObj)
		return nil, nil, fmt.Errorf("邮件体为空")
	}

	// 更新邮件的正文信息
	updateData := map[string]interface{}{}
	if bodySection.TextBody != "" {
		updateData["text_body"] = bodySection.TextBody
	}
	if bodySection.HTMLBody != "" {
		updateData["html_body"] = bodySection.HTMLBody
	}
	if len(bodySection.Attachments) > 0 {
		updateData["has_attachment"] = true
	}
	if len(updateData) > 0 {
		if uErr := f.db.Model(mailObj).Updates(updateData).Error; uErr != nil {
			log.Printf("⚠️  更新邮件正文失败 (mail_id=%d): %v", mailObj.ID, uErr)
		}
	} else {
		// ⭐ 调试日志：记录无内容的邮件，帮助定位问题
		log.Printf("🔍 [DEBUG] 邮件体解析完成但无正文/附件 (mail_id=%d, UID=%d, subject=%q)",
			mailObj.ID, buf.UID, mailObj.Subject)
	}

	// ⭐ 保存所有附件到数据库（包括懒加载模式的元数据）
	for i := range bodySection.Attachments {
		att := &bodySection.Attachments[i]
		att.MailID = mailObj.ID

		if attErr := f.db.Create(att).Error; attErr != nil {
			log.Printf("⚠️  保存附件记录失败 (mail_id=%d, file=%s): %v",
				mailObj.ID, att.Filename, attErr)
			// 清理可能已创建的文件
			if att.FilePath != "" {
				os.Remove(att.FilePath)
			}
		}
	}

	return mailObj, nil, nil
}

// BodyResult 邮件体解析结果
type BodyResult struct {
	TextBody    string
	HTMLBody    string
	Attachments []models.Attachment
}

// fetchBody 获取邮件的完整正文和附件
// mailID 用于大附件流式写入（传入 0 表示尚未入库）
func (f *Fetcher) fetchBody(client *IMAPClient, uid imap.UID, mailID uint) (*BodyResult, error) {
	// 使用 UIDSet 获取单封邮件完整内容
	uidSet := imap.UIDSetNum(uid)

	// 获取 BODY[] 完整原始邮件 + BODYSTRUCTURE（用于获取各 Part 精确大小）
	bodySection := &imap.FetchItemBodySection{}
	fetchOptions := &imap.FetchOptions{
		BodySection:   []*imap.FetchItemBodySection{bodySection},
		BodyStructure: &imap.FetchItemBodyStructure{},
	}

	fetchCmd := client.Client.Fetch(uidSet, fetchOptions)
	defer fetchCmd.Close()

	// 使用 Collect 获取结果
	msgs, err := fetchCmd.Collect()
	if err != nil {
		return nil, fmt.Errorf("获取邮件体失败: %w", err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("无消息返回")
	}

	msg := msgs[0]
	// BodySection 是 map[*FetchItemBodySection][]byte
	if len(msg.BodySection) == 0 {
		return nil, fmt.Errorf("邮件体为空")
	}
	// 取第一个 body section 的数据
	var bodyBytes []byte
	for _, data := range msg.BodySection {
		bodyBytes = data
		break
	}
	if len(bodyBytes) == 0 {
		return nil, fmt.Errorf("邮件体为空")
	}

	result := &BodyResult{}

	// ⭐ 从 BODYSTRUCTURE 提取每个 MIME Part 的精确大小（key: "1", "1.1", "1.2" 等）
	partSizes := make(map[string]int64)
	if msg.BodyStructure != nil {
		msg.BodyStructure.Walk(func(path []int, part imap.BodyStructure) bool {
			// 构造 PartID: [1] -> "1", [1, 2] -> "1.2"
			partID := ""
			for i, idx := range path {
				if i > 0 {
					partID += "."
				}
				partID += fmt.Sprintf("%d", idx)
			}
			// 只处理单部分（附件/文本），跳过 multipart
			if singlePart, ok := part.(*imap.BodyStructureSinglePart); ok && partID != "" {
				partSizes[partID] = int64(singlePart.Size)
				log.Printf("🔍 [DEBUG] BODYSTRUCTURE Part=%s, Size=%d, Type=%s/%s",
					partID, singlePart.Size, singlePart.Type, singlePart.Subtype)
			}
			return true // 继续遍历子部分
		})
		log.Printf("🔍 [DEBUG] 从 BODYSTRUCTURE 获取到 %d 个 Part 大小信息", len(partSizes))
	}

	// 准备附件存储目录（仅当 mailID 可用时）
	var baseDir string
	if mailID > 0 {
		baseDir = filepath.Join(".", "data", "attachments")
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			log.Printf("⚠️  创建附件目录失败: %v", err)
			baseDir = ""
		}
	}

	// 使用 go-message 库解析 MIME 结构
	entity, err := message.Read(bytes.NewReader(bodyBytes))
	if err != nil {
		// 解析失败则作为纯文本处理
		log.Printf("⚠️  message.Read 解析失败 (UID=%d): %v，回退到原始数据", uid, err)
		result.TextBody = decodeTextContent(bodyBytes)
		return result, nil
	}

	// 检查是否为 multipart
	if mr := entity.MultipartReader(); mr != nil {
		// 多部分邮件：从 PartID "1" 开始递归
		partIdx := 1
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("⚠️  读取 MIME 部分失败: %v", err)
				break
			}
			f.parseEntity(part, result, mailID, baseDir, uint32(uid), fmt.Sprintf("%d", partIdx), partSizes)
			partIdx++
		}
	} else {
		// 单部分邮件：PartID 为 "1"
		f.parseEntity(entity, result, mailID, baseDir, uint32(uid), "1", partSizes)
	}

	// ⭐ 调试日志：记录解析结果摘要
	log.Printf("🔍 [DEBUG] MIME解析完成 (UID=%d): text_len=%d, html_len=%d, attachments=%d",
		uid, len(result.TextBody), len(result.HTMLBody), len(result.Attachments))

	return result, nil
}

// parseEntity 解析单个 MIME 实体（文本或附件）
// 支持嵌套的 multipart 结构（如 multipart/mixed > multipart/alternative > text/plain）
// mailID 和 baseDir 用于大附件的流式写入（>5MB 直接写磁盘）
// imapUID 和 partID 用于懒加载模式（大附件只存元数据，按需下载）
// partSizes 是从 BODYSTRUCTURE 获取的各 Part 精确大小（key: "1", "1.2" 等）
func (f *Fetcher) parseEntity(entity *message.Entity, result *BodyResult, mailID uint, baseDir string, imapUID uint32, partID string, partSizes map[string]int64) {
	mediaType, params, _ := entity.Header.ContentType()

	// ⭐ 递归处理嵌套的 multipart 结构
	if strings.HasPrefix(mediaType, "multipart/") {
		if mr := entity.MultipartReader(); mr != nil {
			subIdx := 1
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("⚠️  读取嵌套 MIME 部分失败: %v", err)
					break
				}
				// 子部分的 PartID = 父PartID.子索引
				subPartID := fmt.Sprintf("%s.%d", partID, subIdx)
				f.parseEntity(part, result, mailID, baseDir, imapUID, subPartID, partSizes)
				subIdx++
			}
		}
		return
	}

	// 处理 Content-Disposition 判断是否为附件
	contentDisposition, dispParams, _ := entity.Header.ContentDisposition()
	isAttachment := contentDisposition == "attachment" ||
		(contentDisposition == "inline" && params["name"] != "")

	if isAttachment {
		filename := dispParams["filename"]
		if filename == "" {
			filename = params["name"]
		}
		decodedFilename := decodeRFC2047Filename(filename)

		// ⭐ 优先从 BODYSTRUCTURE 获取精确大小，回退到 Content-Length
		var estimatedSize int64
		if bsSize, ok := partSizes[partID]; ok && bsSize > 0 {
			estimatedSize = bsSize
			log.Printf("🔍 [DEBUG] 使用 BODYSTRUCTURE 大小: Part=%s, Size=%d", partID, bsSize)
		} else {
			contentLength := entity.Header.Get("Content-Length")
			if contentLength != "" {
				fmt.Sscanf(contentLength, "%d", &estimatedSize)
			}
		}

		// ⭐ 根据配置决定缓存策略
		cacheThreshold := f.config.IMAP.GetCacheThreshold()
		maxSize := f.config.IMAP.GetMaxAttachmentSize()

		shouldLazyLoad := (estimatedSize >= cacheThreshold) ||
			(estimatedSize == 0 && mailID > 0) // 无法确定大小时也用懒加载

		if shouldLazyLoad && mailID > 0 {
			// ⭐ 懒加载模式：只保存元数据，不从 IMAP 下载内容
			finalSize := estimatedSize
			if finalSize <= 0 {
				// 无法获取精确大小时，标记为 -1 表示"未知"
				// 前端会显示"未知大小"或类似提示
				finalSize = -1
			}
			att := models.Attachment{
				MailID:      mailID,
				Filename:    decodedFilename,
				ContentType: mediaType,
				Size:        finalSize,
				IMAPUID:     imapUID,
				PartID:      partID,
				IsCached:    false,
				CreatedAt:   time.Now(),
			}
			result.Attachments = append(result.Attachments, att)
			log.Printf("📎 附件懒加载(不下载): %s [Part=%s, Size≈%d]", decodedFilename, partID, finalSize)
			return
		}

		// 小附件或无法懒加载时：正常下载并缓存
		if estimatedSize > maxSize && maxSize > 0 {
			log.Printf("⚠️ 附件超过最大限制 (%d > %d)，跳过: %s", estimatedSize, maxSize, decodedFilename)
			return
		}

		// ⭐ 磁盘空间预检查
		if estimatedSize > 0 && !f.checkDiskSpace(estimatedSize) {
			log.Printf("⚠️  磁盘空间不足，跳过附件: %s (需要 %d bytes)", decodedFilename, estimatedSize)
			return
		}

		// 判断是否需要流式写入磁盘（>5MB 或无法确定大小但 baseDir 可用）
		shouldStream := (estimatedSize > models.MaxDBSize) ||
			(estimatedSize == 0 && mailID > 0 && baseDir != "")

		if shouldStream && mailID > 0 && baseDir != "" {
			// 流式写入磁盘 - 完全不占用内存存储完整内容
			fileName := fmt.Sprintf("%d_%s", mailID, decodedFilename)
			filePath := filepath.Join(baseDir, fileName)

			if outFile, err := os.Create(filePath); err == nil {
				written, copyErr := io.Copy(outFile, entity.Body)
				outFile.Close() // 立即关闭文件句柄

				if copyErr != nil {
					log.Printf("⚠️  写入大附件文件失败: %v", copyErr)
					os.Remove(filePath)
					return
				}

				cacheExpire := time.Now().Add(f.config.IMAP.GetCacheExpireDuration())
				att := models.Attachment{
					MailID:      mailID,
					Filename:    decodedFilename,
					ContentType: mediaType,
					Size:        written,
					FilePath:    filePath,
					IMAPUID:     imapUID,
					PartID:      partID,
					IsCached:    true,
					CacheExpire: &cacheExpire,
					CreatedAt:   time.Now(),
				}
				result.Attachments = append(result.Attachments, att)
				log.Printf("📎 大附件已缓存到本地: %s (%d bytes) [Part=%s]", decodedFilename, written, partID)
			} else {
				log.Printf("⚠️  创建附件文件失败: %v", err)
			}
		} else {
			// 小附件或无 mailID 时：读入内存并存 DB BLOB
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, entity.Body); err == nil {
				att := models.Attachment{
					MailID:      mailID,
					Filename:    decodedFilename,
					ContentType: mediaType,
					Size:        int64(buf.Len()),
					Content:     buf.Bytes(),
					IMAPUID:     imapUID,
					PartID:      partID,
					IsCached:    true, // DB BLOB 视为已缓存
					CreatedAt:   time.Now(),
				}
				result.Attachments = append(result.Attachments, att)
			}
		}
		return
	}

	// 处理文本内容
	charset := strings.ToLower(strings.Trim(params["charset"], "\"' \t"))
	log.Printf("🔍 [CHARSET-DEBUG] mediaType=%s, raw_charset=%q, trimmed_charset=%q", mediaType, params["charset"], charset)
	switch {
	case strings.HasPrefix(mediaType, "text/plain"):
		textData, _ := io.ReadAll(entity.Body)
		log.Printf("🔍 [CHARSET-DEBUG] text/plain: data_len=%d, first_40bytes_hex=%x, isUTF8=%v, will_decode_with=%q",
			len(textData), textData[:min(40, len(textData))], isUTF8(string(textData)), charset)
		result.TextBody = decodeTextContentWithCharset(textData, charset)
		log.Printf("🔍 [CHARSET-DEBUG] textBody after decode: len=%d, first_80chars=%q",
			len(result.TextBody), truncateString(result.TextBody, 80))
	case strings.HasPrefix(mediaType, "text/html"):
		htmlData, _ := io.ReadAll(entity.Body)
		log.Printf("🔍 [CHARSET-DEBUG] text/html: data_len=%d, first_40bytes_hex=%x, isUTF8=%v, will_decode_with=%q",
			len(htmlData), htmlData[:min(40, len(htmlData))], isUTF8(string(htmlData)), charset)
		result.HTMLBody = decodeTextContentWithCharset(htmlData, charset)
		log.Printf("🔍 [CHARSET-DEBUG] htmlBody after decode: len=%d, first_80chars=%q",
			len(result.HTMLBody), truncateString(result.HTMLBody, 80))
	}
}

// checkDiskSpace 检查磁盘剩余空间是否足够
func (f *Fetcher) checkDiskSpace(requiredBytes int64) bool {
	freeBytes, err := getDiskFreeSpace()
	if err != nil {
		log.Printf("⚠️  获取磁盘信息失败: %v", err)
		return true // 无法获取时默认允许
	}

	minFree := f.config.IMAP.GetMinDiskFree()

	// 剩余空间必须 > 最小保留空间 + 本次写入所需空间
	return freeBytes > (minFree + requiredBytes)
}

// --- 工具函数 ---

// extractAddressList 从 go-message/mail 地址列表中提取格式化的地址字符串
func extractAddressList(addrs []*mail.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if addr.Name != "" {
			parts = append(parts, fmt.Sprintf("%s <%s>", addr.Name, addr.Address))
		} else {
			parts = append(parts, addr.Address)
		}
	}
	return strings.Join(parts, ", ")
}

// extractIMAPAddressList 从 go-imap/v2 地址列表中提取格式化的地址字符串
func extractIMAPAddressList(addrs []imap.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if addr.Name != "" {
			parts = append(parts, fmt.Sprintf("%s <%s>", decodeHeader(addr.Name), addr.Addr()))
		} else {
			parts = append(parts, addr.Addr())
		}
	}
	return strings.Join(parts, ", ")
}

// decodeHeader 解码 RFC 2047 编码的头部字段
func decodeHeader(raw string) string {
	if raw == "" {
		return raw
	}
	dec := &mime.WordDecoder{
		CharsetReader: charsetReader,
	}
	decoded, err := dec.DecodeHeader(raw)
	if err != nil {
		log.Printf("[WARN] decodeHeader failed: %v, raw: %s", err, truncateString(raw, 80))
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

// charsetReader 为 mime.WordDecoder 提供字符集解码支持（支持 GBK、GB2312、GB18030 等）
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

// decodeTextContentWithCharset 根据 MIME 声明的 charset 解码文本内容
func decodeTextContentWithCharset(data []byte, charset string) string {
	if len(data) == 0 {
		return ""
	}

	// 规范化：去除可能的引号和空白
	charset = strings.ToLower(strings.Trim(charset, "\"' \t"))

	// 根据 Content-Type 中声明的字符集选择解码方式
	switch {
	case charset == "", charset == "utf-8", charset == "utf8", charset == "us-ascii":
		// UTF-8 或未声明，直接返回
		return string(data)
	case charset == "gb18030", charset == "gbk", charset == "gb2312", charset == "gb2312-80":
		if decoded, err := simplifiedchinese.GBK.NewDecoder().String(string(data)); err == nil {
			return decoded
		}
		log.Printf("[WARN] GBK/GB18030 解码失败（%d 字节），回退到原始数据", len(data))
		return string(data)
	case charset == "iso-8859-1", charset == "latin-1", charset == "latin1":
		if decoded, err := unicode.UTF8.NewDecoder().String(string(data)); err == nil {
			return decoded
		}
		return string(data)
	default:
		// 未知字符集：先尝试 UTF-8，再尝试 GBK
		str := string(data)
		if isUTF8(str) {
			return str
		}
		if decoded, err := simplifiedchinese.GBK.NewDecoder().String(str); err == nil {
			log.Printf("[INFO] 未知charset=%q 尝试GBK解码成功", charset)
			return decoded
		}
		log.Printf("[WARN] 未知charset=%q 且非UTF-8/GBK，返回原始数据 (%d字节)", charset, len(data))
		return str
	}
}

// decodeTextContent 自动检测字符集并解码文本内容（保留用于 MIME 解析失败时的回退）
func decodeTextContent(data []byte) string {
	return decodeTextContentWithCharset(data, "")
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

// decodeCharset 使用 golang.org/x/text 进行字符集解码
func decodeCharset(data []byte, charset string) (string, error) {
	switch strings.ToLower(charset) {
	case "gbk", "gb2312", "gb18030":
		result, err := simplifiedchinese.GBK.NewDecoder().String(string(data))
		if err != nil {
			return "", fmt.Errorf("gbk decode error: %w", err)
		}
		return result, nil
	case "iso-8859-1", "latin-1":
		result, err := unicode.UTF8.NewDecoder().String(string(data))
		if err != nil {
			return "", fmt.Errorf("iso-8859-1 decode error: %w", err)
		}
		return result, nil
	default:
		return "", fmt.Errorf("unsupported charset: %s", charset)
	}
}

// truncateString 截断字符串用于日志输出
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// shouldSync 根据 SyncMode 判断是否需要同步该邮件
// syncMode: unread(只同步未读), all(全部), recent(最近N天)
// cutoffTime: 最近N天的截止时间
func shouldSync(buf *imapclient.FetchMessageBuffer, syncMode string, cutoffTime time.Time) bool {
	switch syncMode {
	case "unread":
		// 只同步未读邮件（没有 Seen 标志）
		for _, flag := range buf.Flags {
			if flag == imap.FlagSeen {
				return false // 已读，跳过
			}
		}
		return true // 未读，同步
	case "recent":
		// 同步最近 N 天的邮件
		if !buf.InternalDate.IsZero() && buf.InternalDate.Before(cutoffTime) {
			return false // 太早了，跳过
		}
		return true
	default:
		// "all" 或其他未知值，全部同步
		return true
	}
}
