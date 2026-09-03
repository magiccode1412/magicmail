# 快速开始

欢迎来到 Magicmail！本页将帮助你在几分钟内完成部署并开始使用。

## 环境要求

根据你的部署方式，环境要求不同：

| 方式 | 要求 |
|------|------|
| **一键部署（服务器）** | 仅需 bash + 网络，无需任何开发工具（Linux / macOS） |
| **Windows** | 直接下载 `.exe` 运行，或使用 Docker Desktop |
| **源码构建** | Go >= 1.21, Node.js >= 18, pnpm/npm |

---

## 方式一：一键部署（推荐）

最适合在服务器上快速部署，自动完成一切配置：

:::: code-group

```bash [GitHub 原始]
curl -fsSL https://raw.githubusercontent.com/magiccode1412/magicmail/main/deploy.sh -o magicmail.sh
```

```bash [jsDelivr 镜像（国内）]
curl -fsSL https://cdn.jsdelivr.net/gh/magiccode1412/magicmail@main/deploy.sh -o magicmail.sh
```

::::

```bash
chmod +x magicmail.sh && sudo ./magicmail.sh install
```

安装后使用 `magicmail` 命令管理服务：

```bash
magicmail status    # 查看状态
magicmail doctor    # 环境自检
magicmail logs      # 查看日志
```

详细用法见 [安装部署](/guide/installation)。

---

## Windows 用户

Windows 不支持一键部署脚本，推荐以下方式：

**直接运行：**
1. 从 [GitHub Releases](https://github.com/magiccode1412/magicmail/releases) 下载 `magicmail-windows-amd64.exe`
2. 放入目标目录，双击运行即可

**注册为系统服务（开机自启）：**
使用 [NSSM](https://nssm.cc/download) 注册 Windows 服务：
```powershell
nssm install magicmail "C:\magicmail\magicmail-windows-amd64.exe"
nssm start magicmail
```

**Docker Desktop：**
```powershell
docker run -d -p 8080:8080 -v C:\data:C:\data --name magicmail --restart unless-stopped magiccode1412/magicmail:latest
```

完整 Windows 教程见 [安装部署 > Windows](/guide/installation#方式四windows-部署)。

---

## 方式二：源码构建

一键完成前端构建 + Go 编译，输出单二进制文件：

```bash
# 构建当前平台版本
./scripts/build.sh

# 交叉编译 Windows 版本
./scripts/build.sh windows amd64

# 交叉编译 macOS Apple Silicon
./scripts/build.sh darwin arm64

# 清理构建产物
./scripts/build.sh clean
```

构建产物输出到 `bin/magicmail`（或 `bin/magicmail.exe`）。

---

## 方式三：开发环境

前后端同时启动，支持热重载：

```bash
# 一键启动
./scripts/dev.sh start

# 停止
./scripts/dev.sh stop
```

或手动分步启动：

```bash
# 终端 1：启动后端 (端口 8080)
cd server && go run .

# 终端 2：启动前端开发服务器 (端口 5173)
cd web && pnpm install && pnpm dev
```

Vite 开发服务器会自动代理 `/api` 请求到后端。

---

## 启动服务

无论哪种方式，最终都是运行二进制文件：

```bash
./bin/magicmail
```

默认监听 **http://localhost:8080**，浏览器打开即可使用。

::: tip 首次使用
首次启动时会自动：
- 创建 SQLite 数据库 (`data/magicmail.db`)
- 生成 JWT 密钥和加密密钥
- 引导注册管理员账号
:::

---

## 下一步

- [安装部署](/guide/installation) - 了解更多部署选项和 `magicmail` 命令详解
- [功能特性](/guide/features) - 浏览完整功能列表
- [API 文档](/api/overview) - 查看 RESTful 接口说明
- [环境变量](/config/environment) - 配置生产环境参数
