<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
  向导 Step2: 动态配置表单
  根据 selectedProvider 类型渲染不同视图：
  - 国内邮箱(QQ/163等): 简化模式（名称+邮箱+授权码）
  - OAuth2邮箱(Outlook/Gmail): 名称+邮箱+OAuth2授权按钮
  - 其他(手动配置): 完整表单
-->
<template>
  <div class="config-step">
    <div class="step-title-bar">
      <ChevronLeft :size="18" class="back-btn" @click="$emit('back')" />
      <h4 class="step-title">配置 {{ providerName }}</h4>
    </div>

    <!-- 国内邮箱简化模式 -->
    <div v-if="isDomesticProvider" class="config-form simple-mode">
      <div class="form-group">
        <label class="form-label">显示名称 <span class="required">*</span></label>
        <input v-model="localForm.name" type="text" class="input" placeholder="如：工作 QQ 邮箱" />
      </div>
      <div class="form-group">
        <label class="form-label">邮箱地址 <span class="required">*</span></label>
        <div class="email-input-row">
          <input v-model="emailPrefix" type="text" class="input email-prefix" placeholder="用户名" />
          <!-- 单域名：静态显示 -->
          <span v-if="domain && !hasDomainOptions" class="email-domain">@{{ domain }}</span>
          <!-- 多域名：下拉选择 -->
          <select v-else-if="hasDomainOptions" v-model="selectedDomain" class="input email-domain-select">
            <option v-for="d in providerDomains" :key="d" :value="d">@{{ d }}</option>
          </select>
          <!-- 无域名（其他/新浪等）：完整输入 -->
          <input v-else v-model="localForm.email" type="email" class="input email-full" placeholder="your@email.com" />
        </div>
      </div>
      <div class="form-group">
        <label class="form-label">授权码/密码 <span class="required">*</span></label>
        <div class="password-field">
          <input v-model="localForm.password" :type="showPwd ? 'text' : 'password'" class="input input-password" :placeholder="'输入' + providerName + ' 授权码'" />
          <button type="button" class="toggle-pwd btn-ghost btn-sm" @click="showPwd = !showPwd">{{ showPwd ? '隐藏' : '显示' }}</button>
        </div>
        <p class="form-hint help-text">{{ helpText }}</p>
      </div>

      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="$emit('back')">返回</button>
        <button type="button" class="btn btn-primary" @click="handleNext" :disabled="!canProceedSimple">下一步</button>
      </div>
    </div>

    <!-- OAuth2 邮箱模式 -->
    <div v-else-if="isOAuth2Provider" class="config-form oauth2-mode">
      <div class="form-group">
        <label class="form-label">显示名称 <span class="required">*</span></label>
        <input v-model="localForm.name" type="text" class="input" placeholder="如：我的 Outlook 邮箱" />
      </div>
      <div class="form-group">
        <label class="form-label">邮箱地址 <span class="optional">（可选）</span></label>
        <input v-model="localForm.email" type="email" class="input" placeholder="your@email.com" />
        <p class="form-hint text-muted">完成授权后将自动获取，也可在此填写以预填微软登录页</p>
      </div>

      <!-- OAuth2 授权区域 -->
      <WizardOAuth2Flow
        :provider-name="oauthProviderName"
        :email="localForm.email"
        :custom-client-id="localForm.custom_client_id"
        @authorized="onAuthorized"
      />

      <!-- 自定义 Client ID（高级选项） -->
      <details class="advanced-section">
        <summary class="advanced-summary">高级选项</summary>
        <div class="advanced-fields">
          <div class="form-group">
            <label class="form-label">自定义 Client ID</label>
            <input v-model="localForm.custom_client_id" type="text" class="input font-mono" placeholder="留空使用默认值" />
            <p class="form-hint text-muted">如需使用自注册的 Azure AD 应用，请填写其 Application (client) ID</p>
          </div>
        </div>
      </details>

      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="$emit('back')">返回</button>
        <button type="button" class="btn btn-primary" @click="handleNext" :disabled="!canProceedOAuth2">
          下一步
          <ChevronRight :size="16" />
        </button>
      </div>
    </div>

    <!-- 其他（手动完整配置） -->
    <div v-else class="config-form manual-mode">
      <div class="form-group">
        <label class="form-label">显示名称 <span class="required">*</span></label>
        <input v-model="localForm.name" type="text" class="input" placeholder="如：我的邮箱" />
      </div>
      <div class="form-group">
        <label class="form-label">邮箱地址</label>
        <input v-model="localForm.email" type="email" class="input" placeholder="your@email.com（可选）" />
      </div>
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label">协议 <span class="required">*</span></label>
          <select v-model="localForm.protocol" class="input">
            <option value="imap">IMAP（推荐）</option>
            <option value="pop3">POP3 (SSL)</option>
          </select>
        </div>
      </div>
      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label">收信服务器 <span class="required">*</span></label>
          <input v-model="localForm.host" type="text" class="input" placeholder="imap.example.com" />
        </div>
        <div class="form-group port-group">
          <label class="form-label">端口</label>
          <input v-model.number="localForm.port" type="number" class="input" min="1" max="65535" placeholder="993" />
        </div>
      </div>
      <div class="form-group">
        <label class="form-label">用户名 <span class="required">*</span></label>
        <input v-model="localForm.username" type="text" class="input" placeholder="通常与邮箱地址相同" />
      </div>
      <div class="form-group">
        <label class="form-label">密码 <span class="required">*</span></label>
        <div class="password-field">
          <input v-model="localForm.password" :type="showPwd ? 'text' : 'password'" class="input input-password" placeholder="输入登录密码" />
          <button type="button" class="toggle-pwd btn-ghost btn-sm" @click="showPwd = !showPwd">{{ showPwd ? '隐藏' : '显示' }}</button>
        </div>
      </div>

      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="$emit('back')">返回</button>
        <button type="button" class="btn btn-primary" @click="handleNext" :disabled="!canProceedManual">下一步</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import WizardOAuth2Flow from './WizardOAuth2Flow.vue'
import { getProviderByKey } from '@/data/providers.js'

const props = defineProps({
  selectedProvider: { type: Object, default: null },
  formData: { type: Object, required: true }
})

const emit = defineEmits(['next', 'back', 'complete', 'oauth2Authorized'])

const showPwd = ref(false)

// 本地表单数据（从 props.formData 初始化）
const localForm = reactive({ ...props.formData })
watch(() => props.formData, (val) => Object.assign(localForm, val), { deep: true })

// Provider 判断
const isDomesticProvider = computed(() => {
  const key = props.selectedProvider?.key
  return ['qq', '163', '126', 'sina', 'aliyun', 'yahoo', 'gmail'].includes(key)
})

const isOAuth2Provider = computed(() => {
  return props.selectedProvider?.oauthRequired || false
})

const providerName = computed(() => props.selectedProvider?.name || '')
const domain = computed(() => props.selectedProvider?.domain || '')
const providerDomains = computed(() => props.selectedProvider?.domains || [])
const oauthProviderName = computed(() => props.selectedProvider?.oauthProvider || '')
const helpText = computed(() => props.selectedProvider?.helpText || '')

// 是否有多个域名可选（下拉模式）
const hasDomainOptions = computed(() => providerDomains.value.length > 0)

// 下拉选中的域名
const selectedDomain = ref('')
// 初始化选中第一个域名
watch(providerDomains, (d) => {
  if (d.length > 0 && !selectedDomain.value) {
    selectedDomain.value = d[0]
  }
}, { immediate: true })

// 邮箱前缀拆分（用于国内邮箱 @domain 后缀自动填充）
const emailPrefix = computed({
  get: () => {
    // 多域名下拉模式：用选中的域名拆分
    const suffixDomain = hasDomainOptions.value ? selectedDomain.value : domain.value
    if (!suffixDomain) return localForm.email
    const suffix = '@' + suffixDomain
    return localForm.email.endsWith(suffix) ? localForm.email.slice(0, -suffix.length) : localForm.email
  },
  set: (val) => {
    // 多域名下拉模式：拼接选中的域名；单域名模式：拼接固定域名
    if (hasDomainOptions.value && selectedDomain.value) {
      localForm.email = `${val}@${selectedDomain.value}`
    } else if (domain.value) {
      localForm.email = `${val}@${domain.value}`
    } else {
      localForm.email = val
    }
  }
})

// 校验
const canProceedSimple = computed(() =>
  localForm.name && localForm.email && localForm.password
)
const canProceedOAuth2 = computed(() =>
  localForm.name && (localForm.refresh_token || localForm.password)
)
const canProceedManual = computed(() =>
  localForm.name && localForm.host && localForm.username && localForm.password
)

function handleNext() {
  // 将本地表单同步到父组件的 formData
  Object.assign(props.formData, localForm)
  emit('next')
}

function onAuthorized(tokenData) {
  // 先将当前已填写的本地表单数据同步到父组件 formData，防止被后续 watch 覆盖
  Object.assign(props.formData, localForm)
  // 授权成功且用户尚未填写邮箱时，自动从授权结果回填邮箱
  if (!localForm.email && tokenData.email) {
    localForm.email = tokenData.email
  }
  emit('oauth2Authorized', tokenData)
}
</script>

<style scoped>
.config-step { display: flex; flex-direction: column; gap: var(--space-lg); }

.step-title-bar {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}
.back-btn {
  cursor: pointer;
  color: var(--text-tertiary);
  padding: 4px;
  border-radius: var(--radius-sm);
  transition: color 0.2s;
}
.back-btn:hover { color: var(--primary-500); background: var(--bg-hover); }
.step-title {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
}

/* ---- 表单通用 ---- */
.config-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}
.form-row { display: flex; gap: var(--space-md); }
.flex-1 { flex: 1; }
.form-group { display: flex; flex-direction: column; gap: 6px; }
.port-group { width: 100px; }

.form-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
}
.required { color: var(--error); }
.optional { color: var(--text-tertiary); font-weight: var(--font-weight-normal); font-size: var(--font-size-xs); }

.input {
  padding: 10px 14px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  font-family: inherit;
  transition: border-color 0.2s;
  outline: none;
}
.input:focus { border-color: var(--primary-400); box-shadow: 0 0 0 3px var(--mail-unread-bg, var(--primary-100)); }
.input::placeholder { color: var(--text-tertiary); opacity: 0.7; }

/* ---- 密码字段 ---- */
.password-field { position: relative; display: flex; align-items: center; }
.input-password { padding-right: 60px; width: 100%; }
.toggle-pwd {
  position: absolute; right: 8px; z-index: 1;
  font-size: var(--font-size-xs); padding: 4px 10px;
  border-radius: var(--radius-sm); border: 1px solid var(--border-color);
  background: transparent; color: var(--text-secondary); cursor: pointer; font-family: inherit;
}
.toggle-pwd:hover { background: var(--bg-hover); color: var(--primary-500); }

/* ---- 邮箱域名后缀 ---- */
.email-input-row { display: flex; align-items: center; gap: 0; }
.email-prefix { border-top-right-radius: 0; border-bottom-right-radius: 0; flex: 1 1 0%; min-width: 0; }
.email-domain {
  padding: 10px 14px;
  background: var(--bg-tertiary);
  border: 1px solid var(--border-color);
  border-left: none;
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
  color: var(--text-secondary);
  white-space: nowrap;
  font-size: var(--font-size-sm);
  flex-shrink: 0;
}
.email-domain-select {
  width: 130px;
  border-left: none;
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
  cursor: pointer;
  background-color: var(--bg-tertiary);
  color: var(--text-secondary);
  flex-shrink: 0;
}
.email-domain-select:focus {
  z-index: 1;
  border-color: var(--primary-400);
}
.email-full {
  flex: 1;
}

.form-hint {
  font-size: var(--font-size-xs);
  margin-top: -2px;
}
.help-text { color: var(--warning); line-height: 1.5; }
.text-muted { color: var(--text-tertiary); }

/* ---- 操作按钮（复用全局主题自适应样式 .btn / .btn-primary / .btn-secondary）---- */
.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-sm);
  padding-top: var(--space-sm);
  margin-top: var(--space-xs);
}

/* ---- 高级选项折叠 ---- */
.advanced-section {
  border: 1px dashed var(--border-color);
  border-radius: var(--radius-md);
  overflow: hidden;
  margin-top: var(--space-sm);
}
.advanced-section summary { list-style: none; }
.advanced-section summary::-webkit-details-marker { display: none; }
.advanced-summary {
  padding: 10px 14px;
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  cursor: pointer;
  user-select: none;
}
.advanced-summary:hover { color: var(--text-secondary); background: var(--bg-hover); }
.advanced-fields { padding: 12px 16px; border-top: 1px solid var(--border-light); display: flex; flex-direction: column; gap: var(--space-sm); }

.font-mono { font-family: 'SF Mono', 'Fira Code', monospace; letter-spacing: 0.3px; }

@media (max-width: 480px) {
  .form-row { flex-direction: column; }
  .port-group { width: unset; }
}
</style>
