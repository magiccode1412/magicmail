// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import request from './request'

/** 获取开放注册开关状态（管理员） */
export function getOpenRegistration() {
  return request.get('/settings/open-registration')
}

/** 设置开放注册开关状态（管理员） */
export function setOpenRegistration(open) {
  return request.put('/settings/open-registration', { open_registration: open })
}
