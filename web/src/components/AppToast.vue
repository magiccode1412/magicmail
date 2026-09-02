<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <Teleport to="body">
    <!-- Toast 通知 -->
    <div class="toast-container">
      <transition-group name="toast" tag="div">
        <div v-for="item in items" :key="item.id"
             class="toast-item" :class="'toast-' + item.type">
          <span class="toast-icon">{{ iconMap[item.type] }}</span>
          <span class="toast-message">{{ item.message }}</span>
          <button class="toast-close" @click="$emit('remove', item.id)">&times;</button>
        </div>
      </transition-group>
    </div>

    <!-- 确认弹窗 -->
    <transition name="fade">
      <div v-if="dialogs.length > 0" class="confirm-overlay" @click.self="handleCancel">
        <div class="confirm-dialog card">
          <h4 class="confirm-title">{{ activeDialog.title }}</h4>
          <p class="confirm-message">{{ activeDialog.message }}</p>
          <div class="confirm-actions">
            <button class="btn btn-secondary" @click="handleCancel">取消</button>
            <button class="btn btn-primary" @click="handleConfirm">确定</button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup>
import { ref, computed } from 'vue'
import { initToast, initConfirm } from '@/composables/useToast'

defineEmits(['remove'])

// --- Toast ---
const items = ref([])
initToast({ items })

const iconMap = {
  success: '\u2713',
  error: '\u2717',
  warning: '\u26A0',
  info: '\u2139'
}

// --- Confirm ---
const dialogs = ref([])
initConfirm({ dialogs })

const activeDialog = computed(() => dialogs.value[dialogs.value.length - 1] || {})

function handleConfirm() {
  const d = dialogs.value.pop()
  if (d?.resolve) d.resolve(true)
}

function handleCancel() {
  const d = dialogs.value.pop()
  if (d?.resolve) d.resolve(false)
}
</script>

<style scoped>
/* ---- Toast 样式 ---- */
.toast-container {
  position: fixed;
  /* 有更新横幅时下移，避免遮住横幅右侧（--update-banner-offset 继承自 .app） */
  top: calc(20px + var(--update-banner-offset, 0px));
  right: 24px;
  z-index: calc(var(--z-modal) + 100);
  display: flex;
  flex-direction: column;
  gap: 10px;
  pointer-events: none;
  max-width: 400px;
  width: 100%;
}

.toast-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border-radius: var(--radius-md);
  background: var(--bg-primary, #fff);
  box-shadow: var(--shadow-lg), 0 4px 16px rgba(0,0,0,0.1);
  border-left: 4px solid var(--color, #999);
  pointer-events: auto;
  font-size: var(--font-size-base);
  line-height: 1.5;
  animation: toast-in 0.35s cubic-bezier(0.16, 1, 0.3, 1);
}

.toast-success { --color: var(--success, #10b981); }
.toast-error   { --color: var(--error, #ef4444); }
.toast-warning { --color: #f59e0b; }
.toast-info    { --color: #6366f1; }

.toast-icon {
  font-size: 15px;
  font-weight: 700;
  flex-shrink: 0;
  width: 22px;
  text-align: center;
}

.toast-success .toast-icon { color: var(--success, #10b981); }
.toast-error .toast-icon   { color: var(--error, #ef4444); }
.toast-warning .toast-icon { color: #f59e0b; }
.toast-info .toast-icon    { color: #6366f1; }

.toast-message {
  flex: 1;
  word-break: break-word;
  color: var(--text-primary, #1a1a2e);
}

.toast-close {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 18px;
  color: var(--text-tertiary, #999);
  padding: 0 2px;
  line-height: 1;
  flex-shrink: 0;
}
.toast-close:hover { color: var(--text-primary, #333); }

/* Toast 动画 */
@keyframes toast-in {
  from { opacity: 0; transform: translateX(40px) scale(0.95); }
  to   { opacity: 1; transform: translateX(0) scale(1); }
}
.toast-enter-active { animation: toast-in 0.35s cubic-bezier(0.16, 1, 0.3, 1); }
.toast-leave-active { animation: toast-out 0.25s cubic-bezier(0.4, 0, 1, 1) forwards; }
.toast-move { transition: transform 0.25s ease; }
@keyframes toast-out {
  to { opacity: 0; transform: translateX(30px); }
}

/* ---- Confirm 弹窗样式 ---- */
.confirm-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: calc(var(--z-modal) + 99);
}

.confirm-dialog {
  width: 400px;
  max-width: 90vw;
  padding: 28px 24px 20px;
  animation: modal-in 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes modal-in {
  from { opacity: 0; transform: scale(0.92) translateY(12px); }
  to   { opacity: 1; transform: scale(1) translateY(0); }
}

.confirm-title {
  margin: 0 0 12px;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.confirm-message {
  margin: 0 0 24px;
  font-size: var(--font-size-base);
  line-height: 1.6;
  color: var(--text-secondary, #555);
  white-space: pre-line;
}

.confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

/* 过渡动画 */
.fade-enter-active,
.fade-leave-active { transition: opacity 0.2s ease; }
.fade-enter-from,
.fade-leave-to { opacity: 0; }

/* 响应式 */
@media (max-width: 480px) {
  .toast-container {
    left: 12px;
    right: 12px;
    max-width: none;
  }
  .toast-item { padding: 10px 14px; font-size: var(--font-size-sm, 13px); }
  .confirm-dialog { width: 92vw; padding: 22px 18px 16px; }
}
</style>
