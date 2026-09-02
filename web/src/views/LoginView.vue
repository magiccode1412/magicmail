<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="login-page">
    <!-- 背景装饰 -->
    <div class="bg-orb bg-orb-1"></div>
    <div class="bg-orb bg-orb-2"></div>
    <div class="bg-orb bg-orb-3"></div>
    <div class="bg-grid"></div>

    <!-- 主题切换按钮 -->
    <button class="login-theme-toggle" @click="appStore.toggleTheme()" :title="themeTitle">
      <!-- 跟随系统 -->
      <svg v-show="appStore.themeMode === 'system'" width="20" height="20" viewBox="0 0 28 28">
        <rect x="2" y="6" width="24" height="16" rx="3" fill="none" stroke="currentColor" stroke-width="1.8"/>
        <circle cx="9.5" cy="14" r="3.5" fill="currentColor"/>
        <path d="M19 10a4 4 0 0 1 0 8" stroke="currentColor" stroke-width="1.5" fill="none"/>
      </svg>
      <!-- 浅色模式 -->
      <svg v-show="appStore.themeMode === 'light'" width="20" height="20" viewBox="0 0 28 28">
        <circle cx="14" cy="14" r="7" fill="currentColor"/>
        <path d="M14 3v2M14 23v2M26 14h-2M4 14H2M22.2 22.2l-1.4-1.4M7.2 7.2L5.8 5.8M22.2 5.8l-1.4 1.4M7.2 20.8L5.8 22.2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
      </svg>
      <!-- 深色模式 -->
      <svg v-show="appStore.themeMode === 'dark'" width="20" height="20" viewBox="0 0 28 28">
        <path d="M21 15a8 8 0 1 1-12-7A9 9 0 0 0 21 15z" fill="currentColor"/>
      </svg>
    </button>

    <div class="login-container" :class="{ 'is-register': isRegister }">
      <!-- 左侧品牌区 -->
      <div class="login-brand">
        <div class="brand-content">
          <div class="brand-icon">
            <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
              <rect x="4" y="10" width="40" height="28" rx="6" stroke="white" stroke-width="2.5"/>
              <path d="M4 16h40" stroke="white" stroke-width="2"/>
              <path d="M20 26l5 4 8-9" stroke="white" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/>
              <circle cx="15" cy="24" r="3.5" stroke="white" stroke-width="2"/>
            </svg>
          </div>
          <h1 class="brand-title">Magicmail</h1>
          <p class="brand-desc">简洁 · 安全 · 高效的邮件管理平台</p>

          <div class="brand-features">
            <div class="feature-item">
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
                <path d="M14.5 7l-5.25 5.5L3.5 6.75" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              <span>多账号统一收信</span>
            </div>
            <div class="feature-item">
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
                <path d="M14.5 7l-5.25 5.5L3.5 6.75" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              <span>实时同步 IMAP / POP3</span>
            </div>
            <div class="feature-item">
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
                <path d="M14.5 7l-5.25 5.5L3.5 6.75" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              <span>Webhook 即时通知</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 右侧表单区 -->
      <div class="login-form-area">

        <!-- 飞牛统一网关提示（绑定在登录/注册时自动完成，无需用户手动选择） -->
        <div v-if="authStore.gatewayAvailable && !authStore.isLoggedIn" class="fnos-panel">
          <div class="fnos-tip">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
              <path d="M12 2L4 6v6c0 5 3.4 8.5 8 10 4.6-1.5 8-5 8-10V6l-8-4z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>
            </svg>
            <span v-if="authStore.fnosUsername">已关联飞牛账号「{{ authStore.fnosUsername }}」，登录后将自动绑定</span>
            <span v-else>检测到飞牛登录，完成账号操作后自动绑定当前飞牛账号</span>
          </div>
        </div>

        <transition name="form-slide" mode="out-in">
          <!-- 登录表单 -->
          <div v-if="!isRegister" key="login" class="form-panel">
            <div class="form-header">
              <h2 class="form-title">欢迎回来</h2>
              <p class="form-subtitle">请输入您的管理员账号</p>
            </div>

            <form @submit.prevent="handleSubmit" class="auth-form" autocomplete="on">
              <div class="field-group" :class="{ error: errors.username, focused: focused === 'username' }">
                <label class="field-label">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <circle cx="8" cy="5" r="3.5" stroke="currentColor" stroke-width="1.4"/>
                    <path d="M2 14c0-3.3 2.7-6 6-6s6 2.7 6 6" stroke="currentColor" stroke-width="1.4"/>
                  </svg>
                  用户名
                </label>
                <input
                  ref="usernameInput"
                  v-model="form.username"
                  type="text"
                  placeholder="请输入用户名"
                  autocomplete="username"
                  @focus="focused = 'username'"
                  @blur="focused = ''"
                />
                <span v-if="errors.username" class="field-error">{{ errors.username }}</span>
              </div>

              <div class="field-group" :class="{ error: errors.password, focused: focused === 'password' }">
                <label class="field-label">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <rect x="3" y="7" width="10" height="7" rx="1.5" stroke="currentColor" stroke-width="1.4"/>
                    <path d="M5 7V5a3 3 0 1 1 6 0v2" stroke="currentColor" stroke-width="1.4"/>
                    <circle cx="8" cy="10.5" r="1" fill="currentColor"/>
                  </svg>
                  密码
                </label>
                <input
                  v-model="form.password"
                  ref="passwordInput"
                  :type="showPassword ? 'text' : 'password'"
                  placeholder="请输入密码"
                  autocomplete="current-password"
                  @focus="focused = 'password'"
                  @blur="focused = ''"
                />
                <button type="button" class="pwd-toggle" @click="showPassword = !showPassword" tabindex="-1">
                  <svg v-if="!showPassword" width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M1 8s2.5-4.5 7-4.5S15 8 15 8s-2.5 4.5-7 4.5S1 8 1 8z" stroke="currentColor" stroke-width="1.3"/>
                    <circle cx="8" cy="8" r="2.5" stroke="currentColor" stroke-width="1.3"/>
                  </svg>
                  <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M2 2l12 12M1 8s2.5-4.5 7-4.5c1.4 0 2.6.35 3.65.95L15 1.05M4.35 12.55C5.35 13.17 6.62 13.5 8 13.5c4.5 0 7-4.5 7-4.5s-.85-1.53-2.32-2.88" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                    <circle cx="8" cy="8" r="2.5" stroke="currentColor" stroke-width="1.3"/>
                  </svg>
                </button>
                <span v-if="errors.password" class="field-error">{{ errors.password }}</span>
              </div>

              <div v-if="errorMsg" class="form-error-bar">
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                  <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.3"/>
                  <path d="M7 4v3M7 9.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                </svg>
                {{ errorMsg }}
              </div>

              <button type="submit" class="btn-submit" :disabled="submitting">
                <span v-if="submitting" class="btn-spinner"></span>
                <template v-else>登 录</template>
              </button>

              <button v-if="canRegister" type="button" class="btn-switch-mode" @click="switchToRegister">
                还没有账号？立即注册
              </button>
            </form>
          </div>

          <!-- 注册表单 -->
          <div v-else key="register" class="form-panel">
            <div class="form-header">
              <h2 class="form-title">{{ registerTitle }}</h2>
              <p class="form-subtitle">{{ registerSubtitle }}</p>
            </div>

            <form @submit.prevent="handleRegister" class="auth-form" autocomplete="on">
              <div class="field-group" :class="{ error: errors.username, focused: focused === 'username' }">
                <label class="field-label">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <circle cx="8" cy="5" r="3.5" stroke="currentColor" stroke-width="1.4"/>
                    <path d="M2 14c0-3.3 2.7-6 6-6s6 2.7 6 6" stroke="currentColor" stroke-width="1.4"/>
                  </svg>
                  用户名
                </label>
                <input
                  v-model="regForm.username"
                  type="text"
                  :placeholder="registerUsernamePlaceholder"
                  autocomplete="new-username"
                  autofocus
                  @focus="focused = 'username'"
                  @blur="focused = ''"
                />
                <span v-if="errors.username" class="field-error">{{ errors.username }}</span>
              </div>

              <div class="field-group" :class="{ error: errors.password, focused: focused === 'password' }">
                <label class="field-label">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <rect x="3" y="7" width="10" height="7" rx="1.5" stroke="currentColor" stroke-width="1.4"/>
                    <path d="M5 7V5a3 3 0 1 1 6 0v2" stroke="currentColor" stroke-width="1.4"/>
                    <circle cx="8" cy="10.5" r="1" fill="currentColor"/>
                  </svg>
                  密码
                </label>
                <input
                  v-model="regForm.password"
                  :type="showRegPassword ? 'text' : 'password'"
                  placeholder="至少 6 位密码"
                  autocomplete="new-password"
                  @focus="focused = 'password'"
                  @blur="focused = ''"
                />
                <button type="button" class="pwd-toggle" @click="showRegPassword = !showRegPassword" tabindex="-1">
                  <svg v-if="!showRegPassword" width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M1 8s2.5-4.5 7-4.5S15 8 15 8s-2.5 4.5-7 4.5S1 8 1 8z" stroke="currentColor" stroke-width="1.3"/><circle cx="8" cy="8" r="2.5" stroke="currentColor" stroke-width="1.3"/>
                  </svg>
                  <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M2 2l12 12M1 8s2.5-4.5 7-4.5c1.4 0 2.6.35 3.65.95L15 1.05M4.35 12.55C5.35 13.17 6.62 13.5 8 13.5c4.5 0 7-4.5 7-4.5s-.85-1.53-2.32-2.88" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                    <circle cx="8" cy="8" r="2.5" stroke="currentColor" stroke-width="1.3"/>
                  </svg>
                </button>
                <span v-if="errors.password" class="field-error">{{ errors.password }}</span>
              </div>

              <div class="field-group" :class="{ error: errors.confirmPwd, focused: focused === 'confirmPwd' }">
                <label class="field-label">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <rect x="3" y="7" width="10" height="7" rx="1.5" stroke="currentColor" stroke-width="1.4"/>
                    <path d="M5 7V5a3 3 0 1 1 6 0v2" stroke="currentColor" stroke-width="1.4"/>
                    <circle cx="8" cy="10.5" r="1" fill="currentColor"/>
                  </svg>
                  确认密码
                </label>
                <input
                  v-model="regForm.confirmPassword"
                  :type="showRegPassword ? 'text' : 'password'"
                  placeholder="再次输入密码"
                  autocomplete="new-password"
                  @focus="focused = 'confirmPwd'"
                  @blur="focused = ''"
                />
                <span v-if="errors.confirmPwd" class="field-error">{{ errors.confirmPwd }}</span>
              </div>

              <div v-if="errorMsg" class="form-error-bar">
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                  <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.3"/>
                  <path d="M7 4v3M7 9.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                </svg>
                {{ errorMsg }}
              </div>

              <button type="submit" class="btn-submit btn-register" :disabled="submitting">
                <span v-if="submitting" class="btn-spinner"></span>
                <template v-else>创 建 账 号</template>
              </button>

              <button type="button" class="btn-switch-mode" @click="isRegister = false; clearErrors()">
                已有账号？返回登录
              </button>
            </form>
          </div>
        </transition>

        <div class="login-footer">
          <span>Magicmail</span>
          <span class="divider">·</span>
          <span>v{{ APP_VERSION }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
defineOptions({ name: 'Login' })
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/authStore'
import { useAppStore } from '@/stores/appStore'
import { APP_VERSION } from '@/appVersion'

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

// 主题切换提示文字
const themeTitle = computed(() => {
  const titles = { system: '跟随系统', light: '浅色模式', dark: '深色模式' }
  return titles[appStore.themeMode] || '切换主题'
})

// 开发环境默认凭据
const DEV_DEFAULT_USERNAME = import.meta.env.DEV ? 'admin' : ''
const DEV_DEFAULT_PASSWORD = import.meta.env.DEV ? 'admin123' : ''

// 表单状态
const form = reactive({ username: DEV_DEFAULT_USERNAME, password: DEV_DEFAULT_PASSWORD })
const regForm = reactive({ username: '', password: '', confirmPassword: '' })
const errors = reactive({ username: '', password: '', confirmPwd: '' })
const errorMsg = ref('')
const submitting = ref(false)
const showPassword = ref(false)
const showRegPassword = ref(false)
const focused = ref('')
const usernameInput = ref(null)
const passwordInput = ref(null)

// 飞牛网关提示状态：自动绑定由后端在登录/注册时完成，前端仅展示提示

// 模式切换：登录 / 注册
const isRegister = ref(authStore.setupRequired)
// 公开注册开启时，已存在用户也可自助注册为普通用户
const canRegister = computed(() => authStore.setupRequired || authStore.openRegistration)
// 是否为首次初始化（创建管理员）
const isInitialSetup = computed(() => authStore.setupRequired)

// 注册表单标题/副标题：区分“初始化管理员”与“开放注册”
const registerTitle = computed(() => isInitialSetup.value ? '初始化管理员' : '创建账号')
const registerSubtitle = computed(() =>
  isInitialSetup.value ? '创建您的管理员账号以开始使用' : '注册一个新账号以使用 Magicmail'
)
const registerUsernamePlaceholder = computed(() =>
  isInitialSetup.value ? '设置管理员用户名' : '设置用户名'
)

function switchToRegister() {
  isRegister.value = true
  clearErrors()
}

function clearErrors() {
  errors.username = ''
  errors.password = ''
  errors.confirmPwd = ''
  errorMsg.value = ''
}

// 登录提交
async function handleSubmit() {
  clearErrors()
  if (!form.username.trim()) { errors.username = '请输入用户名'; return }
  if (!form.password) { errors.password = '请输入密码'; return }

  submitting.value = true
  errorMsg.value = ''
  try {
    await authStore.doLogin(form)
    router.push('/')
  } catch (e) {
    errorMsg.value = e.message || '登录失败，请重试'
  } finally {
    submitting.value = false
  }
}

// 注册提交
async function handleRegister() {
  clearErrors()
  let hasError = false

  if (!regForm.username.trim()) { errors.username = '请输入用户名'; hasError = true }
  else if (regForm.username.length < 3) { errors.username = '用户名至少 3 位'; hasError = true }
  else if (regForm.username.length > 32) { errors.username = '用户名不超过 32 位'; hasError = true }

  if (!regForm.password) { errors.password = '请输入密码'; hasError = true }
  else if (regForm.password.length < 6) { errors.password = '密码至少 6 位'; hasError = true }

  if (!regForm.confirmPassword) { errors.confirmPwd = '请确认密码'; hasError = true }
  else if (regForm.password !== regForm.confirmPassword) { errors.confirmPwd = '两次密码不一致'; hasError = true }

  if (hasError) return

  submitting.value = true
  errorMsg.value = ''
  try {
    await authStore.doRegister(regForm)
    router.push('/')
  } catch (e) {
    errorMsg.value = e.message || '注册失败，请重试'
  } finally {
    submitting.value = false
  }
}

// ---- 飞牛网关登录 ----
// 注意：飞牛账号的绑定与免密登录已自动化处理：
//   - 已绑定用户在 store.init() 中自动免密登录进入系统；
//   - 登录 / 注册提交时后端自动将当前账号绑定到飞牛身份。
// 此处不再提供手动选择「飞牛登录/绑定」的交互。

onMounted(async () => {
  if (!authStore.initialized) {
    await authStore.init()
  }
  isRegister.value = authStore.setupRequired

  // 开发环境自动聚焦到密码框（用户名已预填）
  if (import.meta.env.DEV && form.username) {
    await nextTick()
    passwordInput.value?.focus()
  }
})
</script>

<style scoped>
/* ====== 页面容器 ====== */
.login-page {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  /* 默认深色背景（向后兼容无 JS 情况） */
  background: linear-gradient(135deg, #0f172a 0%, #1e293b 50%, #0f172a 100%);
}

/* 浅色模式下页面背景 */
[data-theme="light"] .login-page {
  background: linear-gradient(135deg, #e2e8f0 0%, #cbd5e1 40%, #e2e8f0 100%);
}
[data-theme="light"] .bg-grid {
  background-image:
    linear-gradient(rgba(0,0,0,0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(0,0,0,0.04) 1px, transparent 1px);
}

/* ====== 主题切换按钮 ====== */
.login-theme-toggle {
  position: fixed;
  top: 20px;
  right: 24px;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border: 1px solid var(--border-light, rgba(255,255,255,0.12));
  border-radius: 12px;
  background: var(--glass-bg, rgba(255,255,255,0.06));
  color: var(--text-tertiary, #94a3b8);
  cursor: pointer;
  transition: all 0.25s ease;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}
[data-theme="light"] .login-theme-toggle {
  border-color: rgba(0,0,0,0.08);
  background: rgba(255,255,255,0.75);
}
.login-theme-toggle:hover {
  color: var(--text-primary, #f1f5f9);
  border-color: var(--primary-300, rgba(79,110,247,0.4));
  box-shadow: 0 0 0 3px var(--mail-unread-bg, rgba(79,110,247,0.12));
  transform: rotate(15deg);
}
.login-theme-toggle svg {
  transition: transform 0.25s ease;
}

/* 背景装饰 - 动态光球 */
.bg-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.4;
  animation: orb-float 12s ease-in-out infinite alternate;
}
.bg-orb-1 {
  width: 500px; height: 500px;
  top: -120px; right: -80px;
  background: radial-gradient(circle, var(--primary-500, #4F6EF7) 0%, transparent 70%);
  animation-delay: 0s;
}
.bg-orb-2 {
  width: 400px; height: 400px;
  bottom: -80px; left: -60px;
  background: radial-gradient(circle, var(--info, #06B6D4) 0%, transparent 70%);
  animation-delay: -4s;
}
.bg-orb-3 {
  width: 300px; height: 300px;
  top: 40%; left: 45%;
  background: radial-gradient(circle, #8B5CF6 0%, transparent 70%);
  opacity: 0.2;
  animation-delay: -8s;
}
@keyframes orb-float {
  0%   { transform: translate(0, 0) scale(1); }
  33%  { transform: translate(30px, -20px) scale(1.05); }
  66%  { transform: translate(-20px, 20px) scale(0.95); }
  100% { transform: translate(10px, 10px) scale(1.02); }
}

/* 网格背景 */
.bg-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(255,255,255,0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255,255,255,0.03) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(ellipse at center, black 30%, transparent 75%);
  pointer-events: none;
}

/* ====== 主容器 ====== */
.login-container {
  position: relative;
  z-index: 1;
  display: flex;
  width: 900px;
  max-width: 94vw;
  min-height: 520px;
  background: rgba(255, 255, 255, 0.03);
  backdrop-filter: blur(24px);
  -webkit-backdrop-filter: blur(24px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 24px;
  overflow: hidden;
  box-shadow:
    0 32px 64px rgba(0, 0, 0, 0.4),
    0 0 0 1px rgba(255, 255, 255, 0.05) inset;
  animation: container-in 0.6s cubic-bezier(0.16, 1, 0.3, 1);
}
[data-theme="light"] .login-container {
  background: rgba(255, 255, 255, 0.82);
  border-color: rgba(0, 0, 0, 0.08);
  box-shadow:
    0 32px 64px rgba(0, 0, 0, 0.12),
    0 0 0 1px rgba(0, 0, 0, 0.04) inset;
}

@keyframes container-in {
  from { opacity: 0; transform: translateY(30px) scale(0.97); }
  to   { opacity: 1; transform: translateY(0) scale(1); }
}

/* ====== 左侧品牌区 ====== */
.login-brand {
  flex: 1;
  padding: 52px 44px;
  display: flex;
  align-items: center;
  background: linear-gradient(160deg, var(--primary-500, #4F6EF7) 0%, var(--primary-600, #6366F1) 60%, #8B5CF6 100%);
  position: relative;
  overflow: hidden;
}
.login-brand::before {
  content: '';
  position: absolute;
  top: -50%; right: -40%;
  width: 400px; height: 400px;
  background: rgba(255,255,255,0.08);
  border-radius: 50%;
  filter: blur(40px);
}
.login-brand::after {
  content: '';
  position: absolute;
  bottom: -30%; left: -20%;
  width: 300px; height: 300px;
  background: rgba(255,255,255,0.06);
  border-radius: 50%;
  filter: blur(30px);
}

.brand-content {
  position: relative;
  z-index: 1;
  color: #fff;
}

.brand-icon {
  margin-bottom: 28px;
  opacity: 0.92;
  animation: icon-bounce 2s ease-in-out infinite;
}
@keyframes icon-bounce {
  0%, 100% { transform: translateY(0); }
  50%      { transform: translateY(-6px); }
}

.brand-title {
  font-size: 36px;
  font-weight: 800;
  letter-spacing: -1px;
  margin-bottom: 12px;
  line-height: 1.2;
}

.brand-desc {
  font-size: 15px;
  opacity: 0.82;
  line-height: 1.6;
  margin-bottom: 36px;
}

.brand-features {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.feature-item {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  opacity: 0.88;
}
.feature-item svg {
  flex-shrink: 0;
  opacity: 0.7;
}

/* ====== 右侧表单区 ====== */
.login-form-area {
  flex: 1.1;
  padding: 48px 44px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  background: rgba(15, 23, 42, 0.6);
}
[data-theme="light"] .login-form-area {
  background: rgba(255, 255, 255, 0.55);
}

.form-panel {
  width: 100%;
}

.form-header {
  margin-bottom: 32px;
}
.form-title {
  font-size: 26px;
  font-weight: 700;
  color: var(--text-primary, #f1f5f9);
  letter-spacing: -0.5px;
  margin-bottom: 6px;
}
.form-subtitle {
  font-size: 14px;
  color: var(--text-secondary, #94a3b8);
}

/* ====== 表单字段 ====== */
.auth-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.field-group {
  position: relative;
}
.field-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-tertiary, #94a3b8);
  margin-bottom: 8px;
  transition: color 0.2s;
}
.field-group.focused .field-label {
  color: var(--primary-500, #4F6EF7);
}
.field-group.error .field-label {
  color: var(--error, #ef4444);
}

.field-group input {
  width: 100%;
  padding: 12px 14px;
  padding-right: 44px;
  font-size: 15px;
  font-family: inherit;
  color: var(--text-primary, #e2e8f0);
  background: var(--input-bg, rgba(255, 255, 255, 0.06));
  border: 1px solid var(--border-light, rgba(255, 255, 255, 0.1));
  border-radius: 12px;
  outline: none;
  box-sizing: border-box;
  transition: all 0.25s ease;
}
[data-theme="light"] .field-group input {
  color: var(--text-primary, #1E293B);
}
.field-group input::placeholder {
  color: var(--text-tertiary, #475569);
}
.field-group input:focus {
  background: rgba(79, 110, 247, 0.06);
  border-color: var(--primary-500, #4F6EF7);
  box-shadow: 0 0 0 3px var(--mail-unread-bg, rgba(79, 110, 247, 0.15));
}
.field-group.error input {
  border-color: var(--error, #ef4444);
  background: var(--error-light, rgba(239, 68, 68, 0.04));
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.1);
}

.pwd-toggle {
  position: absolute;
  right: 10px;
  top: 38px;
  background: none;
  border: none;
  color: var(--text-tertiary, #475569);
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  transition: color 0.2s;
  display: flex;
}
.pwd-toggle:hover { color: var(--text-secondary, #94a3b8); }

.field-error {
  display: block;
  font-size: 12px;
  color: var(--error, #ef4444);
  margin-top: 4px;
}

.form-error-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: var(--error-light, rgba(239, 68, 68, 0.08));
  border: 1px solid rgba(239, 68, 68, 0.2);
  border-radius: 10px;
  color: var(--error, #fca5a5);
  font-size: 13px;
  animation: shake 0.4s ease;
}
@keyframes shake {
  0%, 100% { transform: translateX(0); }
  25% { transform: translateX(-6px); }
  75% { transform: translateX(6px); }
}

/* ====== 提交按钮 ====== */
.btn-submit {
  width: 100%;
  padding: 14px;
  font-size: 15px;
  font-weight: 700;
  font-family: inherit;
  color: #fff;
  background: linear-gradient(135deg, var(--primary-500, #4F6EF7), var(--primary-600, #6366F1));
  border: none;
  border-radius: 12px;
  cursor: pointer;
  letter-spacing: 2px;
  position: relative;
  overflow: hidden;
  transition: all 0.3s ease;
  box-shadow: 0 4px 16px rgba(79, 110, 247, 0.35);
}
.btn-submit::before {
  content: '';
  position: absolute;
  top: 0; left: -100%;
  width: 100%; height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255,255,255,0.15), transparent);
  transition: left 0.5s ease;
}
.btn-submit:hover:not(:disabled)::before {
  left: 100%;
}
.btn-submit:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 6px 24px rgba(79, 110, 247, 0.45);
}
.btn-submit:active:not(:disabled) {
  transform: translateY(0);
}
.btn-submit:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}
.btn-register {
  background: linear-gradient(135deg, var(--info, #06B6D4), #0891B2);
  box-shadow: 0 4px 16px rgba(6, 182, 212, 0.35);
}
.btn-register:hover:not(:disabled) {
  box-shadow: 0 6px 24px rgba(6, 182, 212, 0.45);
}

.btn-spinner {
  display: inline-block;
  width: 18px; height: 18px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
  vertical-align: middle;
}
@keyframes spin { to { transform: rotate(360deg); } }

.btn-switch-mode {
  background: none;
  border: none;
  color: var(--text-tertiary, #64748b);
  font-family: inherit;
  font-size: 13px;
  cursor: pointer;
  text-align: center;
  padding: 4px 0;
  transition: color 0.2s;
}
.btn-switch-mode:hover { color: var(--text-secondary, #94a3b8); }

/* ====== 底部 ====== */
.login-footer {
  text-align: center;
  font-size: 12px;
  color: var(--text-tertiary, #334155);
  margin-top: 28px;
}
.divider { margin: 0 6px; }

/* ====== 表单切换动画 ====== */
.form-slide-enter-active {
  animation: slide-in 0.35s cubic-bezier(0.16, 1, 0.3, 1);
}
.form-slide-leave-active {
  animation: slide-out 0.25s cubic-bezier(0.7, 0, 0.84, 0);
}
@keyframes slide-in {
  from { opacity: 0; transform: translateX(24px); }
  to   { opacity: 1; transform: translateX(0); }
}
@keyframes slide-out {
  from { opacity: 1; transform: translateX(0); }
  to   { opacity: 0; transform: translateX(-24px); }
}

/* ====== 响应式适配 ====== */
@media (max-width: 768px) {
  .login-container {
    flex-direction: column;
    width: calc(100vw - 16px);
    max-width: 420px;
    min-height: auto;
  }
  .login-brand {
    padding: 32px 28px;
  }
  .brand-title { font-size: 28px; }
  .brand-desc { font-size: 13px; margin-bottom: 24px; }
  .brand-features { gap: 10px; }
  .feature-item { font-size: 13px; }
  .feature-item svg { width: 16px; height: 16px; }

  .login-form-area {
    padding: 32px 28px 28px;
  }
  .form-title { font-size: 22px; }
  .form-header { margin-bottom: 24px; }
  .auth-form { gap: 16px; }

  /* 移动端隐藏品牌特性列表 */
  .brand-features { display: none; }
  .brand-icon { margin-bottom: 18px; }

  .bg-orb { opacity: 0.25; }

  .login-theme-toggle {
    top: 12px;
    right: 12px;
    width: 36px;
    height: 36px;
  }
}

@media (max-width: 480px) {
  .login-container {
    border-radius: 18px;
  }
  .login-brand { padding: 28px 24px; }
  .login-form-area { padding: 28px 24px 24px; }
  .brand-title { font-size: 24px; }
  .form-title { font-size: 20px; }
}

/* ====== 飞牛网关提示条 ====== */
.fnos-panel {
  margin-bottom: 24px;
  padding: 12px 16px;
  border: 1px solid rgba(79, 110, 247, 0.28);
  border-radius: 12px;
  background: linear-gradient(135deg, rgba(79, 110, 247, 0.08), rgba(6, 182, 212, 0.06));
}
[data-theme="light"] .fnos-panel {
  background: rgba(79, 110, 247, 0.06);
  border-color: rgba(79, 110, 247, 0.22);
}
.fnos-tip {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-secondary, #94a3b8);
  margin: 0;
  text-align: center;
}
.fnos-tip svg {
  flex-shrink: 0;
  color: var(--info, #06B6D4);
}
</style>
