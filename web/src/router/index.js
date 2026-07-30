// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/authStore'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/LoginView.vue'),
    meta: { title: '登录', public: true }
  },
  {
    path: '/',
    redirect: '/mails'
  },
  {
    path: '/mails',
    name: 'MailList',
    component: () => import('../views/MailListView.vue'),
    meta: { title: '邮件列表' }
  },
  {
    path: '/sent',
    name: 'Sent',
    component: () => import('../views/SentView.vue'),
    meta: { title: '已发送' }
  },
  {
    path: '/drafts',
    name: 'Drafts',
    component: () => import('../views/DraftsView.vue'),
    meta: { title: '草稿' }
  },
  {
    path: '/mails/:id',
    name: 'MailReader',
    component: () => import('../views/MailReaderView.vue'),
    meta: { title: '阅读邮件', parent: '/mails' }
  },
  {
    path: '/accounts',
    name: 'AccountManage',
    component: () => import('../views/AccountManage.vue'),
    meta: { title: '邮箱管理' }
  },
  {
    path: '/compose',
    name: 'Compose',
    component: () => import('../views/ComposeView.vue'),
    meta: { title: '写邮件' }
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/SettingsView.vue'),
    meta: { title: '设置' }
  },
  {
    path: '/about',
    name: 'About',
    component: () => import('../views/AboutView.vue'),
    meta: { title: '关于' }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition
    if (to.meta.parent) return { top: 0 }
    return false
  }
})

// 路由守卫：认证检查 + 页面标题更新
router.beforeEach(async (to) => {
  document.title = to.meta.title
    ? `${to.meta.title} - Magicmail`
    : 'Magicmail'

  // 公开页面（登录）直接放行
  if (to.meta.public) return

  // 检查登录状态
  const authStore = useAuthStore()
  if (!authStore.initialized) {
    await authStore.init()
  }

  // 未登录则跳转登录页
  if (!authStore.isLoggedIn && to.path !== '/login') {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
})

export default router
