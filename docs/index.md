---
layout: home

hero:
  name: Magicmail
  text: 魔法邮箱
  tagline: 基于 IMAP 协议的统一邮件管理平台，一站式收取、管理多个邮箱
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/getting-started
    - theme: alt
      text: API 文档
      link: /api/overview
    - theme: alt
      text: GitHub
      link: https://github.com/magiccode1412/magicmail

features:
  - icon: 🚀
    title: 一键部署
    details: 提供一键部署脚本 (magicmail.sh)，自动识别平台、安装依赖、下载二进制、注册系统服务和全局 CLI 命令，支持 Linux/macOS/Docker。
  - icon: 📥
    title: IMAP 代收
    details: 通过 IMAP 协议代理收取多个邮箱账号的邮件，支持 TLS 连接、全量/增量同步，自动去重存储到本地数据库。
  - icon: ⚡
    title: 实时推送
    details: 后台常驻 IMAP IDLE 监听，新邮件到达即时推送到 PWA 客户端和 Webhook 外部服务。
  - icon: 📱
    title: PWA 客户端
    details: 现代化渐进式 Web 应用，支持安装到桌面/主屏幕、深色模式、完全响应式布局。
  - icon: 🔒
    title: 安全可靠
    details: JWT 认证鉴权、AES 加密密码存储、CORS 跨域保护，安全密钥支持环境变量传入或首次启动自动生成。
  - icon: 🔌
    title: Webhook 通知
    details: 新邮件到达时自动推送通知到外部服务，支持自定义 Header/Body，适用于自动化工作流集成。
  - icon: 🛠️
    title: CLI 管理
    details: 安装后提供 magicmail 全局命令，支持 status/start/stop/restart/update/logs/doctor/uninstall 等子命令。
  - icon: 📦
    title: 单二进制部署
    details: 前端产物嵌入 Go 二进制，纯 Go SQLite 驱动无需 CGO，支持交叉编译至 Linux/macOS/Windows。
---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21+, Fiber v2, GORM, modernc.org/sqlite, go-imap/v2 |
| 前端 | Vue 3 Composition API, Vite 5, Pinia, Vue Router |
| PWA | vite-plugin-pwa, Service Worker (Workbox) |
| 样式 | 原生 CSS + CSS 变量主题系统 |

## 快速体验

:::: code-group

```bash [一键部署（GitHub）]
curl -fsSL https://raw.githubusercontent.com/magiccode1412/magicmail/main/deploy.sh -o magicmail.sh
chmod +x magicmail.sh && sudo ./magicmail.sh install
```

```bash [一键部署（jsDelivr 国内加速）]
curl -fsSL https://cdn.jsdelivr.net/gh/magiccode1412/magicmail@main/deploy.sh -o magicmail.sh
chmod +x magicmail.sh && sudo ./magicmail.sh install
```

```bash [源码构建]
./scripts/build.sh
```

```bash [开发模式]
./scripts/dev.sh start
```

::::

服务启动后访问 **http://localhost:8080** 即可使用。安装后可通过 `magicmail` 命令管理服务：

```bash
magicmail status     # 查看状态
magicmail doctor     # 环境自检
magicmail update     # 一键更新
```
