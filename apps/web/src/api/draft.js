// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import request from './request'

// 获取草稿列表
export function getDrafts(params = {}) {
  return request.get('/drafts', { params })
}

// 获取草稿详情
export function getDraftById(id) {
  return request.get(`/drafts/${id}`)
}

// 保存草稿（新建或更新）
export function saveDraft(data) {
  if (data.id) {
    return request.put(`/drafts/${data.id}`, data)
  }
  return request.post('/drafts', data)
}

// 删除草稿
export function deleteDraft(id) {
  return request.delete(`/drafts/${id}`)
}

// 批量删除草稿
export function batchDeleteDrafts(ids) {
  return request.post('/drafts/batch-delete', { ids })
}
