<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="empty-state">
    <div class="empty-icon" :class="icon">
      <!-- inbox 图标 -->
      <svg v-if="icon === 'inbox'" width="56" height="56" viewBox="0 0 56 56" fill="none">
        <rect x="6" y="14" width="44" height="32" rx="4" stroke="currentColor" stroke-width="2.2"/>
        <path d="M6 22L28 36L50 22" stroke="currentColor" stroke-width="2.2" stroke-linejoin="round"/>
      </svg>

      <!-- error 图标 -->
      <svg v-else-if="icon === 'error'" width="56" height="56" viewBox="0 0 56 56" fill="none">
        <circle cx="28" cy="28" r="20" stroke="currentColor" stroke-width="2.2"/>
        <path d="M28 18v16M28 40v2" stroke="currentColor" stroke-width="2.8" stroke-linecap="round"/>
      </svg>

      <!-- sent (已发送) 图标 -->
      <svg v-else-if="icon === 'sent'" width="56" height="56" viewBox="0 0 56 56" fill="none">
        <path d="M8 20L40 10L30 42L24 28L10 24Z" stroke="currentColor" stroke-width="2.2" stroke-linejoin="round"/>
        <path d="M30 12L22 26" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/>
      </svg>

      <!-- draft (草稿) 图标 -->
      <svg v-else-if="icon === 'draft'" width="56" height="56" viewBox="0 0 56 56" fill="none">
        <path d="M14 10h16l12 12v22a2 2 0 0 1-2 2H14a2 2 0 0 1-2-2V12a2 2 0 0 1 2-2Z" stroke="currentColor" stroke-width="2.2" stroke-linejoin="round"/>
        <path d="M30 10v12h12" stroke="currentColor" stroke-width="2.2" stroke-linejoin="round"/>
        <path d="M20 38l3-1 11-11 4 4-11 11-4 1-3 1Z" stroke="currentColor" stroke-width="2.2" stroke-linejoin="round"/>
        <path d="M36 30l4 4" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/>
      </svg>

      <!-- 默认图标 -->
      <svg v-else width="48" height="48" viewBox="0 0 48 48" fill="none">
        <circle cx="24" cy="24" r="20" stroke="currentColor" stroke-width="2"/>
        <circle cx="24" cy="24" r="2" fill="currentColor"/>
        <path d="M24 12v10" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
      </svg>
    </div>
    
    <h3 class="empty-title">{{ title }}</h3>
    <p v-if="description" class="empty-desc">{{ description }}</p>
  </div>
</template>

<script setup>
defineProps({
  icon: { type: String, default: 'default' },
  title: { type: String, required: true },
  description: { type: String, default: '' }
})
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
}

.empty-icon {
  color: var(--text-tertiary);
  margin-bottom: var(--space-lg);
  opacity: 0.7;
}
.empty-icon.inbox { color: var(--primary-400); opacity: 1; }
.empty-icon.error { color: var(--error); opacity: 1; }
.empty-icon.sent { color: var(--primary-400); opacity: 1; }
.empty-icon.draft { color: var(--primary-400); opacity: 1; }

.empty-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin-bottom: var(--space-xs);
}

.empty-desc {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  max-width: 360px;
}
</style>
