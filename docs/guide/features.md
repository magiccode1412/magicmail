# 功能特性

## 后端服务

### IMAP 收信
- 集成 `go-imap/v2`，完整支持 IMAP4rev1 协议
- TLS / STARTTLS 加密连接
- 全量同步 + 增量同步（基于 UID）
- 基于 Message-ID 全局去重

### 实时监听
- 后台常驻协程运行 [IMAP IDLE](https://tools.ietf.org/html/rfc2177) 推送
- 新邮件到达即时通知 PWA 客户端
- 不支持 IDLE 的服务器自动降级为定时轮询

### 多邮箱管理
- 支持添加任意数量的 IMAP 邮箱账号
- 每个邮箱独立同步状态和错误日志
- 支持手动触发同步、测试连接

### 邮件解析
- 完整 MIME multipart 解析
- 自动字符集转换（UTF-8 / GBK / ISO-8859-*）
- HTML / 纯文本双格式存储
- 附件提取与原始保留

### Webhook 通知
- 新邮件到达时推送 HTTP 回调
- 自定义请求 Header 和 Body 模板
- 支持变量占位符（发件人、主题、摘要等）
- 推送日志记录与查看

### 代理支持
- 支持 HTTP CONNECT 和 SOCKS5 代理
- 按账号独立配置，IMAP/POP3 收信和 SMTP 发信均通过代理
- 代理开关可随时启停，无需删除账号重建

### 安全认证
- JWT Token 鉴权（所有 `/api/v1/*` 接口）
- 用户注册 / 登录
- AES-256-GCM 加密存储邮箱密码
- CORS 跨域白名单控制

## 前端客户端

### PWA 支持
- 可安装到桌面 / 手机主屏幕
- Service Worker 离线缓存
- App Manifest 配置

### 深色模式
- 跟随系统自动切换
- 手动切换浅色 / 深色主题
- CSS 变量驱动的主题系统

### 响应式布局
- 手机、平板、PC 全尺寸适配
- 触摸友好的交互设计
- 侧边栏自适应折叠

### 现代化 UI
- 玻璃态（Glassmorphism）侧边栏
- 流畅的过渡动画
- 毛玻璃模糊效果
- 可定制主题色

### 邮件翻译

- 基于 [MyMemory](https://mymemory.translated.net/) 翻译服务，邮件详情页一键翻译
- **后端代理**调用，凭据加密存储、不暴露前端，统一处理配额与缓存
- 源语言自动检测，目标语言在「设置 → 翻译设置」中按用户独立配置
- 长邮件自动分块（≤ 500 字符/次）并合并译文，译文与原文并列展示
- 详细使用说明见 [翻译功能文档](./translation.md)

## 技术亮点

| 特性 | 说明 |
|------|------|
| 单二进制部署 | 前端通过 `//go:embed` 嵌入 Go 二进制，无需额外静态文件 |
| 零 CGO | 使用 `modernc.org/sqlite` 纯 Go 实现，交叉编译无依赖 |
| 跨平台 | 一套代码编译至 Linux / macOS / Windows |
| 高性能 | Fiber 框架 + 协程并发，轻松处理数千邮件 |
| 实时推送 | SSE < 1 秒到达 + Web Push 浏览器原生通知 |
| 混合附件缓存 | 小文件立即缓存 + 大文件懒加载，灵活的磁盘空间策略 |
| 代理支持 | HTTP CONNECT / SOCKS5 代理，按账号独立配置 |

## 浏览器推送

### Web Push (VAPID)

基于 Web Push Protocol 和 VAPID 标准，即使关闭 Magicmail 页面，浏览器也能收到新邮件通知：

- 自动生成 ECDSA P-256 VAPID 密钥对
- 支持 Chrome (GCM/FCM)、Safari (APS)、Firefox 等主流浏览器
- 内置 `useWebPush` composable，一行代码集成订阅管理

## 草稿箱

- 邮件撰写过程中随时保存为草稿
- 支持富文本 HTML 正文和附件
- 批量删除已保存的草稿
- 可直接从草稿发送邮件
