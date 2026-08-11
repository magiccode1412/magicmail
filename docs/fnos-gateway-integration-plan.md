# Magicmail 飞牛统一网关接入计划

> 目标：让 `fnapp`（魔法邮箱）适配飞牛统一网关，**通过飞牛登录应用**，但**必须绑定到应用原有账号**。
> 适用技能：`fnos-developer`；准则遵循 `security-review.md`。

---

## 1. 设计摘要

### 1.1 核心决策（已与用户确认）

| 决策点 | 结论 |
|---|---|
| 绑定策略 | **方案 A（可选登录方式）**：飞牛登录是可选入口；未绑定用户引导「绑定已有账号」或「注册新账号」（1c） |
| 原有账号密码 | **完整保留**，普通 Docker 部署完全不受影响 |
| 用户角色 | **忽略 `X-Trim-Isadmin`**，沿用 magicmail 自身 `role`（admin/user） |
| 权限模型 | **`allUsers=true`**，多用户共存 |
| JS SDK | 本次**不做**（登录打通仅后端读 Header 即可） |
| 平台架构 | 暂不改 |
| SSE 鉴权 | **保留原 JWT**（`Authorization` / `?token=`），不依赖网关 Header |
| 绑定关系 | **一对一**：一个 `fnos_uid` 至多绑一个 magicmail 账号；一个 magicmail 账号仅可被一个 `fnos_uid` 绑定 |
| 监听方式 | 环境变量控制：未设置→TCP（`0.0.0.0:PORT`）；设置（如 `unix://.../app.sock`）→Unix Socket。**飞牛安装走 Socket** |
| 老数据 | **必须手动绑定**：升级后任何现有账号都不会被自动绑定到飞牛身份 |

### 1.2 登录流程图

```
飞牛 NAS 用户 → 统一网关注入 X-Trim-Userid / X-Trim-Username
        │
        ▼
前端 LoginView（检测到网关环境）
   ├─ 已有绑定 → 调 /api/v1/auth/fnos-login → 后端查 fnos_uid → 签发 JWT → 免密进入
   ├─ 未绑定 + 选「注册新账号」→ 调 /api/v1/auth/fnos-register → 建号并绑 fnos_uid → 签发 JWT
   ├─ 未绑定 + 选「绑定已有账号」→ 输入原账号密码 → 校验后写入 fnos_uid → 签发 JWT
   └─ 普通 Docker（无 X-Trim-*）→ 仅显示原账号密码登录/注册
```

### 1.3 安全红线（务必遵守）

- `X-Trim-Userid` 等网关 Header **只在网关前缀路由下信任**；普通 TCP 端口路由即使伪造该 Header 也不读取、不信任。
- 后端信任网关身份的唯一来源是 `X-Trim-Userid`（数字 ID），**绝不信任客户端请求体声明的 uid**。
- 绑定关系写入数据库前，必须校验：该 `fnos_uid` 未被其他账号占用（一对一约束）；绑定已有账号时必须校验原密码。
- JWT 密钥与加密密钥沿用既有 `EnsureSecuritySecrets`，不新增密钥面。

---

## 2. 改动文件清单

### 2.1 `fnapp/` 包配置（飞牛侧）

#### `fnapp/manifest`
- 新增 `gatewayPrefix = /app/magicmail`
- 新增 `gatewaySocket = app.sock`（统一网关转发到 `TRIM_APPDEST` 下的 `app.sock`）
- `app/ui/config` 的 `port` 移除，改为 `gatewayPrefix` + `gatewaySocket`
- `allUsers` 改为 `true`
- `type=iframe`、`ctl_stop` 保留
- （可选）`micro_app=true` 仅当后续要接 JS SDK 时再加，本次不加

#### `fnapp/app/ui/config`
```json
{
    ".url": {
        "magicmail.Application": {
            "title": "魔法邮箱",
            "icon": "images/icon_{0}.png",
            "type": "iframe",
            "gatewayPrefix": "/app/magicmail",
            "gatewaySocket": "app.sock",
            "url": "/",
            "allUsers": true
        }
    }
}
```

#### `fnapp/cmd/main`（启动脚本）
改动要点（保留现有日志/诊断逻辑）：
- 不再使用 `wizard_app_port` 启动 TCP 端口。
- 改为设置 `MAGICMAIL_LISTEN=unix://${TRIM_APPDEST}/app.sock`，并 `rm -f` 旧 socket 后启动。
- 启动后**不再检测 TCP 端口监听**，改为检测 socket 文件存在且可连接（或检测进程存活 + 短暂 curl 健康检查 `/health`）。
- 停止时除 kill 进程外，清理 `app.sock`。
- 保留 Docker 行为：若 `MAGICMAIL_LISTEN` 未设置（Docker 场景），仍走原 TCP 逻辑（向后兼容）。

伪代码改动（仅启动段）：
```bash
SOCK="${TRIM_APPDEST}/app.sock"
export MAGICMAIL_DSN="${DB_PATH}"
export MAGICMAIL_LISTEN="unix://${SOCK}"
rm -f "${SOCK}"
nohup "${APP_BIN}" >> "${LOG_FILE}" 2>&1 &
# 等待进程存活 + socket 文件出现（替代原 ss 端口检测）
```

### 2.2 `server/`（Go 后端）

#### `server/config/config.go`
- `ServerConfig` 新增字段：
  ```go
  Listen string // 完整监听地址："tcp://0.0.0.0:8080"（默认）或 "unix:///path/app.sock"
  ```
- `Load()` 中读取 `MAGICMAIL_LISTEN`：若为空→默认 `tcp://0.0.0.0:PORT`；否则原样使用。
- 保留 `Port`/`Host` 字段以兼容 Docker 路径（未设 `MAGICMAIL_LISTEN` 时拼出 `tcp://HOST:PORT`）。

#### `server/main.go`
- 把 `app.Listen(cfg.Server.Addr())` 替换为按 `cfg.Server.Listen` 分发：
  - 前缀 `unix://` → `app.Listener` + `net.Listen("unix", path)`（启动前删除旧 socket 文件，进程退出时 `defer` 删除）。
  - 前缀 `tcp://` → 原 `app.Listen(addr)`。
- 在 unix socket 模式下，Fiber 不需要 `Host/Port`，但仍保留 `/health` 供网关探活。

#### `server/models/user.go`
- `User` 增加字段：
  ```go
  FnosUID string `json:"-" gorm:"size:64;uniqueIndex;default:'';column:fnos_uid;comment:飞牛用户ID，空表示未绑定"`
  ```
  - `uniqueIndex` 实现一对一约束（数据库层兜底）。
  - SQLite `ALTER TABLE users ADD COLUMN fnos_uid ...` 向后兼容老数据（默认空字符串）。
- `RegisterRequest` 不变；新增 `FnosBindRequest` / `FnosRegisterRequest` DTO（见下）。

#### `server/services/auth_service.go`
新增方法（保持现有 JWT 签发逻辑不变）：

```go
// GetFnosStatus 根据网关用户ID返回绑定状态
//   - 已绑定 → 返回该 magicmail 用户（含可用于自动登录）
//   - 未绑定 → 返回 nil（前端引导绑定/注册）
func (s *AuthService) GetFnosBind(fnosUID string) (*models.User, error)

// BindExistingByFnOS 绑定已有账号：校验密码后写入 fnos_uid（一对一校验）
func (s *AuthService) BindExistingByFnOS(fnosUID, username, password string) (*models.User, error)

// RegisterByFnOS 注册新账号并绑定 fnos_uid（受开放注册/首次管理员逻辑约束）
func (s *AuthService) RegisterByFnOS(fnosUID, username, password string) (*models.User, error)
```

- `BindExistingByFnOS` 必须：`fnos_uid` 全局唯一查重；原账号密码 bcrypt 校验通过才写入。
- `RegisterByFnOS` 复用现有 `Register` 的「首个用户为 admin / 受开放注册开关约束」逻辑，仅多一步写 `fnos_uid`。
- 上述方法最终都调用现有 `GenerateToken` 签发 JWT，复用 `LoginResponse`。

#### `server/middleware/auth.go`
- **不变**：SSE 与原 API 继续用 JWT（`Authorization` / `?token=`）。
- 新增**只读辅助** `GatewayIdentity(c)`：从 `X-Trim-Userid` 读取网关身份，**仅在请求确由网关前缀进入时才调用**（由路由层保证）。普通端口路由的 handler 不调用此函数，从而伪造 Header 无效。
- 不修改 `AuthRequired`（仍强制 JWT），保证「网关仅用于识别是谁，最终仍签发/校验 magicmail 自己的 JWT」。

#### `server/handlers/auth_handler.go` + `server/routes/routes.go`
新增**公开**路由（无需 JWT，但仅在网关环境可用）：

```go
authGroup.Get("/fnos/status", authHandler.FnosStatus)   // 返回 { bound: bool, username?: string }
authGroup.Post("/fnos/bind", authHandler.FnosBind)       // 绑定已有账号 → 返回 JWT
authGroup.Post("/fnos/register", authHandler.FnosRegister) // 注册新账号并绑定 → 返回 JWT
```

- `FnosStatus`：读 `X-Trim-Userid`，查 `fnos_uid`；返回是否绑定 +（已绑定时）用户名。无该 Header 时返回 `{ bound:false, gateway:false }`，前端据此隐藏飞牛登录入口。
- `FnosBind` / `FnosRegister`：从 Header 取 `fnos_uid`（**不信任 body**），调用 service 层，成功返回 `LoginResponse`（含 JWT）。
- 这三个端点放在 `authGroup`（公开），因为它们本身就是登录入口；但内部读取身份**只信 Header**。

### 2.3 `web/`（前端）

#### `web/src/api/auth.js`
- 新增 `fnosStatus()` / `fnosBind(data)` / `fnosRegister(data)` 三个请求封装（走 `/api/v1/auth/fnos/*`）。

#### `web/src/stores/authStore.js`
- `init()` 中额外调用 `fnosStatus()`，把 `gatewayAvailable` / `fnosBound` 状态存入 store，供 `LoginView` 判断显示入口。

#### `web/src/views/LoginView.vue`
- 新增「飞牛账号登录」入口卡片（仅当 `authStore.gatewayAvailable && !authStore.fnosBound` 显示）：
  - 按钮：「使用飞牛账号登录」→ 展开两个子选项：
    - 「注册新账号」：复用现有注册表单，提交时调 `fnosRegister` 并带网关身份（后端从 Header 取 uid）。
    - 「绑定已有账号」：显示用户名+密码表单，调 `fnosBind`。
  - 已绑定（`fnosBound===true`）：显示「以飞牛账号 {username} 登录」按钮，点击调 `fnosBind` 等价流程直接拿 JWT 进入（免密）。
- 原账号密码登录/注册逻辑**完全保留**，作为非飞牛部署与兜底路径。
- Docker 部署（无 `X-Trim-Userid`）下 `gatewayAvailable=false`，不渲染飞牛入口，与原行为一致。

#### `web/src/composables/useSSE.js`
- **不变**：继续用 `?token=JWT`，不依赖网关 Header。

---

## 3. 数据库迁移

- 新增列 `users.fnos_uid`（带 uniqueIndex，默认 `''`）。
- GORM `AutoMigrate` 在启动时自动执行 `ALTER TABLE`，老数据 `fnos_uid` 默认为空 → **无账号被自动绑定**（符合「必须手动绑定」）。
- 一对一靠 uniqueIndex 兜底；service 层在写入前显式查重，给出友好错误（「该飞牛账号已绑定其他 Magicmail 账号」）。

---

## 4. 构建与验收

### 4.1 构建
- 飞牛 FPK：`build_fpk.sh` 不变（前端仍嵌二进制）。`fnapp/cmd/main` 改为传 `MAGICMAIL_LISTEN=unix://...`。
- Docker：不设置 `MAGICMAIL_LISTEN`，仍走 TCP，`allUsers`/`gatewayPrefix` 等 fnOS 专属配置对 Docker 无效，零影响。

### 4.2 后端单测建议（新增）
- `AuthService.GetFnosBind`：未绑定时返回 nil；已绑定时命中。
- `BindExistingByFnOS`：密码错误拒绝；`fnos_uid` 重复占用拒绝；成功后写入。
- `RegisterByFnOS`：首个用户为 admin；开放注册关闭时拒绝普通注册。
- `gatewayIdentity` 伪造 Header 在 TCP 路由下不生效（由路由层保证，集成测试覆盖）。

### 4.3 验收清单
- [ ] fnOS 部署：应用入口走 `/app/magicmail`，后端监听 `TRIM_APPDEST/app.sock`，网关探活 `/health` 正常。
- [ ] 飞牛用户首次打开：显示「使用飞牛账号登录」，引导注册/绑定，绑定后免密进入。
- [ ] 绑定后再次打开：直接以飞牛身份免密登录。
- [ ] 一个 fnOS 用户换绑另一个 magicmail 账号被拒绝（一对一）。
- [ ] 普通 Docker 部署：行为与改造前完全一致，无飞牛入口。
- [ ] SSE 在两种部署下均正常（JWT 鉴权）。
- [ ] 升级老数据：现有账号不自动绑定，需手动走绑定流程。
- [ ] 安全审查：网关 Header 仅在网关路由信任；伪造 `X-Trim-Userid` 在直连端口下无效。

---

## 5. 风险与回滚

- **风险**：`uniqueIndex` 在已有老数据若该列之前以非空重复值存在会失败——但本列为新增默认空，无此风险。
- **回滚**：`fnapp/manifest` 回退 `gatewayPrefix`/`gatewaySocket`/`allUsers`；`cmd/main` 回退 TCP 启动；后端 `MAGICMAIL_LISTEN` 不设置即回退 TCP。前端飞牛入口在 `gatewayAvailable=false` 时自动隐藏，回滚无残留。
- 数据库 `fnos_uid` 列可保留（空值无副作用），不影响回滚后运行。

---

## 6. 实现记录（已完成）

1. ✅ `server/config/config.go` + `server/main.go`：监听方式切换（TCP/Unix）。`MAGICMAIL_LISTEN` 控制；默认 `tcp://HOST:PORT`，设置 `unix://` 走 Socket（启动前删旧 socket、退出 `defer` 删、权限 0666）。
2. ✅ `server/models/user.go`：加 `fnos_uid` 字段（`uniqueIndex`，默认 `''`）+ 迁移；新增 `FnosStatusResponse` / `FnosBindRequest` / `FnosRegisterRequest` DTO。
3. ✅ `server/services/auth_service.go`：加 `GetFnosBind` / `FnosLogin` / `BindExistingByFnOS` / `RegisterByFnOS`，及错误常量 `ErrFnosUIDEmpty` / `ErrFnosAlreadyBound` / `ErrAccountBoundByFnos` / `ErrFnosNotBound`。
4. ✅ `server/middleware/auth.go`：加 `GatewayIdentity` 只读辅助（仅网关路由调用，不信任 body）。
5. ✅ `server/handlers/auth_handler.go` + `routes/routes.go`：加 `/fnos/status`、`/fnos/login`（已绑定免密）、`/fnos/bind`、`/fnos/register` 公开路由。
6. ✅ `web/src/api/auth.js` + `authStore.js`：飞牛状态与登录封装（`fnosStatus` / `fnosLogin` / `fnosBind` / `fnosRegister`；store 加 `gatewayAvailable` / `fnosBound` / `fnosUsername` 及 `doFnos*` 方法）。
7. ✅ `web/src/views/LoginView.vue`：飞牛登录/绑定/注册 UI（已绑定一键免密；未绑定引导注册/绑定；非网关环境自动隐藏）。
8. ✅ `fnapp/manifest`（加 `gatewayPrefix` / `gatewaySocket` / `checkport=false`）+ `fnapp/app/ui/config`（网关模式、`allUsers=true`）+ `fnapp/cmd/main`（Unix Socket 启动与探活、停止清理 socket）。
9. ⏳ 后端单测 + 验收清单核对（待 CI / 真机验收）。

### 实现中新增的端点说明
- 计划原本只列了 `/fnos/bind` 与 `/fnos/register`。实际实现补充了 **`/fnos/login`**：已绑定用户再次打开应用时，前端「以飞牛账号 XXX 登录」按钮调用此端点，后端仅凭 `X-Trim-Userid` 查到绑定账号并签发 JWT，无需任何密码（真正的免密登录）。`/fnos/status` 返回 `bound` 与 `username` 供前端判断是否展示一键登录。
