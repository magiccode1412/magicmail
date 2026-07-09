// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as authApi from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('magicmail-token') || '')
  const username = ref('')
  const role = ref('')
  const setupRequired = ref(false)
  const openRegistration = ref(false)
  const initialized = ref(false)

  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => role.value === 'admin')

  async function init() {
    if (token.value) {
      // 有 token 时验证是否有效（清库/过期等场景）
      try {
        const res = await authApi.getAuthStatus()
        // token 有效，更新用户名与角色
        const payload = parseTokenPayload(token.value)
        username.value = payload.username || ''
        role.value = payload.role || ''
        openRegistration.value = !!res.open_registration
      } catch (_) {
        // token 无效（401 / 后端已重置等），清除并标记需要重新登录
        logout()
      }
    } else {
      try {
        const res = await authApi.getAuthStatus()
        setupRequired.value = res.setup_required
        openRegistration.value = !!res.open_registration
      } catch (_) {
        setupRequired.value = true
      }
    }
    initialized.value = true
  }

  async function doLogin(loginData) {
    const res = await authApi.login(loginData)
    setToken(res.token, res.username, res.role)
    return res
  }

  async function doRegister(regData) {
    const res = await authApi.register(regData)
    if (res.token) {
      setToken(res.token, res.username, res.role)
    }
    return res
  }

  function setToken(newToken, name, newRole) {
    token.value = newToken
    username.value = name || ''
    role.value = newRole || parseTokenPayload(newToken).role || ''
    localStorage.setItem('magicmail-token', newToken || '')
  }

  function logout() {
    token.value = ''
    username.value = ''
    role.value = ''
    localStorage.removeItem('magicmail-token')
  }

  // 从 JWT payload 解析用户名（纯前端解析，无需后端请求）
  function parseUsername(jwtStr) {
    return parseTokenPayload(jwtStr).username || ''
  }

  // 解析 JWT payload 中的 username / role
  function parseTokenPayload(jwtStr) {
    try {
      return JSON.parse(atob(jwtStr.split('.')[1]))
    } catch { return {} }
  }

  return {
    token, username, role, isLoggedIn, isAdmin,
    setupRequired, openRegistration, initialized,
    init, doLogin, doRegister, logout, parseUsername,
  }
})
