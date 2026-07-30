// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import { ref, onMounted, onUnmounted } from 'vue'
import { useMailStore } from '@/stores/mailStore'

/**
 * 从 localStorage 获取认证 Token
 */
function getToken() {
  return localStorage.getItem('magicmail-token')
}

/**
 * SSE 实时推送 Composable
 * 
 * 用于接收服务器端的邮件更新事件，替代轮询机制
 * 
 * @param {Object} options - 配置选项
 * @param {Function} options.onMailReceived - 新邮件到达回调
 * @param {Function} options.onMailSynced - 同步完成回调
 * @param {Function} options.onConnected - 连接成功回调
 * @param {Function} options.onDisconnected - 断开连接回调
 * @param {Function} options.onError - 错误回调
 * @returns {Object} SSE 控制方法和状态
 */
export function useSSE(options = {}) {
  const eventSource = ref(null)
  const connected = ref(false)
  const connectionMode = ref('unknown') // 'sse' | 'polling' | 'unknown'
  const errorCount = ref(0)
  const maxErrorCount = 5 // 连续错误达到上限后触发 fallback

  let hasFallenBack = false // 是否已触发回退
  let intentionalClose = false // 是否为预期内的断开（如探测完成后主动关闭）

  /**
   * 建立 SSE 连接
   */
  function connect() {
    if (hasFallenBack) return // 已回退，不再尝试
    if (eventSource.value) {
      disconnect()
    }

    intentionalClose = false // 重置：新连接的断开都视为异常（除非再次调用 disconnect）

    const token = getToken()
    if (!token) {
      console.warn('[useSSE] 未找到认证 token，跳过 SSE 连接')
      return
    }

    // 构建带认证的 SSE URL
    const baseUrl = import.meta.env.VITE_API_BASE_URL || ''
    const url = `${baseUrl}/api/v1/mails/stream?token=${encodeURIComponent(token)}`

    try {
      const es = new EventSource(url, { withCredentials: true })

      es.addEventListener('connected', (event) => {
        const data = JSON.parse(event.data)
        connected.value = true
        connectionMode.value = 'sse'
        errorCount.value = 0

        if (typeof options.onConnected === 'function') {
          options.onConnected(data)
        }
      })

      // 新邮件到达事件
      es.addEventListener('mail.received', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onMailReceived === 'function') {
            options.onMailReceived(data)
          }
        } catch (e) {
          console.error('[useSSE] 解析 mail.received 事件失败:', e)
        }
      })

      // 邮件同步完成事件
      es.addEventListener('mail.synced', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onMailSynced === 'function') {
            options.onMailSynced(data)
          }
        } catch (e) {
          console.error('[useSSE] 解析 mail.synced 事件失败:', e)
        }
      })

      // 邮件已发送事件（修复：此前服务端已发布 mail.sent，但前端未消费，成为死事件）
      es.addEventListener('mail.sent', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onMailSent === 'function') {
            options.onMailSent(data)
          } else if (typeof options.onMailReceived === 'function') {
            // 兼容：未单独提供 onMailSent 时回落到通用更新回调
            options.onMailReceived(data)
          }
        } catch (e) {
          console.error('[useSSE] 解析 mail.sent 事件失败:', e)
        }
      })

      // 邮件已删除事件（跨标签页/客户端实时一致：携带被删邮件 ID，供前端从本地列表即时移除）
      es.addEventListener('mail.deleted', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onMailDeleted === 'function') {
            options.onMailDeleted(data)
          }
        } catch (e) {
          console.error('[useSSE] 解析 mail.deleted 事件失败:', e)
        }
      })

      // OAuth2 设备码授权成功（替代前端的 HTTP 轮询）
      es.addEventListener('oauth.authorized', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onOAuthAuthorized === 'function') {
            options.onOAuthAuthorized(data)
          }
        } catch (e) {
          console.error('[useSSE] 解析 oauth.authorized 事件失败:', e)
        }
      })

      // OAuth2 设备码授权过期/失败
      es.addEventListener('oauth.expired', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onOAuthExpired === 'function') {
            options.onOAuthExpired(data)
          }
        } catch (e) {
          console.error('[useSSE] 解析 oauth.expired 事件失败:', e)
        }
      })

      // 账号同步进度事件（手动/自动同步，按账号维度）
      es.addEventListener('account.sync_started', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onAccountSyncStarted === 'function') options.onAccountSyncStarted(data)
        } catch (e) { console.error('[useSSE] 解析 account.sync_started 失败:', e) }
      })
      es.addEventListener('account.sync_done', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onAccountSyncDone === 'function') options.onAccountSyncDone(data)
        } catch (e) { console.error('[useSSE] 解析 account.sync_done 失败:', e) }
      })
      es.addEventListener('account.sync_error', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onAccountSyncError === 'function') options.onAccountSyncError(data)
        } catch (e) { console.error('[useSSE] 解析 account.sync_error 失败:', e) }
      })

      // 草稿已删除事件（跨标签页/客户端实时一致：携带被删草稿 ID，供前端从本地草稿列表即时移除）
      es.addEventListener('draft.deleted', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onDraftDeleted === 'function') {
            options.onDraftDeleted(data)
          }
        } catch (e) {
          console.error('[useSSE] 解析 draft.deleted 事件失败:', e)
        }
      })

      // 账号生命周期事件（创建/更新/删除/状态变更，用于多标签页一致性）
      const accountLifecycleEvents = ['account.created', 'account.updated', 'account.deleted', 'account.status_changed']
      accountLifecycleEvents.forEach((evt) => {
        es.addEventListener(evt, (event) => {
          try {
            const data = JSON.parse(event.data)
            const cbName = {
              'account.created': 'onAccountCreated',
              'account.updated': 'onAccountUpdated',
              'account.deleted': 'onAccountDeleted',
              'account.status_changed': 'onAccountStatusChanged',
            }[evt]
            if (cbName && typeof options[cbName] === 'function') {
              options[cbName](data)
            }
          } catch (e) {
            console.error('[useSSE] 解析 ' + evt + ' 事件失败:', e)
          }
        })
      })

      // 账号连接健康状态变化（Worker 同步成功/失败状态切换时推送）
      es.addEventListener('account.health', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onAccountHealth === 'function') options.onAccountHealth(data)
        } catch (e) { console.error('[useSSE] 解析 account.health 失败:', e) }
      })

      // 账号当前同步模式变化（idle/polling/syncing/stopped），实时刷新详情弹窗中的状态
      es.addEventListener('account.mode_changed', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onAccountModeChanged === 'function') options.onAccountModeChanged(data)
        } catch (e) { console.error('[useSSE] 解析 account.mode_changed 失败:', e) }
      })

      // Webhook 投递结果事件
      es.addEventListener('webhook.delivered', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onWebhookDelivered === 'function') options.onWebhookDelivered(data)
        } catch (e) { console.error('[useSSE] 解析 webhook.delivered 失败:', e) }
      })
      es.addEventListener('webhook.failed', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onWebhookFailed === 'function') options.onWebhookFailed(data)
        } catch (e) { console.error('[useSSE] 解析 webhook.failed 失败:', e) }
      })

      // 统计轻量变更事件（侧边栏角标实时更新，无需刷新列表）
      es.addEventListener('stats.updated', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onStatsUpdated === 'function') options.onStatsUpdated(data)
        } catch (e) { console.error('[useSSE] 解析 stats.updated 失败:', e) }
      })

      // 用户管理事件（管理员创建/删除用户，广播给所有在线管理员，实现多管理员一致性）
      es.addEventListener('user.created', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onUserCreated === 'function') options.onUserCreated(data)
        } catch (e) { console.error('[useSSE] 解析 user.created 失败:', e) }
      })
      es.addEventListener('user.deleted', (event) => {
        try {
          const data = JSON.parse(event.data)
          if (typeof options.onUserDeleted === 'function') options.onUserDeleted(data)
        } catch (e) { console.error('[useSSE] 解析 user.deleted 失败:', e) }
      })

      // 心跳事件
      es.addEventListener('heartbeat', (event) => {
        // 保持连接活跃（可选：取消注释以调试）
        // console.log('[useSSE] 💓 心跳', event.data)
      })

      // 断线重连交给浏览器 EventSource 原生机制处理，
      // onerror 仅负责统计连续失败次数并在达到上限后回退到轮询
      es.onerror = (error) => {
        // 预期内的断开（如探测完成后的主动关闭），静默忽略
        if (intentionalClose) return

        connected.value = false

        if (typeof options.onError === 'function') {
          options.onError(error)
        }

        errorCount.value++

        // 连续错误达到上限，回退到轮询
        if (errorCount.value >= maxErrorCount) {
          hasFallenBack = true
          connectionMode.value = 'polling'
          disconnect()
          if (typeof options.onFallback === 'function') {
            options.onFallback()
          }
          if (typeof options.onDisconnected === 'function') {
            options.onDisconnected()
          }
        }
      }

      eventSource.value = es
    } catch (e) {
      console.error('[useSSE] 创建 EventSource 失败:', e)
    }
  }

  /**
   * 断开 SSE 连接
   */
  function disconnect() {
    intentionalClose = true // 标记为预期断开

    if (eventSource.value) {
      eventSource.value.close()
      eventSource.value = null
      connected.value = false
    }
  }

  /**
   * 手动重新连接
   */
  function reconnect() {
    errorCount.value = 0
    hasFallenBack = false
    disconnect()
    connect()
  }

  // 组件挂载时自动连接（manualConnect 模式由调用方手动 connect，用于按需场景如 OAuth 授权）
  onMounted(() => {
    if (!options.manualConnect) {
      connect()
    }
  })

  // 组件卸载时断开连接
  onUnmounted(() => {
    disconnect()
  })

  return {
    eventSource,
    connected,
    connectionMode,
    errorCount,
    connect,
    disconnect,
    reconnect,
  }
}

/**
 * 使用默认配置的邮件 SSE Hook（便捷方法）
 * 
 * @param {Function} onMailUpdate - 邮件更新时的回调函数
 * @param {Object} extraOptions - 额外配置
 * @param {Function} extraOptions.onFallback - SSE 失败后的回退回调（启动轮询）
 * @returns {Object} SSE 控制方法和状态
 */
export function useMailStream(onMailUpdate, extraOptions = {}) {
  const onUpdate = typeof onMailUpdate === 'function' ? onMailUpdate : () => {}

  // 默认消费 mail.deleted：从共享邮件 store 即时移除被其它标签页/客户端删除的邮件。
  // 若调用方显式传入 onMailDeleted，则优先使用调用方的回调。
  const onDeleted = typeof extraOptions.onMailDeleted === 'function'
    ? extraOptions.onMailDeleted
    : (data) => {
        try {
          const store = useMailStore()
          store.handleMailDeleted(data?.ids || [])
          // 轻量刷新统计（侧边栏角标），确保未读计数与已删邮件一致
          store.fetchStats()
        } catch (e) {
          // store 未初始化时忽略
        }
      }

  return useSSE({
    onMailReceived: onUpdate,
    onMailSynced: onUpdate,
    onMailSent: onUpdate, // 修复 mail.sent 死事件：发送后触发列表/统计刷新
    onMailDeleted: onDeleted, // 跨标签页删除实时一致
    onError: () => {
      // 连接中断由内部自动重连处理
    },
    onFallback: () => {
      if (typeof extraOptions.onFallback === 'function') {
        extraOptions.onFallback()
      }
    },
  })
}
