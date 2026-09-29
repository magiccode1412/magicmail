<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="mail-reader-view">
    <!-- 加载中 -->
    <div v-if="loading" class="reader-loading">
      <div class="spinner"></div>
      <p>加载邮件内容...</p>
    </div>

    <!-- 错误状态 -->
    <div v-else-if="error" class="reader-error">
      <EmptyState icon="error" :title="'邮件不存在或已删除'" :description="error" />
      <router-link to="/mails" class="btn btn-primary mt-md">返回收件箱</router-link>
    </div>

    <!-- 邮件内容 -->
    <template v-else-if="mail">
      <!-- 操作栏 -->
      <div class="reader-toolbar">
        <button class="btn btn-ghost btn-sm" @click="$router.back()">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path d="M10 3L5 8L10 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          返回
        </button>
        <div class="toolbar-actions">
          <button
            class="btn btn-ghost btn-sm"
            :class="{ active: !mail.is_read }"
            @click="toggleRead"
            :title="mail.is_read ? '标记未读' : '标记已读'"
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <rect x="2.5" y="4.5" width="11" height="8.5" rx="1.5" stroke="currentColor" stroke-width="1.4"/>
              <path d="M2.5 6L8 9.5L13.5 6" stroke="currentColor" stroke-width="1.4"/>
            </svg>
            {{ mail.is_read ? '标未读' : '标已读' }}
          </button>
          <button class="btn btn-ghost btn-sm" @click="handleReply" title="回复">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M2 8h12M9 4l4 4-4 4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            回复
          </button>
          <button
            class="btn btn-ghost btn-sm"
            :class="{ active: showTranslation }"
            @click="doTranslate"
            :disabled="translating"
            :title="translating ? '翻译中...' : '翻译正文'"
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M2.5 4.5h7M8 2.5l2 2-2 2M13.5 11.5h-7M8 13.5l-2-2 2-2" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            {{ translating ? '翻译中' : '翻译' }}
          </button>
          <button class="btn btn-ghost btn-sm danger-hover" @click="handleDelete" title="删除">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M3 4h10M5.33 4V2.67a1.33 1.33 0 0 1 1.34-1.34h2.66a1.33 1.33 0 0 1 1.34 1.34V4m2 0v9.33A1.33 1.33 0 0 1 11.34 15H4.67a1.33 1.33 0 0 1-1.34-1.34V4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            删除
          </button>
        </div>
      </div>

      <!-- 邮件头部信息 -->
      <section class="mail-header card">
        <h2 class="mail-subject">{{ mail.subject || '(无主题)' }}</h2>
        <div class="mail-meta">
          <!-- 发件人 -->
          <div class="meta-row meta-from">
            <span class="meta-avatar">{{ avatarLetter(mail.from) }}</span>
            <div class="meta-info">
              <strong class="sender-name">{{ extractName(mail.from) || '未知发件人' }}</strong>
              <span class="sender-email">&lt;{{ extractEmail(mail.from) }}&gt;</span>
            </div>
            <span class="meta-time">
              {{ formatTime(mail.sent_at) }}
            </span>
          </div>

          <div class="meta-divider"></div>

          <!-- 收件人 / 抄送 -->
          <div class="meta-row meta-recipients">
            <span class="meta-label">收件人：</span>
            <span class="meta-value text-muted">{{ formatEmails(mail.to) }}</span>
          </div>
          <div v-if="formatEmails(mail.cc)" class="meta-row meta-recipients">
            <span class="meta-label">抄送：</span>
            <span class="meta-value text-muted">{{ formatEmails(mail.cc) }}</span>
          </div>
        </div>
      </section>

      <!-- 邮件正文 -->
      <section class="mail-body card">
        <MailContent
          :html-body="mail.html_body"
          :text-body="mail.text_body"
        />
      </section>

      <!-- 翻译结果 -->
      <section v-if="showTranslation" class="mail-translation card">
        <div class="translation-header">
          <strong>译文</strong>
          <div class="translation-meta">
            <span v-if="transDetected" class="translation-lang">检测到原文：{{ transDetected }}</span>
            <span class="translation-lang">→ {{ targetLangLabel }}</span>
            <button class="btn btn-ghost btn-sm" @click="closeTranslation">收起</button>
          </div>
        </div>
        <div v-if="translating" class="translation-loading">
          <span class="spinner-xs"></span><span>正在翻译...</span>
        </div>
        <pre v-else-if="translateError" class="translation-error">{{ translateError }}</pre>
        <pre v-else class="translation-text">{{ translatedText }}</pre>
      </section>

      <!-- 附件列表 -->
      <section v-if="mail.attachments && mail.attachments.length > 0" class="mail-attachments card">
        <AttachmentList :attachments="mail.attachments" />
      </section>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMailStore } from '@/stores/mailStore'
import MailContent from '../components/MailContent.vue'
import AttachmentList from '../components/AttachmentList.vue'
import EmptyState from '../components/EmptyState.vue'
import * as translateApi from '@/api/translate'

const route = useRoute()
import { useToast } from '@/composables/useToast'

const toast = useToast()
const router = useRouter()
const mailStore = useMailStore()

const loading = ref(true)
const error = ref(null)
const mail = ref(null)

// --- 翻译状态 ---
const translatedText = ref('')
const transDetected = ref('')
const targetLangLabel = ref('')
const translating = ref(false)
const translateError = ref('')
const showTranslation = ref(false)

const targetLangMap = {
  'zh-CN': '简体中文', 'zh-TW': '繁体中文', 'en': '英语', 'ja': '日语', 'ko': '韩语',
  'fr': '法语', 'de': '德语', 'es': '西班牙语', 'ru': '俄语', 'pt': '葡萄牙语',
  'it': '意大利语', 'vi': '越南语', 'th': '泰语', 'id': '印尼语', 'ms': '马来语',
  'ar': '阿拉伯语', 'nl': '荷兰语', 'pl': '波兰语', 'tr': '土耳其语',
}

async function doTranslate() {
  if (!mail.value) return
  translating.value = true
  translateError.value = ''
  translatedText.value = ''
  transDetected.value = ''
  showTranslation.value = true
  try {
    const res = await translateApi.translateMail(mail.value.id)
    translatedText.value = res.translated_text
    transDetected.value = res.detected_source || ''
    targetLangLabel.value = targetLangMap[res.target_lang] || res.target_lang || ''
  } catch (e) {
    translateError.value = e.message || '翻译失败'
  } finally {
    translating.value = false
  }
}

function closeTranslation() {
  showTranslation.value = false
  translatedText.value = ''
  translateError.value = ''
  transDetected.value = ''
}

// --- 数据获取 ---
onMounted(async () => {
  const id = Number(route.params.id)
  try {
    mail.value = await mailStore.fetchMailDetail(id)
  } catch (e) {
    error.value = e.message || '邮件加载失败'
  } finally {
    loading.value = false
  }
})

// --- 格式化邮箱字段（解析 JSON 数组） ---
function formatEmails(val) {
  if (!val || val === 'null' || val === '[]') return ''
  const trimmed = String(val).trim()
  if (trimmed.startsWith('[')) {
    try {
      const arr = JSON.parse(trimmed)
      if (Array.isArray(arr) && arr.length > 0) return arr.join(', ')
      return ''
    } catch (_) { /* 不是合法 JSON */ }
  }
  return trimmed
}

// --- 操作 ---
async function toggleRead() {
  if (!mail.value) return
  await mailStore.markAsRead(mail.value.id, !mail.value.is_read)
  mail.value.is_read = !mail.value.is_read
}

async function handleDelete() {
  if (!await toast.confirm('确定要删除这封邮件吗？')) return
  try {
    const res = await mailStore.deleteMail(mail.value.id)
    if (res?.deleted_from_server) {
      toast.success('邮件已删除，并已从源服务器同步删除')
    } else if (res?.server_delete_error) {
      toast.warning('邮件已本地删除，但源服务器同步失败: ' + res.server_delete_error)
    } else {
      toast.success('邮件已删除')
    }
    router.push('/mails')
  } catch (e) {
    toast.error('删除失败: ' + e.message)
  }
}

function handleReply() {
  const fromEmail = extractEmail(mail.value.from) || mail.value.from || ''
  const subject = (mail.value.subject || '').startsWith('Re: ')
    ? mail.value.subject
    : 'Re: ' + (mail.value.subject || '')
  const query = new URLSearchParams({
    replyTo: fromEmail,
    replySubject: subject,
    from: route.fullPath
  })
  router.push(`/compose?${query.toString()}`)
}

// --- 工具函数 ---
function avatarLetter(from) {
  if (!from) return '?'
  const name = extractName(from)
  return (name || from[0] || '?').charAt(0).toUpperCase()
}

function extractName(fromStr) {
  if (!fromStr) return ''
  // 格式: "名称 <email>" 或 "email"
  const match = fromStr.match(/^(.+?)\s*<.*>$/)
  return match ? match[1].trim() : ''
}

function extractEmail(fromStr) {
  if (!fromStr) return ''
  const match = fromStr.match(/<(.+?)>/)
  return match ? match[1] : fromStr.trim()
}

function formatTime(dateStr) {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now - date
  const diffMin = Math.floor(diffMs / 60000)

  if (diffMin < 1) return '刚刚'
  if (diffMin < 60) return `${diffMin} 分钟前`

  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr} 小时前`

  const diffDay = Math.floor(diffHr / 24)
  if (diffDay < 7) return `${diffDay} 天前`

  return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}
</script>

<style scoped>
.mail-reader-view {
  width: 100%;
}

/* ---- 加载/错误状态 ---- */
.reader-loading, .reader-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-md);
  padding: 80px 20px;
  color: var(--text-tertiary);
}

/* ---- 工具栏 ---- */
.reader-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-md);
  gap: var(--space-md);
  position: sticky;
  top: -1px;
  z-index: 10;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: var(--space-sm);
}

.toolbar-actions {
  display: flex;
  gap: var(--space-xs);
}

.danger-hover:hover { color: var(--error); }

/* ---- 邮件头部 ---- */
.mail-header { margin-bottom: var(--space-md); }
.mail-subject {
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  line-height: var(--line-height-tight);
  margin-bottom: var(--space-lg);
  word-break: break-word;
}

.mail-meta { display: flex; flex-direction: column; gap: var(--space-sm); }

.meta-from {
  display: flex;
  align-items: center;
  gap: var(--space-md);
}

.meta-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--primary-500), var(--primary-400));
  color: #fff;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  flex-shrink: 0;
}

.meta-info {
  flex: 1;
  min-width: 0;
}
.sender-name {
  font-size: var(--font-size-base);
  display: block;
}
.sender-email {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  display: block;
}

.meta-time {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  white-space: nowrap;
  flex-shrink: 0;
}

.meta-divider {
  height: 1px;
  background: var(--border-color);
  margin: var(--space-xs) 0;
}

.meta-recipients {
  display: flex;
  gap: var(--space-xs);
  font-size: var(--font-size-sm);
}
.meta-label { color: var(--text-secondary); white-space: nowrap; flex-shrink: 0; }
.meta-value { word-break: break-all; }

/* ---- 正文区域 ---- */
.mail-body {
  padding: var(--space-xl);
  overflow-x: auto;
  margin-bottom: var(--space-md);
}

/* ---- 附件区域 ---- */
.mail-attachments {
  padding: var(--space-lg);
}

/* ---- 翻译结果区域 ---- */
.mail-translation {
  padding: var(--space-lg);
  margin-bottom: var(--space-md);
}
.translation-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-md);
  flex-wrap: wrap;
}
.translation-header strong {
  font-size: var(--font-size-base);
  color: var(--text-primary);
}
.translation-meta {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex-wrap: wrap;
}
.translation-lang {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}
.translation-loading {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  color: var(--text-tertiary);
  font-size: var(--font-size-sm);
  padding: var(--space-md) 0;
}
.translation-text {
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
  line-height: var(--line-height-relaxed);
  color: var(--text-primary);
  margin: 0;
}
.translation-error {
  white-space: pre-wrap;
  word-break: break-word;
  font-size: var(--font-size-sm);
  color: var(--error, #dc2626);
  margin: 0;
}

@media (max-width: 768px) {
  .reader-toolbar {
    flex-direction: row-reverse;
  }
  
  .mail-subject {
    font-size: var(--font-size-xl);
  }

  .mail-body { padding: var(--space-lg); }
  .mail-attachments { padding: var(--space-md); }
}
</style>
