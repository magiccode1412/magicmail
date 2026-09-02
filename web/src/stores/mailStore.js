// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import { defineStore, acceptHMRUpdate } from 'pinia'
import { ref, computed } from 'vue'
import { getMails, getMailById, markAsRead as apiMarkRead, deleteMail as apiDeleteMail, batchDeleteMails as apiBatchDelete, batchMarkAsRead as apiBatchMarkRead, markAllAsRead as apiMarkAllAsRead, getMailStats } from '@/api/mail'

export const useMailStore = defineStore('mail', () => {
  // --- 状态 ---
  const mails = ref([])
  const currentMail = ref(null)
  const total = ref(0)
  const currentPage = ref(1)
  const pageSize = ref(20)
  
  // 筛选条件
  const filters = ref({
    account_id: '',
    folder: '',
    keyword: '',
    is_read: null,
    has_attachment: null,
    sort_by: 'sent_at',
    sort_order: 'desc'
  })

  const loading = ref(false)
  const error = ref(null)
  const stats = ref({})

  // 待删除邮件 ID 集合：删除操作后立即生效，但服务器对账（re-fetch）可能仍返回陈旧数据。
  // 用于防止“已删除的邮件”在对账拉取中被重新显示（即“复活”），从而无需手动刷新。
  // 删除请求成功即加入，待对账完成后移除（见 deleteMail / batchDeleteMails 的 finally）。
  const pendingDeletes = new Set()

  // --- 计算属性 ---
  const hasMore = computed(() => mails.value.length < total.value)

  // --- 加载邮件列表 ---
  async function fetchMails(page = 1) {
    loading.value = true
    error.value = null

    try {
      const params = {
        page,
        page_size: pageSize.value,
        ...filters.value
      }

      // 清理空值
      Object.keys(params).forEach(key => {
        if (params[key] === '' || params[key] === null || params[key] === undefined) {
          delete params[key]
        }
      })

      const res = await getMails(params)
      mails.value = res.data || []
      // 过滤掉刚删除、但服务器对账尚未生效的邮件，避免陈旧数据“复活”
      // （例如删除后 reconcile 的即时拉取，或 SSE 推送触发的列表刷新先于删除生效）
      if (pendingDeletes.size > 0) {
        mails.value = mails.value.filter(m => !pendingDeletes.has(m.id))
      }
      total.value = res.total || 0
      currentPage.value = res.page || page
    } catch (e) {
      error.value = e.message
      console.error('[mailStore] 获取邮件列表失败:', e.message)
    } finally {
      loading.value = false
    }
  }

  // --- 与服务器对账：操作后刷新列表与统计，确保 UI 与最新状态一致（无需手动刷新）---
  async function reconcile() {
    try {
      await fetchMails(currentPage.value)
      await fetchStats()
    } catch (e) {
      console.error('[mailStore] 操作后对账失败:', e.message)
    }
  }

  // --- 加载邮件详情 ---
  async function fetchMailDetail(id) {
    try {
      const mail = await getMailById(id)
      currentMail.value = mail
      
      // 自动标记已读（如果未读）
      if (!mail.is_read) {
        await markAsRead(id, true)
        // 与服务器对账，确保返回列表时该邮件已为已读状态
        await fetchMails(currentPage.value)
      }
      
      return mail
    } catch (e) {
      console.error('[mailStore] 获取邮件详情失败:', e.message)
      throw e
    }
  }

  // --- 标记已读/未读 ---
  async function markAsRead(id, isRead) {
    try {
      // 获取操作前的状态用于计算计数变化
      const mail = mails.value.find(m => m.id === id)
      const wasUnread = mail ? !mail.is_read : false

      await apiMarkRead(id, isRead)
      // 更新本地状态
      const idx = mails.value.findIndex(m => m.id === id)
      if (idx !== -1) {
        mails.value[idx].is_read = isRead
      }
      if (currentMail.value?.id === id) {
        currentMail.value.is_read = isRead
      }

      // 同步更新未读计数器
      if (wasUnread && isRead) {
        // 未读 → 已读：计数减 1
        stats.value = { ...stats.value, unread: Math.max(0, (stats.value.unread || 0) - 1) }
      } else if (!wasUnread && !isRead) {
        // 已读 → 未读：计数加 1
        stats.value = { ...stats.value, unread: (stats.value.unread || 0) + 1 }
      }
    } catch (e) {
      console.error('[mailStore] 标记已读失败:', e.message)
    }
  }

  // --- 删除邮件 ---
  async function deleteMail(id) {
    try {
      const deletedMail = mails.value.find(m => m.id === id)
      const res = await apiDeleteMail(id)
      // 删除成功后立即标记为待删除：后续对账拉取（reconcile）若返回陈旧数据，
      // 该邮件会被 fetchMails 过滤掉，避免“复活”，从而无需手动刷新即可更新列表
      pendingDeletes.add(id)
      mails.value = mails.value.filter(m => m.id !== id)
      total.value--
      // 更新统计信息（未读计数等）
      if (deletedMail && !deletedMail.is_read) {
        stats.value = { ...stats.value, unread: Math.max(0, (stats.value.unread || 0) - 1) }
      }
      // 与服务器对账，确保列表即时同步（替代手动刷新）
      await reconcile()
      return res
    } catch (e) {
      console.error('[mailStore] 删除邮件失败:', e.message)
      throw e
    } finally {
      pendingDeletes.delete(id)
    }
  }

  // --- 批量删除邮件 ---
  async function batchDeleteMails(ids) {
    try {
      const res = await apiBatchDelete(ids)
      // 仅移除“云端删除成功、本地也已删除”的邮件；云端删除失败（保留本地）的邮件需继续展示
      const failedSet = new Set((res.failed || []).map(String))
      const successIds = ids.filter(id => !failedSet.has(String(id)))
      const successSet = new Set(successIds)
      const deletedMails = mails.value.filter(m => successSet.has(m.id))
      const unreadDeleted = deletedMails.filter(m => !m.is_read).length

      // 删除成功后立即标记为待删除，防止对账拉取返回陈旧数据导致“复活”
      successIds.forEach(id => pendingDeletes.add(id))
      mails.value = mails.value.filter(m => !successSet.has(m.id))
      total.value -= res.deleted || successIds.length
      // 更新统计信息（未读计数等）
      stats.value = { ...stats.value, unread: Math.max(0, (stats.value.unread || 0) - unreadDeleted) }
      // 与服务器对账，确保列表即时同步（替代手动刷新）
      await reconcile()
      return res
    } catch (e) {
      console.error('[mailStore] 批量删除失败:', e.message)
      throw e
    } finally {
      ids.forEach(id => pendingDeletes.delete(id))
    }
  }

  // --- 处理来自其它标签页/客户端的删除事件（SSE mail.deleted）---
  // 从本地列表即时移除指定邮件，无需整列表刷新，实现跨标签页实时一致。
  // 设计为幂等：若本地已不存在这些 ID（例如发起删除的标签页已自行移除），则直接返回，避免重复扣减计数。
  function handleMailDeleted(ids) {
    if (!ids || !ids.length) return
    const idSet = new Set(ids.map(String))

    let removed = 0
    let unreadRemoved = 0
    const remaining = []
    for (const m of mails.value) {
      if (idSet.has(String(m.id))) {
        removed++
        if (!m.is_read) unreadRemoved++
      } else {
        remaining.push(m)
      }
    }

    if (removed === 0) return // 本地已移除（如发起删除的标签页），幂等退出

    mails.value = remaining
    total.value -= removed
    if (unreadRemoved > 0) {
      stats.value = { ...stats.value, unread: Math.max(0, (stats.value.unread || 0) - unreadRemoved) }
    }
    // 正在查看的邮件被删除时清空详情，避免停留在已不存在的邮件上
    if (currentMail.value && idSet.has(String(currentMail.value.id))) {
      currentMail.value = null
    }
  }

  // --- 批量标记已读/未读 ---
  async function batchMarkAsRead(ids, isRead) {
    try {
      // 统计操作前的状态变化（用于计算未读计数）
      const targetMails = mails.value.filter(m => ids.includes(m.id))

      // 计算未读计数变化量
      let unreadDelta = 0
      if (isRead) {
        // 标记为已读：原来未读的邮件数量 = 未读计数减少量
        unreadDelta = -targetMails.filter(m => !m.is_read).length
      } else {
        // 标记为未读：原来已读的邮件数量 = 未读计数增加量
        unreadDelta = targetMails.filter(m => m.is_read).length
      }

      // 调用 API
      await apiBatchMarkRead(ids, isRead)

      // 更新本地列表状态
      const idSet = new Set(ids)
      mails.value.forEach(mail => {
        if (idSet.has(mail.id)) {
          mail.is_read = isRead
        }
      })

      // 同步更新未读计数器
      if (unreadDelta !== 0) {
        stats.value = { ...stats.value, unread: Math.max(0, (stats.value.unread || 0) + unreadDelta) }
      }

      // 与服务器对账，确保列表即时同步（替代手动刷新）
      reconcile()

      return { updated: targetMails.length }
    } catch (e) {
      console.error('[mailStore] 批量标记已读失败:', e.message)
      throw e
    }
  }

  // --- 更新筛选条件并刷新 ---
  function setFilter(key, value) {
    filters.value[key] = value
    return fetchMails(1)
  }

  // --- 重置筛选 ---
  function resetFilters() {
    filters.value = {
      account_id: '',
      folder: '',
      keyword: '',
      is_read: null,
      has_attachment: null,
      sort_by: 'sent_at',
      sort_order: 'desc'
    }
    return fetchMails(1)
  }

  // --- 获取统计信息 ---
  async function fetchStats(accountId) {
    try {
      const params = accountId ? { account_id: accountId } : {}
      const res = await getMailStats(params)
      stats.value = res || {}
      return res
    } catch (e) {
      console.error('[mailStore] 获取统计失败:', e.message)
      return {}
    }
  }

  // --- 一键标记所有邮件为已读 ---
  async function markAllAsRead() {
    // 使用当前筛选条件，仅影响当前视图中的邮件
    const params = { ...filters.value }
    Object.keys(params).forEach(key => {
      if (params[key] === '' || params[key] === null || params[key] === undefined) {
        delete params[key]
      }
    })

    const res = await apiMarkAllAsRead(params)

    // 与服务器对账，确保列表即时同步（替代手动刷新）
    reconcile()

    return res
  }

  return {
    mails, currentMail, total, currentPage, pageSize,
    filters, loading, error, hasMore, stats,
    fetchMails, fetchMailDetail, markAsRead, deleteMail, batchDeleteMails, handleMailDeleted, batchMarkAsRead, markAllAsRead,
    setFilter, resetFilters, fetchStats,
  }
})

// 支持 HMR 热更新（Pinia setup store 需手动启用，否则新增的 action 不会注入到已存在的 store 实例）
if (import.meta.hot) {
  import.meta.hot.accept(acceptHMRUpdate(useMailStore))
}
