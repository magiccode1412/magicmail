<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <aside class="app-sidebar" :class="{ collapsed, open: mobileOpen }">
    <!-- Logo 区域 -->
    <div class="sidebar-logo">
      <button class="logo-btn" @click="$emit('navigate', '/mails')">
        <svg width="28" height="28" viewBox="0 0 32 32" fill="none">
          <rect width="32" height="32" rx="8" fill="url(#logo-grad)" />
          <path d="M8 12L16 17L24 12" stroke="white" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
          <rect x="7" y="10" width="18" height="13" rx="2" stroke="white" stroke-width="1.8" fill="none"/>
          <defs>
            <linearGradient id="logo-grad" x1="0" y1="0" x2="32" y2="32">
              <stop stop-color="var(--primary-500, #4F6EF7)"/>
              <stop offset="1" stop-color="var(--primary-400, #6C8AFF)"/>
            </linearGradient>
          </defs>
        </svg>
        <span v-show="!collapsed" class="logo-text">Magicmail</span>
      </button>
      <button v-if="isMobile && !collapsed" class="btn-icon btn-ghost close-btn" @click="mobileOpen = false">
        <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
          <path d="M4 4L14 14M14 4L4 14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
        </svg>
      </button>
    </div>

    <!-- 导航菜单 -->
    <nav class="sidebar-nav">
      <!-- 邮件列表 (主页) -->
      <router-link
        to="/mails"
        class="nav-item"
        active-class="active"
        @click="handleNavClick('/mails')"
      >
        <span class="nav-icon">
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <rect x="2.5" y="5" width="15" height="11" rx="2" stroke="currentColor" stroke-width="1.6"/>
            <path d="M2.5 6.5L10 11L17.5 6.5" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>
          </svg>
        </span>
        <transition name="fade-text">
          <span v-show="!collapsed" class="nav-label">收件箱</span>
        </transition>
        <span v-show="!collapsed" class="nav-badge" v-if="unreadCount > 0">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
      </router-link>

      <!-- 已发送 -->
      <router-link
        to="/sent"
        class="nav-item"
        active-class="active"
        @click="handleNavClick('/sent')"
      >
        <span class="nav-icon">
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path d="M3 10L17 4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M3 10L17 16" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M15 5.5V3M15 14.5V17" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
          </svg>
        </span>
        <transition name="fade-text">
          <span v-show="!collapsed" class="nav-label">已发送</span>
        </transition>
      </router-link>

      <!-- 草稿 -->
      <router-link
        to="/drafts"
        class="nav-item"
        active-class="active"
        @click="handleNavClick('/drafts')"
      >
        <span class="nav-icon">
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <rect x="3.5" y="4.5" width="13" height="13" rx="2.5" stroke="currentColor" stroke-width="1.6"/>
            <path d="M7 4V3a1 1 0 011-1h4a1 1 0 011 1v1M7 10l1.5 1.5L13 7" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </span>
        <transition name="fade-text">
          <span v-show="!collapsed" class="nav-label">草稿</span>
        </transition>
      </router-link>

      <!-- 写邮件 (次要操作) -->
      <router-link
        to="/compose"
        class="nav-item compose-item"
        active-class="active"
        @click="handleNavClick('/compose')"
      >
        <span class="nav-icon">
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path d="M3 10L17 4L11 18L9 12L3 10Z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>
            <path d="M9 12L17 4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
          </svg>
        </span>
        <transition name="fade-text">
          <span v-show="!collapsed" class="nav-label">写邮件</span>
        </transition>
      </router-link>

      <!-- 邮箱管理 -->
      <router-link
        to="/accounts"
        class="nav-item"
        active-class="active"
        @click="handleNavClick('/accounts')"
      >
        <span class="nav-icon">
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <rect x="2.5" y="3.5" width="15" height="13" rx="2" stroke="currentColor" stroke-width="1.6"/>
            <circle cx="7" cy="9" r="2" stroke="currentColor" stroke-width="1.4"/>
            <path d="M11.5 8H16M11.5 11H14" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
        </span>
        <transition name="fade-text">
          <span v-show="!collapsed" class="nav-label">邮箱管理</span>
        </transition>
      </router-link>

      <!-- 分割线 -->
      <div class="nav-divider"></div>

      <!-- 设置 -->
      <router-link
        to="/settings"
        class="nav-item"
        active-class="active"
        @click="handleNavClick('/settings')"
      >
        <span class="nav-icon">
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <circle cx="10" cy="10" r="6" stroke="currentColor" stroke-width="1.6"/>
            <path d="M10 2V3.5M10 16.5V18M18 10H16.5M3.5 10H2M15.66 15.66l-1.06-1.06M5.4 5.4L4.34 4.34M15.66 4.34l-1.06 1.06M5.4 14.6l-1.06 1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
        </span>
        <transition name="fade-text">
          <span v-show="!collapsed" class="nav-label">设置</span>
        </transition>
      </router-link>
    </nav>

    <!-- 底部信息 -->
    <div class="sidebar-footer">
      <div v-show="!collapsed" class="footer-status">
        <span class="status-dot online"></span>
        <span class="status-text">服务在线</span>
      </div>
      <button class="btn-icon btn-ghost toggle-btn" @click="$emit('toggle')" :title="collapsed ? '展开侧边栏' : '收起侧边栏'">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
          <path v-if="collapsed" d="M6 3L11 8L6 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
          <path v-else d="M10 3L5 8L10 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
    </div>

    <!-- 移动端遮罩层 -->
    <div
      v-if="isMobile && (open || mobileOpen)"
      class="sidebar-overlay"
      @click="mobileOpen = false"
    ></div>
  </aside>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAccountStore } from '@/stores/accountStore'
import { useMailStore } from '@/stores/mailStore'
import { useSSE } from '@/composables/useSSE'

const props = defineProps({
  collapsed: Boolean,
})

const emit = defineEmits(['toggle', 'navigate'])

const route = useRoute()
const accountStore = useAccountStore()
const mailStore = useMailStore()

// 统计实时更新：邮件数据变化时刷新侧边栏未读角标（轻量，仅需 fetchStats）
useSSE({
  onStatsUpdated() {
    mailStore.fetchStats()
  },
})

const mobileOpen = ref(false)
const isMobile = ref(window.innerWidth <= 1024)

// 统一使用 mailStore.stats 作为未读数据源（与邮件列表页同步）
const unreadCount = computed(() => mailStore.stats?.unread || 0)
const open = computed(() => {
  return isMobile.value && mobileOpen.value
})

// 响应式检测
function handleResize() {
  isMobile.value = window.innerWidth <= 1024
  if (!isMobile.value) {
    mobileOpen.value = false
  }
}

function handleNavClick(path) {
  emit('navigate', path)
  if (isMobile.value) {
    mobileOpen.value = false
  }
}

onMounted(() => {
  // 仅当无缓存数据时加载账号
  if (!accountStore.accounts.length) {
    accountStore.fetchAccounts()
  }
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.app-sidebar {
  position: fixed;
  top: var(--space-md);
  left: var(--space-md);
  bottom: var(--space-md);
  width: var(--sidebar-width);
  display: flex;
  flex-direction: column;
  background: var(--bg-primary);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-light);
  box-shadow: var(--float-shadow);
  transition: width var(--transition-slow), transform var(--transition-slow), box-shadow var(--transition-slow);
  z-index: var(--z-sidebar);
  overflow: hidden;
}

.app-sidebar.collapsed { width: var(--sidebar-collapsed-width); }

/* ---- Logo 区域 ---- */
.sidebar-logo {
  display: flex;
  align-items: center;
  padding: 16px 16px 12px;
  gap: 8px;
  min-height: var(--header-height);
}

.logo-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-primary);
  font-family: inherit;
  flex: 1;
  border-radius: var(--radius-md);
  padding: 6px;
}
.logo-btn:hover { background: var(--bg-hover); }

.logo-text {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-bold);
  background: linear-gradient(135deg, var(--primary-500), var(--primary-400));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  letter-spacing: -0.3px;
}

.close-btn { flex-shrink: 0; }

/* ---- 底部状态 / 折叠按钮 ---- */
.sidebar-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-top: 1px solid var(--border-light);
  border-radius: 0 0 var(--radius-lg) var(--radius-lg);
}

.toggle-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  flex-shrink: 0;
  margin-left: auto;
  border-radius: var(--radius-md);
}

.collapsed .sidebar-footer {
  padding: 12px 8px;
  justify-content: center;
}

.collapsed .toggle-btn {
  margin-left: 0;
}

/* ---- 导航 ---- */
.sidebar-nav {
  flex: 1;
  padding: 8px 10px;
  overflow-y: auto;
  overflow-x: hidden;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  text-decoration: none;
  cursor: pointer;
  transition: all var(--transition-fast);
  position: relative;
  white-space: nowrap;
  margin-bottom: 2px;
}
.nav-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.nav-item.active {
  background: var(--mail-unread-bg);
  color: var(--primary-500);
  font-weight: var(--font-weight-medium);
}
.nav-item.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 60%;
  background: linear-gradient(180deg, var(--primary-500), var(--primary-400));
  border-radius: 0 3px 3px 0;
}

.collapsed .nav-item { justify-content: center; padding: 10px; }

.nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 22px;
  height: 22px;
}

.nav-label {
  flex: 1;
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-normal);
}

.nav-badge {
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  font-size: 11px;
  font-weight: var(--font-weight-semibold);
  background: var(--error);
  color: #fff;
  border-radius: var(--radius-full);
  line-height: 20px;
  text-align: center;
}

.nav-divider {
  height: 1px;
  background: var(--border-light);
  margin: 8px 8px 12px;
}

/* ---- 底部状态 ---- */
.footer-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  animation: pulse-dot 2s ease-in-out infinite;
}
.status-dot.online { background: var(--success); }
@keyframes pulse-dot {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.35); }
  50% { opacity: 0.8; box-shadow: 0 0 0 4px rgba(16, 185, 129, 0); }
}

/* ---- 文本展开/折叠过渡 ---- */
.fade-text-enter-active, .fade-text-leave-active {
  transition: opacity 0.15s ease;
}
.fade-text-enter-from, .fade-text-leave-to { opacity: 0; }

/* ---- 移动端遮罩 ---- */
.sidebar-overlay {
  display: none;
}

@media (max-width: 1024px) {
  .app-sidebar {
    top: 0;
    left: 0;
    bottom: 0;
    border-radius: 0;
    box-shadow: none;
  }

  .sidebar-overlay {
    display: block;
    position: fixed;
    inset: 0;
    background: var(--bg-overlay);
    z-index: calc(var(--z-sidebar) - 1);
  }

  .app-sidebar.open + .sidebar-overlay,
  .app-sidebar.open ~ .app-overlay {
    display: block;
  }
}
</style>
