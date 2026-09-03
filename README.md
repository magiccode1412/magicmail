<div align="center">

<img src="./public/images/icon_512.png" alt="logo" width="128" height="128">

# Magicmail - 魔法邮箱

一套完整的邮件代收系统，基于 **Go (Fiber + GORM + SQLite)** 后端 + **Vue3 PWA** 前端。通过 IMAP 协议代理收取多个邮箱账号的邮件，统一存储至本地数据库，以现代化 PWA 客户端呈现。

[使用文档](https://160621.xyz/magicmail) | [GitHub](https://github.com/magiccode1412/magicmail) | [API 文档](https://160621.xyz/magicmail/api/overview) | [功能特性](https://160621.xyz/magicmail/guide/features) | [安装部署](https://160621.xyz/magicmail/guide/installation)

</div>

## 交流&打赏

<table>
  <tr>
    <td align="center">
      <a href="https://qm.qq.com/q/wWS78gByRa">点此加入QQ群</a>
      <br>
      <img src="./public/images/qq-group.jpg" alt="qq-group" height="256px">
    </td>
    <td align="center">
      <a href="https://pd.qq.com/s/eveskv89x">点此加入QQ频道</a>
      <br>
      <img src="./public/images/qq-channel.jpg" alt="qq-channel" height="256px">
    </td>
    <td align="center">
      <a href="https://pd.qq.com/s/eveskv89x">支付宝</a>
      <br>
      <img src="./public/images/alipay.png" alt="alipay" height="256px">
    </td>
    <td align="center">
      <a href="https://pd.qq.com/s/eveskv89x">微信</a>
      <br>
      <img src="./public/images/wechat.png" alt="wechat" height="256px">
    </td>
  </tr>
</table>

## 快速开始

### 方式一：一键部署（推荐用于服务器）

```bash
# 国内推荐（jsDelivr 加速）
curl -fsSL https://cdn.jsdelivr.net/gh/magiccode1412/magicmail@main/deploy.sh -o magicmail.sh
chmod +x magicmail.sh && sudo ./magicmail.sh install
```

安装后可通过 `magicmail` 命令管理服务：

```bash
magicmail status     # 查看运行状态
magicmail start      # 启动服务
magicmail stop       # 停止服务
magicmail restart    # 重启服务
magicmail update     # 更新到最新版本
magicmail doctor     # 环境健康自检
magicmail uninstall  # 卸载
```

### 方式二：Docker Compose（推荐用于容器环境）

```bash
mkdir -p docker-data
docker compose -f docker-compose.prebuilt.yml up -d
```

> **注意**：请确保以**普通用户身份**运行上述命令（非 root）。容器内进程使用固定 UID=1000 运行，与宿主机普通用户自动匹配，无需手动修改目录权限。如需以 root 运行，请执行 `chown -R 1000:1000 docker-data` 后再启动。

### 方式三：开发模式

```bash
./scripts/dev.sh start
```

> 详细安装教程、环境变量配置、Windows 部署等，请访问：[使用文档 > 安装部署](https://160621.xyz/magicmail/guide/installation)

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21+, Fiber v2, GORM, modernc.org/sqlite (纯 Go, 无 CGO), go-imap/v2 |
| 前端 | Vue 3 Composition API, Vite 5, Pinia, Vue Router |
| PWA | vite-plugin-pwa, Service Worker (Workbox) |
| 实时推送 | SSE (Server-Sent Events), Web Push (VAPID / Web Push Protocol) |
| 样式 | 原生 CSS + CSS 变量主题系统 |

## License

Copyright (C) 2026 [magiccode (魔法代码)](https://github.com/magiccode1412/magicmail)

本程序基于 **AGPLv3** 开源协议发布，网络使用需提供源代码获取方式。

[查看完整协议文本 →](./LICENSE)
