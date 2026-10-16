# 项目架构

## 整体架构图

```
┌──────────────────────────────────────────────┐
│                   Client                      │
│         Browser / PWA (Vue3 + Vite)           │
│                                              │
│  ┌─────────┐ ┌──────────┐ ┌──────────────┐   │
│  │  Views   │→│ Stores   │→│ Axios API     │   │
│  │(页面)     │ │(Pinia)   │ │(REST Client) │   │
│  └─────────┘ └───────────┘ └──────┬───────┘   │
│                                       │        │
│  ┌──────────┐ ┌───────────────────┐  │        │
│  │ useSSE    │ │ useWebPush        │  │        │
│  │ (实时推送) │ │ (浏览器推送)      │  │        │
│  └────┬─────┘ └────────┬──────────┘  │        │
│       │ EventSource     │ Push API      │        │
└───────┼─────────────────┼──────────────┼────────┘
        │ HTTP/SSE         │ Push Subscribe│ HTTP/JSON
┌───────▼─────────────────▼──────────────▼────────┐
│                    Server (Go/Fiber)              │
│                          ▼                        │
│  ┌──────────┐  ┌───────────┐  ┌────────────┐     │
│  │ Routes   │→│ Middleware │→│  Handlers   │     │
│  │(路由注册) │  │(CORS/Auth)│  │(HTTP 处理器)│     │
│  └──────────┘  └───────────┘  └─────┬──────┘     │
│                                      │            │
│                              ┌───────▼──────┐     │
│                              │   Services    │     │
│                              │ (业务逻辑层)   │     │
│                              └───────┬──────┘     │
│               ┌──────────────────────┼─────┐      │
│               ▼                     ▼       │      │
│  ┌──────────────────┐  ┌─────────────────┐  │      │
│  │  IMAP Worker     │  │  Notifier       │  │      │
│  │  · 连接池管理     │  │  · Webhook 推送 │  │      │
│  │  · IDLE 监听     │  │  · Push 回调    │  │      │
│  │  · 定时轮询      │  │                 │  │      │
│  │  · 邮件解析      │  │                 │  │      │
│  └────┬─────────────┘  └────────┬────────┘  │      │
│       │                         │            │      │
│  ┌────▼──────────┐  ┌──────────▼─────────┐  │      │
│  │ POP3 / SMTP   │  │  SSE Broker        │  │      │
│  │·POP3 收信     │  │·发布-订阅广播中心   │  │      │
│  │·SMTP 发信     │  │·心跳保活           │  │      │
│  └────┬──────────┘  └──────────┬─────────┘  │      │
│       │                        │            │      │
│  ┌────▼──────────┐  ┌──────────▼─────────┐  │      │
│  │ Proxy Client  │  │ Crypto / VAPID      │  │      │
│  │·HTTP CONNECT  │  │·AES-256-GCM 加密   │  │      │
│  │·SOCKS5 代理   │  │·ECDSA P-256 密钥   │  │      │
│  └────┬──────────┘  └──────────┬─────────┘  │      │
│       │                        │            │      │
│  ┌────▼────────────────────────▼────────────┘      │
│  │  SQLite (GORM)                                  │
│  │  modernc.org/sqlite (纯Go)                      │
│  └─────────────────────────────────────────────────┘
```

## 后端分层详解

### Routes（路由层）
`apps/server/routes/` - 定义 URL 路径与 Handler 的映射关系，组织 API 版本结构（含 VAPID 密钥初始化）。

### Middleware（中间件）
`apps/server/middleware/`
- **CORS**：跨域请求处理，支持白名单配置
- **Auth**：JWT Token 解析与鉴权，保护 `/api/v1/*` 接口

### Handlers（处理器层）
`apps/server/handlers/` - 接收 HTTP 请求，解析参数，调用 Service，返回 JSON 响应。
包含 MailHandler、AccountHandler、AuthHandler、AttachmentHandler、WebhookHandler、DraftHandler、PushHandler。

### Services（业务逻辑层）
`apps/server/services/` - 核心业务逻辑，包括：
- 用户认证（注册 / 登录 / Token 管理）
- 邮箱账号 CRUD
- 邮件操作（标记、搜索、删除、批量删除、发送）
- 草稿管理（CRUD / 批量删除）
- 附件管理（含懒加载 / 混合缓存）
- **PushService**：Web Push 订阅管理与推送发送
- **VAPID 密钥管理**：ECDSA P-256 密钥对自动生成与持久化

### IMAP 模块
`apps/server/imap/` - IMAP 邮件收取核心
- **client.go**：IMAP 连接建立与管理
- **fetcher.go**：邮件拉取与 MIME 解析
- **worker.go**：后台调度协程（IDLE + 轮询 + SSE 事件发布）

### POP3 模块
`apps/server/pop3/` - POP3 协议支持（自动降级模式）

### SMTP 模块
`apps/server/smtp/` - SMTP 发信支持（HTML 正文、附件、抄送/密送）

### Proxy 模块
`apps/server/proxy/` - HTTP CONNECT / SOCKS5 代理客户端，按账号独立配置

### SSE 模块
`apps/server/sse/` - Server-Sent Events 实时推送
- **broker.go**：事件广播中心（发布-订阅模式、心跳保活）
- **handler.go**：HTTP 端点处理器

### Crypto 模块
`apps/server/crypto/` - 加密/解密工具
- AES-256-GCM 对称加密（邮箱密码存储）
- VAPID ECDSA P-256 密钥对管理

### Notifier（通知引擎）
`apps/server/notifier/` - 异步 HTTP 回调推送（Webhook）+ Push 回调集成。

### Models（数据模型）
`apps/server/models/` - GORM 模型定义，对应数据库表结构（含 AppConfig VAPID 字段）。

## 前端架构详解

### 视图层 (`views/`)
页面级组件：MailList、MailDetail、Settings、Compose 等。

### 组件层 (`components/`)
可复用 UI 组件：AppSidebar、MailItem、SearchBar 等。

### 状态管理 (`stores/`)
Pinia Store：
- **authStore**：用户认证状态
- **mailStore**：邮件数据与筛选条件
- **accountStore**：邮箱账号管理
- **appStore**：全局 UI 状态（主题、侧边栏）

### 组合式函数 (`composables/`)
- **useSSE.js**：SSE 实时推送连接管理（指数退避重连、心跳保活）
- **useWebPush.js**：Web Push 浏览器推送订阅管理（权限请求、生命周期）
- **useUpdateCheck.js**：版本更新检测
- **useToast.js**：全局 Toast 通知

### API 层 (`api/`)
Axios 实例封装，统一拦截器处理 Token 注入与错误响应。

### 路由 (`router/`)
Vue Router 配置，包含路由守卫（未登录跳转）。

## 数据流

```
用户操作 → View dispatch → Store action → API 请求 → Handler
                                                              ↓
SSE/Push ← useSSE/useWebPush ← 事件监听 ← Broker/PushService   │
                                                              ↓
响应 ← View 更新 ← Store state ← 数据转换 ← Service ← Model ← DB
```
