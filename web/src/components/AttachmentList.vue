<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="attachment-list">
    <h4 class="list-title">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
        <path d="M10.5 4l3.5 4v5a1.5 1.5 0 0 1-1.5 1.5h-8A1.5 1.5 0 0 1 2.5 13V5a1.5 1.5 0 0 1 1.5-1.5h6.5z" stroke="currentColor" stroke-width="1.3"/>
        <path d="M10.5 4v3.5H14" stroke="currentColor" stroke-width="1.3"/>
      </svg>
      附件 ({{ attachments.length }})
    </h4>

    <div class="attachments">
      <div
        v-for="att in attachments"
        :key="att.id"
        class="attachment-item"
      >
        <!-- 文件类型图标 -->
        <div class="file-icon" :class="getFileCategory(att.content_type)">
          {{ getFileExtension(att.filename) }}
        </div>

        <!-- 文件信息 -->
        <div class="file-info">
          <div class="file-name-row">
            <span class="file-name" :title="att.filename">{{ att.filename }}</span>
            <!-- 缓存状态标识 -->
            <span
              class="cache-badge"
              :class="att.is_cached ? 'cache-local' : 'cache-cloud'"
              :title="att.is_cached ? '已缓存到本地磁盘' : '存储在邮件服务器，首次下载将从服务器获取'"
            >
              <!-- 本地/云端图标 -->
              <svg v-if="att.is_cached" width="12" height="12" viewBox="0 0 12 12" fill="none">
                <rect x="1" y="2" width="9" height="7" rx="1" stroke="currentColor" stroke-width="1.2"/>
                <path d="M3 5h5M3 7h3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                <path d="M10 4v5a1 1 0 0 1-1 1H3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
              </svg>
              <svg v-else width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path d="M6 1.5c-2.5 0-4.5 2-4.5 4.5 0 3 4.5 5 4.5 5s4.5-2 4.5-5c0-2.5-2-4.5-4.5-4.5z" stroke="currentColor" stroke-width="1.2"/>
                <circle cx="6" cy="6" r="1.5" stroke="currentColor" stroke-width="1.2"/>
              </svg>
              {{ att.is_cached ? '本地' : '云端' }}
            </span>
          </div>
          <span class="file-meta">
            {{ att.size_human }}
            <template v-if="att.content_type"> · {{ formatContentType(att.content_type) }}</template>
          </span>

          <!-- 下载进度条 (仅小文件模式显示) -->
          <div v-if="getState(att.id).downloading && getState(att.id).progress !== -1" class="progress-wrapper">
            <div class="progress-bar">
              <div
                class="progress-fill"
                :style="{ width: getState(att.id).progress + '%' }"
              ></div>
            </div>
            <span class="progress-text">
              {{ formatProgress(getState(att.id)) }}
            </span>
          </div>

          <!-- 大文件直连下载提示 -->
          <div v-if="getState(att.id).downloading && getState(att.id).progress === -1" class="direct-download-hint">
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
              <path d="M6 9V2m0 7L3.5 5M6 9L8.5 5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M10.5 9v1a.5.5 0 0 1-.5.5H2a.5.5 0 0 1-.5-.5V9" stroke="currentColor" stroke-width="1.2"/>
            </svg>
            已触发浏览器下载...
          </div>

          <!-- 错误提示 -->
          <div v-if="getState(att.id).error && !getState(att.id).downloading" class="error-hint">
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
              <circle cx="6" cy="6" r="5" stroke="currentColor" stroke-width="1.2"/>
              <path d="M6 4v2.5M6 8v.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            </svg>
            {{ getState(att.id).errorMsg }}
            <a href="#" class="retry-link" @click.prevent="handleDownload(att)">重试</a>
          </div>
        </div>

        <!-- 下载按钮 -->
        <button
          @click="handleDownload(att)"
          :disabled="getState(att.id).downloading"
          class="btn btn-secondary btn-sm download-btn"
          :title="getBtnTitle(att)"
        >
          <!-- 下载中: spinner -->
          <svg v-if="getState(att.id).downloading" class="spin-icon" width="14" height="14" viewBox="0 0 14 14" fill="none">
            <path d="M7 2a5 5 0 1 1-5 5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
          </svg>
          <!-- 已完成 -->
          <svg v-else-if="getState(att.id).completed" width="14" height="14" viewBox="0 0 14 14" fill="none">
            <path d="M3 7l2.5 2.5L11 5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <!-- 默认下载 -->
          <svg v-else width="14" height="14" viewBox="0 0 14 14" fill="none">
            <path d="M7 10V3m0 7L4 7m3 3L10 7" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M12 11v1a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1v-1" stroke="currentColor" stroke-width="1.4"/>
          </svg>
          {{ getBtnText(att) }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive } from 'vue'
import { getAttachmentDownloadUrl, downloadAttachment } from '@/api/attachment'

const props = defineProps({
  attachments: {
    type: Array,
    default: () => []
  }
})

// 每个附件的下载状态: { id -> { downloading, progress, loaded, total, error, errorMsg, completed } }
const downloadStates = reactive({})

function getState(id) {
  return downloadStates[id] || { downloading: false, progress: 0, loaded: 0, total: 0, error: false, errorMsg: '', completed: false }
}

function initOrResetState(att) {
  downloadStates[att.id] = {
    downloading: true,
    progress: 0,
    loaded: 0,
    total: att.size || 0,
    error: false,
    errorMsg: '',
    completed: false
  }
}

// 大文件阈值: 10MB
const LARGE_FILE_THRESHOLD = 10 * 1024 * 1024

async function handleDownload(att) {
  // 防止重复点击
  if (getState(att.id).downloading) return

  // 智能选择: 小文件用 fetch+blob(带进度), 大文件用 a 标签直链(省内存)
  if (att.size && att.size >= LARGE_FILE_THRESHOLD) {
    return downloadDirectly(att)
  }

  return downloadWithProgress(att)
}

/**
 * 大文件直连下载（不经过 JS 内存，浏览器原生处理）
 */
function downloadDirectly(att) {
  // 显示"正在下载"提示状态
  downloadStates[att.id] = {
    downloading: true,
    progress: -1, // 不确定模式(无进度)
    loaded: 0,
    total: att.size || 0,
    error: false,
    errorMsg: '',
    completed: false
  }

  try {
    // 使用 a 标签触发浏览器原生下载
    const url = getAttachmentDownloadUrl(att.id)
    const link = document.createElement('a')
    link.href = url
    link.download = att.filename || 'attachment'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)

    // 短暂显示"已触发下载"状态后清除
    setTimeout(() => {
      delete downloadStates[att.id]
    }, 2000)
  } catch (err) {
    downloadStates[att.id].downloading = false
    downloadStates[att.id].error = true
    downloadStates[att.id].errorMsg = '下载失败'
    console.error(`[附件下载] ${att.filename} 下载失败:`, err)
  }
}

/**
 * 小文件带进度条下载（fetch + blob 方式）
 */
async function downloadWithProgress(att) {
  initOrResetState(att)
  const state = downloadStates[att.id]

  try {
    const response = await downloadAttachmentWithProgress(att.id, (progressEvent) => {
      if (progressEvent.total) {
        state.progress = Math.round((progressEvent.loaded / progressEvent.total) * 100)
        state.loaded = progressEvent.loaded
        state.total = progressEvent.total
      } else {
        state.loaded = progressEvent.loaded
        state.progress = -1 // 标记为不确定模式
      }
    })

    triggerBlobDownload(response.data, att.filename)

    state.downloading = false
    state.completed = true
    state.progress = 100

    setTimeout(() => {
      delete downloadStates[att.id]
    }, 3000)
  } catch (err) {
    state.downloading = false
    state.error = true
    state.errorMsg = err.message || '下载失败'
    console.error(`[附件下载] ${att.filename} 下载失败:`, err)
  }
}

/**
 * 带进度的下载请求（使用原生 axios 避免 request 拦截器对 blob 的干扰）
 */
async function downloadAttachmentWithProgress(id, onProgress) {
  const token = localStorage.getItem('magicmail-token')
  const headers = {}
  if (token) headers['Authorization'] = `Bearer ${token}`

  const axios = (await import('axios')).default
  const baseUrl = import.meta.env.VITE_API_BASE_URL || import.meta.env.BASE_URL
  const url = `${baseUrl}api/v1/attachments/${id}/download`

  const response = await axios.get(url, {
    responseType: 'blob',
    headers,
    timeout: 300000, // 流式传输可能耗时较长，设为 5 分钟
    onDownloadProgress: onProgress
  })

  return response
}

function triggerBlobDownload(blobData, filename) {
  const url = URL.createObjectURL(blobData)
  const link = document.createElement('a')
  link.href = url
  link.download = filename || 'attachment'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/* ---- UI 辅助函数 ---- */

function getFileExtension(filename) {
  const ext = filename.split('.').pop()?.toUpperCase()
  return ext?.length <= 4 ? ext : 'FILE'
}

function getFileCategory(contentType) {
  if (!contentType) return 'other'
  
  const type = contentType.toLowerCase()
  if (type.startsWith('image/')) return 'image'
  if (type.startsWith('video/')) return 'video'
  if (type.startsWith('audio/')) return 'audio'
  if (type.includes('pdf')) return 'document'
  if (type.includes('zip') || type.includes('rar') || type.includes('archive')) return 'archive'
  if (type.includes('text') || type.includes('document') || type.includes('sheet') || type.includes('presentation')) return 'document'
  
  return 'other'
}

function formatContentType(ct) {
  if (!ct) return ''
  const sub = ct.split('/')[1]
  if (!sub) return ct
  return sub.toUpperCase().replace(/X-/g, '')
}

function formatProgress(state) {
  if (state.progress === -1) {
    return formatSize(state.loaded) + ' / ...'
  }
  if (state.total > 0) {
    return `${state.progress}% (${formatSize(state.loaded)} / ${formatSize(state.total)})`
  }
  return `${state.progress}%`
}

function formatSize(bytes) {
  if (bytes === undefined || bytes === null || bytes < 0) return '未知'
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024
    i++
  }
  return bytes.toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

function getBtnText(att) {
  const s = getState(att.id)
  if (s.downloading && s.progress !== -1) return '下载中'  // 小文件: 显示进度
  if (s.downloading && s.progress === -1) return '下载中'   // 大文件: 也显示
  if (s.completed) return '完成'
  return '下载'
}

function getBtnTitle(att) {
  const s = getState(att.id)
  if (s.downloading) return '正在下载...'
  if (s.completed) return '下载完成'
  if (!att.is_cached) return '从邮件服务器实时获取'
  return '下载附件'
}
</script>

<style scoped>
.list-title {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin-bottom: var(--space-md);
}

.attachments {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.attachment-item {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-sm) var(--space-md);
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  transition: all var(--transition-fast);
}
.attachment-item:hover {
  background: var(--bg-hover);
  box-shadow: var(--shadow-sm);
}

/* ---- 文件图标 ---- */
.file-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: var(--radius-sm);
  font-size: 9px;
  font-weight: var(--font-weight-bold);
  flex-shrink: 0;
}
.file-icon.image { background: linear-gradient(135deg, #DBEAFE, #BFDBFE); color: #2563EB; }
.file-icon.document { background: linear-gradient(135deg, #FEF3C7, #FDE68A); color: #D97706; }
.file-icon.video { background: linear-gradient(135deg, #FEE2E2, #FECACA); color: #DC2626; }
.file-icon.audio { background: linear-gradient(135deg, #ECFDF5, #D1FAE5); color: #059669; }
.file-icon.archive { background: linear-gradient(135deg, #EDE9FE, #DDD6FE); color: #7C3AED; }
.file-icon.other { background: var(--bg-tertiary); color: var(--text-tertiary); }

/* ---- 文件信息 ---- */
.file-info {
  flex: 1;
  min-width: 0;
}

.file-name-row {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  min-width: 0;
}

.file-name {
  display: block;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.file-meta {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  display: block;
  margin-top: 2px;
}

/* ---- 缓存状态标识 ---- */
.cache-badge {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 1px 6px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 600;
  line-height: 1.4;
  white-space: nowrap;
  flex-shrink: 0;
}
.cache-badge.cache-local {
  background: rgba(5, 150, 105, 0.1);
  color: #059669;
}
.cache-badge.cache-cloud {
  background: rgba(59, 130, 246, 0.1);
  color: #3B82F6;
}

/* ---- 进度条 ---- */
.progress-wrapper {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin-top: 6px;
}

.progress-bar {
  flex: 1;
  height: 6px;
  background: var(--bg-tertiary);
  border-radius: 3px;
  overflow: hidden;
  min-width: 80px;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #3B82F6, #60A5FA);
  border-radius: 3px;
  transition: width 0.15s ease-out;
  min-width: 2px;
}

.progress-text {
  font-size: 10px;
  color: var(--text-secondary);
  white-space: nowrap;
  flex-shrink: 0;
  min-width: fit-content;
}

/* ---- 大文件直连下载提示 ---- */
.direct-download-hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 11px;
  color: var(--text-secondary);
}

/* ---- 错误提示 ---- */
.error-hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 11px;
  color: #DC2626;
  background: rgba(220, 38, 38, 0.06);
  padding: 3px 8px;
  border-radius: var(--radius-sm);
}

.retry-link {
  color: #DC2626;
  text-decoration: underline;
  cursor: pointer;
  margin-left: 4px;
}
.retry-link:hover {
  color: #B91C1C;
}

/* ---- 下载按钮 ---- */
.download-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.download-btn:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

/* ---- 旋转动画 ---- */
.spin-icon {
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
