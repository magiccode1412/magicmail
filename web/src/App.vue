<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <!-- 未初始化时显示加载占位 -->
  <div v-if="!authInitialized" class="app-loading">
    <div class="loading-spinner"></div>
  </div>

  <!-- 已登录：主应用界面 -->
  <div v-else-if="isLoggedIn" class="app" :class="{ 'dark': isDark }">
    <!-- 更新提示横幅 -->
    <div v-if="hasUpdate" class="update-banner" role="banner">
      <div class="update-banner-inner">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" class="update-icon">
          <path d="M8 2v6l3 3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          <circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="1.5"/>
        </svg>
        <span>
          发现新版本 <strong>v{{ latestVersion }}</strong>（当前 v{{ __APP_VERSION__ }}）
          <a v-if="downloadUrl" :href="downloadUrl" target="_blank" rel="noopener" class="update-link">查看更新</a>
        </span>
      </div>
      <button class="update-dismiss" @click="dismissUpdate" title="忽略此版本">&times;</button>
    </div>

    <!-- 侧边导航栏 (桌面端) -->
    <AppSidebar
      :collapsed="sidebarCollapsed"
      @toggle="sidebarCollapsed = !sidebarCollapsed"
      @navigate="handleNavigate"
    />

    <!-- 全局 Toast 通知 -->
    <AppToast @remove="(id) => toast.remove(id)" />

    <!-- 主内容区域 -->
    <main class="app-main" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
      <!-- 顶栏 -->
      <AppHeader
        :unread-count="totalUnread"
        @toggle-sidebar="sidebarCollapsed = !sidebarCollapsed"
        @toggle-theme="toggleTheme"
        @search="handleSearch"
        @refresh="handleRefresh"
      />

      <!-- 路由视图 -->
      <div class="app-content">
        <router-view v-slot="{ Component, route }">
          <transition name="fade">
            <keep-alive :include="['MailList', 'Sent', 'Drafts', 'AccountManage', 'Settings']">
              <component v-if="route.name !== 'Login'" :is="Component" :key="route.name === 'MailReader' ? route.fullPath : route.name" />
            </keep-alive>
          </transition>
        </router-view>
      </div>
    </main>
  </div>

  <!-- 未登录：由路由渲染 LoginView -->
  <router-view v-else />
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import AppSidebar from './components/AppSidebar.vue'
import AppHeader from './components/AppHeader.vue'
import AppToast from './components/AppToast.vue'
import { useAppStore } from './stores/appStore'
import { useMailStore } from './stores/mailStore'
import { useAccountStore } from './stores/accountStore'
import { useAuthStore } from './stores/authStore'
import { useToast } from './composables/useToast'
import { useUpdateCheck } from './composables/useUpdateCheck'

const toast = useToast()
const { hasUpdate, latestVersion, checkUpdate, dismiss: dismissUpdate } = useUpdateCheck()

const router = useRouter()
const route = useRoute()
const appStore = useAppStore()
const mailStore = useMailStore()
const accountStore = useAccountStore()
const authStore = useAuthStore()

// 认证状态
const isLoggedIn = computed(() => authStore.isLoggedIn)
const authInitialized = ref(false)

// 侧边栏折叠状态（从 localStorage 恢复，默认根据屏幕宽度判断）
const SIDEBAR_KEY = 'magicmail-sidebar-collapsed'
function getInitialCollapsed() {
  const stored = localStorage.getItem(SIDEBAR_KEY)
  if (stored !== null) return stored === 'true'
  // 首次访问：移动端默认收起，PC端展开
  return window.innerWidth <= 768
}
const sidebarCollapsed = ref(getInitialCollapsed())

// 状态变化时持久化
watch(sidebarCollapsed, (val) => {
  localStorage.setItem(SIDEBAR_KEY, String(val))
})

const isDark = computed(() => appStore.isDark)
const totalUnread = computed(() => appStore.unreadCount)

// 切换主题
function toggleTheme() {
  appStore.toggleTheme()
}

// 导航处理
function handleNavigate(path) {
  router.push(path)
}

// 搜索处理
function handleSearch(keyword) {
  if (route.path !== '/mails') {
    router.push({ path: '/mails', query: { keyword } })
  }
  // 通过 store 更新搜索关键词（MailListView 会响应）
  appStore.setSearchKeyword(keyword)
}

// 刷新处理（根据当前页面执行对应刷新逻辑）
function handleRefresh() {
  if (route.path === '/mails') {
    mailStore.fetchMails(mailStore.currentPage)
    mailStore.fetchStats()
  } else if (route.path === '/sent') {
    mailStore.fetchMails(mailStore.currentPage)
    mailStore.fetchStats()
  } else if (route.path === '/accounts') {
    accountStore.fetchAccounts()
  }
  // 草稿页面的刷新由 DraftsView 自身管理
}

// 监听路由变化恢复侧边栏状态
watch(() => route.path, () => {
  // 移动端自动收起侧边栏
  if (window.innerWidth <= 768) {
    sidebarCollapsed.value = true
  }
})

onMounted(async () => {
  // 初始化认证状态
  await authStore.init()
  authInitialized.value = true

  // 初始化主题
  appStore.initTheme()

  // 检查版本更新（静默检查，不阻塞 UI）
  checkUpdate().catch(() => {})
})
</script>

<style scoped>
.app {
  display: flex;
  height: 100vh;
  overflow: hidden;
  background: var(--canvas-bg);
  color: var(--text-primary);
  transition: background-color 0.3s ease, color 0.3s ease;
  /* 悬浮卡片布局：四周留出间距 */
  padding: var(--space-md);
  box-sizing: border-box;
}

.app-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  margin-left: calc(var(--sidebar-width) + var(--space-md));
  transition: margin-left 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  /* header 与内容区间距 */
  gap: var(--space-md);
}

.app-main.sidebar-collapsed {
  margin-left: calc(var(--sidebar-collapsed-width) + var(--space-md));
}

.app-content {
  flex: 1;
  padding: 28px;
  overflow-y: auto;
  /*max-width: 1400px;*/
  width: 100%;
  margin: 0 auto;
  /* 内容区悬浮卡片效果 */
  background: var(--bg-primary);
  border-radius: var(--radius-lg);
  box-shadow: var(--float-shadow);
  border: 1px solid var(--border-light);
}

/* 响应式适配：小屏幕取消悬浮间距 */
@media (max-width: 768px) {
  .app { padding: 0; gap: 0; }
  .app-content {
    border-radius: 0;
    box-shadow: none;
    padding: 16px;
  }
}

@media (max-width: 1024px) {
  .app-main { gap: 0; }
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.fade-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

/* 认证初始化加载 */
.app-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  background: var(--canvas-bg, #f1f5f9);
}
.loading-spinner {
  width: 36px; height: 36px;
  border: 3px solid var(--border-light, #e2e8f0);
  border-top-color: var(--primary-500, #4F6EF7);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* 更新提示横幅 */
.update-banner {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 200;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px var(--space-md);
  background: linear-gradient(135deg, #4F6EF7, #6366f1);
  color: #fff;
  font-size: var(--font-size-sm, 13px);
  box-shadow: 0 2px 12px rgba(79,110,247,0.3);
  animation: slideDown 0.3s ease-out;
}
@keyframes slideDown { from { transform: translateY(-100%); } to { transform: translateY(0); } }

.update-banner-inner {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.update-icon { flex-shrink: 0; opacity: 0.9; }

.update-banner span {
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}

.update-link {
  color: #fff; text-decoration: underline; text-underline-offset: 2px;
  margin-left: 4px;
}
.update-link:hover { opacity: 0.85; }

.update-dismiss {
  flex-shrink: 0;
  background: none; border: none;
  color: rgba(255,255,255,0.8);
  font-size: 22px; line-height: 1;
  cursor: pointer; padding: 2px 4px; margin-left: 12px;
}
.update-dismiss:hover { color: #fff; }
</style>
