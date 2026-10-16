# Web Push 浏览器推送

基于 [Web Push Protocol](https://tools.ietf.org/html/rfc8030) 和 VAPID (Voluntary Application Server Identification)，实现浏览器原生推送通知。即使页面关闭或服务端在后台运行，用户也能收到新邮件通知。

## 端点

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/v1/push/vapid-public-key` | 否 | 获取 VAPID 公钥（前端订阅必需）|
| POST | `/api/v1/push/subscribe` | ✅ JWT | 订阅浏览器推送 |
| POST | `/api/v1/push/unsubscribe` | ✅ JWT | 取消订阅 |
| GET | `/api/v1/push/subscriptions` | ✅ JWT | 查看当前用户的订阅列表 |
| POST | `/api/v1/push/test` | ✅ JWT | 发送测试推送 |

## 工作流程

```
1. 前端获取 VAPID 公钥
   GET /api/v1/push/vapid-public-key

2. 用户授权 → 浏览器生成 PushSubscription
   (使用 VAPID 公钥调用 pushManager.subscribe())

3. 前端将 PushSubscription 发送给后端保存
   POST /api/v1/push/subscribe  { endpoint, keys: { p256dh, auth } }

4. 新邮件到达时，后端通过 WebPush 库发送通知到浏览器 Push Service
   (GCM/FCM for Chrome, APS for Safari, etc.)

5. 浏览器收到通知 → 显示桌面通知（即使页面已关闭）
```

## 订阅请求格式

**POST `/api/v1/push/subscribe`**

```json
{
  "endpoint": "https://fcm.googleapis.com/fcm/send/...",
  "keys": {
    "p256dh": "BPC...公钥...",
    "auth": "base64auth..."
  }
}
```

## 响应示例

```json
{ "code": 0, "message": "success", "data": { "id": 42 } }
```

## 取消订阅

**POST `/api/v1/push/unsubscribe`**

```json
{ "endpoint": "https://fcm.googleapis.com/fcm/send/..." }
```

## 测试推送

**POST `/api/v1/push/test`**

向当前用户的所有活跃订阅发送一条测试通知：

```json
{
  "title": "测试推送",
  "body": "这是一条来自 Magicmail 的测试通知",
  "icon": "/icon_192.png"
}
```

## VAPID 密钥管理

VAPID 密钥对用于标识应用服务器来源，确保只有你的服务器能向订阅者发送推送。

- **首次启动时自动生成** ECDSA P-256 密钥对并持久化到数据库
- **支持环境变量覆盖**：
  - `MAGICMAIL_VAPID_PUBLIC_KEY`
  - `MAGICMAIL_VAPID_PRIVATE_KEY`
- **密钥优先级**：环境变量 > 数据库存储值 > 首次启动自动生成

## 客户端集成

项目提供了封装好的 `useWebPush` composable：

```javascript
import { useWebPush } from '@/composables/useWebPush'

const { isSupported, isSubscribed, subscribe, unsubscribe } = useWebPush()

// 检查浏览器是否支持
if (!isSupported.value) {
  console.log('当前浏览器不支持 Web Push')
}

// 切换订阅状态
async function togglePush() {
  if (isSubscribed.value) {
    await unsubscribe()
  } else {
    await subscribe()
  }
}
```

**特性**：
- 自动检测浏览器支持情况
- 处理 Notification 权限请求
- 管理 PushSubscription 生命周期
- 页面加载时自动恢复已有订阅状态

## 兼容性要求

- 浏览器需支持 [Push API](https://developer.mozilla.org/en-US/docs/Web/API/Push_API) 和 [Notifications API](https://developer.mozilla.org/en-US/docs/Web/API/Notifications_API)
- 必须通过 HTTPS 访问（或 localhost）
- 大部分现代浏览器均支持：
  - ✅ Chrome 50+
  - ✅ Firefox 44+
  - ✅ Edge 17+
  - ✅ Safari 16+
  - ❌ IE / 旧版浏览器不支持
