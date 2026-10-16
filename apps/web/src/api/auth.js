// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import request from './request'

/** 查询认证状态（是否需要注册）。公开接口，不校验 token，不可用于探活 */
export function getAuthStatus() {
  return request.get('/auth/status')
}

/** 登录态探活：当前用户（受保护）。401 即代表本地 token 已失效 */
export function getMe() {
  return request.get('/auth/me')
}

/** 登录 */
export function login(data) {
  return request.post('/auth/login', data)
}

/** 注册（仅首次可用） */
export function register(data) {
  return request.post('/auth/register', data, { timeout: 10000 })
}

/** 查询飞牛网关登录绑定状态 */
export function fnosStatus() {
  return request.get('/auth/fnos/status')
}

/** 已绑定用户免密登录（飞牛 uid 由网关 Header 注入） */
export function fnosLogin() {
  return request.post('/auth/fnos/login')
}

/** 绑定已有账号到飞牛身份（飞牛 uid 由网关 Header 注入） */
export function fnosBind(data) {
  return request.post('/auth/fnos/bind', data)
}

/** 注册新账号并绑定飞牛身份 */
export function fnosRegister(data) {
  return request.post('/auth/fnos/register', data, { timeout: 10000 })
}
