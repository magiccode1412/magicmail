# 开发概览

欢迎参与 Magicmail 的开发！本文档将帮助你快速了解项目结构并开始贡献代码。

## 技术栈总览

| 层级 | 技术 | 版本要求 |
|------|------|----------|
| 语言 | Go | >= 1.21 |
| Web 框架 | Fiber v2 | - |
| ORM | GORM | - |
| 数据库 | SQLite (modernc.org/sqlite) | 纯 Go 实现 |
| IMAP 客户端 | go-imap/v2 | - |
| 推送协议 | SSE + Web Push (VAPID) | - |
| 加密 | AES-256-GCM / ECDSA P-256 | - |
| 前端框架 | Vue 3 | Composition API |
| 构建工具 | Vite | 5.x |
| 状态管理 | Pinia | - |
| PWA | vite-plugin-pwa + Workbox | - |

## 项目结构概览

```
magicmail/
├── server/                  # Go 后端
│   ├── main.go              # 入口：初始化 DB / 路由 / VAPID / 启动服务
│   ├── config/              # 配置加载（环境变量）
│   ├── crypto/              # 加密工具（AES-256-GCM, VAPID 密钥管理）
│   ├── database/            # GORM 初始化与自动迁移
│   ├── models/              # 数据模型定义（含 AppConfig VAPID 字段）
│   ├── handlers/            # HTTP 请求处理器（Mail/Draft/Push 等 Handler）
│   ├── services/            # 业务逻辑层（PushService, DraftService 等）
│   ├── routes/              # 路由注册（含 VAPID 初始化）
│   ├── middleware/          # 中间件（CORS / Auth）
│   ├── imap/                # IMAP 连接、拉取、Worker (IDLE + 轮询)
│   ├── pop3/                # POP3 协议支持
│   ├── smtp/                # SMTP 发信支持
│   ├── proxy/               # HTTP CONNECT / SOCKS5 代理客户端
│   ├── sse/                 # SSE 实时推送 Broker
│   ├── notifier/            # Webhook 通知引擎 + Push 回调
│   └── embedfs/             # //go:embed 嵌入前端产物
│
├── web/                     # Vue3 前端
│   ├── src/
│   │   ├── views/           # 页面组件
│   │   ├── components/      # 通用组件
│   │   ├── stores/          # Pinia Store
│   │   ├── api/             # Axios 封装
│   │   ├── router/          # 路由配置
│   │   ├── composables/     # 组合式函数 (useSSE / useWebPush / useUpdateCheck)
│   │   └── styles/          # CSS 变量 / 全局样式
│   └── vite.config.js       # Vite + PWA 配置
│
├── fnapp/                   # 飞牛 fnOS 应用配置
├── scripts/                 # 构建 / 发版 / 部署脚本
├── docs/                    # 本文档站 (VitePress)
└── README.md                # 项目说明
```

## 快速启动开发环境

```bash
# 方式一：一键启动（推荐）
./scripts/dev.sh start

# 方式二：手动启动
cd server && go run .          # 终端 1
cd web && pnpm dev             # 终端 2
```

## 开发规范

### 后端 (Go)
- 遵循分层架构：Handler → Service → Model
- 错误使用统一的 code/message 格式
- 所有公开 API 需要 JWT 鉴权
- 新增 API 端点需同步更新 `docs/api/` 下对应文档

### 前端 (Vue)
- 使用 `<script setup>` 和 Composition API
- 组件命名采用 PascalCase
- 样式使用 CSS 变量，避免硬编码颜色值
- 新增 composable 放置在 `web/src/composables/` 目录

### 新增模块指南
1. **后端新模块**：创建 `server/<module>/` 目录，实现 Service → Handler，在 `routes.go` 注册路由
2. **前端新功能**：创建 View 组件，注册路由，必要时新增 Store 或 Composable
3. **文档更新**：API 变更需更新 `docs/api/`，架构变更需更新 `docs/dev/architecture.md`

## 下一步

- [项目架构](/dev/architecture) - 详细架构说明
- [后端开发](/dev/backend) - Go 后端开发指南
- [前端开发](/dev/frontend) - Vue 前端开发指南
- [发布流程](/dev/release) - 版本发布、tag 规范与 CI/CD
