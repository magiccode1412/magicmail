/**
 * useWebPush - Web Push 离线推送 composable
 *
 * 完整流程：
 *   1. 检测浏览器是否支持 Service Worker + Push API
 *   2. 获取 VAPID 公钥
 *   3. 注册/获取 SW registration
 *   4. pushManager.subscribe() → 得到 PushSubscription
 *   5. POST /api/v1/push/subscribe 存储到后端
 *
 * 使用方式：
 *   const { supported, subscribing, isSubscribed, error, subscribe, unsubscribe } = useWebPush()
 */

import { ref, computed } from 'vue'
import * as pushApi from '@/api/push'

export function useWebPush() {
  const isSubscribed = ref(false)
  const subscribing = ref(false)
  const error = ref(null)
  let currentEndpoint = null

  // 浏览器是否支持完整的 Web Push 能力
  const supported = computed(() => {
    return ('serviceWorker' in navigator) &&
           ('PushManager' in window) &&
           ('Notification' in window)
  })

  /** 获取或注册 Service Worker（带超时保护） */
  async function getSWRegistration() {
    // 先检查是否已有激活的 SW
    if (navigator.serviceWorker.controller) {
      return await navigator.serviceWorker.getRegistration()
    }

    // 用 race 防止 ready 永远挂起
    return await Promise.race([
      navigator.serviceWorker.ready,
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error('ServiceWorker 就绪超时')), 5000)
      ),
    ])
  }

  /**
   * 订阅 Web Push 完整流程
   * @returns {boolean} 是否成功订阅
   */
  async function subscribe() {
    if (!supported.value) {
      error.value = '您的浏览器不支持离线推送（需要 Service Worker + Push API 支持）'
      return false
    }

    subscribing.value = true
    error.value = null

    try {
      // Step 1: 请求 Notification 权限
      console.log('[WebPush] Step 1: 请求通知权限...')
      const permission = await Notification.requestPermission()
      if (permission !== 'granted') {
        error.value = permission === 'denied'
          ? '通知权限已被拒绝，请在浏览器设置中手动开启'
          : '未获得通知授权'
        subscribing.value = false
        return false
      }
      console.log('[WebPush] ✓ 权限已授权')

      // Step 2: 获取 VAPID 公钥
      console.log('[WebPush] Step 2: 获取 VAPID 公钥...')
      const vapidPublicKey = await pushApi.getVapidPublicKey()
      if (!vapidPublicKey) {
        throw new Error('无法获取 VAPID 公钥')
      }
      console.log('[WebPush] ✓ VAPID 公钥已获取, 长度:', vapidPublicKey.length)

      // Step 3: 注册/获取 Service Worker
      let reg
      try {
        reg = await getSWRegistration()
        console.log('[WebPush] ✓ SW 已就绪:', reg.scope)
      } catch (swErr) {
        console.warn('[WebPush] SW 未就绪，尝试注册:', swErr.message)

        if (import.meta.env.DEV) {
          throw new Error('开发模式不支持 Web Push 注册（需要构建后的 Service Worker），请在生产环境测试')
        }

        reg = await navigator.serviceWorker.register('/sw.js')
        console.log('[WebPush] ✓ SW 已注册:', reg.scope)

        if (reg.installing) {
          await new Promise((resolve) => {
            const sw = reg.installing
            sw.addEventListener('statechange', function handler(e) {
              console.log('[WebPush] SW 状态:', e.target.state)
              if (e.target.state === 'activated') {
                sw.removeEventListener('statechange', handler)
                resolve()
              }
            })
          })
        }
      }

      // Step 4: 如果存在旧订阅（VAPID key 可能已变更），先清除
      console.log('[WebPush] Step 4: 检查并清除旧订阅...')
      const existingSub = await reg.pushManager.getSubscription()
      if (existingSub) {
        console.log('[WebPush] 发现旧订阅，正在清除...')
        await existingSub.unsubscribe()
        console.log('[WebPush] ✓ 旧订阅已清除')
        // 同步通知后端删除
        try {
          await pushApi.unsubscribe(existingSub.endpoint)
        } catch (_) {
          /* 后端删除失败不阻塞 */
        }
      }

      // Step 5: 订阅 Push
      console.log('[WebPush] Step 5: 订阅 Push...')
      // 将 base64url VAPID 公钥转换为 Uint8Array
      const applicationServerKey = urlBase64ToUint8Array(vapidPublicKey)
      const subscription = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey,
      })
      console.log('[WebPush] ✓ Push 已订阅, endpoint:', subscription.endpoint)

      // Step 6: 发送到后端存储
      console.log('[WebPush] Step 6: 发送订阅到后端...')
      const payload = {
        endpoint: subscription.endpoint,
        keys: {
          p256dh: subscription.getKey('p256dh')
            ? arrayBufferToBase64(subscription.getKey('p256dh'))
            : '',
          auth: subscription.getKey('auth')
            ? arrayBufferToBase64(subscription.getKey('auth'))
            : '',
        },
        user_agent: navigator.userAgent,
      }

      await pushApi.subscribe(payload)

      // 成功！
      currentEndpoint = subscription.endpoint
      isSubscribed.value = true
      subscribing.value = false

      console.log('[WebPush] ✓ 订阅成功')
      return true

    } catch (err) {
      console.error('[WebPush] 订阅失败:', err)
      error.value = err.message || '订阅失败'
      subscribing.value = false
      return false
    }
  }

  /**
   * 取消 Web Push 订阅
   */
  async function unsubscribe() {
    if (!currentEndpoint) {
      // 尝试从已有的 SW registration 中找到并取消
      try {
        const reg = await getSWRegistration()
        const sub = await reg.pushManager.getSubscription()
        if (sub) {
          await sub.unsubscribe()
          currentEndpoint = sub.endpoint
        }
      } catch (_) {
        // SW 不可用时直接标记为已取消
      }
    }

    if (currentEndpoint) {
      try {
        await pushApi.unsubscribe(currentEndpoint)
      } catch (err) {
        console.warn('[WebPush] 后端取消失败（本地已取消）:', err)
      }
    }

    isSubscribed.value = false
    currentEndpoint = null
    error.value = null
    console.log('[WebPush] ✗ 已取消订阅')
  }

  /**
   * 初始化：检查是否已经订阅过
   */
  async function init() {
    if (!supported.value) return

    try {
      const reg = await getSWRegistration()
      const sub = await reg.pushManager.getSubscription()
      if (sub) {
        isSubscribed.value = true
        currentEndpoint = sub.endpoint
        console.log('[Push] 已有活跃订阅')
      }
    } catch (_) {
      // SW 可能还没注册，静默忽略
    }
  }

  return {
    supported,
    subscribing,
    isSubscribed,
    error,
    subscribe,
    unsubscribe,
    init,
  }
}

// --- 工具函数 ---

/**
 * 将 base64url 字符串转换为 Uint8Array
 * @param {string} base64String
 * @returns {Uint8Array}
 */
function urlBase64ToUint8Array(base64String) {
  const PAD_CHAR = "="
  const padding = PAD_CHAR.repeat((4 - base64String.length % 4) % 4)
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/")
  const rawData = atob(base64)
  const output = new Uint8Array(rawData.length)
  for (let i = 0; i < rawData.length; ++i) {
    output[i] = rawData.charCodeAt(i)
  }
  return output
}

/**
 * 将 ArrayBuffer 转换为 base64 字符串
 * @param {ArrayBuffer} buffer
 * @returns {string}
 */
function arrayBufferToBase64(buffer) {
  if (!buffer) return ''
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (const b of bytes) {
    binary += String.fromCharCode(b)
  }
  return btoa(binary)
}
