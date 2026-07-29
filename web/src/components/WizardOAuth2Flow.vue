<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
  OAuth2 设备码授权交互界面（内嵌于 Step2）
-->
<template>
  <div class="oauth2-flow" :class="{ 'is-active': isActive }">
    <!-- 未激活状态 — 显示启动按钮 -->
    <div v-if="!isActive" class="oauth2-trigger">
      <div class="oauth2-prompt">
        <Lock :size="18" class="prompt-icon" />
        <div class="prompt-text">
          <strong>该邮箱要求使用 OAuth2 安全登录</strong>
          <span class="prompt-sub">推荐方式：无需输入密码，浏览器一键授权</span>
        </div>
      </div>
      <button type="button" class="btn btn-oauth2-start" @click="startAuth">
        <Zap :size="16" /> 开始 OAuth2 授权
      </button>
    </div>

    <!-- 激活状态 — 设备码授权流程 -->
    <div v-else class="oauth2-panel">
      <!-- 加载中 / 等待设备码 -->
      <div v-if="status === 'loading'" class="flow-state flow-loading">
        <div class="spinner-lg"></div>
        <p>正在获取授权验证码...</p>
      </div>

      <!-- 展示验证码 -->
      <div v-else-if="status === 'pending'" class="flow-state flow-pending">
        <p class="flow-instruction">请点击下方链接打开授权页面，然后输入验证码：</p>

        <div class="auth-link-row">
          <a :href="deviceCodeData?.verification_uri" target="_blank" rel="noopener noreferrer" class="auth-link clickable">
            {{ deviceCodeData?.verification_uri }}
            <ExternalLink :size="13" class="link-icon" />
          </a>
          <button type="button" class="btn-copy" @click="copyToClipboard(deviceCodeData?.verification_uri, '链接')">
            <Copy :size="13" />
          </button>
        </div>

        <div class="code-display">
          <label class="code-label">验证码</label>
          <div class="code-value" @click="copyToClipboard(deviceCodeData?.user_code, '验证码')">
            {{ deviceCodeData?.user_code }}
            <Copy :size="14" class="code-copy-hint" />
          </div>
        </div>

        <!-- 倒计时进度条 -->
        <div class="countdown-bar">
          <div class="countdown-fill" :style="{ width: countdownPercent + '%' }"></div>
          <span class="countdown-text">{{ countdownDisplay }} · {{ countdownPercent }}%</span>
        </div>

        <div class="polling-status">
          <div class="spinner-sm"></div>
          <span>等待您在浏览器中完成授权...</span>
        </div>

        <button type="button" class="btn btn-ghost btn-sm btn-cancel-flow" @click="cancelAuth">取消授权</button>
      </div>

      <!-- 授权成功 -->
      <div v-else-if="status === 'success'" class="flow-state flow-success">
        <CheckCircle :size="40" class="success-icon" />
        <strong>OAuth2 授权成功！</strong>
        <p>您的邮箱已安全连接，可以继续下一步了。</p>
      </div>

      <!-- 错误 -->
      <div v-else-if="status === 'error'" class="flow-state flow-error">
        <AlertCircle :size="32" />
        <strong>授权失败</strong>
        <p>{{ errorMessage }}</p>
        <button type="button" class="btn btn-secondary btn-sm" @click="retryAuth">重试</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onUnmounted, watch } from 'vue'
import { Lock, Zap, Copy, CheckCircle, AlertCircle, ExternalLink } from 'lucide-vue-next'
import { requestDeviceCode, pollToken } from '@/api/account'
import { useToast } from '@/composables/useToast'

const props = defineProps({
  providerName: { type: String, default: '' },
  email: { type: String, default: '' },
  customClientId: { type: String, default: '' }
})

const emit = defineEmits(['authorized'])

const toast = useToast()

const isActive = ref(false)
const status = ref('idle') // idle | loading | pending | success | error
const deviceCodeData = ref(null)
const errorMessage = ref('')
let pollTimer = null
let countdownTimer = null
let countdownSeconds = ref(0)
const countdownTotal = ref(0)

const countdownPercent = computed(() => {
  if (countdownTotal.value <= 0) return 0
  return Math.max(0, Math.round((countdownSeconds.value / countdownTotal.value) * 100))
})

const countdownDisplay = computed(() => {
  const min = Math.floor(countdownSeconds.value / 60)
  const sec = countdownSeconds.value % 60
  return `${String(min).padStart(2, '0')}:${String(sec).padStart(2, '0')}`
})

async function startAuth() {
  if (!props.providerName) return
  status.value = 'loading'
  isActive.value = true

  try {
    const res = await requestDeviceCode(props.providerName, { email: props.email, custom_client_id: props.customClientId || undefined })
    deviceCodeData.value = res.data
    countdownTotal.value = res.data.expires_in || 900
    countdownSeconds.value = res.data.expires_in || 900
    status.value = 'pending'

    startCountdown()
    startPolling(res.data.device_code)
  } catch (e) {
    status.value = 'error'
    errorMessage.value = e.message || '无法获取验证码'
  }
}

function startPolling(deviceCode) {
  const interval = deviceCodeData.value?.interval || 5
  pollTimer = setInterval(async () => {
    try {
      const res = await pollToken(props.providerName, { device_code: deviceCode, custom_client_id: props.customClientId || undefined })
      if (res.pending) {
        // 继续轮询
        return
      }
      // 成功或错误
      stopPolling()
      if (res.success && res.data) {
        status.value = 'success'
        emit('authorized', res.data)
      }
    } catch (e) {
      if (e.response?.status === 202) return // still pending
      status.value = 'error'
      errorMessage.value = e.message || '授权过程中发生错误'
      stopPolling()
    }
  }, interval * 1000)
}

function startCountdown() {
  countdownTimer = setInterval(() => {
    countdownSeconds.value--
    if (countdownSeconds.value <= 0) {
      stopPolling()
      status.value = 'error'
      errorMessage.value = '授权超时，请重新发起'
    }
  }, 1000)
}

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
}

function cancelAuth() {
  stopPolling()
  status.value = 'idle'
  isActive.value = false
  deviceCodeData.value = null
}

function retryAuth() {
  cancelAuth()
  startAuth()
}

async function copyToClipboard(text, label) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label}已复制到剪贴板`)
  } catch {
    toast.error('复制失败，请手动选择复制')
  }
}

onUnmounted(() => stopPolling())

watch(() => props.providerName, () => {
  cancelAuth()
})
</script>

<style scoped>
.oauth2-flow { margin-top: var(--space-sm); }

/* ---- 触发区域 ---- */
.oauth2-trigger {
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: var(--space-md);
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.oauth2-prompt {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}
.prompt-icon {
  color: var(--primary-500);
  flex-shrink: 0;
  padding: 8px;
  background: var(--primary-50);
  border-radius: var(--radius-md);
}
.prompt-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.prompt-text strong {
  font-size: var(--font-size-base);
  color: var(--text-primary);
}
.prompt-sub {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

.btn-oauth2-start {
  width: 100%;
  justify-content: center;
  background: linear-gradient(135deg, var(--primary-500), var(--primary-600));
  color: var(--text-on-primary);
  border: none;
  padding: 10px var(--space-lg);
  border-radius: var(--radius-md);
  font-weight: var(--font-weight-semibold);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  transition: all 0.25s ease;
  font-family: inherit;
  font-size: inherit;
}
.btn-oauth2-start:hover {
  opacity: 0.92;
  transform: translateY(-1px);
  box-shadow: 0 4px 16px var(--shadow-glow);
}

/* ---- 授权面板 ---- */
.oauth2-panel {
  border: 1px solid var(--primary-200);
  border-radius: var(--radius-lg);
  padding: var(--space-lg);
  background: var(--primary-50);
  animation: panel-in 0.3s ease;
}
@keyframes panel-in {
  from { opacity: 0; transform: translateY(-8px); }
  to { opacity: 1; transform: translateY(0); }
}

.flow-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-sm);
  text-align: center;
}

.flow-loading { padding: var(--space-xl) 0; }

.flow-instruction {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  text-align: left;
  margin-bottom: var(--space-sm);
  line-height: 1.6;
}

.auth-link-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 10px 12px;
  width: 100%;
}
.auth-link {
  font-size: 13px;
  color: var(--primary-500);
  word-break: break-all;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  text-decoration: none;
  transition: opacity 0.2s, text-decoration 0.2s;
}
.auth-link.clickable:hover {
  opacity: 0.8;
  text-decoration: underline;
}
.link-icon {
  flex-shrink: 0;
}
.btn-copy {
  flex-shrink: 0;
  padding: 5px 8px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  font-family: inherit;
}
.btn-copy:hover {
  background: var(--bg-hover);
  color: var(--primary-500);
}

.code-display {
  width: 100%;
  margin-top: var(--space-md);
}
.code-label {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  display: block;
  margin-bottom: 6px;
}
.code-value {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 28px;
  font-weight: 700;
  letter-spacing: 3px;
  color: var(--text-primary);
  background: var(--bg-secondary);
  border: 2px dashed var(--primary-300);
  border-radius: var(--radius-lg);
  padding: var(--space-md) var(--space-lg);
  text-align: center;
  cursor: pointer;
  position: relative;
  user-select: all;
  transition: border-color 0.2s;
}
.code-value:hover {
  border-color: var(--primary-500);
}
.code-copy-hint {
  position: absolute;
  bottom: 6px;
  right: 10px;
  color: var(--text-tertiary);
  opacity: 0.5;
}

.countdown-bar {
  width: 100%;
  height: 6px;
  background: var(--bg-tertiary);
  border-radius: 3px;
  position: relative;
  margin-top: var(--space-md);
  overflow: hidden;
}
.countdown-fill {
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  background: linear-gradient(90deg, var(--primary-500), var(--primary-400));
  border-radius: 3px;
  transition: width 1s linear;
}
.countdown-text {
  display: block;
  text-align: center;
  font-size: 12px;
  color: var(--text-tertiary);
  margin-top: 4px;
}

.polling-status {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-tertiary);
  font-size: var(--font-size-sm);
  margin-top: var(--space-md);
}

.btn-cancel-flow {
  margin-top: var(--space-md);
  color: var(--text-tertiary);
}

/* 成功/错误 */
.flow-success { padding: var(--space-xl) 0; }
.success-icon { color: var(--success); }
.flow-success strong { color: var(--success); font-size: var(--font-size-base); }
.flow-success p { color: var(--text-secondary); font-size: var(--font-size-sm); }

.flow-error { padding: var(--space-xl) 0; }
.flow-error svg { color: var(--error); }
.flow-error strong { color: var(--error); }
.flow-error p { color: var(--text-secondary); font-size: var(--font-size-sm); }

/* Spinner */
.spinner-lg {
  width: 32px; height: 32px;
  border: 3px solid var(--border-color);
  border-top-color: var(--primary-500);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
.spinner-sm {
  width: 14px; height: 14px;
  border: 2px solid var(--border-color);
  border-top-color: var(--primary-500);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
</style>
