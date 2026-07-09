---
name: multi-user-management
overview: 为 Magicmail 添加多用户管理：User 模型增加角色(admin/normal，首个注册为 admin，其后为 normal)；每用户数据隔离；新增"开放注册"开关(默认关闭，仅管理员可见，存入 AppConfig)；管理员可后台查看/创建/删除用户(含关联数据)。涉及后端模型/服务/Handler/中间件/路由改造及前端设置页与登录页适配。
design:
  architecture:
    framework: vue
  styleKeywords:
    - Glassmorphism
    - 渐变
    - 卡片式布局
    - 开关控件
    - 管理员专属
  fontSystem:
    fontFamily: PingFang SC, system-ui, -apple-system
    heading:
      size: 20px
      weight: 600
    subheading:
      size: 16px
      weight: 600
    body:
      size: 14px
      weight: 400
  colorSystem:
    primary:
      - "#4F6EF7"
      - "#6366F1"
      - "#06B6D4"
    background:
      - "#0F172A"
      - "#1E293B"
      - "#FFFFFF"
    text:
      - "#F1F5F9"
      - "#94A3B8"
      - "#1E293B"
    functional:
      - "#22C55E"
      - "#DC2626"
      - "#F59E0B"
todos:
  - id: backend-models-db
    content: 后端模型新增 Role/OpenRegistration/user_id 字段，并在 database.go 完成历史数据归属迁移
    status: completed
  - id: backend-auth-logic
    content: 实现 auth_service 角色分配、注册开关校验、管理员用户 CRUD 与开放注册配置读写，JWT 增加 role
    status: completed
    dependencies:
      - backend-models-db
  - id: backend-data-isolation
    content: 各业务 service/handler 按 user_id 过滤隔离数据，IMAP Worker 与 SSE 携带并过滤 user_id
    status: completed
    dependencies:
      - backend-models-db
  - id: backend-admin-routes
    content: 新增 user_handler/settings_handler、AdminRequired 中间件并注册管理员路由
    status: completed
    dependencies:
      - backend-auth-logic
      - backend-data-isolation
  - id: frontend-auth-state
    content: 前端 authStore 解析存储 isAdmin，auth.js 适配 open_registration 状态
    status: completed
    dependencies:
      - backend-auth-logic
  - id: frontend-login
    content: 登录页按开放注册状态展示注册入口并区分初始化管理员/普通注册文案
    status: completed
    dependencies:
      - frontend-auth-state
  - id: frontend-settings
    content: 设置页新增“用户与权限”区块：开放注册开关、用户列表与添加/删除用户（仅管理员可见）
    status: completed
    dependencies:
      - frontend-auth-state
      - backend-admin-routes
---

## 用户需求

为 Magicmail 邮件代收系统新增多用户管理能力，将原有单用户（仅一个管理员）模式升级为支持多账号的权限体系，并实现按用户隔离数据。

## 产品概述

后端由单用户模式改造为“管理员 + 普通用户”双角色体系；首个注册的账号自动成为管理员，之后注册的账号为普通用户。新增“开放注册”系统开关（默认关闭，仅管理员可见可配置）。开启后，公开注册页允许他人自助注册为普通用户；关闭时仅禁用公开自助注册，管理员仍可在后台手动添加/删除用户。所有邮件数据（邮箱账号、邮件、草稿、Webhook、推送订阅）严格按用户隔离，各用户仅能查看与操作自己拥有的数据。

## 核心特性

- 角色体系：User 模型新增 Role 字段（admin/user），首个注册用户为 admin，后续为 user；JWT 携带 role，中间件提供 AdminRequired 校验。
- 开放注册开关：存入 AppConfig（默认 false）；AuthStatus 返回该值供前端判断是否展示注册入口；后端 /auth/register 在“已有用户且未开放注册”时拒绝，确保开关为唯一公开自助注册控制点。
- 管理员用户管理：查看用户列表、后台手动创建用户（不受开关限制，默认普通用户）、删除用户（级联删除其邮箱账号/邮件/附件/草稿/Webhook/推送订阅）；禁止删除自身。
- 数据隔离：mail_accounts / mails / webhooks 新增 user_id 并从 JWT 透传过滤；drafts / push_subscriptions 已有 user_id 需确保按用户过滤；IMAP 同步 Worker 写入邮件时携带 owner user_id；SSE 推送按当前连接用户过滤。
- 历史数据迁移：将旧版单用户库中所有 user_id=0 记录归属到唯一管理员；管理员账号 Role 置为 admin。
- 前端：登录页根据开放注册状态展示注册入口并区分“初始化管理员/创建账号”文案；设置页新增“用户与权限”区块（仅管理员可见），含开放注册开关、用户列表与添加/删除用户操作。

## 技术栈

- 后端：Go 1.21 + Fiber v2 + GORM + modernc.org/sqlite（保持现有栈）
- 前端：Vue 3 Composition API + Pinia + Vue Router + 原生 CSS 变量主题（保持现有栈，不引入组件库）
- 鉴权：JWT（HS256，7 天有效期），在 claims 中增加 role

## 实现方案

### 总体策略

在现有分层架构（Handler → Service → Model）上扩展，新增 Role 与 OpenRegistration 配置，把每个受保护接口从 `c.Locals("user_id")` 取出并透传给 Service 进行 `WHERE user_id = ?` 过滤。新增 AdminRequired 中间件保护用户管理与配置接口。注册逻辑改为：count==0 或 open_registration==true 时允许；首注册为 admin。

### 关键技术决策

1. **开放注册配置存于 AppConfig**：复用现有单行配置表，新增 `OpenRegistration bool`（默认 false），在 `EnsureSecuritySecrets` 创建首行时初始化，避免新增配置表与迁移复杂度。
2. **user_id 直接落地到每张数据表**：mails/webhooks 新增 user_id 列并建立索引；mail_accounts 已规划 user_id。查询统一加 `Where("user_id = ?", uid)`，避免跨表 JOIN，降低 IMAP/SSE 高频路径开销。
3. **IMAP Worker 写入带 user_id**：同步落库时从 `account.UserID` 取值写入 `mail.user_id`，保证后台拉取的邮件归属正确，无需事后回填。
4. **前端 isAdmin 由 JWT 解析**：避免额外请求；AuthStatus 仅补充非敏感的 open_registration 布尔。
5. **删除用户级联**：在 user_service/handler 内用事务删除该用户全部子资源，防止孤儿数据。

### 性能与可靠性

- 所有 user_id 查询列加索引，列表分页保持原有 `Limit/Offset`。
- 历史迁移为一次性启动期操作（AutoMigrate 后执行），仅对 user_id=0 行更新，成本可控。
- SSE 推送增加 user_id 过滤，避免把 A 用户邮件推给 B 用户。
- 开放注册校验在后端强制（不仅前端隐藏），防止越权注册。

```mermaid
flowchart TD
    A[客户端] -->|JWT| B[AuthRequired 中间件]
    B -->|user_id, role| C[受保护 Handler]
    C -->|uid| D[Service 层 Where user_id=?]
    D --> E[(SQLite 按用户隔离)]
    B -->|role=admin| F[AdminRequired]
    F --> G[/admin/users, /settings/*]
    G --> H[用户管理/开放注册配置]
    R[/auth/register] -->|open_registration?| I{允许?}
    I -->|是| J[创建 user/admin]
    I -->|否| K[403 拒绝]
```

## 实现要点（防回归）

- 复用现有 `c.Locals("user_id")` 约定，所有受保护 Handler 新增从 locals 取 uid 并传给 Service，保持与现有 `accountService` 调用一致。
- `GenerateToken` 仅新增 `role` claim，`ParseToken` 不变，不影响现有令牌解析。
- SSE（sse 包）需读取连接用户 identity 并按 user_id 过滤事件，避免改动破坏现有实时推送。
- 迁移函数对 `mail_accounts/mails/webhooks/drafts/push_subscriptions` 中 user_id=0 的行 UPDATE 归属到 admin 用户；mails.user_id 通过 `UPDATE mails SET user_id = (SELECT user_id FROM mail_accounts WHERE id = mails.account_id)` 回填。
- 日志复用 `log.Printf`，不打印密码/令牌。

## 架构设计

保持现有 Handler/Service/Model 分层；新增 `user_handler.go`、`settings_handler.go` 与 `AdminRequired` 中间件；`AuthService` 承担角色分配、注册开关校验、用户 CRUD；各业务 Service 增加 `userID` 参数做数据隔离。前端 `authStore` 增加 `isAdmin`，`SettingsView` 增加管理员专属区块，新增 `users.js`/`settings.js` API。

## 目录结构与文件清单

```
server/
├── models/
│   ├── user.go            # [MODIFY] User 增加 Role 字段；新增 UserResponse/AdminCreateUserRequest；IsAdmin() 辅助
│   ├── app_config.go      # [MODIFY] AppConfig 增加 OpenRegistration bool（默认 false）
│   ├── mail_account.go    # [MODIFY] MailAccount 增加 UserID（索引）；AccountListDTO 增加 user_id
│   ├── mail.go            # [MODIFY] Mail 增加 UserID（索引）
│   ├── webhook.go         # [MODIFY] Webhook 增加 UserID（索引）
│   ├── draft.go           # [MODIFY] 确认 UserID 索引（已存在字段）
│   └── push_subscription.go # [MODIFY] 确认 UserID 索引（已存在字段）
├── services/
│   ├── auth_service.go    # [MODIFY] Register 角色分配+开关校验；AdminCreateUser/ListUsers/DeleteUser/GetOpenRegistration/SetOpenRegistration；GenerateToken 加 role；GetAuthStatus 加 open_registration；SeedDefaultUser 置 admin
│   ├── account_service.go # [MODIFY] List/GetByID/Create/Update/Delete/SetStatus 按 user_id 过滤与归属
│   ├── mail_service.go    # [MODIFY] 列表/统计/增删按 user_id 过滤；落库写入 account.UserID
│   ├── webhook_service.go # [MODIFY] 全部按 user_id 过滤与归属
│   ├── draft_service.go   # [MODIFY] 确认按 user_id 过滤（已有字段）
│   └── push_service.go    # [MODIFY] 确认按 user_id 过滤（已有字段）
├── handlers/
│   ├── auth_handler.go    # [MODIFY] Register 透传；Status 返回 open_registration；新增管理员创建/删除/列表入口（或转 user_handler）
│   ├── user_handler.go    # [NEW] 管理员：用户列表 GET、创建 POST、删除 DELETE（级联）
│   ├── settings_handler.go# [NEW] GET/PUT 开放注册开关（仅管理员）
│   ├── account_handler.go # [MODIFY] 从 locals 取 user_id 传给 service
│   ├── mail_handler.go    # [MODIFY] 同上
│   ├── draft_handler.go   # [MODIFY] 同上
│   ├── webhook_handler.go # [MODIFY] 同上
│   ├── attachment_handler.go # [MODIFY] 校验附件所属邮件属于当前用户
│   └── push_handler.go    # [MODIFY] 同上
├── middleware/
│   └── auth.go            # [MODIFY] 写入 role 到 locals；新增 AdminRequired 中间件
├── routes/
│   └── routes.go          # [MODIFY] 注册 /admin/users、/settings/open-registration；公开 /auth/register 受开关约束；受保护接口注入 user_id
└── database/
    └── database.go        # [MODIFY] EnsureSecuritySecrets 初始化 OpenRegistration=false；新增迁移函数归属历史 user_id=0 数据到 admin

web/
├── src/
│   ├── stores/authStore.js   # [MODIFY] 解析并存储 role / isAdmin
│   ├── api/auth.js           # [MODIFY] status 返回 open_registration
│   ├── api/users.js          # [NEW] 用户列表/创建/删除 API
│   ├── api/settings.js       # [NEW] 开放注册开关获取/设置 API
│   ├── views/LoginView.vue   # [MODIFY] canRegister = setupRequired || open_registration；注册文案区分初始化管理员/普通注册
│   └── views/SettingsView.vue# [MODIFY] 新增“用户与权限”区块（v-if=isAdmin）：开放注册开关+用户列表+添加/删除用户
```

## 关键代码结构

```
// models/user.go —— 角色字段与辅助
type User struct {
    ID           uint      `json:"id" gorm:"primaryKey"`
    Username     string    `json:"username" gorm:"uniqueIndex;size:64;not null"`
    PasswordHash string    `json:"-" gorm:"size:255;not null"`
    Role         string    `json:"role" gorm:"size:16;not null;default:'user'"` // "admin" | "user"
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}
func (u User) IsAdmin() bool { return u.Role == "admin" }

// services/auth_service.go —— 核心方法签名
func (s *AuthService) Register(req models.RegisterRequest) error                                  // 首注册=admin；否则需 open_registration
func (s *AuthService) AdminCreateUser(username, password, role string) (*models.UserResponse, error)
func (s *AuthService) ListUsers() ([]models.UserResponse, error)
func (s *AuthService) DeleteUser(id uint) error                                                   // 事务级联删除
func (s *AuthService) GetOpenRegistration() (bool, error)
func (s *AuthService) SetOpenRegistration(enabled bool) error

// middleware/auth.go —— 管理员中间件
func AdminRequired() fiber.Handler // 校验 c.Locals("role")=="admin"，否则 403
```

在现有设置页中新增“用户与权限”卡片区块（仅管理员可见），沿用项目既有的玻璃拟态/渐变设计语言与 CSS 变量主题系统（--primary-500、--bg-secondary、--text-primary 等）。区块包含：开放注册开关（复用现有 .toggle-switch 样式）、用户列表（复用 .webhook-card 风格的行布局，展示用户名/角色徽章/删除按钮）、添加用户表单（复用现有 .form-grid 弹窗风格）。登录页注册模式增加“创建账号”普通注册文案（非初始化时），视觉沿用现有注册表单。整体保持深色/浅色主题自适应，不引入新组件库。