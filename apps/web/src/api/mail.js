// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import request from './request'

// 获取邮件列表
export function getMails(params = {}) {
  return request.get('/mails', { params })
}

// 获取邮件详情
export function getMailById(id) {
  return request.get(`/mails/${id}`)
}

// 标记已读/未读
export function markAsRead(id, isRead) {
  return request.put(`/mails/${id}/read`, { is_read: isRead })
}

// 标记星标
export function markAsStarred(id, starred) {
  return request.put(`/mails/${id}/star`, { starred })
}

// 删除邮件
export function deleteMail(id) {
  return request.delete(`/mails/${id}`)
}

// 批量删除邮件
export function batchDeleteMails(ids) {
  return request.post('/mails/batch-delete', { ids })
}

// 批量标记已读/未读
export function batchMarkAsRead(ids, isRead) {
  return request.post('/mails/batch-read', { ids, is_read: isRead })
}

// 一键标记所有邮件为已读（按筛选条件）
export function markAllAsRead(params = {}) {
  return request.post('/mails/mark-all-read', params)
}

// 获取统计
export function getMailStats(params = {}) {
  return request.get('/mails/stats', { params })
}

// 发送邮件
export function sendEmail(data) {
  return request.post('/mails/send', data)
}
