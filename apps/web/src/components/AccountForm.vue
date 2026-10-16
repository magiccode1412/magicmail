<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <Teleport to="body">
    <!-- 遮罩层 -->
    <div class="form-overlay" @click.self="$emit('close')">
      <div class="form-modal card" @click.stop>
        <!-- 标题 -->
        <div class="modal-header">
          <h3 class="modal-title">{{ isEdit ? '编辑邮箱账号' : '添加邮箱账号' }}</h3>
          <button class="btn-icon btn-ghost" @click="$emit('close')">
            <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
              <path d="M4 4L14 14M14 4L4 14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
            </svg>
          </button>
        </div>

        <!-- 表单内容 -->
        <div class="modal-body">
          <form @submit.prevent="handleSubmit" class="account-form">
            <!-- 显示名称 -->
            <div class="form-group">
              <label class="form-label">显示名称 <span class="required">*</span></label>
              <input
                v-model="form.name"
                type="text"
                class="input"
                placeholder="如：工作邮箱、私人邮箱"
                required
                maxlength="100"
              />
            </div>

            <!-- 邮箱地址 -->
            <div class="form-group">
              <label class="form-label">邮箱地址 <span class="required">*</span></label>
              <input
                v-model="form.email"
                type="email"
                class="input"
                placeholder="user@example.com"
                required
              />
            </div>

            <!-- 邮箱服务商预设（仅新建时显示） -->
            <div v-if="!isEdit" class="form-group">
              <label class="form-label">快速配置</label>
              <div class="provider-presets">
                <button
                  v-for="p in providerPresets"
                  :key="p.key"
                  type="button"
                  class="provider-chip"
                  :class="{ active: selectedPreset === p.key }"
                  @click="applyPreset(p)"
                >
                  {{ p.name }}
                </button>
              </div>
              <p v-if="selectedPreset" class="form-hint text-muted provider-hint">
                已自动填充 {{ getSelectedPresetName }} 的服务器配置，请核对后填写密码
              </p>
            </div>

            <!-- 协议选择 -->
            <div class="form-group">
              <label class="form-label">邮件协议 <span class="required">*</span></label>
              <div class="protocol-options">
                <label
                  v-for="opt in protocolOptions"
                  :key="opt.value"
                  class="protocol-option"
                  :class="{ active: form.protocol === opt.value }"
                >
                  <input
                    type="radio"
                    :value="opt.value"
                    v-model="form.protocol"
                    @change="onProtocolChange"
                    hidden
                  />
                  <span class="protocol-label">{{ opt.label }}</span>
                  <span class="protocol-desc">{{ opt.desc }}</span>
                </label>
              </div>
            </div>

            <!-- 服务器地址 + 端口 (同一行) -->
            <div class="form-row">
              <div class="form-group flex-1">
                <label class="form-label">{{ serverLabel }} <span class="required">*</span></label>
                <input
                  v-model="form.host"
                  type="text"
                  class="input"
                  :placeholder="serverPlaceholder"
                  required
                />
              </div>
              <div class="form-group form-group-port">
                <label class="form-label">端口</label>
                <input
                  v-model.number="form.port"
                  type="number"
                  class="input"
                  min="1"
                  max="65535"
                  :placeholder="String(defaultPort)"
                />
              </div>
            </div>

            <!-- 用户名（可选） -->
            <div class="form-group">
              <label class="form-label">用户名</label>
              <input
                v-model="form.username"
                type="text"
                class="input"
                placeholder="留空则使用邮箱地址"
              />
              <p v-if="!form.username && form.email" class="form-hint text-muted">
                将自动使用：{{ form.email }}
              </p>
            </div>

            <!-- 密码 -->
            <div class="form-group">
              <label class="form-label">密码 <span class="required">*</span></label>
              <div class="password-field">
                <input
                  v-model="form.password"
                  :type="showPassword ? 'text' : 'password'"
                  class="input input-password"
                  :placeholder="'输入' + serverLabel + '密码'"
                  :required="!isEdit"
                />
                <button
                  type="button"
                  class="toggle-pwd btn-ghost btn-sm"
                  @click="showPassword = !showPassword"
                  tabindex="-1"
                >
                  {{ showPassword ? '隐藏' : '显示' }}
                </button>
              </div>
              <p v-if="isEdit && !form.password" class="form-hint text-muted">
                留空则保持原密码不变
              </p>
            </div>

            <!-- SMTP 发信配置（可折叠） -->
            <details class="smtp-section" :open="smtpOpen">
              <summary class="smtp-summary" @click.prevent="smtpOpen = !smtpOpen">
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                  <path d="M2 7h10M9 3l4 4-4 4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                SMTP 发信配置（可选）
                <span class="smtp-hint">用于发送邮件，不填则使用收信服务器</span>
                <span class="toggle-arrow">▸</span>
              </summary>
              <div class="smtp-fields">
                <div class="form-row">
                  <div class="form-group flex-1">
                    <label class="form-label">SMTP 服务器</label>
                    <input
                      v-model="form.smtp_host"
                      type="text"
                      class="input"
                      placeholder="如: smtp.example.com（默认同收信服务器）"
                    />
                  </div>
                  <div class="form-group form-group-port">
                    <label class="form-label">端口</label>
                    <input
                      v-model.number="form.smtp_port"
                      type="number"
                      class="input"
                      min="1"
                      max="65535"
                      placeholder="587"
                    />
                  </div>
                </div>
                <p class="form-hint text-muted smtp-tip">常用端口：587 (STARTTLS)、465 (SSL/TLS)、25 (明文)</p>
              </div>
            </details>

            <!-- 同步设置（可折叠） -->
            <details class="smtp-section" :open="syncOpen">
              <summary class="smtp-summary" @click.prevent="syncOpen = !syncOpen">
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                  <path d="M1.5 7h11M9 3l4 4-4 4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                同步设置
                <span class="sync-hint">控制拉取哪些邮件</span>
                <span class="toggle-arrow">▸</span>
              </summary>
              <div class="smtp-fields">
                <div class="form-group">
                  <label class="form-label">同步范围</label>
                  <div class="sync-mode-options">
                    <label
                      v-for="opt in syncModeOptions"
                      :key="opt.value"
                      class="sync-mode-option"
                      :class="{ active: form.sync_mode === opt.value }"
                    >
                      <input
                        type="radio"
                        :value="opt.value"
                        v-model="form.sync_mode"
                        hidden
                      />
                      <span class="sync-mode-label">{{ opt.label }}</span>
                      <span class="sync-mode-desc">{{ opt.desc }}</span>
                    </label>
                  </div>
                </div>
                <div class="form-group" v-if="form.sync_mode === 'recent'">
                  <label class="form-label">最近天数</label>
                  <div class="sync-days-row">
                    <input
                      v-model.number="form.sync_days"
                      type="number"
                      class="input sync-days-input"
                      min="1"
                      max="365"
                    />
                    <span class="sync-days-unit">天</span>
                  </div>
                  <p class="form-hint text-muted smtp-tip">仅拉取最近 N 天内收到的邮件，超过该时间的邮件将被忽略</p>
                </div>
                <p v-if="form.protocol !== 'imap' && form.sync_mode === 'unread'" class="form-hint text-warning smtp-tip">
                  ⚠️ POP3 协议不支持区分已读/未读状态，将自动降级为同步全部邮件（通过 Message-ID 去重避免重复）
                </p>
                <!-- 删除时同步到源服务器 -->
                <div class="form-group">
                  <div class="proxy-toggle-row">
                    <label class="form-label">删除时同步到源服务器</label>
                    <label class="toggle-switch">
                      <input type="checkbox" v-model="form.delete_on_server" />
                      <span class="toggle-slider"></span>
                    </label>
                  </div>
                  <p class="form-hint text-muted smtp-tip">
                    开启后，在应用中删除邮件时会同时删除邮箱服务器上的原邮件（不可恢复）
                  </p>
                </div>
              </div>
            </details>

            <!-- HTTP 代理配置（可折叠） -->
            <details class="smtp-section" :open="proxyOpen">
              <summary class="smtp-summary" @click.prevent="proxyOpen = !proxyOpen">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                  <circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="2"/>
                  <path d="M12 2v3m0 14v3M4.93 4.93l2.12 2.12m9.9 9.9l2.12 2.12M2 12h3m14 0h3M4.93 19.07l2.12-2.12m9.9-9.9l2.12-2.12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                </svg>
                HTTP 代理（可选）
                <span class="toggle-arrow">▸</span>
              </summary>
              <div class="smtp-fields">
                <div class="form-group">
                  <div class="proxy-toggle-row">
                    <label class="form-label">启用代理</label>
                    <label class="toggle-switch">
                      <input type="checkbox" v-model="form.proxy_enabled" />
                      <span class="toggle-slider"></span>
                    </label>
                  </div>
                </div>
                <div class="form-group" v-if="form.proxy_enabled">
                  <label class="form-label">代理地址</label>
                  <input
                    v-model="form.proxy_url"
                    type="text"
                    class="input"
                    placeholder="http://user:pass@proxy-host:8080"
                  />
                </div>
                <p class="form-hint text-muted smtp-tip">
                  支持 HTTP/HTTPS/SOCKS5 代理。格式：<code>http://host:port</code> 或 <code>socks5://host:1080</code>
                </p>
              </div>
            </details>

            <!-- 操作按钮 -->
            <div class="form-actions">
              <button type="button" class="btn btn-secondary" @click="$emit('close')">
                取消
              </button>
              <button
                type="button"
                class="btn btn-ghost btn-sm test-btn"
                @click="handleTest"
                :disabled="testing"
              >
                <span v-if="testing" class="spinner"></span>
                {{ testing ? '测试中...' : '测试连接' }}
              </button>
              <button type="submit" class="btn btn-primary" :disabled="submitting || testing">
                <span v-if="submitting" class="spinner"></span>
                {{ isEdit ? '保存修改' : '添加账号' }}
              </button>
            </div>

            <!-- 测试结果提示 -->
            <div v-if="testResult" class="test-result" :class="testResult.success ? 'success' : 'error'">
              {{ testResult.message }}
            </div>
          </form>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { testConnection } from '@/api/account'
import { useToast } from '@/composables/useToast'

const toast = useToast()
import { useAccountStore } from '@/stores/accountStore'

const props = defineProps({
  account: { type: Object, default: null }
})

const emit = defineEmits(['close', 'saved'])
const accountStore = useAccountStore()

const isEdit = computed(() => !!props.account)
const submitting = ref(false)
const testing = ref(false)
const showPassword = ref(false)
const testResult = ref(null)
const smtpOpen = ref(false)
const proxyOpen = ref(false)
const syncOpen = ref(false)

// 协议选项
const protocolOptions = [
  { value: 'imap', label: 'IMAP', desc: '推荐，支持双向同步' },
  { value: 'pop3', label: 'POP3 (SSL)', desc: '仅收信，部分老邮箱使用' },
]

// 同步模式选项
const syncModeOptions = [
  { value: 'unread', label: '只同步未读', desc: '默认，仅拉取未读邮件，速度快' },
  { value: 'all',    label: '全部邮件',   desc: '同步收件箱所有邮件（含已读）' },
  { value: 'recent', label: '最近N天',     desc: '只同步最近一段时间内的邮件' },
]

// 邮箱服务商预设
const providerPresets = [
  { key: 'qq',     name: 'QQ 邮箱',      domain: 'qq.com',       protocol: 'imap', host: 'imap.qq.com',        port: 993, smtpHost: 'smtp.qq.com',    smtpPort: 465 },
  { key: '163',    name: '163 邮箱',     domain: '163.com',      protocol: 'imap', host: 'imap.163.com',       port: 993, smtpHost: 'smtp.163.com',   smtpPort: 465 },
  { key: '126',    name: '126 邮箱',     domain: '126.com',      protocol: 'imap', host: 'imap.126.com',       port: 993, smtpHost: 'smtp.126.com',   smtpPort: 465 },
  { key: 'sina',   name: '新浪邮箱',     domain: 'sina.com',     protocol: 'imap', host: 'imap.sina.com',      port: 993, smtpHost: 'smtp.sina.com',  smtpPort: 465 },
  { key: 'outlook',name: 'Outlook/Hotmail', domain: '',          protocol: 'imap', host: 'outlook.office365.com',port: 993, smtpHost: 'smtp.office365.com', smtpPort: 587 },
  { key: 'gmail',  name: 'Gmail',         domain: 'gmail.com',    protocol: 'imap', host: 'imap.gmail.com',     port: 993, smtpHost: 'smtp.gmail.com', smtpPort: 587 },
  { key: 'yahoo',  name: 'Yahoo 邮箱',    domain: 'yahoo.com',    protocol: 'imap', host: 'imap.mail.yahoo.com',port: 993, smtpHost: 'smtp.mail.yahoo.com', smtpPort: 465 },
  { key: 'aliyun', name: '阿里云邮箱',    domain: 'aliyun.com',   protocol: 'imap', host: 'imap.aliyun.com',    port: 993, smtpHost: 'smtp.aliyun.com', smtpPort: 465 },
]

const selectedPreset = ref('')
const getSelectedPresetName = computed(() => {
  const p = providerPresets.find(p => p.key === selectedPreset.value)
  return p ? p.name : ''
})

const defaultPort = computed(() => form.protocol === 'pop3' ? 995 : 993)
const serverLabel = computed(() => form.protocol === 'pop3' ? 'POP3 服务器' : 'IMAP 服务器')
const serverPlaceholder = computed(() => form.protocol === 'pop3' ? 'pop.example.com' : 'imap.example.com')

function onProtocolChange() {
  // 切换协议时自动更新默认端口（用户未手动修改过端口时）
  const portMap = { imap: 993, pop3: 995 }
  if (form.port === 993 || form.port === 995 || !form.port) {
    form.port = portMap[form.protocol] || 993
  }
}

function applyPreset(preset) {
  selectedPreset.value = selectedPreset.value === preset.key ? '' : preset.key
  if (!selectedPreset.value) return

  form.protocol = preset.protocol
  form.host = preset.host
  form.port = preset.port
  form.smtp_host = preset.smtpHost
  form.smtp_port = preset.smtpPort
}

// 表单数据
const form = reactive({
  name: '',
  email: '',
  protocol: 'imap',
  host: '',
  port: 993,
  smtp_host: '',
  smtp_port: null,
  username: '',
  password: '',
  proxy_enabled: false,
  proxy_url: '',
  sync_mode: 'unread',
  sync_days: 30,
  delete_on_server: false
})

// 初始化表单数据
onMounted(() => {
  if (props.account) {
    Object.assign(form, {
      name: props.account.name,
      email: props.account.email,
      protocol: props.account.protocol || 'imap',
      host: props.account.host,
      port: props.account.port,
      smtp_host: props.account.smtp_host || '',
      smtp_port: props.account.smtp_port || null,
      username: props.account.username,
      password: '', // 编辑时不回显密码
      proxy_enabled: props.account.proxy_enabled || false,
      proxy_url: props.account.proxy_url || '',
      sync_mode: props.account.sync_mode || 'unread',
      sync_days: props.account.sync_days || 30,
      delete_on_server: props.account.delete_on_server || false
    })
    smtpOpen.value = !!(props.account.smtp_host || props.account.smtp_port)
    proxyOpen.value = !!props.account.proxy_enabled
    syncOpen.value = !!(form.sync_mode && form.sync_mode !== 'unread')
  }
})

async function handleTest() {
  const effectiveUsername = form.username || form.email
  // 基本校验
  if (!form.host || !effectiveUsername || !(!isEdit.value || form.password)) {
    testResult.value = { success: false, message: `请填写${serverLabel.value}、邮箱地址和密码` }
    return
  }

  testing.value = true
  testResult.value = null

  try {
    await testConnection({ ...form, username: effectiveUsername })
    testResult.value = { success: true, message: '✅ 连接成功！配置正确' }
  } catch (e) {
    testResult.value = { success: false, message: `❌ 连接失败：${e.message}` }
  } finally {
    testing.value = false
  }
}

async function handleSubmit() {
  submitting.value = true
  
  try {
    // 用户名为空时自动使用邮箱地址
    const submitData = { ...form }
    if (!submitData.username && submitData.email) {
      submitData.username = submitData.email
    }

    if (isEdit.value) {
      await accountStore.editAccount(props.account.id, submitData)
    } else {
      await accountStore.addAccount(submitData)
    }
    
    emit('saved')
  } catch (e) {
    toast.error(isEdit.value ? '保存失败' : '创建失败：' + e.message)
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.form-overlay {
  position: fixed;
  inset: 0;
  background: var(--bg-overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: var(--z-modal);
  padding: var(--space-md);
  animation: overlay-in 0.2s ease-out;
}
@keyframes overlay-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

.form-modal {
  width: 100%;
  max-width: 540px;
  max-height: 90vh;
  overflow-y: auto;
  animation: modal-slide-up 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  border-radius: var(--radius-xl) !important;
}
@keyframes modal-slide-up {
  from { transform: translateY(24px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-lg);
}
.modal-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-bold);
}

.modal-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

/* ---- 表单 ---- */
.account-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.form-row {
  display: flex;
  gap: var(--space-md);
}
.flex-1 { flex: 1; }
.form-group-port { width: 110px; }

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

/* ---- 协议选择器 ---- */
.provider-presets {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-xs);
}

.provider-chip {
  padding: 6px 14px;
  font-size: var(--font-size-sm);
  font-family: inherit;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-full);
  background: var(--bg-secondary);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all var(--transition-fast);
  user-select: none;
}
.provider-chip:hover {
  border-color: var(--primary-300);
  color: var(--text-primary);
  background: var(--bg-hover);
}
.provider-chip.active {
  border-color: var(--primary-500);
  background: var(--mail-unread-bg);
  color: var(--primary-500);
  font-weight: var(--font-weight-medium);
}

.provider-hint { margin-top: 4px; }

.protocol-options {
  display: flex;
  gap: var(--space-sm);
}

.protocol-option {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: var(--space-md);
  border: 2px solid var(--border-color);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: all var(--transition-fast);
  user-select: none;
}
.protocol-option:hover {
  border-color: var(--primary-300);
}
.protocol-option.active {
  border-color: var(--primary-500);
  background: var(--mail-unread-bg);
  box-shadow: 0 0 0 3px var(--mail-unread-bg);
}
.protocol-label {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin-bottom: 2px;
}
.protocol-desc {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

.form-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
}
.required { color: var(--error); }

.password-field {
  position: relative;
  display: flex;
  align-items: center;
}
.input-password { padding-right: 64px; }
.toggle-pwd {
  position: absolute;
  right: 8px;
  z-index: 1;
  font-size: var(--font-size-xs);
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  border: 1px solid var(--border-color);
}
.toggle-pwd:hover {
  background: var(--bg-hover);
  color: var(--primary-500);
  border-color: var(--primary-200);
}

.form-hint {
  font-size: var(--font-size-xs);
  margin-top: -4px;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-sm);
  padding-top: var(--space-sm);
  border-top: 1px solid var(--border-light);
  margin-top: var(--space-md);
}

.test-btn {
  margin-right: auto;
}
.test-btn:disabled { opacity: 0.6; cursor: not-allowed; }

/* ---- SMTP 发信配置 ---- */
.smtp-section {
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  overflow: hidden;
}
.smtp-section summary {
  list-style: none;
}
.smtp-section summary::-webkit-details-marker { display: none; }

.smtp-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: var(--bg-secondary);
  cursor: pointer;
  user-select: none;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  transition: background var(--transition-fast);
}
.smtp-summary:hover { background: var(--bg-hover); }

.smtp-summary svg {
  flex-shrink: 0;
  color: var(--primary-500);
}

.smtp-hint {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  margin-left: auto;
}

.toggle-arrow {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--text-tertiary);
  transition: transform var(--transition-fast);
}
.smtp-section[open] .toggle-arrow { transform: rotate(90deg); }

.smtp-fields {
  padding: 14px 16px 10px;
  border-top: 1px solid var(--border-light);
}

.smtp-tip { margin-top: 4px; }
.smtp-tip code { 
  background: var(--bg-tertiary);
  padding: 1px 5px;
  border-radius: 3px;
  font-size: inherit;
}

/* ---- 同步设置 ---- */
.sync-hint {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  margin-left: auto;
}

.sync-mode-options {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
}

.sync-mode-option {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  padding: var(--space-sm) var(--space-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all var(--transition-fast);
  user-select: none;
}
.sync-mode-option:hover {
  border-color: var(--primary-300);
  background: var(--bg-hover);
}
.sync-mode-option.active {
  border-color: var(--primary-500);
  background: var(--mail-unread-bg);
}

.sync-mode-label {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
  min-width: 72px;
}

.sync-mode-desc {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

.sync-days-row {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}
.sync-days-input {
  width: 100px;
}
.sync-days-unit {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.text-warning { color: #B45309; }

/* ---- 代理开关 ---- */
.proxy-toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.toggle-switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
  cursor: pointer;
}
.toggle-switch input {
  opacity: 0;
  width: 0;
  height: 0;
}
.toggle-slider {
  position: absolute;
  inset: 0;
  background: var(--border-color);
  border-radius: 12px;
  transition: background 0.2s;
}
.toggle-slider::after {
  content: '';
  position: absolute;
  width: 18px;
  height: 18px;
  left: 3px;
  bottom: 3px;
  background: #fff;
  border-radius: 50%;
  transition: transform 0.2s;
}
.toggle-switch input:checked + .toggle-slider {
  background: var(--primary-500);
}
.toggle-switch input:checked + .toggle-slider::after {
  transform: translateX(20px);
}

/* ---- 测试结果 ---- */
.test-result {
  padding: var(--space-sm) var(--space-md);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  animation: fade-in 0.25s ease;
}
.test-result.success { background: var(--success-light); color: var(--success); }
.test-result.error { background: var(--error-light); color: var(--error); }

@keyframes fade-in {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

/* ---- 滚动条隐藏（弹窗内） ---- */
.form-modal::-webkit-scrollbar { display: none; }

@media (max-width: 480px) {
  .form-modal { max-height: 95vh; }
  .form-row { flex-direction: column; }
  .form-group-port { width: unset; }
  
  .form-actions {
    flex-wrap: wrap;
  }
  .test-btn { 
    order: -1; /* 测试按钮移到最前 */
    width: 100%;
  }
}
</style>
