// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import request from './request'

/** 获取用户列表（管理员） */
export function listUsers() {
  return request.get('/admin/users')
}

/** 管理员后台创建用户 */
export function createUser(data) {
  return request.post('/admin/users', data)
}

/** 删除用户（含关联数据） */
export function deleteUser(id) {
  return request.delete(`/admin/users/${id}`)
}
