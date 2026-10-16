// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  getAccounts, createAccount as apiCreate,
  updateAccount as apiUpdate, deleteAccount as apiDelete,
  toggleAccountStatus as apiToggleStatus
} from '@/api/account'

export const useAccountStore = defineStore('account', () => {
  // --- 状态 ---
  const accounts = ref([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref(null)

  // --- 计算属性 ---
  const activeAccounts = computed(() =>
    accounts.value.filter(a => a.status === 'active')
  )

  const totalUnread = computed(() =>
    accounts.value.reduce((sum, a) => sum + (a.unread_count || 0), 0)
  )

  // --- 加载账号列表 ---
  async function fetchAccounts() {
    loading.value = true
    error.value = null

    try {
      const res = await getAccounts()
      accounts.value = res.data || []
      total.value = res.total || 0
    } catch (e) {
      error.value = e.message
      console.error('[accountStore] 获取邮箱列表失败:', e.message)
      // 失败时保留旧数据，不清空 accounts
    } finally {
      loading.value = false
    }
  }

  // --- 创建账号 ---
  async function addAccount(data) {
    loading.value = true
    try {
      const account = await apiCreate(data)
      accounts.value.unshift(account)
      return account
    } catch (e) {
      console.error('[accountStore] 创建失败:', e.message)
      throw e
    } finally {
      loading.value = false
    }
  }

  // --- 更新账号 ---
  async function editAccount(id, data) {
    try {
      const updated = await apiUpdate(id, data)
      const idx = accounts.value.findIndex(a => a.id === id)
      if (idx !== -1) {
        accounts.value[idx] = { ...accounts.value[idx], ...updated }
      }
      return updated
    } catch (e) {
      console.error('[accountStore] 更新失败:', e.message)
      throw e
    }
  }

  // --- 删除账号 ---
  async function removeAccount(id) {
    try {
      await apiDelete(id)
      accounts.value = accounts.value.filter(a => a.id !== id)
    } catch (e) {
      console.error('[accountStore] 删除失败:', e.message)
      throw e
    }
  }

  // --- 停用/启用账号 ---
  async function toggleStatus(id, status) {
    try {
      await apiToggleStatus(id, status)
      const idx = accounts.value.findIndex(a => a.id === id)
      if (idx !== -1) {
        accounts.value[idx].status = status
      }
    } catch (e) {
      console.error('[accountStore] 状态切换失败:', e.message)
      throw e
    }
  }

  return {
    accounts, total, loading, error,
    activeAccounts, totalUnread,
    fetchAccounts, addAccount, editAccount, removeAccount, toggleStatus,
  }
})
