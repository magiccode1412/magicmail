<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
  添加邮箱向导容器 - 管理步骤状态和流转
-->
<template>
  <Teleport to="body">
    <div class="wizard-overlay" @click.self="handleCancel">
      <div class="wizard-modal" @click.stop>
        <!-- 步骤指示器 -->
        <div class="wizard-header">
          <div class="step-indicators">
            <div v-for="s in steps" :key="s.num"
              class="step-dot" :class="{ active: step >= s.num, current: step === s.num }"
              :title="s.label"></div>
          </div>
          <h3 class="wizard-title">{{ currentStepLabel }}</h3>
          <button class="btn-icon btn-ghost close-btn" @click="handleCancel">
            <X :size="18" />
          </button>
        </div>

        <!-- 动态内容区 -->
        <div class="wizard-body">
          <component
            :is="currentStepComponent"
            :selected-provider="selectedProvider"
            :form-data="formData"
            @select-provider="onSelectProvider"
            @next="goNext"
            @back="goBack"
            @complete="handleComplete"
            @oauth2-authorized="onOAuth2Authorized"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, reactive, computed, defineAsyncComponent } from 'vue'
import { X } from 'lucide-vue-next'
import { createAccount } from '@/api/account'
import { useToast } from '@/composables/useToast'
import WizardStepProvider from './WizardStepProvider.vue'
import WizardStepConfig from './WizardStepConfig.vue'
import WizardStepAdvanced from './WizardStepAdvanced.vue'

const emit = defineEmits(['close', 'saved'])
const toast = useToast()

const step = ref(1)
const totalSteps = 3
const selectedProvider = ref(null)

// 表单数据（跨步骤共享）
const formData = reactive({
  name: '',
  email: '',
  protocol: 'imap',
  host: '',
  port: 993,
  smtp_host: '',
  smtp_port: null,
  username: '',
  password: '',
  auth_type: '',
  oauth_provider: '',
  refresh_token: '',
  custom_client_id: '',
  proxy_enabled: false,
  proxy_url: '',
  sync_mode: 'unread',
  sync_days: 30,
  delete_on_server: false
})

const steps = [
  { num: 1, label: '选择服务商' },
  { num: 2, label: '配置信息' },
  { num: 3, label: '高级设置' }
]

const currentStepLabel = computed(() => {
  const s = steps.find(st => st.num === step.value)
  return s ? `添加邮箱 — ${s.label}` : ''
})

const currentStepComponent = computed(() => {
  switch (step.value) {
    case 1: return WizardStepProvider
    case 2: return WizardStepConfig
    case 3: return WizardStepAdvanced
    default: return WizardStepProvider
  }
})

function onSelectProvider(provider) {
  selectedProvider.value = provider
  // 预填充服务商预设配置
  if (provider.preset) {
    Object.assign(formData, provider.preset)
  } else if (provider.key === '__manual__') {
    // 手动配置：清空服务器预填值，让用户自行填写
    formData.host = ''
    formData.port = 993
    formData.smtp_host = ''
    formData.smtp_port = null
  }
  // 选中服务商后直接进入下一步配置，无需再点“下一步”
  goNext()
}

function goNext() {
  if (step.value < totalSteps) {
    step.value++
  }
}

function goBack() {
  if (step.value > 1) {
    step.value--
  }
}

function onOAuth2Authorized(tokenData) {
  // OAuth2 授权成功，保存 token 信息到表单数据
  formData.auth_type = `oauth2_${tokenData.provider}`
  formData.oauth_provider = tokenData.provider
  formData.refresh_token = tokenData.refresh_token
  formData.token_expires_at = tokenData.token_expires_at
  // 授权成功时后端会回传邮箱（从 id_token 解析），自动填充
  if (tokenData.email) {
    formData.email = tokenData.email
  }
}

async function handleComplete() {
  try {
    const submitData = { ...formData }
    if (!submitData.username && submitData.email) {
      submitData.username = submitData.email
    }

    await createAccount(submitData)
    toast.success('邮箱账号添加成功')
    emit('saved')
  } catch (e) {
    toast.error('添加失败：' + e.message)
  }
}

function handleCancel() {
  if (formData.name || formData.email) {
    toast.confirm('确定要放弃当前填写的内容吗？').then(confirmed => {
      if (confirmed) emit('close')
    })
  } else {
    emit('close')
  }
}
</script>

<style scoped>
.wizard-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: var(--z-modal);
  padding: var(--space-md);
  animation: overlay-in 0.25s ease-out;
}
@keyframes overlay-in {
  from { opacity: 0; backdrop-filter: blur(0); }
  to { opacity: 1; backdrop-filter: blur(8px); }
}

.wizard-modal {
  width: 100%;
  max-width: 560px;
  max-height: 90vh;
  background: var(--bg-primary);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-xl), 0 0 80px var(--shadow-glow);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: wizard-in 0.35s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes wizard-in {
  from { transform: translateY(30px) scale(0.96); opacity: 0; }
  to { transform: translateY(0) scale(1); opacity: 1; }
}

.wizard-header {
  padding: var(--space-lg) var(--space-lg) var(--space-md);
  text-align: center;
  position: relative;
}

.step-indicators {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-bottom: var(--space-sm);
}

.step-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--border-color);
  transition: all 0.3s ease;
}
.step-dot.active {
  background: var(--primary-500);
  box-shadow: 0 0 8px var(--shadow-glow);
}
.step-dot.current {
  transform: scale(1.4);
}

.wizard-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
}

.close-btn {
  position: absolute;
  right: var(--space-md);
  top: 50%;
  transform: translateY(-50%);
}

.wizard-body {
  flex: 1;
  overflow-y: auto;
  padding: 0 var(--space-lg) var(--space-md);
}

@media (max-width: 480px) {
  .wizard-modal { max-height: 95vh; border-radius: var(--radius-lg); }
  .wizard-body { padding: 0 var(--space-md) var(--space-sm); }
  .wizard-footer { padding: var(--space-sm) var(--space-md); }
}
</style>
