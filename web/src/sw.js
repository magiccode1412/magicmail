// Magicmail Service Worker
// 处理 Web Push 推送事件 + Workbox 预缓存

import { precacheAndRoute } from 'workbox-precaching'
import { registerRoute } from 'workbox-routing'
import { StaleWhileRevalidate, CacheFirst } from 'workbox-strategies'
import { ExpirationPlugin } from 'workbox-expiration'

// --- 预缓存声明（由 vite-plugin-pwa injectManifest 注入） ---
precacheAndRoute(self.__WB_MANIFEST)

// --- Push 事件：收到服务端推送时显示系统通知 ---
self.addEventListener('push', (event) => {
  let data = { title: 'Magicmail', body: '您有新消息', icon: '/icons/icon-192x192.png' }

  if (event.data) {
    try {
      data = event.data.json()
    } catch (_) {
      data.body = event.data.text()
    }
  }

  const options = {
    body: data.body || '您有新消息',
    icon: data.icon || '/icons/icon-192x192.png',
    badge: '/icons/icon-72x72.png',
    vibrate: [200, 100, 200],
    tag: data.tag || 'magicmail-mail',
    requireInteraction: true,
    data: data.data || {},
    actions: [
      { action: 'view', title: '📬 查看邮件', icon: '/icons/icon-72x72.png' },
      { action: 'close', title: '关闭' },
    ],
  }

  event.waitUntil(
    self.registration.showNotification(data.title || 'Magicmail', options)
  )
})

// --- Notification Click/Action：点击通知或按钮时处理 ---
self.addEventListener('notificationclick', (event) => {
  event.notification.close()

  // 点击"关闭"按钮则不执行任何操作
  if (event.action === 'close') {
    return
  }

  const targetUrl = '/mails'

  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      // 如果已有打开的窗口，聚焦到它并可选导航到 /mails
      for (const client of clientList) {
        if ('focus' in client) {
          // 如果当前窗口不在 /mails，导航过去
          if (!client.url.includes('/mails')) {
            client.navigate(targetUrl)
          }
          return client.focus()
        }
      }
      // 没有已打开的窗口则打开新窗口
      if (self.clients.openWindow) {
        return self.clients.openWindow(targetUrl)
      }
    })
  )
})

// --- 运行时缓存策略 ---

// API 缓存（排除 SSE 流）
registerRoute(
  ({ url }) => url.pathname.startsWith(import.meta.env.BASE_URL + 'api') && !url.pathname.includes('/mails/stream'),
  new StaleWhileRevalidate({
    cacheName: 'api-cache',
    plugins: [
      new ExpirationPlugin({ maxEntries: 100, maxAgeSeconds: 5 * 60 }),
    ],
  })
)

// 图片缓存
registerRoute(
  ({ request }) => request.destination === 'image',
  new CacheFirst({
    cacheName: 'image-cache',
    plugins: [
      new ExpirationPlugin({ maxEntries: 50, maxAgeSeconds: 30 * 24 * 60 * 60 }),
    ],
  })
)
