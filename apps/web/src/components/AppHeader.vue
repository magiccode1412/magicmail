<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <header class="app-header">
    <!-- 左侧：菜单按钮 + 页面标题 -->
    <div class="header-left">
      <button class="btn-icon btn-ghost hamburger" @click="$emit('toggle-sidebar')" title="菜单">
        <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
          <path d="M3 5.5h14M3 10h14M3 14.5h14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
        </svg>
      </button>
      <div class="header-title-area">
        <h1 class="header-title">{{ pageTitle }}</h1>
        <span v-if="unreadCount > 0" class="unread-indicator">
          {{ unreadCount }} 封未读
        </span>
      </div>
    </div>

    <!-- 中间：搜索框 -->
    <div class="header-center">
      <div class="search-box">
        <svg class="search-icon" width="16" height="16" viewBox="0 0 16 16" fill="none">
          <circle cx="7" cy="7" r="4.5" stroke="currentColor" stroke-width="1.6"/>
          <path d="M11 11L14 14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
        </svg>
        <input
          ref="searchInput"
          type="text"
          class="search-input"
          placeholder="搜索邮件（发件人、主题）..."
          :value="searchKeyword"
          @input="handleSearch"
          @keydown.escape="clearSearch"
        />
        <button v-if="searchKeyword" class="search-clear btn-icon btn-ghost" @click="clearSearch" title="清除搜索">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
            <path d="M3 3l8 8M11 3L3 11" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
          </svg>
        </button>
      </div>
    </div>

    <!-- 右侧：操作按钮组 -->
    <div class="header-right">
      <!-- 刷新按钮 -->
      <button class="btn-icon btn-ghost" @click="$emit('refresh')" title="刷新">
        <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
          <path d="M15 9A6 6 0 1 1 9 3c1.75 0 3.33.76 4.42 1.97M15 2v4h-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>

      <!-- 主题切换 -->
      <button class="btn-icon btn-ghost theme-toggle" @click="$emit('toggle-theme')" title="切换主题">
        <!-- 跟随系统 (半明半暗显示器图标) -->
        <svg v-show="themeMode === 'system'" width="18" height="18" viewBox="0 0 28 28" xmlns="http://www.w3.org/2000/svg">
          <rect x="2" y="6" width="24" height="16" rx="3" fill="none" stroke="currentColor" stroke-width="1.8"/>
          <circle cx="9.5" cy="14" r="3.5" fill="currentColor"/>
          <path d="M19 10a4 4 0 0 1 0 8" stroke="currentColor" stroke-width="1.5" fill="none"/>
        </svg>
        <!-- 浅色模式 (太阳) -->
        <svg v-show="themeMode === 'light'" width="18" height="18" viewBox="0 0 28 28" xmlns="http://www.w3.org/2000/svg">
          <circle cx="14" cy="14" r="7" fill="currentColor"/>
          <path d="M14 3v2M14 23v2M26 14h-2M4 14H2M22.2 22.2l-1.4-1.4M7.2 7.2L5.8 5.8M22.2 5.8l-1.4 1.4M7.2 20.8L5.8 22.2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
        </svg>
        <!-- 深色模式 (月亮) -->
        <svg v-show="themeMode === 'dark'" width="18" height="18" viewBox="0 0 28 28" xmlns="http://www.w3.org/2000/svg">
          <path d="M21 15a8 8 0 1 1-12-7A9 9 0 0 0 21 15z" fill="currentColor"/>
        </svg>
      </button>

      <!-- 分隔线 -->
      <div class="header-divider"></div>

      <!-- 退出登录 -->
      <button class="btn-icon btn-ghost logout-btn" @click="handleLogout" title="退出登录">
        <svg width="18" height="18" viewBox="0 0 20 20" fill="none">
          <path d="M7 3h6a2 2 0 012 2v10a2 2 0 01-2 2H7m8-8h3m0 0l-2-2m2 2l-2 2" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
    </div>
  </header>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/appStore'
import { useAuthStore } from '@/stores/authStore'

defineProps({
  unreadCount: { type: Number, default: 0 },
})

const emit = defineEmits(['toggle-sidebar', 'toggle-theme', 'search', 'refresh'])

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const searchInput = ref(null)
const searchKeyword = computed(() => appStore.searchKeyword)
const isDark = computed(() => appStore.isDark)
const themeMode = computed(() => appStore.themeMode)

const pageTitle = computed(() => {
  const titles = {
    '/mails': '收件箱',
    '/accounts': '邮箱管理',
    '/settings': '设置',
  }
  return titles[route.path] || route.meta?.title || 'Magicmail'
})

function handleSearch(e) {
  emit('search', e.target.value)
}

function clearSearch() {
  appStore.setSearchKeyword('')
  if (searchInput.value) {
    searchInput.value.value = ''
  }
  emit('search', '')
}

function handleLogout() {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.app-header {
  position: sticky;
  top: 0;
  z-index: var(--z-header);
  display: flex;
  align-items: center;
  gap: var(--space-md);
  padding: 0 24px;
  height: var(--header-height);
  background: var(--bg-primary);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-light);
  box-shadow: var(--float-shadow);
  transition: background-color var(--transition-base), border-color var(--transition-base);
}

.header-left {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex-shrink: 0;
}

.hamburger { display: none; }

.header-title-area {
  display: flex;
  align-items: baseline;
  gap: var(--space-sm);
}

.header-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  letter-spacing: -0.3px;
}

.unread-indicator {
  font-size: var(--font-size-xs);
  color: var(--primary-500);
  background: var(--mail-unread-bg);
  padding: 2px 8px;
  border-radius: var(--radius-full);
  font-weight: var(--font-weight-medium);
}

/* ---- 搜索框 ---- */
.header-center {
  flex: 1;
  max-width: 480px;
  margin: 0 auto;
}

.search-box {
  position: relative;
  display: flex;
  align-items: center;
  width: 100%;
}

.search-icon {
  position: absolute;
  left: 12px;
  color: var(--text-tertiary);
  pointer-events: none;
}

.search-input {
  width: 100%;
  padding: 8px 34px 8px 36px;
  font-size: var(--font-size-sm);
  font-family: inherit;
  background: var(--bg-secondary);
  border: 1px solid transparent;
  border-radius: var(--radius-full);
  color: var(--text-primary);
  outline: none;
  transition: all var(--transition-fast);
}
.search-input:focus {
  background: var(--bg-elevated);
  border-color: var(--primary-300);
  box-shadow: 0 0 0 3px var(--mail-unread-bg);
}
.search-input::placeholder { color: var(--text-tertiary); }

.search-clear {
  position: absolute;
  right: 6px;
  width: 24px !important;
  height: 24px !important;
}

/* ---- 右侧操作区 ---- */
.header-right {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.theme-toggle { border-radius: var(--radius-full); }
.theme-toggle svg { transition: transform var(--transition-base); }
.theme-toggle:hover svg { transform: rotate(20deg); }

.header-divider {
  width: 1px;
  height: 18px;
  background: var(--border-light);
}

.logout-btn {
  color: var(--text-tertiary);
  transition: color var(--transition-fast);
}
.logout-btn:hover {
  color: #ef4444;
}

/* ---- 响应式适配 ---- */
@media (max-width: 1024px) {
  .app-header {
    padding: 0 16px;
    border-radius: 0;
    box-shadow: none;
    /* 确保所有元素垂直居中，避免偏移 */
    align-items: center;
  }
  .hamburger { display: inline-flex; }
  .header-center { max-width: 320px; }
}

@media (max-width: 640px) {
  .app-header {
    gap: var(--space-sm);
    padding: 0 12px;
  }

  .header-left {
    /* 防止左侧区域高度异常导致按钮偏移 */
    height: var(--header-height);
  }

  .hamburger {
    /* 移动端确保图标完全居中 */
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .header-center {
    order: 3;
    max-width: unset;
    flex: 1;
    min-width: 0;
  }

  .header-title-area { display: none; }
  .header-right { gap: 0; }

  .search-input { font-size: var(--font-size-xs); padding: 7px 30px 7px 32px; }
}
</style>
