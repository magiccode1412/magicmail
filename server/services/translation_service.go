// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"magicmail/crypto"
	"magicmail/models"

	"gorm.io/gorm"
)

// myMemoryEndpoint MyMemory 翻译 API 端点
const myMemoryEndpoint = "https://api.mymemory.translated.net/get"

// maxChunkRunes 单次请求的最大字符数（MyMemory 单条 query 上限为 500 字符，长邮件需切分）
const maxChunkRunes = 500

// translationCacheTTL 翻译结果缓存时长（避免因重复打开同一封邮件而重复消耗额度）
const translationCacheTTL = 7 * 24 * time.Hour

// TranslationService 翻译服务（后端代理 MyMemory，统一处理凭据、配额、缓存、长文本分块）
type TranslationService struct {
	db         *gorm.DB
	httpClient *http.Client
	mu         sync.Mutex
	cache      map[string]cacheItem
}

type cacheItem struct {
	text string
	exp  int64
}

// TranslateResult 翻译结果
type TranslateResult struct {
	TranslatedText string `json:"translated_text"`
	DetectedSource string `json:"detected_source,omitempty"` // 自动检测到的源语言（如 en-GB）
	TargetLang     string `json:"target_lang"`
	QuotaFinished  bool   `json:"quota_finished"` // 今日免费额度是否已耗尽
}

// NewTranslationService 创建翻译服务实例
func NewTranslationService(db *gorm.DB) *TranslationService {
	return &TranslationService{
		db:         db,
		httpClient: &http.Client{Timeout: 20 * time.Second},
		cache:      make(map[string]cacheItem),
	}
}

// GetSettings 获取（不存在则创建默认）用户的翻译设置
func (s *TranslationService) GetSettings(userID uint) (*models.UserTranslationSetting, error) {
	var setting models.UserTranslationSetting
	err := s.db.Where("user_id = ?", userID).First(&setting).Error
	if err == gorm.ErrRecordNotFound {
		setting = models.UserTranslationSetting{UserID: userID, TargetLang: "zh-CN"}
		if cerr := s.db.Create(&setting).Error; cerr != nil {
			return nil, cerr
		}
		return &setting, nil
	}
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// SaveSettings 保存用户翻译设置：targetLang 明文存储；mmEmail/mmKey 留空表示「保持原配置不变」
// （避免用户仅修改目标语言时把已配置的凭据误清空）。mmKey 以 AES-256-GCM 密文存储。
func (s *TranslationService) SaveSettings(userID uint, targetLang, mmEmail, mmKey string) error {
	setting, err := s.GetSettings(userID)
	if err != nil {
		return err
	}
	setting.TargetLang = targetLang
	if mmEmail != "" {
		setting.MMEmail = mmEmail
	}
	if mmKey != "" {
		enc, eerr := crypto.Encrypt(mmKey)
		if eerr != nil {
			return fmt.Errorf("加密 key 失败: %w", eerr)
		}
		setting.MMKeyEnc = enc
	}
	return s.db.Save(&setting).Error
}

// Translate 翻译文本：自动选择目标语言、可选源语言（空=自动检测）、分块翻译并合并、命中缓存直接返回
func (s *TranslationService) Translate(userID uint, text, sourceLang string) (*TranslateResult, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("没有可翻译的内容")
	}

	setting, err := s.GetSettings(userID)
	if err != nil {
		return nil, err
	}
	target := setting.TargetLang
	if target == "" {
		target = "zh-CN"
	}

	langpair := target
	if sourceLang != "" {
		langpair = sourceLang + "|" + target
	} else {
		langpair = "Autodetect|" + target
	}

	// 缓存命中
	cacheKey := fmt.Sprintf("%x", sha256.Sum256([]byte(langpair+"|"+text)))
	if cached, ok := s.getCache(cacheKey); ok {
		return &TranslateResult{TranslatedText: cached, TargetLang: target, DetectedSource: sourceLang}, nil
	}

	chunks := splitChunks(text, maxChunkRunes)
	var sb strings.Builder
	detected := ""
	for i, chunk := range chunks {
		translated, src, qf, cerr := s.translateChunk(chunk, langpair, setting.MMEmail, setting.MMKeyEnc)
		if cerr != nil {
			return nil, cerr
		}
		if i == 0 && detected == "" && src != "" {
			detected = src
		}
		sb.WriteString(translated)
		if i < len(chunks)-1 {
			sb.WriteString("\n\n")
		}
		_ = qf
	}

	result := &TranslateResult{
		TranslatedText: sb.String(),
		DetectedSource: detected,
		TargetLang:     target,
		QuotaFinished:  false,
	}
	s.setCache(cacheKey, sb.String())
	return result, nil
}

// flexInt 兼容 MyMemory 将 responseStatus 既可能返回数字又可能返回字符串的情况
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := bytes.TrimSpace(b)
	// 去除可能的引号（当 API 以 "200" 形式返回时）
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	n, err := strconv.Atoi(string(s))
	if err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}

// mmResponse MyMemory API 响应结构（仅取所需字段）
type mmResponse struct {
	ResponseData struct {
		TranslatedText string  `json:"translatedText"`
		Match          float64 `json:"match"`
	} `json:"responseData"`
	QuotaFinished   bool            `json:"quotaFinished"`
	ResponseStatus  flexInt         `json:"responseStatus"`
	ResponseDetails string          `json:"responseDetails"`
	Matches         json.RawMessage `json:"matches"`
}

// translateChunk 调用 MyMemory 翻译单个文本块
func (s *TranslationService) translateChunk(text, langpair, de, keyEnc string) (translated, source string, quotaFinished bool, err error) {
	form := url.Values{}
	form.Set("q", text)
	form.Set("langpair", langpair)
	form.Set("mt", "1") // 仅使用机器翻译，避免引入不合适的翻译记忆库结果
	if de != "" {
		form.Set("de", de)
	}
	if keyEnc != "" {
		dec, derr := crypto.Decrypt(keyEnc)
		if derr != nil {
			return "", "", false, fmt.Errorf("解密 key 失败: %w", derr)
		}
		form.Set("key", dec)
	}

	req, rerr := http.NewRequest("POST", myMemoryEndpoint, strings.NewReader(form.Encode()))
	if rerr != nil {
		return "", "", false, rerr
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, derr := s.httpClient.Do(req)
	if derr != nil {
		return "", "", false, fmt.Errorf("翻译服务请求失败: %w", derr)
	}
	defer resp.Body.Close()

	body, berr := io.ReadAll(resp.Body)
	if berr != nil {
		return "", "", false, fmt.Errorf("读取翻译响应失败: %w", berr)
	}

	var mm mmResponse
	if jerr := json.Unmarshal(body, &mm); jerr != nil {
		return "", "", false, fmt.Errorf("翻译服务响应解析失败: %w", jerr)
	}

	if int(mm.ResponseStatus) != 200 {
		detail := mm.ResponseDetails
		if detail == "" {
			detail = fmt.Sprintf("翻译服务返回状态 %d", int(mm.ResponseStatus))
		}
		return "", "", false, fmt.Errorf("%s", detail)
	}

	if mm.QuotaFinished {
		return "", "", true, fmt.Errorf("今日免费翻译额度已用完，请在「设置 → 翻译设置」中配置 MyMemory 邮箱或 Key")
	}

	translated = mm.ResponseData.TranslatedText
	if len(mm.Matches) > 0 {
		var m []struct {
			Source string `json:"source"`
		}
		if json.Unmarshal(mm.Matches, &m) == nil && len(m) > 0 {
			source = m[0].Source
		}
	}
	return translated, source, false, nil
}

// splitChunks 将长文本按段落/行切分为若干不超过 maxRunes 的块，尽量保持段落完整性
func splitChunks(text string, maxRunes int) []string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return []string{text}
	}

	lines := strings.Split(text, "\n")
	var chunks []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
	}
	for _, line := range lines {
		lineRunes := []rune(line)
		if len(lineRunes) > maxRunes {
			// 单行超长：先 flush 已有内容，再对单行硬切
			flush()
			for i := 0; i < len(lineRunes); i += maxRunes {
				end := i + maxRunes
				if end > len(lineRunes) {
					end = len(lineRunes)
				}
				chunks = append(chunks, string(lineRunes[i:end]))
			}
			continue
		}
		if current.Len() > 0 && current.Len()+len(lineRunes)+1 > maxRunes {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}
	flush()
	if len(chunks) == 0 {
		return []string{text}
	}
	return chunks
}

func (s *TranslationService) getCache(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.cache[key]
	if !ok {
		return "", false
	}
	if item.exp < time.Now().Unix() {
		delete(s.cache, key)
		return "", false
	}
	return item.text, true
}

func (s *TranslationService) setCache(key, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = cacheItem{text: text, exp: time.Now().Add(translationCacheTTL).Unix()}
}
