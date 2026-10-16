// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"fmt"
	"io"
	"log"
	"magicmail/config"
	"magicmail/imap"
	"magicmail/models"
	"magicmail/services"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// AttachmentHandler 附件 API 处理器（支持混合缓存模式）
type AttachmentHandler struct {
	service *services.AttachmentService
	config  *config.Config
}

// NewAttachmentHandler 创建附件 Handler 实例
func NewAttachmentHandler(svc *services.AttachmentService) *AttachmentHandler {
	return &AttachmentHandler{
		service: svc,
		config:  config.Load(),
	}
}

// Download 下载附件（支持三种模式）：
//
//	1. 已缓存（DB BLOB / 文件系统）→ 直接返回
//	2. 懒加载（有 IMAP 元数据但未缓存）→ 从 IMAP 流式传输 + 异步缓存
//	3. 无元数据 → 返回错误
//
// @Summary 下载附件（流式/懒加载）
// @Tags 附件管理
// @Produce application/octet-stream
// @Param id path int true "附件ID"
// @Success 200 {file} binary "附件文件"
// @Router /api/v1/attachments/{id}/download [get]
func (h *AttachmentHandler) Download(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的 ID"})
	}

	att, err := h.service.GetByID(uint(id), getUserID(c))
	if err != nil {
		if err.Error() == "POP3 附件暂不可用（无缓存且不支持按需下载）" {
			return c.Status(503).JSON(fiber.Map{
				"error": "附件暂不可用（POP3 协议不支持按需下载，请重新同步邮件）",
				"hint":  "POP3 协议不支持部分获取，该附件可能在同步时丢失",
			})
		}
		return c.Status(404).JSON(fiber.Map{"error": "附件不存在"})
	}

	// ========== 模式1：已缓存 → 直接返回 ==========
	if att.IsCached {
		c.Set("Content-Type", att.ContentType)
		c.Set("Content-Disposition", `attachment; filename="`+att.Filename+`"`)
		c.Set("Content-Length", strconv.FormatInt(att.Size, 10))

		if att.IsFileBased() {
			return c.SendFile(att.FilePath)
		}
		if len(att.Content) > 0 {
			return c.Send(att.Content)
		}
		// 标记为缓存但实际无内容，降级到懒加载
		log.Printf("⚠️ 附件标记为缓存但无内容 (ID=%d)，尝试从 IMAP 获取", id)
	}

	// ========== 模式2：懒加载 → 从 IMAP 流式传输 ==========
	if att.IMAPUID > 0 && att.PartID != "" {
		return h.streamFromIMAP(c, att)
	}

	// ========== 模式3：异常状态 ==========
	return c.Status(503).JSON(fiber.Map{
		"error": "附件暂不可用（无缓存且无法从源服务器获取）",
	})
}

// streamFromIMAP 从 IMAP 服务器流式传输附件到 HTTP 响应
func (h *AttachmentHandler) streamFromIMAP(c *fiber.Ctx, att *models.Attachment) error {
	log.Printf("📥 [DEBUG] 开始懒加载下载 (ID=%d, MailID=%d, UID=%d, PartID=%q)", att.ID, att.MailID, att.IMAPUID, att.PartID)

	// 获取邮件关联的账号信息
	mailObj, account, err := h.service.GetMailAndAccount(att.MailID)
	if err != nil || account == nil {
		log.Printf("❌ [DEBUG] 获取邮件账号失败 (MailID=%d): %v", att.MailID, err)
		return c.Status(500).JSON(fiber.Map{
			"error":  "无法获取邮件账号信息",
			"detail": err.Error(),
		})
	}
	log.Printf("📥 [DEBUG] 账号信息获取成功: %s@%s", account.Email, account.ImapHost)

	// 创建 IMAP 连接（临时的，仅用于本次下载）
	imapClient, err := imap.NewIMAPClient(account, h.config)
	if err != nil {
		log.Printf("❌ [DEBUG] IMAP 连接失败: %v", err)
		return c.Status(502).JSON(fiber.Map{
			"error":  "无法连接 IMAP 服务器",
			"detail": err.Error(),
		})
	}
	defer imapClient.Close()

	// 认证
	if err := imapClient.Authenticate(); err != nil {
		log.Printf("❌ [DEBUG] IMAP 认证失败: %v", err)
		return c.Status(502).JSON(fiber.Map{
			"error":  "IMAP 认证失败",
			"detail": err.Error(),
		})
	}

	// 选择对应文件夹
	var mailboxName string
	switch mailObj.Folder {
	case "sent":
		mailboxName = "Sent"
	default:
		mailboxName = "INBOX"
	}
	if _, err := imapClient.SelectMailbox(mailboxName); err != nil {
		log.Printf("❌ [DEBUG] 选择文件夹 %s 失败: %v", mailboxName, err)
		return c.Status(502).JSON(fiber.Map{
			"error":  "无法选择 IMAP 文件夹",
			"detail": err.Error(),
		})
	}

	log.Printf("📥 [DEBUG] 已选择文件夹: %s，开始 FETCH BODY[%s]", mailboxName, att.PartID)

	// 从 IMAP 流式获取指定 MIME 部分
	reader, contentType, size, err := h.service.StreamFromIMAP(imapClient.Client, att.IMAPUID, att.PartID)
	if err != nil {
		log.Printf("❌ [DEBUG] StreamFromIMAP 失败: %v", err)
		return c.Status(502).JSON(fiber.Map{
			"error":  "从 IMAP 服务器获取附件失败",
			"detail": err.Error(),
		})
	}
	defer reader.Close()

	log.Printf("📥 [DEBUG] StreamFromIMAP 成功: contentType=%s, size=%d", contentType, size)

	// 设置响应头（RFC 6266 兼容：支持特殊字符文件名）
	finalContentType := contentType
	if finalContentType == "" {
		finalContentType = att.ContentType
	}
	if finalContentType == "" {
		finalContentType = "application/octet-stream"
	}

	c.Set("Content-Type", finalContentType)
	
	// ⭐ 使用 URL 编码的 filename* (RFC 5987) 处理特殊字符
	safeFilename := sanitizeFilename(att.Filename)
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, 
		safeFilename, url.PathEscape(att.Filename)))
	
	if size > 0 {
		c.Set("Content-Length", strconv.FormatInt(size, 10))
	}
	c.Set("X-Cache-Status", "miss") // 告诉前端这是实时流

	// ⭐ 自动缓存：检查是否需要同时写入本地
	var teeReader io.Reader = reader
	if h.service.ShouldAutoCache(att) {
		// 使用 TeeReader 在流式传输的同时写入缓存管道
		pipeReader, pipeWriter := io.Pipe()
		teeReader = io.TeeReader(reader, pipeWriter)

		// 异步执行缓存写入（不阻塞 HTTP 响应）
		go func() {
			defer pipeWriter.Close()
			if err := h.service.CacheAttachment(att, pipeReader); err != nil {
				log.Printf("❌ 自动缓存失败 (ID=%d): %v", att.ID, err)
			}
		}()

		log.Printf("📥 附件 %d 启用自动缓存（流式传输+本地缓存）", att.ID)
	}

	// ⭐ 流式写入 HTTP 响应（零内存拷贝）
	log.Printf("📥 [DEBUG] 开始写入 HTTP 响应...")
	// 使用 c.SendStream 进行流式传输，避免 BodyWriter 潜在的问题
	// SendStream 第二个参数为 int（-1 表示未知大小，不设置 Content-Length）
	var streamSize int = -1
	if size > 0 && size <= int64(2<<30) { // 限制在 int 范围内
		streamSize = int(size)
	}
	if err := c.SendStream(teeReader, streamSize); err != nil {
		log.Printf("❌ [DEBUG] 写入 HTTP 响应失败: %v", err)
		return err
	}
	log.Printf("✅ [DEBUG] 下载完成: ID=%d", att.ID)

	return nil
}

// ListByMailID 获取邮件的附件列表（含缓存状态）
//
// @Summary 获取邮件附件列表
// @Tags 附件管理
// @Produce json
// @Param mail_id path int true "邮件ID"
// @Success 200 {array} models.AttachmentResp
// @Router /api/v1/attachments/mail/{mail_id} [get]
func (h *AttachmentHandler) ListByMailID(c *fiber.Ctx) error {
	mailID, err := c.ParamsInt("mail_id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的邮件 ID"})
	}

	attachments, err := h.service.GetByMailID(uint(mailID), getUserID(c))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  "获取附件列表失败",
			"detail": err.Error(),
		})
	}
	return c.JSON(attachments)
}

// sanitizeFilename 生成安全的 HTTP header 文件名（只保留 ASCII 安全字符）
func sanitizeFilename(filename string) string {
	var safe strings.Builder
	for _, r := range filename {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '.' || r == '-' || r == '_' || r == ' ' {
			safe.WriteRune(r)
		} else {
			safe.WriteRune('_')
		}
	}
	result := strings.TrimSpace(safe.String())
	if result == "" {
		return "attachment"
	}
	return result
}
