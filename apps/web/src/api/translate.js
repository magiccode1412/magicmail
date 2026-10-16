// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import request from './request'

// 获取当前用户的翻译设置（不含敏感凭据明文，仅返回是否已配置）
export function getTranslationSettings() {
  return request.get('/settings/translation')
}

// 保存当前用户的翻译设置
// data: { target_lang, mm_email, mm_key }（email/key 留空表示保持原配置不变）
export function saveTranslationSettings(data) {
  return request.put('/settings/translation', data)
}

// 翻译指定邮件正文
export function translateMail(mailId, sourceLang = '') {
  return request.post(`/mails/${mailId}/translate`, { source_lang: sourceLang })
}

// 翻译任意文本
export function translateText(text, sourceLang = '') {
  return request.post('/translate', { text, source_lang: sourceLang })
}
