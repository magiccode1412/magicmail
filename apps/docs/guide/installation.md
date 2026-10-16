# 安装部署

## 方式一：一键部署脚本（推荐）

> 最简单的方式，一条命令完成所有配置。支持 Linux 各发行版、macOS 和 Docker 环境。

### 快速安装

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

> 脚本内置 **GitHub 镜像自动切换**：当直连 `github.com` 失败时，会依次尝试以下镜像加速：
> - **jsDelivr CDN** — 最快最稳定（仅 raw 文件）
> - **mirror.ghproxy.com** — 全类型通用代理
> - **gh-proxy.com** / **ghfast.top** / **github.moeyy.xyz** — 备用镜像
>
> 无需任何额外配置，全程自动。

脚本会自动：
1. 检测系统环境（操作系统、发行版、CPU 架构）
2. 检查系统资源（磁盘空间、内存）
3. 检测端口占用并提示防火墙配置
4. 自动安装缺失的依赖工具
5. 从 GitHub Release 下载对应平台的二进制文件
6. 注册系统服务并设置开机自启
7. 注册 `magicmail` 全局命令
8. 启动服务并输出汇总信息

### 安装选项

```bash
# 指定端口
sudo ./magicmail.sh install --port 3000

# 安装指定版本
sudo ./magicmail.sh install --version v1.0.0

# 非交互模式（跳过所有确认，适合 CI/CD）
sudo ./magicmail.sh install -y

# 组合使用
sudo ./magicmail.sh install --port 9090 --version v1.2.0 -y
```

### magicmail 命令行工具

安装完成后，`magicmail` 命令会注册到 `/usr/local/bin`，可在终端直接使用：

#### 服务管理

```bash
magicmail status       # 查看服务运行状态和关键路径信息
magicmail start        # 启动服务（自动提权 sudo）
magicmail stop         # 停止服务
magicmail restart      # 重启服务
magicmail logs         # 查看最近 100 行日志
magicmail logs 50      # 查看最近 50 行日志
```

#### 维护操作

```bash
magicmail update       # 更新到最新版本（需确认）
magicmail update -y    # 无交互更新
magicmail doctor       # 环境健康自检
magicmail version      # 显示已安装版本和远程最新版对比
magicmail uninstall    # 卸载程序（数据可选保留）
```

#### 帮助信息

```bash
magicmail help         # 查看 CLI 命令帮助
./magicmail.sh help       # 查看完整部署脚本帮助
```

### doctor 自检命令

`magicmail doctor` 会全面检查运行环境的健康状态，涵盖以下维度：

| 检查项目 | 说明 |
|----------|------|
| 操作系统 / 内核版本 | 系统基础信息 |
| CPU 架构检测 | amd64 / arm64 |
| 二进制文件 | 是否存在且可执行 |
| 部署脚本 | 是否存在于安装目录 |
| 版本记录 | 已安装版本号 |
| 数据主目录 | `/var/lib/magicmail` |
| 数据库文件 | SQLite 数据库是否存在及大小 |
| 日志文件 | 是否可读 |
| systemd / LaunchDaemon 服务 | 服务文件及运行状态 |
| 端口监听 | 目标端口是否在监听 |
| 网络连通性 | DNS 可达性 |
| CLI 命令 | `magicmail` 命令是否可用 |
| 磁盘空间 | 可用空间是否充足 |
| 内存 | 总内存是否满足最低要求 |

### 支持的平台

| 平台 | 包管理器 | 服务管理 |
|------|----------|----------|
| Ubuntu / Debian / Linux Mint | apt | systemd |
| CentOS / RHEL / Rocky / Alma / Fedora | dnf / yum | systemd |
| Arch Linux / Manjaro | pacman | systemd |
| Alpine Linux | apk | systemd |
| OpenSUSE / SLES | zypper | systemd |
| macOS (Intel & Apple Silicon) | brew | LaunchDaemon |
| **Windows** | — | NSSM / WinSW / Docker Desktop |
| Docker 容器 | — | nohup 后台模式 |

::: tip Docker 环境说明
脚本会自动检测 Docker 容器环境，此时跳过 systemd 服务创建，改用 nohup 后台运行模式。
:::

### 安装目录结构

```
/opt/magicmail/              # 程序安装目录
├── magicmail                # 主程序二进制
├── deploy.sh               # 部署脚本副本（供 CLI wrapper 调用）
└── .version                # 版本记录文件

/var/lib/magicmail/          # 数据目录
└── data/
    └── magicmail.db        # SQLite 数据库

/var/log/
└── magicmail.log           # 运行日志

/usr/local/bin/
└── magicmail               # CLI 命令（软链/wrapper）

/etc/systemd/system/
└── magicmail.service        # systemd 服务文件（仅 Linux）
```

### 更新与卸载

```bash
# 更新到最新版本
sudo ./magicmail.sh update
# 或
sudo magicmail update

# 卸载（保留数据）
sudo ./magicmail.sh uninstall
# 或
sudo magicmail uninstall

# 彻底卸载（含数据），卸载时选择删除数据即可
```

---

## 方式二：直接下载二进制

在 [GitHub Releases](https://github.com/magiccode1412/magicmail/releases) 下载对应平台的预编译二进制：

| 平台 | 文件 |
|------|------|
| Linux amd64 | `magicmail-linux-amd64` |
| Linux arm64 | `magicmail-linux-arm64` |
| macOS Intel | `magicmail-macos-x86_64` |
| macOS Apple Silicon | `magicmail-macos-arm64` |
| Windows amd64 | `magicmail-windows-amd64.exe` |

下载后赋予执行权限并运行：

```bash
chmod +x magicmail-linux-amd64
./magicmail-linux-amd64
```

---

## 方式三：Docker 部署

### 3.1 Docker Compose（预构建镜像）⭐ 推荐

使用官方预构建镜像，无需本地编译，拉取即用：

```bash
# 1. 创建数据目录（用于持久化）
mkdir -p docker-data

# 2. 使用预构建镜像启动（首次会自动拉取镜像）
docker compose -f docker-compose.prebuilt.yml up -d

# 3. 查看日志
docker compose -f docker-compose.prebuilt.yml logs -f

# 4. 停止服务
docker compose -f docker-compose.prebuilt.yml down

# 5. 停止并删除数据（⚠️ 会删除数据库）
docker compose -f docker-compose.prebuilt.yml down -v
```

> 镜像地址: `magiccode1412/magicmail:latest`
>
> Docker Hub: https://hub.docker.com/r/magiccode1412/magicmail

#### 目录权限说明

容器通过入口脚本自动处理数据目录权限，**所有场景下均无需手动设置**：

| 运行场景 | 是否需要额外操作 | 说明 |
|---------|-----------------|------|
| **普通用户运行** | ✅ 无需操作 | 入口脚本自动 `chown` 修正归属 |
| **Dockhand / Unraid 等 NAS** | ✅ 无需操作 | 完美解决 NAS 管理器创建的目录权限问题 |
| **Windows Docker Desktop** | ✅ 无需操作 | 自动处理权限映射 |
| **Mac Docker Desktop** | ✅ 无需操作 | 自动处理权限映射 |
| **Root 用户运行** | ✅ 无需操作 | 入口脚本自动修正后降权运行 |

::: tip 权限原理
容器启动时以 **root 身份进入入口脚本**，通过 `chown` 自动修正 bind-mount 目录（`./docker-data`）的归属，然后通过 `su-exec` 以 `PUID:PGID`（默认 1000:1000）降权运行。这样无论宿主机目录归谁所有、由谁创建，都能自动适配。

如果希望宿主机的数据文件归属特定用户，可通过 `PUID` 和 `PGID` 环境变量指定（在宿主执行 `id 用户名` 获取 UID/GID）。
:::

#### 故障排查：数据库连接失败

如果遇到错误 `unable to open database file: out of memory(14)`，这是 SQLite 对**权限错误**的误报。通常容器入口脚本已自动处理，如仍出现请检查：

```bash
# 1. 确认入口脚本日志（查看 chown 和降权信息）
docker compose -f docker-compose.prebuilt.yml logs | grep entrypoint

# 2. 如需要，指定与宿主机匹配的 PUID/PGID（默认为 1000:1000）
#    在 .env 或环境变量中设置：
#    PUID=1000
#    PGID=1000

# 3. 重启容器
docker compose -f docker-compose.prebuilt.yml restart
```

> 更多排查思路：确保**未在 dockhand 等管理器中将容器运行用户强制设为非 root**（入口脚本需要 root 才能 `chown`）。

#### 环境变量配置（可选）

可通过 `.env` 文件或直接在命令中设置环境变量：

```bash
# 创建 .env 文件（可选）
cat > .env << 'EOF'
PUID=1000
PGID=1000
MAGICMAIL_PORT=8080
TZ=Asia/Shanghai
MAGICMAIL_MEMORY_LIMIT=512M
MAGICMAIL_MEMORY_RESERVATION=128M
EOF
```

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PUID` | `1000` | 容器内运行用户的 UID（入口脚本自动 `chown` 数据目录至此用户） |
| `PGID` | `1000` | 容器内运行用户的 GID |
| `MAGICMAIL_PORT` | `8080` | 映射到宿主机的端口 |
| `TZ` | `Asia/Shanghai` | 容器时区 |
| `MAGICMAIL_MEMORY_LIMIT` | `512M` | 内存上限 |
| `MAGICMAIL_MEMORY_RESERVATION` | `128M` | 内存预留 |

### 3.2 Docker Compose（源码构建）

项目根目录提供了基于源码构建的 `docker-compose.yml`，适合需要自定义修改的场景：

```bash
# 1. 复制环境变量模板（可选，按需修改端口等配置）
cp .env.example .env

# 2. 构建并启动
docker compose up -d --build

# 3. 查看日志
docker compose logs -f

# 4. 停止服务
docker compose down

# 5. 停止并删除数据（⚠️ 会删除数据库）
docker compose down -v
```

数据持久化在 `./docker-data/` 目录。可通过 `.env` 文件修改端口、时区、资源限制等配置。

### 3.3 手动 docker run（预构建镜像）

```bash
# 拉取镜像
docker pull magiccode1412/magicmail:latest

# 运行（入口脚本自动处理数据目录权限，通过 PUID/PGID 控制运行用户）
docker run -d \
  -p 8080:8080 \
  -v ./docker-data:/app/data \
  -e PUID=1000 \
  -e PGID=1000 \
  --name magicmail \
  --restart unless-stopped \
  magiccode1412/magicmail:latest
```

### 3.4 手动 docker run（源码构建）

```bash
docker build -t magicmail .
docker run -d \
  -p 8080:8080 \
  -v ./data:/app/data \
  -e PUID=1000 \
  -e PGID=1000 \
  --name magicmail \
  --restart unless-stopped \
  magicmail
```

也可以通过一键部署脚本在容器内安装（自动适配 Docker 环境）。

---

## 方式四：Windows 部署

> Windows 不支持一键部署脚本，推荐直接下载预编译二进制或使用 Docker Desktop。

### 方式 A：直接运行二进制（最简）

1. 前往 [GitHub Releases](https://github.com/magiccode1412/magicmail/releases) 下载 `magicmail-windows-amd64.exe`
2. 将 `.exe` 放入目标目录（如 `C:\magicmail\`）
3. 双击或在 PowerShell / CMD 中运行：

```powershell
# PowerShell / CMD
.\magicmail-windows-amd64.exe
```

程序启动后访问 `http://localhost:8080` 即可。默认情况下数据库和附件保存在程序同目录下。

### 方式 B：注册为 Windows 服务（开机自启）

使用 **NSSM (Non-Sucking Service Manager)** 将 Magicmail 注册为系统后台服务：

```powershell
# 1. 下载 NSSM: https://nssm.cc/download
#    解压后得到 nssm.exe（win64 目录）

# 2. 注册服务
nssm install magicmail "C:\magicmail\magicmail-windows-amd64.exe"

# 3. （可选）配置工作目录和数据目录 — 在 NSSM GUI 中设置：
#    IIS > Application → Working directory = C:\magicmail
#    IIS > AppExit = Restart（崩溃自动重启）

# 4. 启动服务
nssm start magicmail

# --- 常用管理命令 ---
nssm stop magicmail          # 停止
nssm restart magicmail       # 重启
nssm status magicmail        # 查看状态
nssm remove magicmail confirm # 卸载服务（confirm 确认删除）
```

> 💡 **替代方案**：也可使用 [WinSW](https://github.com/winsw/winsw)（XML 配置式）注册服务，适合需要更复杂配置的场景。

### 方式 C：Docker Desktop

如果已安装 Docker Desktop for Windows，推荐使用预构建镜像（无需本地编译）：

```powershell
# 方法一: 使用 docker-compose.prebuilt.yml（推荐）
cp .env.example .env
docker compose -f docker-compose.prebuilt.yml up -d

# 方法二: 手动 docker run
docker pull magiccode1412/magicmail:latest
docker run -d `
  -p 8080:8080 `
  -v ./docker-data:/app/data `
  --name magicmail `
  --restart unless-stopped `
  magiccode1412/magicmail:latest
```

如需从源码构建（开发/自定义场景）：

```powershell
cp .env.example .env
docker compose up -d --build
```

### Windows 路径说明

| 项目 | 默认路径 |
|------|----------|
| 工作目录 | 可执行文件所在目录 |
| 数据库 | `%CD%\magicmail.db` |
| 日志 | 控制台输出（服务模式写入 Windows 事件日志） |
| 监听端口 | `8080` |

可通过环境变量修改默认值：

```powershell
# CMD
set MAGICMAIL_PORT=3000
set MAGICMAIL_DSN=C:\data\magicmail.db
.\magicmail-windows-amd64.exe

# PowerShell
$env:MAGICMAIL_PORT=3000
$env:MAGICMAIL_DSN="C:\data\magicmail.db"
.\magicmail-windows-amd64.exe
```

---

## 方式五：systemd 手动配置（Linux）

如果需要手动配置 systemd 服务，可参考以下步骤：

```bash
# 复制服务文件
sudo cp apps/server/magicmail.service /etc/systemd/system/

# 根据实际路径修改 ExecStart
sudo systemctl daemon-reload
sudo systemctl enable magicmail
sudo systemctl start magicmail
```

或使用一键部署脚本自动完成以上全部步骤。

---

## 反向代理配置

### Nginx

```nginx
server {
    listen 80;
    server_name mail.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE 长连接支持（实时推送必需）
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }
}
```

### Caddy

```
mail.example.com {
    reverse_proxy localhost:8080

    # SSE 长连接支持
    flush_interval -1
}
```

::: tip SSE 代理注意事项
Magicmail 使用 SSE (Server-Sent Events) 实现实时邮件推送。反向代理必须禁用响应缓冲，否则前端无法收到实时推送。
:::
