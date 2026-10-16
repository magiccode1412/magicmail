<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
  向导 Step1: 服务商选择卡片网格
-->
<template>
  <div class="provider-select-step">
    <p class="step-desc">选择你的邮箱服务商，我们会自动填充服务器配置</p>

    <div class="provider-grid">
      <button
        v-for="p in providers"
        :key="p.key"
        type="button"
        class="provider-card"
        :class="{ active: selectedKey === p.key, 'oauth-required': p.oauthRequired }"
        :style="{ '--brand-color': p.brandColor }"
        @click="$emit('selectProvider', p)"
      >
        <div class="card-icon" :style="p.svgIcon ? {} : { backgroundColor: p.brandColor + '18', color: p.brandColor }">
          <!-- SVG 图标 -->
          <span v-if="p.svgIcon" class="icon-svg" v-html="p.svgIcon"></span>
          <!-- 文字图标 fallback -->
          <template v-else>{{ p.icon }}</template>
        </div>
        <span class="card-name">{{ p.name }}</span>
        <span v-if="p.oauthRequired" class="oauth-badge">OAuth2</span>
      </button>

      <!-- 其他（手动配置） -->
      <button
        type="button"
        class="provider-card manual-config"
        :class="{ active: selectedKey === '__manual__' }"
        @click="$emit('selectProvider', { key: '__manual__', name: '其他', oauthRequired: false })"
      >
        <div class="card-icon icon-manual">
          <Settings :size="20" />
        </div>
        <span class="card-name">其他</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Settings } from 'lucide-vue-next'
import { providers as allProviders } from '@/data/providers.js'

const props = defineProps({
  selectedProvider: { type: Object, default: null }
})

defineEmits(['selectProvider', 'next', 'back', 'complete'])

const providers = allProviders

const selectedKey = computed(() => props.selectedProvider?.key || '')
</script>

<style scoped>
.provider-select-step {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

.step-desc {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  text-align: center;
  margin: 0;
}

.provider-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-md);
}

.provider-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: var(--space-md) var(--space-sm);
  border: 2px solid var(--border-color);
  border-radius: var(--radius-lg);
  background: var(--bg-primary);
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  user-select: none;
}

.provider-card:hover {
  border-color: var(--primary-300);
  background: var(--bg-hover);
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.provider-card.active {
  border-color: var(--primary-500);
  background: var(--selected-bg, var(--primary-50));
  box-shadow: 0 0 0 3px var(--mail-selected-bg, var(--primary-100)), var(--shadow-md);
  transform: translateY(-2px);
}

.card-icon {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: 700;
  transition: transform 0.25s ease;
}
.icon-svg {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}
.icon-svg :deep(svg) {
  width: 36px;
  height: 36px;
}

.provider-card:hover .card-icon {
  transform: scale(1.08);
}

.card-name {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
  text-align: center;
  line-height: 1.3;
}

.card-hint {
  font-size: 11px;
  color: var(--text-tertiary);
}

.oauth-badge {
  position: absolute;
  top: 6px;
  right: 6px;
  font-size: 9px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: var(--radius-full);
  background: linear-gradient(135deg, var(--primary-500), var(--primary-400));
  color: var(--text-on-primary);
  letter-spacing: 0.5px;
}

/* 手动配置特殊卡片 */
.manual-config {
  border-style: dashed;
}
.icon-manual {
  background: var(--bg-tertiary) !important;
  color: var(--text-tertiary) !important;
}

@media (max-width: 560px) {
  .provider-grid {
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-sm);
  }
}

@media (max-width: 400px) {
  .provider-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
