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
  // 飞牛统一网关登录状态
  const gatewayAvailable = ref(false) // 是否处于飞牛网关环境（请求带 X-Trim-Userid）
  const fnosBound = ref(false)         // 当前飞牛用户是否已绑定 magicmail 账号
  const fnosUsername = ref('')         // 已绑定时返回的 magicmail 用户名

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
    // 探测飞牛网关状态（无论是否登录都探测，用于显示飞牛登录入口）
    try {
      const fnos = await authApi.fnosStatus()
      gatewayAvailable.value = !!fnos.gateway
      fnosBound.value = !!fnos.bound
      fnosUsername.value = fnos.username || ''
    } catch (_) {
      gatewayAvailable.value = false
      fnosBound.value = false
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

  // 飞牛网关：绑定已有账号（校验原密码）
  async function doFnosBind(bindData) {
    const res = await authApi.fnosBind(bindData)
    setToken(res.token, res.username, res.role)
    fnosBound.value = true
    fnosUsername.value = res.username || ''
    return res
  }

  // 飞牛网关：注册新账号并绑定
  async function doFnosRegister(regData) {
    const res = await authApi.fnosRegister(regData)
    setToken(res.token, res.username, res.role)
    fnosBound.value = true
    fnosUsername.value = res.username || ''
    return res
  }

  // 飞牛网关：已绑定用户免密登录
  async function doFnosLogin() {
    const res = await authApi.fnosLogin()
    setToken(res.token, res.username, res.role)
    fnosBound.value = true
    fnosUsername.value = res.username || ''
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
    gatewayAvailable, fnosBound, fnosUsername,
    init, doLogin, doRegister, doFnosBind, doFnosRegister, doFnosLogin, logout, parseUsername,
  }
})
