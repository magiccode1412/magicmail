<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="settings-view">
    <h2 class="page-title">设置</h2>

    <div class="settings-sections">
      <!-- 外观设置 -->
      <section class="settings-section card">
        <h3 class="section-title">外观</h3>
        <p class="section-desc">选择你喜欢的界面风格</p>

        <div class="theme-options">
          <button
            v-for="opt in themeOptions"
            :key="opt.value"
            class="theme-option"
            :class="{ active: currentTheme === opt.value }"
            @click="setTheme(opt.value)"
          >
            <span class="theme-icon" v-html="opt.icon"></span>
            <span class="theme-label">{{ opt.label }}</span>
          </button>
        </div>

        <!-- 主题色选择 -->
        <div class="theme-color-section">
          <div class="section-subtitle">主题色</div>
          <p class="color-desc">自定义界面主色调，影响按钮、链接、高亮等元素颜色</p>
          <div class="color-presets">
            <button
              v-for="c in presetColors"
              :key="c.value"
              class="color-swatch"
              :class="{ active: currentPrimaryColor === c.value }"
              :style="{ backgroundColor: c.value }"
              :title="c.label"
              @click="handlePrimaryColorChange(c.value)"
            ></button>
            <label class="color-swatch color-custom" title="自定义颜色" :class="{ active: isCustomColor }">
              <input type="color" :value="currentPrimaryColor" @input="onCustomColorInput($event)" />
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                <path d="M12 2a2.12 2.12 0 0 1 3 3L7.5 12.5 3 14l1.5-4.5L12 2z" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
                <path d="M10 6l2 2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
              </svg>
            </label>
          </div>
          <div v-if="currentPrimaryColor" class="color-value-display">
            当前：{{ currentPrimaryColor }}
          </div>
        </div>
      </section>

      <!-- 通知设置 -->
      <section class="settings-section card">
        <h3 class="section-title">通知</h3>
        <p class="section-desc">新邮件到达时的提醒方式</p>

        <div class="setting-row">
          <div class="setting-info">
            <strong>离线推送</strong>
            <small>{{ pushSupported ? '即使关闭浏览器也能收到新邮件提醒' : '您的浏览器不支持离线推送（需要 Chrome / Edge / Firefox）' }}</small>
          </div>
          <div style="display: flex; align-items: center; gap: 10px;">
            <span v-if="!pushSupported" class="badge badge-default">不支持</span>
            <label v-else class="toggle-switch" :class="{ 'toggle-disabled': pushSubscribing }">
              <input type="checkbox"
                :checked="pushEnabled"
                :disabled="pushSubscribing"
                @change.prevent="toggleNotification" />
              <span class="toggle-slider"></span>
              <span v-if="pushSubscribing" class="toggle-hint">连接中</span>
              <span v-else-if="pushError" class="toggle-hint toggle-error-hint" :title="pushError">!</span>
            </label>
          </div>
        </div>
        <p v-if="pushError" class="push-error-hint">{{ pushError }}</p>
      </section>

      <!-- 数据同步设置 -->
      <section class="settings-section card">
        <h3 class="section-title">数据同步</h3>
        <p class="section-desc">配置邮件列表的自动刷新方式</p>

        <div class="setting-row" style="align-items: flex-start; padding-top: var(--space-md);">
          <div class="setting-info">
            <strong>连接状态</strong>
            <small>当前使用的实时更新方式</small>
          </div>
          <div class="connection-status" :class="'status-' + connectionStatus.mode">
            <span class="status-dot"></span>
            <span class="status-text">{{ connectionStatus.label }}</span>
          </div>
        </div>

        <div class="setting-row" style="align-items: flex-start; padding-top: var(--space-md);">
          <div class="setting-info">
            <strong>轮询间隔</strong>
            <small>SSE 不可用时的定时刷新间隔</small>
          </div>
          <select
            class="poll-select"
            :value="currentPollInterval"
            @change="setPollInterval(Number($event.target.value))"
          >
            <option v-for="opt in pollOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </div>

        <div class="setting-row">
          <div class="setting-info">
            <strong>下次刷新</strong>
            <small>距离下次自动刷新的剩余时间</small>
          </div>
          <span class="next-refresh">{{ nextRefreshCountdown }}</span>
        </div>
      </section>

      <!-- Webhook 通知 -->
      <section class="settings-section card">
        <div class="section-header-row">
          <div>
            <h3 class="section-title">Webhook 通知</h3>
            <p class="section-desc">配置自定义回调地址，新邮件到达时自动推送通知</p>
          </div>
          <button class="btn btn-primary btn-sm" @click="showForm = true; editingHook = null; resetForm()">
            + 添加 Webhook
          </button>
          <button class="btn btn-secondary btn-sm" @click="simulateMailAction" :disabled="simulatingMail">
            <span v-if="simulatingMail" class="spinner-xs"></span>
            📨 模拟接收邮件
          </button>
        </div>

        <!-- Webhook 列表 -->
        <div v-if="webhooks.length === 0 && !loadingWebhooks" class="empty-hint">
          暂未配置 Webhook，点击上方按钮添加
        </div>
        <div v-else class="webhook-list">
          <div v-for="hook in webhooks" :key="hook.id" class="webhook-card" :class="{ disabled: !hook.enabled }">
            <div class="webhook-info">
              <div class="webhook-name-row">
                <span class="webhook-name">{{ hook.name }}</span>
                <span class="badge" :class="hook.enabled ? 'badge-success' : 'badge-default'">
                  {{ hook.enabled ? '已启用' : '已禁用' }}
                </span>
                <span v-if="hook.last_status === 'success'" class="badge badge-success-light">推送正常</span>
                <span v-else-if="hook.last_status === 'error'" class="badge badge-error-light">推送异常</span>
              </div>
              <div class="webhook-url">{{ truncate(hook.url, 50) }}</div>
              <div class="webhook-meta">
                <span>事件: {{ formatEvents(hook.events) }}</span>
                <span v-if="hook.last_trigger_at">最近触发: {{ formatTime(hook.last_trigger_at) }}</span>
              </div>
              <div v-if="hook.error_msg && hook.last_status === 'error'" class="webhook-error">
                {{ truncate(hook.error_msg, 80) }}
              </div>
            </div>
            <div class="webhook-actions">
              <button class="btn-icon-text btn-sm" @click="testWebhookAction(hook.id)" :disabled="testingId === hook.id">
                <span v-if="testingId === hook.id" class="spinner-xs"></span>
                测试
              </button>
              <button class="btn-icon-text btn-sm" @click="editHook(hook)">编辑</button>
              <button class="btn-icon-text btn-sm text-danger" @click="deleteHookAction(hook.id)">删除</button>
            </div>
          </div>
        </div>

        <!-- Webhook 编辑/新建 弹窗 -->
        <Teleport to="body">
          <div v-if="showForm" class="webhook-modal-overlay" @click.self="closeModal">
            <div class="webhook-modal">
              <div class="modal-header">
                <h4 class="form-title">{{ editingHook ? '编辑 Webhook' : '新建 Webhook' }}</h4>
                <button class="modal-close" @click="closeModal">&times;</button>
              </div>
              <div class="modal-body">
                <div class="form-grid">
                  <div class="form-field" style="grid-column: 1 / -1;">
                    <label>名称</label>
                    <input v-model="form.name" type="text" placeholder="例：飞书通知、企业微信" maxlength="100" />
                  </div>
                  <div class="form-field" style="grid-column: 1 / -1;">
                    <label>URL *</label>
                    <input v-model="form.url" type="url" placeholder="https://your-server.com/webhook" required />
                  </div>
                  <div class="form-field">
                    <label>签名密钥</label>
                    <input v-model="form.secret" type="password" placeholder="可选，用于 HMAC-SHA256 签名验证" />
                  </div>
                  <div class="form-field">
                    <label>启用状态</label>
                    <label class="toggle-switch" style="margin-top: 6px;">
                      <input type="checkbox" v-model="form.enabled" />
                      <span class="toggle-slider"></span>
                    </label>
                  </div>
                  <div class="form-field" style="grid-column: 1 / -1;">
                    <label>触发事件（逗号分隔）</label>
                    <input v-model="form.events" placeholder="mail.received,mail.*" />
                    <small class="field-hint">支持通配符 *，如 mail.* 匹配所有邮件事件。默认: mail.received</small>
                  </div>
                  <div class="form-field" style="grid-column: 1 / -1;">
                    <label>自定义 Headers（JSON 格式）</label>
                    <textarea v-model="form.headers" rows="2" placeholder='{"Authorization": "Bearer xxx"}'></textarea>
                    <small class="field-hint">可选，JSON 对象格式</small>
                  </div>
                  <div class="form-field" style="grid-column: 1 / -1;">
                    <label>自定义 Body（JSON 模板）</label>
                    <textarea v-model="form.body" rows="3" placeholder='{"title":"📧 {{data.subject}}","content":"**来自:** {{data.from}}\n**时间:** {{data.sent_at}}\n\n{{data.text_body}}","type":"markdown"}' ></textarea>
                    <small class="field-hint">可选，留空则使用默认结构。变量：&#123;{event}}、&#123;{timestamp}}、&#123;{data.subject}}、&#123;{data.text_body}}、&#123;{data.html_body}}、&#123;{data.preview}} 等</small>
                  </div>
                </div>
              </div>
              <div class="modal-footer">
                <a href="https://www.160621.xyz/magicmail/guide/webhooks.html" target="_blank" rel="noopener" class="doc-link">
                  📖 参考文档
                  <svg width="12" height="12" viewBox="0 0 12 12" fill="none" style="display: inline-block; vertical-align: middle;">
                    <path d="M9 3L4.5 7.5M4.5 7.5L9 12M4.5 7.5H1.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                  </svg>
                </a>
                <div style="flex:1"></div>
                <button class="btn btn-secondary btn-sm" @click="closeModal">取消</button>
                <button class="btn btn-primary btn-sm" @click="saveHook" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button>
              </div>
            </div>
          </div>
        </Teleport>

      </section>

      <!-- 用户与权限（仅管理员可见） -->
      <section v-if="authStore.isAdmin" class="settings-section card">
        <div class="section-header-row">
          <div>
            <h3 class="section-title">用户与权限</h3>
            <p class="section-desc">管理平台用户与公开注册开关，各用户仅能查看与操作自己的数据</p>
          </div>
        </div>

        <!-- 开放注册开关 -->
        <div class="setting-row">
          <div class="setting-info">
            <strong>开放公开注册</strong>
            <small>开启后，访客可在登录页自助注册为普通用户；关闭后仅管理员可后台手动添加</small>
          </div>
          <label class="toggle-switch" :class="{ 'toggle-disabled': savingOpenReg }">
            <input type="checkbox"
              :checked="openRegistration"
              :disabled="savingOpenReg"
              @change.prevent="toggleOpenRegistration" />
            <span class="toggle-slider"></span>
            <span v-if="savingOpenReg" class="toggle-hint">保存中</span>
          </label>
        </div>

        <!-- 用户列表 -->
        <div class="user-list-header">
          <span class="user-list-title">用户列表（{{ users.length }}）</span>
          <button class="btn btn-primary btn-sm" @click="openAddUser">+ 添加用户</button>
        </div>

        <div v-if="loadingUsers" class="empty-hint">加载中...</div>
        <div v-else-if="users.length === 0" class="empty-hint">暂无其他用户</div>
        <div v-else class="webhook-list">
          <div v-for="u in users" :key="u.id" class="webhook-card">
            <div class="webhook-info">
              <div class="webhook-name-row">
                <span class="webhook-name">{{ u.username }}</span>
                <span class="badge" :class="u.role === 'admin' ? 'badge-success' : 'badge-default'">
                  {{ u.role === 'admin' ? '管理员' : '普通用户' }}
                </span>
                <span v-if="u.username === authStore.username" class="badge badge-default">当前账号</span>
              </div>
              <div class="webhook-meta">
                <span>注册于 {{ formatTime(u.created_at) }}</span>
              </div>
            </div>
            <div class="webhook-actions">
              <button class="btn-icon-text btn-sm text-danger" @click="deleteUserAction(u)">删除</button>
            </div>
          </div>
        </div>

        <!-- 添加用户弹窗 -->
        <Teleport to="body">
          <div v-if="showAddUser" class="webhook-modal-overlay" @click.self="closeAddUser">
            <div class="webhook-modal">
              <div class="modal-header">
                <h4 class="form-title">添加用户</h4>
                <button class="modal-close" @click="closeAddUser">&times;</button>
              </div>
              <div class="modal-body">
                <div class="form-grid">
                  <div class="form-field" style="grid-column: 1 / -1;">
                    <label>用户名 *</label>
                    <input v-model="addForm.username" type="text" placeholder="至少 3 个字符" maxlength="32" />
                  </div>
                  <div class="form-field" style="grid-column: 1 / -1;">
                    <label>密码 *</label>
                    <input v-model="addForm.password" type="password" placeholder="至少 6 位" maxlength="64" />
                  </div>
                  <div class="form-field" style="grid-column: 1 / -1;">
                    <label>角色</label>
                    <select v-model="addForm.role" class="poll-select" style="width: 100%;">
                      <option value="user">普通用户</option>
                      <option value="admin">管理员</option>
                    </select>
                    <small class="field-hint">默认为普通用户，仅可管理自己的数据</small>
                  </div>
                </div>
              </div>
              <div class="modal-footer">
                <div style="flex:1"></div>
                <button class="btn btn-secondary btn-sm" @click="closeAddUser">取消</button>
                <button class="btn btn-primary btn-sm" @click="submitAddUser" :disabled="savingUser">
                  {{ savingUser ? '创建中...' : '创建用户' }}
                </button>
              </div>
            </div>
          </div>
        </Teleport>
      </section>

      <!-- 邮件渲染设置 -->
      <section class="settings-section card">
        <h3 class="section-title">邮件渲染</h3>
        <p class="section-desc">自定义邮件正文的显示方式</p>

        <div class="setting-row" style="align-items: flex-start; padding-top: var(--space-md);">
          <div class="setting-info">
            <strong>渲染模式</strong>
            <small>选择邮件正文的渲染方式</small>
          </div>
          <div class="render-mode-options">
            <button
              v-for="opt in renderModeOptions"
              :key="opt.value"
              class="render-mode-btn"
              :class="{ active: mailRenderMode === opt.value }"
              @click="appStore.setMailRenderMode(opt.value)"
            >
              <span class="rm-label">{{ opt.label }}</span>
              <span class="rm-desc">{{ opt.desc }}</span>
            </button>
          </div>
        </div>

        <div class="setting-row">
          <div class="setting-info">
            <strong>按钮自动居中</strong>
            <small>修复邮件中 display:block 的按钮无法居中的问题</small>
          </div>
          <label class="toggle-switch">
            <input type="checkbox" :checked="mailButtonCenter" @change.prevent="appStore.setMailButtonCenter(!mailButtonCenter)" />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div class="setting-row">
          <div class="setting-info">
            <strong>加载远程图片</strong>
            <small>默认不加载以保护隐私，开启后可查看内嵌图片</small>
          </div>
          <label class="toggle-switch">
            <input type="checkbox" :checked="mailLoadImages" @change.prevent="appStore.setMailLoadImages(!mailLoadImages)" />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div class="setting-row" style="align-items: flex-start; padding-top: var(--space-lg);">
          <div class="setting-info">
            <strong>字体大小</strong>
            <small>调整邮件正文的文字大小</small>
          </div>
          <div class="font-size-options">
            <button
              v-for="opt in fontSizeOptions"
              :key="opt.value"
              class="font-size-btn"
              :class="{ active: mailFontSize === opt.value }"
              @click="appStore.setMailFontSize(opt.value)"
            >{{ opt.label }}</button>
          </div>
        </div>

        <div class="setting-row" style="align-items: flex-start; padding-top: var(--space-lg);">
          <div class="setting-info">
            <strong>每页显示数量</strong>
            <small>邮件列表每页显示的邮件数量</small>
          </div>
          <select
            class="poll-select"
            :value="mailPageSize"
            @change="handlePageSizeChange(Number($event.target.value))"
          >
            <option v-for="opt in pageSizeOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </div>
      </section>

    </div>
  </div>
</template>

<script setup>
defineOptions({ name: 'Settings' })
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useAppStore } from '@/stores/appStore'
import { PRESET_COLORS, POLL_INTERVAL_OPTIONS, PAGE_SIZE_OPTIONS } from '@/stores/appStore'
import { useSSE } from '@/composables/useSSE'
import { useToast } from '@/composables/useToast'
import { useWebPush } from '@/composables/useWebPush'
import * as webhookApi from '@/api/webhook'
import { useAuthStore } from '@/stores/authStore'
import * as usersApi from '@/api/users'
import * as settingsApi from '@/api/settings'

const toast = useToast()
const appStore = useAppStore()
const authStore = useAuthStore()

// --- 主题 & 通知 ---
const currentTheme = computed(() => appStore.themeMode)
const currentPrimaryColor = computed(() => appStore.primaryColor)
const presetColors = PRESET_COLORS

// Web Push 离线推送（完整实现）
const {
  supported: pushSupported,
  subscribing: pushSubscribing,
  isSubscribed: pushEnabled,
  error: pushError,
  subscribe: doPushSubscribe,
  unsubscribe: doPushUnsubscribe,
  init: initWebPush,
} = useWebPush()

// --- 数据同步设置 ---
const pollOptions = POLL_INTERVAL_OPTIONS
const currentPollInterval = computed(() => appStore.pollInterval)
let sseInstance = null // 保存 SSE 实例用于清理

// 连接状态显示
const connectionStatus = computed(() => {
  const mode = appStore.connectionMode
  switch (mode) {
    case 'sse':
      return { mode, label: 'SSE 实时推送', color: '#22c55e' }
    case 'polling':
      return { mode, label: '定时轮询', color: '#f59e0b' }
    default:
      return { mode: 'unknown', label: '检测中...', color: '#94a3b8' }
  }
})

// 下次刷新倒计时（实时倒数）
const nextRefreshCountdown = ref('--')
let countdownTimer = null

function calcCountdown() {
  const mode = appStore.connectionMode
  if (mode === 'sse') { nextRefreshCountdown.value = '实时推送'; return }
  if (mode !== 'polling') { nextRefreshCountdown.value = '--'; return }

  const last = appStore.lastRefreshAt
  if (!last) { nextRefreshCountdown.value = '--'; return }

  const interval = appStore.pollInterval || 60000
  const elapsed = Date.now() - last
  const remain = Math.max(0, Math.ceil((interval - elapsed) / 1000))

  if (remain <= 0) {
    nextRefreshCountdown.value = '即将刷新...'
  } else if (remain >= 60) {
    nextRefreshCountdown.value = `${Math.floor(remain / 60)}分${remain % 60}秒`
  } else {
    nextRefreshCountdown.value = `${remain} 秒`
  }
}

function startCountdown() {
  stopCountdown()
  calcCountdown()
  countdownTimer = setInterval(calcCountdown, 1000)
}

function stopCountdown() {
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
}

function setPollInterval(ms) {
  appStore.setPollInterval(ms)
  const label = ms >= 60000 ? `${ms / 60000} 分钟` : `${ms / 1000} 秒`
  toast.success(`轮询间隔已设为 ${label}`)
  calcCountdown()
}

/**
 * 检测 SSE 是否可用（快速探测，带超时）
 * 如果连接成功 → 设为 sse；如果失败/超时 → 设为 polling
 */
async function detectConnectionMode() {
  console.log('[Settings] 正在检测连接模式...')

  // 清理旧实例
  if (sseInstance) { sseInstance.disconnect(); sseInstance = null }

  // 快速探测：创建 SSE 实例并手动触发连接
  // （不能用 useSSE 内部的 onMounted 自动连接，因为此时已处于 onMounted 阶段，
  //   useSSE 内部注册的 onMounted 不会再被执行）
  sseInstance = useSSE({
    onConnected: () => {
      console.log('[Settings] ✓ SSE 可用')
      appStore.setConnectionMode('sse')
      calcCountdown()
      // 探测成功后断开（MailListView 会维护自己的连接）
      setTimeout(() => { if (sseInstance) { sseInstance.disconnect(); sseInstance = null } }, 500)
      clearTimeout(detectTimeout)
    },
    onFallback: () => {
      console.log('[Settings] ✗ SSE 不可用，使用轮询模式')
      appStore.setConnectionMode('polling')
      // 初始化时间戳让倒计时开始跑
      if (!appStore.lastRefreshAt) appStore.touchRefresh()
      calcCountdown()
      clearTimeout(detectTimeout)
    },
    onError: () => {
      // 首次错误就标记为 polling（不等待多次重试）
      if (appStore.connectionMode === 'unknown') {
        appStore.setConnectionMode('polling')
        if (!appStore.lastRefreshAt) appStore.touchRefresh()
        calcCountdown()
        clearTimeout(detectTimeout)
      }
    },
  })

  // 手动发起连接（关键！因为 onMounted 阶段注册的 useSSE 内部 onMounted 不会被执行）
  sseInstance.connect()

  // 设置超时兜底：8 秒内没有结果就判定为 polling
  const detectTimeout = setTimeout(() => {
    if (appStore.connectionMode === 'unknown') {
      console.log('[Settings] ⏱ 检测超时，默认使用轮询模式')
      appStore.setConnectionMode('polling')
      updateCountdown()
      if (sseInstance) { sseInstance.disconnect(); sseInstance = null }
    }
  }, 8000)
}

// --- 邮件渲染设置 ---
const mailButtonCenter = computed(() => appStore.mailButtonCenter)
const mailLoadImages = computed(() => appStore.mailLoadImages)
const mailFontSize = computed(() => appStore.mailFontSize)
const mailRenderMode = computed(() => appStore.mailRenderMode)
const mailPageSize = computed(() => appStore.mailPageSize)

const fontSizeOptions = [
  { value: 'small', label: '小' },
  { value: 'medium', label: '中' },
  { value: 'large', label: '大' },
]

const renderModeOptions = [
  { value: 'inline', label: '内联渲染', desc: '直接嵌入页面，支持自定义样式覆盖' },
  { value: 'iframe', label: 'iframe 沙箱', desc: '隔离渲染，更接近原始邮件效果' },
]

const pageSizeOptions = PAGE_SIZE_OPTIONS

function handlePageSizeChange(size) {
  appStore.setMailPageSize(size)
  toast.success(`每页显示数量已设为 ${size} 封`)
}

// --- Webhook 管理 ---
const webhooks = ref([])
const loadingWebhooks = ref(false)
const showForm = ref(false)
const editingHook = ref(null)
const saving = ref(false)
const testingId = ref(null)
const simulatingMail = ref(false)

const form = ref({
  name: '',
  url: '',
  events: 'mail.received',
  secret: '',
  headers: '',
  body: '',
  enabled: true,
})

function resetForm() {
  form.value = { name: '', url: '', events: 'mail.received', secret: '', headers: '', body: '', enabled: true }
}

function closeModal() {
  showForm.value = false
  editingHook.value = null
}

function editHook(hook) {
  editingHook.value = hook
  form.value = {
    name: hook.name,
    url: hook.url,
    events: hook.events,
    secret: '',
    headers: hook.headers || '',
    body: hook.body || '',
    enabled: hook.enabled,
  }
  showForm.value = true
}

async function fetchWebhooks() {
  loadingWebhooks.value = true
  try {
    const res = await webhookApi.listWebhooks()
    webhooks.value = res.data || []
  } catch (e) {
    console.error('获取 Webhook 列表失败:', e)
  } finally {
    loadingWebhooks.value = false
  }
}

// Webhook 投递结果 / 用户变更 实时刷新（SSE 推送，无需手动刷新设置页）
const webhookSSE = useSSE({
  onWebhookDelivered() { fetchWebhooks() },
  onWebhookFailed() { fetchWebhooks() },
  // 多管理员场景下，其他管理员创建/删除用户时本页实时同步
  onUserCreated() { fetchUsers() },
  onUserDeleted() { fetchUsers() },
})

async function saveHook() {
  if (!form.value.name || !form.value.url) return
  saving.value = true
  try {
    if (editingHook.value) {
      await webhookApi.updateWebhook(editingHook.value.id, form.value)
      toast.success('Webhook 已更新')
    } else {
      await webhookApi.createWebhook(form.value)
      toast.success('Webhook 已创建')
    }
    showForm.value = false
    fetchWebhooks()
  } catch (e) {
    toast.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function deleteHookAction(id) {
  if (!await toast.confirm('确定要删除这个 Webhook 吗？')) return
  try {
    await webhookApi.deleteWebhook(id)
    toast.success('已删除')
    fetchWebhooks()
  } catch (e) {
    toast.error(e.message || '删除失败')
  }
}

async function testWebhookAction(id) {
  testingId.value = id
  try {
    const res = await webhookApi.testWebhook(id)
    if (res.success) {
      toast.show(`测试成功 · 状态码 ${res.status_code} · 耗时 ${res.duration_ms}ms`, 'success', 5000)
    } else {
      toast.show(`测试失败 · 状态码 ${res.status_code}${res.error ? ': ' + res.error : ''}`, 'error', 6000)
    }
  } catch (e) {
    toast.error('测试请求失败: ' + e.message)
  } finally {
    testingId.value = null
  }
}

async function simulateMailAction() {
  simulatingMail.value = true
  try {
    const res = await webhookApi.simulateMailReceived()
    if (res.success) {
      toast.show(`${res.message}（事件: ${res.event}）`, 'success', 5000)
      fetchWebhooks() // 刷新列表以更新 last_trigger_at
    } else {
      toast.error(res.error || '模拟触发失败')
    }
  } catch (e) {
    toast.error('模拟请求失败: ' + e.message)
  } finally {
    simulatingMail.value = false
  }
}

function formatEvents(events) {
  const map = { 'mail.received': '收到新邮件', 'mail.sent': '发送邮件', 'test': '测试' }
  return events.split(',').map(e => map[e.trim()] || e.trim()).join(', ')
}

function formatTime(t) {
  if (!t) return ''
  const d = new Date(t)
  return d.toLocaleString('zh-CN')
}

function truncate(str, len) {
  return str && str.length > len ? str.slice(0, len) + '...' : str
}

// --- 用户与权限（仅管理员） ---
const openRegistration = ref(false)
const savingOpenReg = ref(false)
const users = ref([])
const loadingUsers = ref(false)
const showAddUser = ref(false)
const savingUser = ref(false)

const addForm = ref({
  username: '',
  password: '',
  role: 'user',
})

async function fetchOpenRegistration() {
  try {
    const res = await settingsApi.getOpenRegistration()
    openRegistration.value = !!res.open_registration
  } catch (e) {
    console.error('获取开放注册状态失败:', e)
  }
}

async function toggleOpenRegistration() {
  savingOpenReg.value = true
  const next = !openRegistration.value
  try {
    const res = await settingsApi.setOpenRegistration(next)
    openRegistration.value = !!res.open_registration
    toast.success(next ? '已开放公开注册' : '已关闭公开注册')
  } catch (e) {
    toast.error(e.message || '保存失败')
  } finally {
    savingOpenReg.value = false
  }
}

async function fetchUsers() {
  loadingUsers.value = true
  try {
    const res = await usersApi.listUsers()
    users.value = res.data || []
  } catch (e) {
    console.error('获取用户列表失败:', e)
  } finally {
    loadingUsers.value = false
  }
}

function openAddUser() {
  addForm.value = { username: '', password: '', role: 'user' }
  showAddUser.value = true
}

function closeAddUser() {
  showAddUser.value = false
}

async function submitAddUser() {
  if (!addForm.value.username || !addForm.value.password) {
    toast.error('用户名和密码不能为空')
    return
  }
  savingUser.value = true
  try {
    await usersApi.createUser({
      username: addForm.value.username,
      password: addForm.value.password,
      role: addForm.value.role,
    })
    toast.success('用户已创建')
    showAddUser.value = false
    fetchUsers()
  } catch (e) {
    toast.error(e.message || '创建失败')
  } finally {
    savingUser.value = false
  }
}

async function deleteUserAction(u) {
  if (u.username === authStore.username) {
    toast.error('不能删除当前登录的账号')
    return
  }
  if (!await toast.confirm(`确定要删除用户「${u.username}」吗？其邮箱账号、邮件等关联数据将一并删除。`)) return
  try {
    await usersApi.deleteUser(u.id)
    toast.success('已删除')
    fetchUsers()
  } catch (e) {
    toast.error(e.message || '删除失败')
  }
}

// --- 主题选项 ---
const themeOptions = [
  {
    value: 'system',
    label: '跟随系统',
    icon: '<svg width="28" height="28" viewBox="0 0 28 28"><rect x="2" y="6" width="24" height="16" rx="3" fill="#E2E8F0"/><circle cx="9" cy="14" r="4" fill="#94A3B8"/><path d="M19 10a4 4 0 0 1 0 8" stroke="#64748B" stroke-width="1.5" fill="none"/></svg>'
  },
  {
    value: 'light',
    label: '浅色模式',
    icon: '<svg width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="14" r="7" fill="#FBBF24"/><path d="M14 3v2M14 23v2M26 14h-2M4 14H2M22.2 22.2l-1.4-1.4M7.2 7.2L5.8 5.8M22.2 5.8l-1.4 1.4M7.2 20.8L5.8 22.2" stroke="#F59E0B" stroke-width="1.8" stroke-linecap="round"/></svg>'
  },
  {
    value: 'dark',
    label: '深色模式',
    icon: '<svg width="28" height="28" viewBox="0 0 28 28"><path d="M21 15a8 8 0 1 1-12-7A9 9 0 0 0 21 15z" fill="#475569"/></svg>'
  }
]

function setTheme(mode) {
  appStore.setTheme(mode)
}

// --- 主题色 ---
const isCustomColor = computed(() => !presetColors.some(c => c.value === currentPrimaryColor.value))

function handlePrimaryColorChange(color) {
  appStore.setPrimaryColor(color)
  toast.success('主题色已更改')
}

// 自定义颜色防抖
let colorDebounceTimer = null

function onCustomColorInput(event) {
  const color = event.target.value
  if (!color || !color.startsWith('#')) return
  // 实时更新 store 状态（用于 UI 预览）
  appStore.primaryColor = color.toUpperCase()
  // 防抖：停止拖动后才正式应用全部变量 + 提示
  if (colorDebounceTimer) clearTimeout(colorDebounceTimer)
  colorDebounceTimer = setTimeout(() => {
    appStore.setPrimaryColor(color)
    toast.success('主题色已更改')
  }, 200)
}

// --- 通知权限 ---
onMounted(() => {
  checkNotifStatus()
  fetchWebhooks()
  startCountdown()

  if (authStore.isAdmin) {
    fetchOpenRegistration()
    fetchUsers()
  }

  // 监听连接模式变化（MailListView 可能会更新它）
  watch(() => appStore.connectionMode, () => { calcCountdown() }, { immediate: true })

  // 如果还是 unknown，主动检测 SSE 可用性
  if (appStore.connectionMode === 'unknown') {
    detectConnectionMode()
  }
})

onUnmounted(() => {
  stopCountdown()
  if (sseInstance) { sseInstance.disconnect(); sseInstance = null }
})

function checkNotifStatus() {
  initWebPush()
}

async function toggleNotification() {
  // 关闭操作
  if (pushEnabled.value) {
    await doPushUnsubscribe()
    toast.info('离线推送已关闭')
    return
  }

  // 打开操作：执行完整 Web Push 订阅流程
  const ok = await doPushSubscribe()
  if (ok) {
    toast.success('离线推送已启用！即使关闭浏览器也能收到新邮件提醒')

    // 发送测试推送验证通道
    try {
      const { sendTest } = await import('@/api/push')
      await sendTest()
    } catch (_) {}
  }
}

/** 推送状态描述文字 */
const pushStatusLabel = computed(() => {
  if (!pushSupported.value) return '不支持'
  if (pushSubscribing.value) return '连接中...'
  if (pushError.value) return '错误'
  if (pushEnabled.value) return '已启用'
  return '未启用'
})
</script>

<style scoped>
.page-title {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  margin-bottom: var(--space-lg);
}

.settings-sections {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  max-width: 720px;
  margin: 0 auto;
  width: 100%;
}

.settings-section { padding: var(--space-xl); }

.section-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin-bottom: var(--space-xs);
}
.section-desc {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  margin-bottom: var(--space-lg);
}

/* ---- 主题选择器 ---- */
.theme-options {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-md);
}

.theme-option {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-sm);
  padding: var(--space-lg) var(--space-md);
  background: var(--bg-secondary);
  border: 2px solid transparent;
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.theme-option:hover {
  border-color: var(--primary-200);
  background: var(--bg-hover);
}
.theme-option.active {
  border-color: var(--primary-500);
  background: var(--mail-unread-bg);
  box-shadow: 0 0 0 3px var(--mail-unread-bg);
}

.theme-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  overflow: hidden;
}
.theme-icon :deep(svg), .theme-icon :deep(img) {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.theme-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
}

/* ---- 主题色选择器 ---- */
.theme-color-section {
  margin-top: var(--space-xl);
  padding-top: var(--space-lg);
  border-top: 1px solid var(--border-light);
}

.section-subtitle {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin-bottom: var(--space-xs);
}

.color-desc {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  margin-bottom: var(--space-md);
}

.color-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.color-swatch {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-full);
  border: 2px solid transparent;
  cursor: pointer;
  transition: all var(--transition-fast);
  position: relative;
  outline: none;
  padding: 0;
  box-shadow: inset 0 0 0 1px rgba(0,0,0,0.08);
}
.color-swatch:hover {
  transform: scale(1.12);
  box-shadow: 0 2px 8px rgba(0,0,0,0.15);
}
.color-swatch.active {
  border-color: var(--text-primary);
  box-shadow: 0 0 0 2px var(--bg-primary), 0 0 0 4px currentColor;
  transform: scale(1.08);
}

.color-custom {
  background: conic-gradient(from 0deg, #f00, #ff0, #0f0, #0ff, #00f, #f0f, #f00);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  overflow: hidden;
}
.color-custom svg {
  position: relative;
  z-index: 1;
  filter: drop-shadow(0 1px 2px rgba(0,0,0,0.4));
}
.color-custom input[type="color"] {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
  -webkit-appearance: none;
}

.color-value-display {
  margin-top: var(--space-sm);
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  font-family: var(--font-mono), monospace;
}

/* ---- 设置行 ---- */
.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-md) 0;
  gap: var(--space-md);
}
.setting-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.setting-info strong {
  font-size: var(--font-size-base);
  color: var(--text-primary);
}
.setting-info small {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

/* ---- 开关 ---- */
.toggle-switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
  flex-shrink: 0;
}
.toggle-switch input { opacity: 0; width: 0; height: 0; }

.toggle-slider {
  position: absolute;
  inset: 0;
  background: var(--border-color);
  border-radius: var(--radius-full);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.toggle-slider::before {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  background: #fff;
  border-radius: 50%;
  box-shadow: 0 1px 3px rgba(0,0,0,0.15);
  transition: transform var(--transition-fast);
}
.toggle-switch input:checked + .toggle-slider {
  background: linear-gradient(135deg, var(--primary-500), var(--primary-400));
}
.toggle-switch input:checked + .toggle-slider::before {
  transform: translateX(20px);
}
.toggle-switch.toggle-disabled {
  opacity: 0.7;
  pointer-events: none;
}
.toggle-hint {
  position: absolute;
  right: -48px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 11px;
  color: var(--text-tertiary);
  white-space: nowrap;
  animation: pulse-text 1s ease-in-out infinite;
}
.toggle-error-hint {
  color: var(--error, #dc2626);
  cursor: help;
  font-weight: bold;
}

/* ---- 关于信息 ---- */
.about-info {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  padding: var(--space-md) 0;
  border-bottom: 1px solid var(--border-light);
  margin-bottom: var(--space-md);
}
.info-item {
  display: flex;
  justify-content: space-between;
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}
.info-item .text-success { color: var(--success); }
.info-item .text-tertiary { color: var(--text-tertiary); }

.setting-actions {
  display: flex;
  gap: var(--space-sm);
  flex-wrap: wrap;
}

@media (max-width: 480px) {
  .theme-options {
    grid-template-columns: 1fr;
  }

  .settings-section { padding: var(--space-lg); }

  /* 渲染模式 / 字体大小等宽控件行：移动端改为上下堆叠 */
  .setting-row:has(.render-mode-options),
  .setting-row:has(.font-size-options) {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-sm);
  }

  .render-mode-options {
    flex-direction: column;
    width: 100%;
    min-width: unset;
  }
  .render-mode-btn {
    width: 100%;
    box-sizing: border-box;
  }
}

/* ---- Webhook 通知 ---- */
.section-header-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-lg);
  flex-wrap: wrap;
}
.section-header-row .section-title { margin-bottom: var(--space-xs); }

.webhook-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.user-list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  margin-top: var(--space-lg);
  padding-top: var(--space-lg);
  border-top: 1px solid var(--border-light);
}
.user-list-title {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-secondary);
}

.webhook-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  padding: var(--space-md);
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-light);
  transition: border-color var(--transition-fast), opacity var(--transition-fast);
}
.webhook-card:hover { border-color: var(--primary-200); }
.webhook-card.disabled { opacity: 0.6; }

.webhook-info { flex: 1; min-width: 0; }

.webhook-name-row {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  margin-bottom: 4px;
}
.webhook-name {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  font-size: var(--font-size-base);
}

.badge {
  display: inline-block;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: var(--font-weight-medium);
  border-radius: var(--radius-full);
  line-height: 1.5;
  white-space: nowrap;
}
.badge-success { background: var(--success-light); color: var(--success); }
.badge-default { background: var(--bg-hover); color: var(--text-tertiary); }
.badge-success-light { background: rgba(16,185,129,0.08); color: #059669; }
.badge-error-light { background: var(--error-light); color: #dc2626; }

.webhook-url {
  font-size: var(--font-size-sm);
  color: var(--primary-500);
  word-break: break-all;
  margin-bottom: 4px;
}

.webhook-meta {
  display: flex;
  gap: var(--space-md);
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

.webhook-error {
  margin-top: 4px;
  font-size: var(--font-size-xs);
  color: var(--error);
}

.webhook-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.btn-icon-text {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-family: inherit;
  font-size: var(--font-size-sm);
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  transition: all var(--transition-fast);
}
.btn-icon-text:hover { background: var(--bg-hover); color: var(--text-primary); }
.btn-icon-text.text-danger:hover { background: var(--error-light); color: var(--error); }
.btn-icon-text:disabled { opacity: 0.5; cursor: not-allowed; }

/* ---- Webhook 弹窗 ---- */
.webhook-modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.45);
  animation: fadeIn 0.15s ease-out;
}
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }

.webhook-modal {
  width: min(540px, 90vw);
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-primary);
  border-radius: var(--radius-lg);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.2);
  animation: slideUp 0.2s ease-out;
  overflow: hidden;
}
@keyframes slideUp { from { transform: translateY(20px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-lg) var(--space-xl);
  border-bottom: 1px solid var(--border-light);
}

.modal-close {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: var(--text-tertiary);
  line-height: 1;
  padding: 0 4px;
  transition: color var(--transition-fast);
}
.modal-close:hover { color: var(--text-primary); }

.form-title {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin-bottom: 0;
}

.modal-body {
  padding: var(--space-lg) var(--space-xl);
  overflow-y: auto;
  flex: 1;
}

.modal-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-sm);
  padding: var(--space-md) var(--space-xl);
  border-top: 1px solid var(--border-light);
}
.doc-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: var(--font-size-sm);
  color: var(--primary-500);
  text-decoration: none;
  margin-right: auto;
}
.doc-link:hover { color: var(--primary-600); text-decoration: underline; }

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-md);
}
.form-field label {
  display: block;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
  margin-bottom: 4px;
}
.form-field input, .form-field textarea {
  width: 100%;
  padding: 8px 12px;
  font-size: var(--font-size-sm);
  font-family: inherit;
  color: var(--text-primary);
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  outline: none;
  transition: border-color var(--transition-fast);
  box-sizing: border-box;
}
.form-field input:focus, .form-field textarea:focus { border-color: var(--primary-400); }
.field-hint {
  display: block;
  margin-top: 3px;
  font-size: 11px;
  color: var(--text-tertiary);
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-sm);
  margin-top: var(--space-md);
  padding-top: var(--space-md);
  border-top: 1px solid var(--border-light);
}

.spinner-xs {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border-color);
  border-top-color: var(--primary-500);
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
  display: inline-block;
}
@keyframes spin { to { transform: rotate(360deg); } }
@keyframes pulse-text {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.empty-hint {
  text-align: center;
  padding: var(--space-xl) 0;
  color: var(--text-tertiary);
  font-size: var(--font-size-sm);
}

/* ---- 连接状态 ---- */
.connection-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: var(--radius-full);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
}
.status-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  animation: pulse-dot 2s ease-in-out infinite;
}
@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
.connection-status.status-sse .status-dot { background: #22c55e; box-shadow: 0 0 6px rgba(34,197,94,0.4); }
.connection-status.status-sse .status-text { color: #16a34a; }
.connection-status.status-polling .status-dot { background: #f59e0b; }
.connection-status.status-polling .status-text { color: #d97706; }
.connection-status.status-unknown .status-dot { background: #94a3b8; }
.connection-status.status-unknown .status-text { color: #64748b; }

/* ---- 轮询间隔选择器 ---- */
.poll-select {
  padding: 6px 12px;
  border: 1px solid var(--border-color);
  background: var(--bg-secondary);
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  font-family: inherit;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: border-color var(--transition-fast);
  flex-shrink: 0;
}
.poll-select:hover { border-color: var(--primary-300); }
.poll-select:focus {
  outline: none;
  border-color: var(--primary-500);
  box-shadow: 0 0 0 2px var(--primary-200);
}

.next-refresh {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  font-family: var(--font-mono), monospace;
  padding: 4px 12px;
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
}

/* ---- 字体大小选择器 ---- */
.font-size-options {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}
.font-size-btn {
  padding: 6px 16px;
  border: 1px solid var(--border-color);
  background: var(--bg-secondary);
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  font-family: inherit;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.font-size-btn:hover { border-color: var(--primary-300); }
.font-size-btn.active {
  border-color: var(--primary-500);
  background: var(--mail-unread-bg);
  color: var(--primary-600);
  font-weight: var(--font-weight-medium);
}

/* ---- 渲染模式选择器 ---- */
.render-mode-options {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
  min-width: 220px;
}
.render-mode-btn {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  padding: 10px 14px;
  border: 1px solid var(--border-color);
  background: var(--bg-secondary);
  color: var(--text-secondary);
  font-family: inherit;
  text-align: left;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.render-mode-btn:hover { border-color: var(--primary-300); }
.render-mode-btn.active {
  border-color: var(--primary-500);
  background: var(--mail-unread-bg);
}
.render-mode-btn .rm-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
}
.render-mode-btn .rm-desc {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  line-height: 1.4;
}
.render-mode-btn.active .rm-label { color: var(--primary-600); }

/* ---- 离线推送 ---- */
.push-error-hint {
  margin-top: 6px;
  font-size: var(--font-size-xs);
  color: var(--error);
  padding-left: 0;
}

</style>
