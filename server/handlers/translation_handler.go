// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"regexp"
	"strings"

	"magicmail/models"
	"magicmail/services"

	"github.com/gofiber/fiber/v2"
)

// TranslationHandler 翻译相关 API 处理器
type TranslationHandler struct {
	service     *services.TranslationService
	mailService *services.MailService
}

// NewTranslationHandler 创建翻译 Handler 实例
func NewTranslationHandler(svc *services.TranslationService, mailSvc *services.MailService) *TranslationHandler {
	return &TranslationHandler{service: svc, mailService: mailSvc}
}

// allowedTargetLangs 支持的目标语言白名单（MyMemory 语言代码）
var allowedTargetLangs = map[string]bool{
	"zh-CN": true, "zh-TW": true, "en": true, "ja": true, "ko": true,
	"fr": true, "de": true, "es": true, "ru": true, "pt": true, "it": true,
	"vi": true, "th": true, "id": true, "ms": true, "ar": true, "nl": true,
	"pl": true, "tr": true,
}

// GetSettings 获取当前用户的翻译设置（不返回敏感凭据明文）
//
//	@Summary		获取翻译设置
//	@Tags			翻译
//	@Produce		json
//	@Success		200 {object} map[string]interface{}
//	@Router			/api/v1/settings/translation [get]
func (h *TranslationHandler) GetSettings(c *fiber.Ctx) error {
	userID := getUserID(c)
	setting, err := h.service.GetSettings(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "获取翻译设置失败", "detail": err.Error()})
	}
	return c.JSON(fiber.Map{
		"target_lang": setting.TargetLang,
		"has_email":   setting.MMEmail != "",
		"has_key":     setting.MMKeyEnc != "",
	})
}

// SaveSettings 保存当前用户的翻译设置
//
//	@Summary		保存翻译设置
//	@Tags			翻译
//	@Accept			json
//	@Produce		json
//	@Param			body body map[string]string true "{ target_lang, mm_email, mm_key }"
//	@Success		200 {object} map[string]interface{}
//	@Router			/api/v1/settings/translation [put]
func (h *TranslationHandler) SaveSettings(c *fiber.Ctx) error {
	var req struct {
		TargetLang string `json:"target_lang"`
		MMEmail    string `json:"mm_email"`
		MMKey      string `json:"mm_key"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}
	if req.TargetLang == "" {
		req.TargetLang = "zh-CN"
	}
	if !allowedTargetLangs[req.TargetLang] {
		return c.Status(400).JSON(fiber.Map{"error": "不支持的目标语言: " + req.TargetLang})
	}
	// 出于安全考虑：前端只在变更时提交明文 key；留空表示「保持原有配置不变」。
	if err := h.service.SaveSettings(getUserID(c), req.TargetLang, req.MMEmail, req.MMKey); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "保存翻译设置失败", "detail": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "message": "翻译设置已保存"})
}

// TranslateText 翻译任意文本
//
//	@Summary		翻译文本
//	@Tags			翻译
//	@Accept			json
//	@Produce		json
//	@Param			body body map[string]string true "{ text, source_lang }"
//	@Success		200 {object} services.TranslateResult
//	@Router			/api/v1/translate [post]
func (h *TranslationHandler) TranslateText(c *fiber.Ctx) error {
	var req struct {
		Text       string `json:"text"`
		SourceLang string `json:"source_lang"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数解析失败"})
	}
	result, err := h.service.Translate(getUserID(c), req.Text, req.SourceLang)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "翻译失败", "detail": err.Error()})
	}
	return c.JSON(result)
}

// TranslateMail 翻译指定邮件正文（校验归属用户；优先纯文本，否则从 HTML 提取纯文本）
//
//	@Summary		翻译邮件正文
//	@Tags			翻译
//	@Accept			json
//	@Produce		json
//	@Param			id path int true "邮件ID"
//	@Success		200 {object} services.TranslateResult
//	@Router			/api/v1/mails/{id}/translate [post]
func (h *TranslationHandler) TranslateMail(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "无效的邮件 ID"})
	}
	userID := getUserID(c)
	mail, err := h.mailService.GetByID(uint(id), userID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "邮件不存在"})
	}

	text := extractTranslateText(mail)
	if strings.TrimSpace(text) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "该邮件没有可翻译的正文内容"})
	}

	var sr struct {
		SourceLang string `json:"source_lang"`
	}
	_ = c.BodyParser(&sr)

	result, err := h.service.Translate(userID, text, sr.SourceLang)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "翻译失败", "detail": err.Error()})
	}
	return c.JSON(result)
}

// extractTranslateText 提取可翻译的纯文本：优先 text_body，否则从 html_body 剥离标签
func extractTranslateText(mail *models.MailResponse) string {
	if mail.TextBody != nil && strings.TrimSpace(*mail.TextBody) != "" {
		return *mail.TextBody
	}
	if mail.HTMLBody != nil && strings.TrimSpace(*mail.HTMLBody) != "" {
		return stripHTMLLite(*mail.HTMLBody)
	}
	return ""
}

// stripHTMLLite 简易 HTML 转纯文本：移除 style/script 与标签、解码常见实体、折叠空白
func stripHTMLLite(html string) string {
	re := regexp.MustCompile(`(?is)<(style|script)[^>]*>.*?</\1>`)
	s := re.ReplaceAllString(html, " ")
	re2 := regexp.MustCompile(`(?is)<[^>]+>`)
	s = re2.ReplaceAllString(s, " ")
	repl := []string{"&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'"}
	for i := 0; i < len(repl); i += 2 {
		s = strings.ReplaceAll(s, repl[i], repl[i+1])
	}
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}
