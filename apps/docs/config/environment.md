# 环境变量参考

所有环境变量均为可选，不设置则使用默认值。

可通过以下方式设置：

```bash
# 方式一：命令行导出
export MAGICMAIL_PORT=9090
./magicmail

# 方式二：.env 文件（放置在程序运行目录下）
echo "MAGICMAIL_PORT=9090" > .env
./magicmail
```

## 运行模式

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MAGICMAIL_ENV` | 空（开发模式） | 设为 `production` 关闭 SQL 日志并启用 release 模式 |

## 服务配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MAGICMAIL_PORT` | `8080` | HTTP 监听端口 |
| `MAGICMAIL_HOST` | `0.0.0.0` | HTTP 监听地址 |
| `MAGICMAIL_DSN` | `data/magicmail.db` | SQLite 数据库文件路径（相对于运行目录）|
| `MAGICMAIL_CORS_ORIGINS` | 允许所有来源 | CORS 白名单（逗号分隔）|

**CORS 示例**：

```bash
# 允许多个域名
export MAGICMAIL_CORS_ORIGINS="https://app.example.com,https://web.example.com"

# 允许所有（默认行为）
export MAGICMAIL_CORS_ORIGINS="*"
```

## IMAP 同步配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MAGICMAIL_POLL_INTERVAL` | `60`（1 分钟）| IMAP 定时轮询间隔（秒），需大于 10 秒 |
| `MAGICMAIL_IDLE_HEARTBEAT` | `0`（跟随 `MAGICMAIL_POLL_INTERVAL`）| IDLE 期间兜底同步间隔（秒），最低 60 秒，见下方说明 |
| `MAGICMAIL_IDLE_ENABLED` | `true` | 启用 IMAP IDLE 实时推送（设为 `false` 或 `0` 关闭）|
| `MAGICMAIL_MAX_CONCURRENT` | `10` | IMAP 最大并发连接数 |
| `MAGICMAIL_SYNC_BATCH_SIZE` | `50` | 每次同步拉取邮件数量上限 |

### `MAGICMAIL_IDLE_HEARTBEAT` 说明（僵尸 IDLE 兜底）

IDLE 依赖服务器主动推送。但存在一类“僵尸连接”：TCP 连接看起来完全正常（既不报错也不断开），服务器却从不推送新邮件通知。此时客户端无法感知异常，只能干等到 25 分钟的 IDLE 重启窗口。

为覆盖该场景，Worker 在 IDLE 期间会按固定节奏额外主动同步一次，即**心跳兜底**：

- 使用**独立连接**拉取，不中断当前 IDLE 长连接、也不需要重连，代价仅是一次普通的收信往返；
- 未显式配置时跟随 `MAGICMAIL_POLL_INTERVAL`；
- 下限 **60 秒**，避免把 IDLE 退化成高频轮询。

同理，若连接被服务端正常关闭（FIN/RST），go-imap 的 `Wait()` 会立即返回，Worker 走“退避 → 重连 → 连续失败 4 次后降级为轮询”流程，不会等到超时。

## 附件缓存配置（混合模式）

> 以下变量控制附件的缓存策略：**小文件立即缓存 + 大文件懒加载按需下载**。
> 单位统一为 **MB**，直接填数字即可。

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MAGICMAIL_AUTO_CACHE` | `false` | 启用**自动缓存**（懒加载的附件在用户首次下载后异步缓存到本地）|
| `MAGICMAIL_CACHE_THRESHOLD` | `2` | 缓存阈值（MB）。小于此值的附件同步时**立即缓存**；大于此值的仅存元数据，用户下载时从 IMAP 按需获取 |
| `MAGICMAIL_MIN_DISK_FREE` | `1024` | 最小保留磁盘空间（MB）。剩余空间低于此值时停止缓存新附件 |
| `MAGICMAIL_MAX_ATTACHMENT_SIZE` | `50` | 单附件大小上限（MB）。超过此值的附件直接跳过不处理 |
| `MAGICMAIL_CACHE_EXPIRE_DAYS` | `30` | 缓存过期天数。超期未访问的缓存将被自动清理释放空间 |

### 自动缓存工作流程

```
用户点击下载附件
       ↓
检查 MAGICMAIL_AUTO_CACHE == true ?
   ├─ NO → 直接流式传输给用户（不缓存）
   └─ YES → 检查条件
             ├─ 已缓存？→ 跳过（直接返回本地）
             ├─ 超过 CACHE_THRESHOLD？→ 跳过（仅流式传输）
             ├─ 超过 MAX_ATTACHMENT_SIZE？→ 跳过
             ├─ 磁盘空间不足？→ 跳过 + 日志警告
             └─ ✅ 全部通过 → io.TeeReader 边传边写
                              ├─ HTTP 响应（用户立即收到文件）
                              └─ 异步写入本地磁盘（下次秒传）
```

### 配置示例

```bash
# 生产环境推荐（30GB VPS）
export MAGICMAIL_AUTO_CACHE=true          # 开启自动缓存
export MAGICMAIL_CACHE_THRESHOLD=5        # 5MB 以下立即缓存
export MAGICMAIL_MIN_DISK_FREE=5120       # 保留 5GB 空间
export MAGICMAIL_MAX_ATTACHMENT_SIZE=100  # 最大允许 100MB
export MAGICMAIL_CACHE_EXPIRE_DAYS=30     # 30 天清理

# 小机器保守模式
export MAGICMAIL_AUTO_CACHE=false         # 不缓存，纯懒加载
export MAGICMAIL_CACHE_THRESHOLD=0        # 所有附件都懒加载
export MAGICMAIL_MIN_DISK_FREE=512        # 保留 512MB
```

::: tip 提示
- 默认 `MAGICMAIL_AUTO_CACHE=false`，即**不开启自动缓存**，所有大附件都是纯懒加载模式，不占用额外磁盘空间
- 只有明确设置 `MAGICMAIL_AUTO_CACHE=true` 才会启用首次下载后的自动缓存功能
- 懒加载模式下，每次下载都需要临时建立 IMAP 连接（~100ms 开销），但对大多数场景可忽略
:::

## 安全密钥

> 以下密钥**默认自动生成**并持久化到数据库，通常无需手动配置。
> 如需在多实例间共享密钥或确保重启后密钥不变，可通过环境变量显式指定（优先级高于数据库存储值）。

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MAGICMAIL_JWT_SECRET` | 自动生成 | JWT 签名密钥 |
| `MAGICMAIL_ENCRYPT_KEY` | 自动生成 | AES-256-GCM 加密密钥（用于加密存储邮箱密码）|

### 密钥加载优先级

```
环境变量 > 数据库存储值 > 首次启动自动生成
```

1. **首次启动**：若未设置环境变量，系统自动生成随机密钥并存入数据库
2. **后续启动**：
   - 未设置环境变量 → 从数据库读取
   - 设置了环境变量且与数据库不同 → 使用环境变量值，并同步更新数据库

::: warning 生产部署建议
生产环境建议显式设置 `MAGICMAIL_JWT_SECRET` 和 `MAGICMAIL_ENCRYPT_KEY`，避免因数据库丢失导致无法解密已存储的邮箱密码。密钥长度建议 ≥ 32 字符。
:::

## 日志与诊断

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MAGICMAIL_LOG_LEVEL` | 生产 `info`，开发 `debug` | 日志级别。设为 `debug` 会输出大量 `[DEBUG]` 调试日志（含邮件解析、IMAP FETCH、MIME 解码等细节）；设为其他值（如 `info`）则在生产环境默认关闭 `[DEBUG]` 日志，避免日志膨胀与敏感调用链外泄 |
| `MAGICMAIL_DIAG` | `false` | 启动诊断脚本是否向 `diag.log` 输出**完整环境变量**。设为 `1` 或 `true` 时才会打印全部环境变量（用于深度排障）；默认仅打印**脱敏后**的环境变量（自动过滤含 `SECRET`/`KEY`/`TOKEN`/`PASSWORD`/`CREDENTIAL`/`PRIVATE` 的变量）|

::: warning 安全提示
- 默认情况下启动诊断日志**不会**输出 `MAGICMAIL_JWT_SECRET`、`MAGICMAIL_ENCRYPT_KEY` 等密钥（已自动脱敏）。只有显式设置 `MAGICMAIL_DIAG=1` 才会打印完整环境变量，请谨慎使用并确保诊断日志不被外传。
- 生产环境建议保持 `MAGICMAIL_LOG_LEVEL` 为非 `debug`（默认即为关闭），仅在临时排查时开启。
:::

## 完整示例

```bash
#!/bin/bash
# production.env

# 运行模式
export MAGICMAIL_ENV=production

# 监听配置
export MAGICMAIL_HOST=0.0.0.0
export MAGICMAIL_PORT=8080

# 数据库
export MAGICMAIL_DSN=/app/data/magicmail.db

# CORS（按需调整）
export MAGICMAIL_CORS_ORIGINS="https://mail.yourdomain.com"

# IMAP 同步调优
export MAGICMAIL_POLL_INTERVAL=180
export MAGICMAIL_MAX_CONCURRENT=20

# 安全密钥（生产环境必设！）
export MAGICMAIL_JWT_SECRET="your-super-secret-jwt-key-here"
export MAGICMAIL_ENCRYPT_KEY="your-32-byte-encryption-key-1234567890"

# 日志与诊断（可选，按需开启）
# export MAGICMAIL_LOG_LEVEL=debug   # 临时排查时开启，日常保持关闭（默认已关闭）
# export MAGICMAIL_DIAG=1            # 仅需在 diag.log 中查看完整环境变量排障时开启
```

```bash
source production.env && ./magicmail
```
