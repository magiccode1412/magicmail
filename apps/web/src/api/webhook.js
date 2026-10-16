// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import request from './request'

/**
 * Webhook API
 */

/** 获取 Webhook 列表 */
export function listWebhooks() {
  return request.get('/webhooks')
}

/** 获取 Webhook 详情 */
export function getWebhook(id) {
  return request.get(`/webhooks/${id}`)
}

/** 创建 Webhook */
export function createWebhook(data) {
  return request.post('/webhooks', data)
}

/** 更新 Webhook */
export function updateWebhook(id, data) {
  return request.put(`/webhooks/${id}`, data)
}

/** 删除 Webhook */
export function deleteWebhook(id) {
  return request.delete(`/webhooks/${id}`)
}

/** 测试 Webhook（发送测试请求） */
export function testWebhook(id) {
  return request.post(`/webhooks/${id}/test`)
}

/** 模拟邮件接收，触发完整通知链路（Webhook + SSE + Web Push） */
export function simulateMailReceived(data = {}) {
  return request.post('/webhooks/simulate-mail', data)
}

/** 获取 Webhook 推送日志 */
export function getWebhookLogs(id, limit = 20) {
  return request.get(`/webhooks/${id}/logs`, { params: { limit } })
}
