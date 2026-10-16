// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import request from './request'

// 获取邮件附件列表
export function getAttachmentsByMailId(mailId) {
  return request.get(`/attachments/mail/${mailId}`)
}

// 下载附件（返回 blob URL）
export async function downloadAttachment(id) {
  const response = await request.get(`/attachments/${id}/download`, {
    responseType: 'blob'
  })
  return response
}

// 构造下载链接（直接浏览器下载）
export function getAttachmentDownloadUrl(id) {
  const token = localStorage.getItem('magicmail-token')
  const baseUrl = import.meta.env.VITE_API_BASE_URL || import.meta.env.BASE_URL
  const url = `${baseUrl}api/v1/attachments/${id}/download`
  return token ? `${url}?token=${encodeURIComponent(token)}` : url
}
