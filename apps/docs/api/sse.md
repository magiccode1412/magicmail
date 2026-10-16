# SSE 实时推送

基于 Server-Sent Events (SSE) 的服务端推送机制，实现邮件更新的亚秒级前端通知。

## 端点

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/v1/mails/stream` | ✅ JWT | SSE 邮件更新流 |
| GET | `/api/v1/mails/stream/health` | ✅ JWT | SSE 服务健康检查 |

## 工作原理

```
IMAP Worker (syncOnce)
    ↓ 发现新邮件
SSE Broker (发布-订阅)
    ↓ 广播事件
所有在线客户端 (EventSource)
```

### 服务端组件

- **Broker** (`apps/server/sse/broker.go`)：事件广播中心，维护在线连接池
- **Handler** (`apps/server/sse/handler.go`)：HTTP 端点处理器，管理连接生命周期

### 防阻塞机制

- 广播通道满时丢弃旧事件（防止阻塞 Worker）
- 客户端响应慢时自动断开（避免内存泄漏）
- 15 秒心跳保活连接

## 事件类型

| 事件名 | 触发时机 | 数据结构 |
|--------|----------|----------|
| `connected` | 连接建立时 | `{client_id, server_time, online_count}` |
| `mail.received` | 新邮件到达时 | `{account_id, account_email, mail_count, mails[...], timestamp}` |
| `mail.synced` | 邮件同步完成时 | `{account_id, account_email, timestamp}` |
| `heartbeat` | 每 15 秒 | `{time}` |

## 客户端集成

### JavaScript 原生

```javascript
const token = 'YOUR_JWT_TOKEN'
const es = new EventSource(`/api/v1/mails/stream?token=${token}`)

es.addEventListener('connected', (e) => {
  console.log('SSE 已连接', JSON.parse(e.data))
})

es.addEventListener('mail.received', (e) => {
  const data = JSON.parse(e.data)
  alert(`收到 ${data.mail_count} 封新邮件!`)
  // 刷新邮件列表
})

es.addEventListener('mail.synced', (e) => {
  // 同步完成后刷新统计
})
```

### curl 测试

```bash
# 获取 Token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

# 监听 SSE 流
curl -N http://localhost:8080/api/v1/mails/stream?token=$TOKEN
```

### 内置 Composable（Vue 项目）

项目提供了封装好的 `useSSE` composable：

```javascript
import { useMailStream } from '@/composables/useSSE'

useMailStream(() => {
  console.log('收到更新事件，刷新列表')
  mailStore.fetchMails()
  mailStore.fetchStats()
})
```

**特性**：
- 组件挂载时自动连接，卸载时自动断开
- 指数退避重连策略（最多 10 次，最大 30 秒间隔）
- JWT Token 通过 URL 参数传递
- 15 秒心跳保持连接活跃

## 兼容性

| 浏览器 | 支持 |
|--------|------|
| Chrome 6+ | ✅ |
| Firefox 6+ | ✅ |
| Safari 5+ | ✅ |
| Edge (全部版本) | ✅ |
| IE | ❌ 不支持 |

> **降级方案**：当浏览器不支持 SSE 或连接失败时，前端自动降级为 120 秒定时轮询。
