<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <article
    class="mail-item"
    :class="{ unread: !mail.is_read, selected: selected }"
    @click="handleClick"
    @contextmenu.prevent="showContextMenu($event)"
  >
    <!-- 选择框 -->
    <label v-if="selectable" class="item-checkbox" @click.stop>
      <input
        type="checkbox"
        :checked="selected"
        @change="$emit('toggle-select', mail.id)"
      />
      <span class="checkmark"></span>
    </label>

    <!-- 发件人头像 -->
    <div class="item-avatar" :class="{ unread: !mail.is_read }">
      {{ avatarLetter }}
    </div>

    <!-- 主要信息区 -->
    <div class="item-body">
      <div class="item-header">
        <span class="item-from" :class="{ 'font-semibold': !mail.is_read }">
          {{ extractName(mail.from) || '未知发件人' }}
        </span>
        <div class="item-right">
          <span class="item-time">{{ relativeTime }}</span>
          <!-- 附件图标 -->
          <svg
            v-if="mail.has_attachment"
            width="14"
            height="14"
            viewBox="0 0 16 16"
            fill="none"
            class="attachment-icon"
            title="有附件"
          >
            <path d="M10.5 4l3.5 4v5a1.5 1.5 0 0 1-1.5 1.5h-8A1.5 1.5 0 0 1 2.5 13V5a1.5 1.5 0 0 1 1.5-1.5h6.5z" stroke="currentColor" stroke-width="1.3"/>
            <path d="M10.5 4v3.5H14" stroke="currentColor" stroke-width="1.3"/>
            <circle cx="7" cy="9" r="1.5" stroke="currentColor" stroke-width="1.2"/>
          </svg>
        </div>
      </div>

      <h3 class="item-subject" :class="{ 'font-semibold': !mail.is_read }">
        {{ mail.subject || '(无主题)' }}
      </h3>

      <span class="item-to text-muted">
        <svg width="12" height="12" viewBox="0 0 14 14" fill="none" style="vertical-align: -1px; margin-right: 4px;">
          <path d="M12 5L7 9 2 5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          <rect x="2" y="3" width="10" height="8" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
        </svg>
        {{ extractName(mail.to) || mail.to || '收件人未知' }}
      </span>
    </div>

    <!-- 右键菜单 -->
    <Teleport to="body">
      <transition name="context-menu">
        <div
          v-if="contextVisible"
          class="context-menu card"
          :style="contextStyle"
          @click.stop
        >
          <button
            class="context-item"
            @click="handleAction('mark-read')"
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path v-if="mail.is_read" d="M11.5 4L5.75 9.75L2.5 6.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
              <circle v-else cx="7" cy="7" r="4.5" stroke="currentColor" stroke-width="1.4"/>
            </svg>
            {{ mail.is_read ? '标为未读' : '标为已读' }}
          </button>
          <button
            class="context-item danger-text"
            @click="handleAction('delete')"
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path d="M2 4h10m-8.67 0V2.67A1.33 1.33 0 0 1 4.67 1.34h4.66a1.33 1.33 0 0 1 1.34 1.33V4m2 0v7.33A1.33 1.33 0 0 1 11.34 13H2.67a1.33 1.33 0 0 1-1.34-1.33V4" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            删除邮件
          </button>
        </div>
      </transition>
    </Teleport>
  </article>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  mail: { type: Object, required: true },
  selected: { type: Boolean, default: false },
  selectable: { type: Boolean, default: false },
})

const emit = defineEmits(['click', 'mark-read', 'delete', 'star', 'toggle-select'])

// ---- 右键菜单 ----
const contextVisible = ref(false)
const contextStyle = reactive({ top: '0px', left: '0px' })

function showContextMenu(event) {
  const x = event.clientX
  const y = event.clientY

  // 防止超出视窗右侧/底部
  const menuWidth = 160
  const menuHeight = 88
  const finalX = x + menuWidth > window.innerWidth ? x - menuWidth : x
  const finalY = y + menuHeight > window.innerHeight ? y - menuHeight : y

  contextStyle.top = `${finalY}px`
  contextStyle.left = `${finalX}px`
  contextVisible.value = true
}

function closeContextMenu() {
  contextVisible.value = false
}

function handleAction(action) {
  closeContextMenu()
  if (action === 'mark-read') emit('mark-read', !props.mail.is_read)
  if (action === 'delete') emit('delete')
}

function handleClick() {
  if (props.selectable) return
  emit('click')
}

function onDocumentClick(e) {
  if (contextVisible.value && !e.target.closest('.context-menu')) {
    closeContextMenu()
  }
}

onMounted(() => document.addEventListener('click', onDocumentClick))
onUnmounted(() => document.removeEventListener('click', onDocumentClick))

const avatarLetter = computed(() => {
  const name = props.mail.from?.match(/^(.+?)\s*<?/)?.[1]
  return (name || (props.mail.from && props.mail.from[0]) || '?').charAt(0).toUpperCase()
})

function extractName(from) {
  if (!from) return ''
  // 尝试解析 JSON 数组格式: ["a@b.com", "c@d.com"]
  const trimmed = from.trim()
  if (trimmed.startsWith('[')) {
    try {
      const arr = JSON.parse(trimmed)
      if (Array.isArray(arr)) return arr.join(', ')
    } catch (_) { /* 不是合法 JSON，走下方逻辑 */ }
  }
  const match = trimmed.match(/^(.+?)\s*<.*>$/)
  return match ? match[1].trim() : trimmed
}

const relativeTime = computed(() => {
  const date = new Date(props.mail.sent_at)
  const now = new Date()
  const diffMs = now - date
  const diffMin = Math.floor(diffMs / 60000)

  if (diffMin < 1) return '刚刚'
  if (diffMin < 60) return `${diffMin}分钟前`

  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}小时前`

  const diffDay = Math.floor(diffHr / 24)
  if (diffDay < 7) return `${diffDay}天前`

  return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
})
</script>

<style scoped>
.mail-item {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-md) var(--space-lg);
  cursor: pointer;
  border-radius: var(--radius-md);
  transition: all var(--transition-fast);
  border-left: 3px solid transparent;
  background: var(--bg-secondary, #ffffff);
  height: 72px;
}
.mail-item.selectable { cursor: default; }
.mail-item:hover {
  background: var(--bg-hover, rgba(0, 0, 0, 0.03));
  border-left-color: var(--primary-300);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}
.mail-item.unread {
  background: var(--mail-unread-bg, rgba(79, 110, 247, 0.05));
}
.mail-item.selected {
  background: var(--mail-selected-bg, rgba(79, 110, 247, 0.1));
}

/* ---- 选择框 ---- */
.item-checkbox {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  cursor: pointer;
}
.item-checkbox input[type="checkbox"] {
  appearance: none;
  -webkit-appearance: none;
  width: 18px;
  height: 18px;
  border: 2px solid var(--border-color);
  border-radius: 4px;
  cursor: pointer;
  transition: all var(--transition-fast);
  position: relative;
  flex-shrink: 0;
}
.item-checkbox input[type="checkbox"]:checked {
  background: var(--primary-500);
  border-color: var(--primary-500);
}
.item-checkbox input[type="checkbox"]:checked::after {
  content: '';
  position: absolute;
  left: 5px;
  top: 2px;
  width: 4px;
  height: 8px;
  border: solid #fff;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}
.item-checkbox input[type="checkbox"]:hover {
  border-color: var(--primary-400);
}

/* ---- 头像 ---- */
.item-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--bg-tertiary);
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  flex-shrink: 0;
  transition: all var(--transition-fast);
}
.item-avatar.unread {
  background: linear-gradient(135deg, var(--primary-500), var(--primary-400));
  color: #fff;
}

/* ---- 正文 ---- */
.item-body {
  flex: 1;
  min-width: 0;
}

.item-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  margin-bottom: 2px;
}
.item-from {
  font-size: var(--font-size-base);
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.item-time {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  white-space: nowrap;
  flex-shrink: 0;
}

.item-subject {
  font-size: var(--font-size-base);
  color: var(--text-primary);
  margin-bottom: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-to {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ---- 右侧信息区 ---- */
.item-right {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.attachment-icon {
  color: var(--text-tertiary);
  flex-shrink: 0;
}

/* ---- 右键菜单 ---- */
.context-menu {
  position: fixed;
  z-index: 9999;
  min-width: 160px;
  padding: 4px;
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-lg), 0 8px 24px rgba(0,0,0,0.12);
}
.context-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: none;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  font-family: inherit;
  color: var(--text-primary);
  cursor: pointer;
  text-align: left;
  white-space: nowrap;
  transition: background var(--transition-fast);
}
.context-item:hover {
  background: var(--bg-hover);
}
.context-item.danger-text { color: var(--error); }
.context-item.danger-text:hover { background: var(--error-light); }

/* 右键菜单动画 */
.context-menu-enter-active {
  animation: context-in 0.15s ease-out;
}
.context-menu-leave-active {
  animation: context-out 0.1s ease-in;
}
@keyframes context-in {
  from { opacity: 0; transform: scale(0.95); }
  to { opacity: 1; transform: scale(1); }
}
@keyframes context-out {
  from { opacity: 1; transform: scale(1); }
  to { opacity: 0; transform: scale(0.95); }
}

@media (max-width: 480px) {
  .mail-item { padding: var(--space-sm) var(--space-md); height: 64px; }
  .item-avatar { width: 34px; height: 34px; font-size: var(--font-size-sm); }
}
</style>
