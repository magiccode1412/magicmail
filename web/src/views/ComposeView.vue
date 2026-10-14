<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="compose-page">
    <div class="compose-header">
      <h2 class="page-title">写邮件</h2>
      <button class="btn btn-primary" @click="goBack" :disabled="sending">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
          <path d="M10 3L5 8L10 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        返回
      </button>
    </div>

    <!-- 发件人选择 -->
    <div class="form-section">
      <label class="form-label">发件账号</label>
      <select v-model="form.account_id" class="form-select">
        <option value="" disabled>选择发送邮箱</option>
        <option v-for="acc in accounts" :key="acc.id" :value="acc.id">
          {{ acc.name }} ({{ acc.email }})
        </option>
      </select>
    </div>

    <!-- 收件人 -->
    <div class="form-section">
      <label class="form-label">收件人</label>
      <div class="input-group">
        <input
          v-model="toInput"
          type="email"
          class="form-input"
          placeholder="多个收件人用逗号分隔，如: a@b.com, c@d.com"
          @keydown.enter.prevent="addToRecipient"
          @blur="addToRecipient"
        />
      </div>
    </div>

    <!-- 抄送（可折叠） -->
    <details class="cc-details" :open="showCc">
      <summary class="form-label cc-toggle" @click.prevent="showCc = !showCc">
        抄送 / 密送
        <span class="toggle-icon">{{ showCc ? '−' : '+' }}</span>
      </summary>
      <div class="cc-fields">
        <div class="input-group">
          <input
            v-model="ccInput"
            class="form-input"
            placeholder="抄送（可选）"
            @keydown.enter.prevent="addCc"
            @blur="addCc"
          />
        </div>
        <div class="input-group">
          <input
            v-model="bccInput"
            class="form-input"
            placeholder="密送（可选）"
            @keydown.enter.prevent="addBcc"
            @blur="addBcc"
          />
        </div>
      </div>
    </details>

    <!-- 已选收件人标签 -->
    <div v-if="form.to.length || form.cc.length || form.bcc.length" class="recipient-tags">
      <span
        v-for="(addr, idx) in form.to"
        :key="'to-'+idx"
        class="tag tag-to"
      >
        {{ addr }}
        <button class="tag-remove" @click="removeAddr('to', idx)">×</button>
      </span>
      <span
        v-for="(addr, idx) in form.cc"
        :key="'cc-'+idx"
        class="tag tag-cc"
      >
        C: {{ addr }}
        <button class="tag-remove" @click="removeAddr('cc', idx)">×</button>
      </span>
      <span
        v-for="(addr, idx) in form.bcc"
        :key="'bcc-'+idx"
        class="tag tag-bcc"
      >
        BCC: {{ addr }}
        <button class="tag-remove" @click="removeAddr('bcc', idx)">×</button>
      </span>
    </div>

    <!-- 主题 -->
    <div class="form-section">
      <label class="form-label">主题</label>
      <input
        v-model="form.subject"
        class="form-input"
        type="text"
        placeholder="请输入邮件主题"
      />
    </div>

    <!-- 正文编辑器 -->
    <div class="form-section editor-area">
      <div class="editor-toolbar">
        <button class="toolbar-btn" title="加粗" @click="formatText('bold')"><b>B</b></button>
        <button class="toolbar-btn" title="斜体" @click="formatText('italic')"><i>I</i></button>
        <button class="toolbar-btn" title="插入链接" @click="insertLink">🔗</button>
        <span class="toolbar-sep"></span>
        <label class="editor-mode-toggle">
          <input type="checkbox" v-model="useHtmlMode" />
          HTML 模式
        </label>
      </div>
      <textarea
        v-if="!useHtmlMode"
        ref="textEditor"
        v-model="form.body"
        class="form-textarea editor-text"
        rows="14"
        placeholder="请输入邮件正文..."
      ></textarea>
      <textarea
        v-else
        v-model="form.html_body"
        class="form-textarea editor-text code-font"
        rows="14"
        placeholder="<html>...</html>"
      ></textarea>
    </div>

    <!-- 发送按钮 -->
    <div class="compose-actions">
      <button class="btn btn-secondary btn-lg draft-btn" :disabled="saving" @click="handleSaveDraft">
        <template v-if="!saving">
          <svg width="16" height="16" viewBox="0 0 20 20" fill="none">
            <rect x="3.5" y="4.5" width="13" height="13" rx="2.5" stroke="currentColor" stroke-width="1.6"/>
            <path d="M7 2v3M13 2v3M5 8.5h10M9 12l1 1 2-3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          暂存草稿
        </template>
        <template v-else>
          <span class="spinner-sm"></span>
          暂存中...
        </template>
      </button>
      <button class="btn btn-primary btn-lg send-btn" :disabled="!canSend || sending" @click="handleSend">
        <template v-if="!sending">
          <svg width="18" height="18" viewBox="0 0 20 20" fill="none">
            <path d="M3 10L17 4L11 18L9 12L3 10Z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>
            <path d="M9 12L17 4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
          </svg>
          发送邮件
        </template>
        <template v-else>
          <span class="spinner-sm"></span>
          发送中...
        </template>
      </button>
      <span v-if="sendResult" class="send-result" :class="{ error: sendError }">
        {{ sendError ? '✗ ' + sendResult : '✓ ' + sendResult }}
      </span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onActivated, defineOptions } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAccountStore } from '@/stores/accountStore'
import { useToast } from '@/composables/useToast'

const toast = useToast()
import { sendEmail } from '@/api/mail'
import { saveDraft, getDraftById } from '@/api/draft'

defineOptions({ name: 'Compose' })

const router = useRouter()
const route = useRoute()
const accountStore = useAccountStore()

// 账号列表
const accounts = ref([])

// 表单数据
const form = ref({
  account_id: '',
  to: [],
  cc: [],
  bcc: [],
  subject: '',
  body: '',
  html_body: ''
})

// 输入框临时值
const toInput = ref('')
const ccInput = ref('')
const bccInput = ref('')

const showCc = ref(false)
const useHtmlMode = ref(false)
const sending = ref(false)
const saving = ref(false)
const sendResult = ref('')
const sendError = ref(false)
const editingDraftId = ref(null) // 正在编辑的草稿ID（如果有）

const textEditor = ref(null)

const canSend = computed(() => {
  return form.value.account_id && form.value.to.length > 0 && form.value.subject.trim() !== ''
})

function goBack() {
  if (route.query.from) {
    router.push(route.query.from)
  } else {
    router.push('/mails')
  }
}

function parseEmails(str) {
  if (!str) return []
  return str.split(/[,;，；]/).map(s => s.trim().toLowerCase()).filter(s => s.includes('@'))
}

function addToRecipient() {
  const emails = parseEmails(toInput.value)
  for (const e of emails) {
    if (!form.value.to.includes(e)) form.value.to.push(e)
  }
  toInput.value = ''
}

function addCc() {
  const emails = parseEmails(ccInput.value)
  for (const e of emails) {
    if (!form.value.cc.includes(e) && !form.value.to.includes(e)) form.value.cc.push(e)
  }
  ccInput.value = ''
}

function addBcc() {
  const emails = parseEmails(bccInput.value)
  for (const e of emails) {
    if (!form.value.bcc.includes(e) && !form.value.to.includes(e) && !form.value.cc.includes(e)) form.value.bcc.push(e)
  }
  bccInput.value = ''
}

function removeAddr(type, index) {
  form.value[type].splice(index, 1)
}

async function handleSend() {
  if (!canSend.value) return

  sending.value = true
  sendResult.value = ''
  sendError.value = false

  try {
    const payload = {
      account_id: parseInt(form.value.account_id),
      to: form.value.to,
      subject: form.value.subject,
      body: form.value.body || ''
    }

    if (form.value.cc.length > 0) payload.cc = form.value.cc
    if (form.value.bcc.length > 0) payload.bcc = form.value.bcc
    if (useHtmlMode.value && form.value.html_body) payload.html_body = form.value.html_body

    await sendEmail(payload)
    sendResult.value = '邮件已成功发送'
    sendError.value = false

    // 发送成功后删除对应的草稿（如果有）
    if (editingDraftId.value) {
      try {
        const { deleteDraft } = await import('@/api/draft')
        await deleteDraft(editingDraftId.value)
        editingDraftId.value = null
      } catch (_) { /* 忽略草稿删除错误 */ }
    }

    // 发送成功后跳转到已发送页面
    setTimeout(() => router.push('/sent'), 1500)
  } catch (e) {
    const msg = e.response?.data?.detail || e.message || '未知错误'
    sendResult.value = `发送失败: ${msg}`
    sendError.value = true
  } finally {
    sending.value = false
  }
}

async function handleSaveDraft() {
  saving.value = true

  try {
    const payload = {
      id: editingDraftId.value,
      account_id: form.value.account_id ? parseInt(form.value.account_id) : 0,
      to: form.value.to,
      cc: form.value.cc,
      bcc: form.value.bcc,
      subject: form.value.subject,
      body: form.value.body || '',
      html_body: useHtmlMode.value ? form.value.html_body : '',
    }

    // 更新已有草稿时需要传 id 字段让后端走更新逻辑
    if (editingDraftId.value) {
      payload.id = editingDraftId.value
    }

    const res = await saveDraft(payload)

    if (!editingDraftId.value && res.data?.id) {
      editingDraftId.value = res.data.id
    }

    sendResult.value = '草稿已保存'
    sendError.value = false

    setTimeout(() => { sendResult.value = '' }, 2000)
  } catch (e) {
    const msg = e.response?.data?.detail || e.message || '未知错误'
    sendResult.value = `暂存失败: ${msg}`
    sendError.value = true
  } finally {
    saving.value = false
  }
}

function formatText(command) {
  // 纯文本模式下提示用户
  if (!useHtmlMode.value) {
    toast.warning('请先开启 HTML 模式使用富文本格式')
    return
  }
}

async function insertLink() {
  if (!useHtmlMode.value) return
  const url = await toast.prompt('请输入链接地址:', 'https://')
  if (url) {
    form.value.html_body += `<a href="${url}">${url}</a>`
  }
}

onMounted(async () => {
  await accountStore.fetchAccounts()
  accounts.value = accountStore.accounts || []

  // 从路由参数预填信息（回复）
  if (route.query.replyTo) form.value.to = [route.query.replyTo]
  if (route.query.replySubject) form.value.subject = route.query.replySubject
  if (route.query.accountId) form.value.account_id = route.query.accountId

  // 从草稿加载
  if (route.query.draftId) {
    editingDraftId.value = parseInt(route.query.draftId)
    try {
      const draft = await getDraftById(editingDraftId.value)
      if (draft) {
        form.value.account_id = String(draft.account_id)
        try { form.value.to = JSON.parse(draft.to) } catch { form.value.to = [] }
        try { form.value.cc = JSON.parse(draft.cc || '[]') } catch { form.value.cc = [] }
        form.value.subject = draft.subject || ''
        form.value.body = draft.body || ''
        if (draft.html_body) {
          useHtmlMode.value = true
          form.value.html_body = draft.html_body || ''
        }
      }
    } catch (e) {
      console.error('加载草稿失败:', e)
      editingDraftId.value = null
    }
  }
})

onActivated(() => {
  accountStore.fetchAccounts()
})
</script>

<style scoped>
.compose-page {
  max-width: 800px;
  margin: 0 auto;
  padding: 24px 32px 48px;
}

.compose-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.page-title {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0;
}

.form-section {
  margin-bottom: 16px;
}

.form-label {
  display: block;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
  margin-bottom: 6px;
}

.form-select,
.form-input,
.form-textarea {
  width: 100%;
  padding: 10px 14px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--bg-card);
  color: var(--text-primary);
  font-family: inherit;
  font-size: var(--font-size-base);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  outline: none;
}
.form-select:focus,
.form-input:focus,
.form-textarea:focus {
  border-color: var(--primary-500);
  box-shadow: 0 0 0 3px var(--mail-unread-bg);
}

.form-textarea { resize: vertical; line-height: 1.7; }

/* 抄送/密送折叠区 */
.cc-details {
  margin-bottom: 16px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--bg-secondary);
  overflow: hidden;
}
.cc-details summary {
  list-style: none;
  cursor: pointer;
  user-select: none;
}
.cc-details summary::-webkit-details-marker { display: none; }

.cc-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border-radius: var(--radius-md);
  transition: background var(--transition-fast);
}
.cc-toggle:hover { background: var(--bg-hover); }

.toggle-icon {
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--primary-100);
  border-radius: var(--radius-full);
  font-size: 14px;
  color: var(--primary-500);
}

.cc-fields { padding: 0 14px 10px; display: flex; flex-direction: column; gap: 8px; }

/* 收件人标签 */
.recipient-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 12px;
  padding: 8px 12px;
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  min-height: 40px;
}

.tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: var(--radius-full);
  font-size: var(--font-size-sm);
}
.tag-to { background: var(--mail-unread-bg); color: var(--primary-600); }
.tag-cc { background: rgba(245, 158, 11, 0.12); color: #b45309; }
.tag-bcc { background: rgba(107, 114, 128, 0.12); color: #374151; }

.tag-remove {
  background: none;
  border: none;
  color: inherit;
  opacity: 0.55;
  cursor: pointer;
  font-size: 15px;
  padding: 0 2px;
  line-height: 1;
  transition: opacity var(--transition-fast);
}
.tag-remove:hover { opacity: 1; }

/* 编辑器 */
.editor-area {
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  overflow: hidden;
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
}
.editor-area:focus-within {
  border-color: var(--primary-500);
  box-shadow: 0 0 0 3px var(--mail-unread-bg);
}

.editor-toolbar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-light);
}

.toolbar-btn {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: none;
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.toolbar-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
  border-color: var(--border-color);
}

.toolbar-sep { width: 1px; height: 22px; background: var(--border-light); margin: 0 6px; }

.editor-mode-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  cursor: pointer;
  margin-left: auto;
}
.editor-mode-toggle input[type="checkbox"] {
  accent-color: var(--primary-500);
  width: 14px;
  height: 14px;
}

.editor-text { border: none !important; border-radius: 0 !important; padding: 14px 16px !important; min-height: 300px; }
.editor-text:focus { box-shadow: none !important; border-color: transparent !important; }
.code-font { font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace; font-size: 13px; line-height: 1.6; }

/* 底部操作栏 */
.compose-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--border-light);
}

.btn-lg { padding: 12px 28px; font-size: var(--font-size-base); font-weight: var(--font-weight-semibold); }

.send-btn {
  gap: 8px;
  white-space: nowrap;
}

.draft-btn {
  gap: 8px;
  white-space: nowrap;
}

.spinner-sm {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255,255,255,.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin .6s linear infinite;
}

.send-result {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
}
.send-result.error { color: var(--error); }
.send-result:not(.error) { color: var(--success); }

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* 响应式 */
@media (max-width: 640px) {
  .compose-page { padding: 16px; }
  .compose-header { flex-direction: rows; gap: 12px; align-items: flex-start; }
  .btn-lg { width: 100%; justify-content: center; }
}
</style>
