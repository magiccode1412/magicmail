<div align="center">

<img src="./public/images/icon_512.png" alt="logo" width="128" height="128">

# Magicmail - 魔法邮箱

一套完整的邮件代收系统，通过 IMAP 协议代理收取多个邮箱账号的邮件，统一存储至本地数据库，以现代化 PWA 客户端呈现。

  <p>
    </a>
    <a href="https://www.160621.xyz/magicpush-dev/guide/changelog.html" target="_blank">
      <img alt="Latest Version"
        src="https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fraw.githubusercontent.com%2Fmagiccode1412%2Fmagicmail%2Frefs%2Fheads%2Fmain%2Fversion.json&query=%24.latest&prefix=v&style=flat&label=version&labelColor=orange">
    </a>
    <a href="https://hub.docker.com/r/magiccode1412/magicmail" target="_blank">
      <img alt="Docker Pulls"
        src="https://img.shields.io/docker/pulls/magiccode1412/magicpush?labelColor=%20%23528bff&color=%20%23155EEF">
    </a>
    <a href="https://hub.docker.com/r/magiccode1412/magicmail" target="_blank">
      <img alt="Docker Image Size"
        src="https://img.shields.io/docker/image-size/magiccode1412/magicpush">
    </a>
  </p>

[官网](https://magicmail.160621.xyz/) | [GitHub](https://github.com/magiccode1412/magicmail) | [反馈渠道](https://cnb.cool/magiccode1412/magicmail/-/issues)

</div>

## ⚠️注意⚠️
GitHub仓库只用于存储源代码，意见、建议或pr请移步[CNB云原生构建平台](https://cnb.cool/magiccode1412/magicmail)

## 交流&打赏

<table>
  <tr>
    <td align="center">
      支付宝
      <br>
      <img src="./public/images/alipay.webp" alt="alipay" height="256px">
    </td>
    <td align="center">
      微信
      <br>
      <img src="./public/images/wechat.webp" alt="wechat" height="256px">
    </td>
  </tr>
</table>

## 安装使用

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

## 开发

```bash
./scripts/dev.sh start
```

> 详细安装教程、环境变量配置、Windows 部署等，请访问：[使用文档 > 安装部署](https://magicmail.160621.xyz/guide/installation)

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21+, Fiber v2, GORM, modernc.org/sqlite (纯 Go, 无 CGO), go-imap/v2 |
| 前端 | Vue 3 Composition API, Vite 5, Pinia, Vue Router |
| PWA | vite-plugin-pwa, Service Worker (Workbox) |
| 实时推送 | SSE (Server-Sent Events), Web Push (VAPID / Web Push Protocol) |
| 样式 | 原生 CSS + CSS 变量主题系统 |

## License

Copyright (C) 2026 [magiccode (魔法代码)](https://cnb.cool/magiccode1412/magicmail)

本程序基于 **AGPLv3** 开源协议发布，网络使用需提供源代码获取方式。

[查看完整协议文本 →](./LICENSE)
