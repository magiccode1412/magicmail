// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import axios from 'axios'

// 创建 axios 实例
// 飞牛统一网关等场景通过 Vite base 注入前缀（如 /app/magicmail）
const request = axios.create({
  baseURL: import.meta.env.BASE_URL + 'api/v1',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 模块级路由引用（由 main.js 或 App.vue 注入，避免循环依赖）
let _router = null

/** 注入 Vue Router 实例 */
export function setRouter(routerInstance) {
  _router = routerInstance
}

function navigateToLogin() {
  if (_router && _router.currentRoute.value.path !== '/login') {
    _router.push('/login')
  } else if (window.location.pathname !== '/login') {
    // 带前缀部署（如飞牛统一网关 /app/magicmail）下不能直接跳 '/login'，
    // 否则会跳到 NAS 站点根路径。BASE_URL 在非飞牛构建时为 '/'，行为与原来一致。
    window.location.href = import.meta.env.BASE_URL + 'login'
  }
}

// 请求拦截器：自动附加 token
request.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('magicmail-token')
    if (token) config.headers.Authorization = `Bearer ${token}`
    return config
  },
  (error) => Promise.reject(error)
)

// 响应拦截器：统一错误处理
request.interceptors.response.use(
  (response) => {
    return response.data
  },
  (error) => {
    const { response } = error

    if (!response) {
      console.error('[API] 网络异常，请检查后端服务是否启动')
      return Promise.reject(new Error('网络连接失败'))
    }

    const status = response.status
    let message = '请求失败'
    
    if (response.data?.detail) {
      message = response.data.detail
    } else if (response.data?.error) {
      message = response.data.error
    }

    switch (status) {
      case 400:
        console.warn(`[API] 请求参数错误: ${message}`)
        break
      case 401:
        console.warn('[API] 未认证，跳转登录页')
        localStorage.removeItem('magicmail-token')
        navigateToLogin()
        break
      case 403:
        console.warn('[API] 无权限访问')
        break
      case 404:
        console.warn(`[API] 资源不存在: ${message}`)
        break
      case 500:
        console.error(`[API] 服务端错误: ${message}`)
        break
      default:
        console.error(`[API] 错误 [${status}]: ${message}`)
    }

    return Promise.reject(new Error(message))
  }
)

export default request
