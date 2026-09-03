# Changelog

所有重要变更都会记录在此文件中。

## [v1.2.1] - 2026-09-02

### 安全
- 修复 JWT 以**空密钥**签发与校验：`routes.Register` 内部重复调用 `config.Load()`，而该方法返回的 `Security.JWTSecret` 恒为空串（约定由 `EnsureSecuritySecrets` 填充），导致 `main.go` 中生成的真实密钥从未生效。攻击者可用空密钥伪造任意 `user_id` 的 token 接管任意账号（含管理员）。现由 `main.go` 统一传入已完成初始化的配置
- ⚠️ **破坏性变更**：密钥由空值改为真实值后，所有已签发 token 立即失效，升级后需重新登录一次

### 修复
- 修复飞牛统一网关下**登录后所有 API 全部失效**（响应为 `invalid token`）：统一网关注用了 `Authorization` 头，将其当作飞牛自己的凭证校验，失败时直接代答 `HTTP 200 + 纯文本 invalid token`，请求根本到不了应用。业务 JWT 改走自定义头 `X-Auth-Token`，`Authorization` 保留为回退以兼容 Docker / curl 调试
- 修复登录态无法自愈：`init()` 此前用公开的 `/api/v1/auth/status` 探测 token，而该接口对任何 token 都返回 200，失效 token 永不清除，表现为「已登录但所有数据请求 401」。新增受保护的 `/api/v1/auth/me` 用于探活
- 修复网关代答被当成正常响应消费：网关拒绝时返回 200 + 英文 `invalid token`，前端既不报错也不清 token，会拿该字符串当数据用。现识别为「NAS 会话失效」并**保留应用登录态**（不再误踢用户）
- 修复飞牛自动绑定失败被静默吞掉：绑定结果此前只写入 `c.Locals` 无人消费，无从判断「理论上会自动绑定」是否生效。现成功与失败均落日志
- 修复 Service Worker 缓存带鉴权的 API 响应：Cache API 以 URL 为键、不区分 `Authorization`，切换账号会返回他人数据；已停止缓存 API 响应
- 修复静态资源缺少 `Cache-Control`：`index.html` 被引擎长期缓存后，升级仍会加载旧版前端（其 Vite base 可能与当前部署不一致）。现 `index.html` / `sw.js` / `*.webmanifest` 为 `no-cache`，带内容 hash 的 `assets/*` 长缓存

### 部署 & CI/CD
- 修复 Docker 镜像 `latest` 标签从未更新的问题：浮动标签的启用条件误用了在 tag 推送时恒为 false 的 `is_default_branch`
- 修复 cnb 同步到 GitHub 时不推送标签的问题：`git-sync` 的 `push_tags` 默认为 false，标签全部留在 cnb，导致 GitHub Actions 中 `tags: ['v*']` 的工作流永远不会触发
- 修复在 cnb 推送 tag 不触发同步的问题：`push` 事件只响应分支推送，已单独配置 `tag_push` 事件
- 修复 cnb 镜像构建读取不到版本号的问题：字段应为 `latest` 而非 `version`，此前镜像标签被打成 `:null`
- 修复 `web_trigger_docker_dev` 从未成功执行的问题：其调用的 `scripts/docker.sh` 此前并不存在
- 发布入口新增校验：`v*` tag 必须打在 `main` 分支上，打在其他分支会直接失败
- Release 正文改为从 `docs/guide/changelog.md` 提取对应版本条目，修复正文中混入字面量 `format=`、以及内容严重缺失的问题
- 文档站改用官方 Pages Action 部署，不再依赖 `gh-pages` 分支（原方案因 `publish_dir` 路径错误从未生效过）

### 新增
- dev 分支 push 时自动执行全量构建校验：6 个平台交叉编译、飞牛双架构 FPK 打包、网关前缀与二进制架构断言
- 新增发布流程文档，说明分支模型、版本号同步、tag 归属校验、同步链路与验收清单
- 文档站按构建渠道区分正式版 / 开发版：开发版带顶部横幅、页面标题后缀与 `noindex`，避免未发布内容被当成正式文档或被搜索引擎收录

### 构建
- 修复 `build_fpk.sh` 可能打包到上一轮残留的 `.fpk`：`*.fpk` 的 glob 顺序使 `magicmail-1.2.0-x86.fpk` 先于 `magicmail.fpk` 被命中（`-` 0x2D < `.` 0x2E），又因新旧同名而跳过重命名，导致新包以 `magicmail.fpk` 留在目录、旧包原封不动，极易装错。现打包前先清理历史产物

## [v1.2.0] - 2026-09-02

### 安全
- 飞牛形态改为**仅通过统一网关访问**（只监听 Unix Socket，不再监听 TCP 端口），避免额外的无保护暴露面；需要端口访问请部署 Docker / 独立版本
- Unix Socket 权限收紧为 `0660`，仅应用与网关可访问，防止本机任意进程伪造 `X-Trim-Userid` 免密登录
- 网关身份以**连接来源**为信任边界：只有 Unix Socket 连接才会读取 `X-Trim-Userid`，TCP 伪造 Header 无效

### 重构
- 后端改为**单套根路由**，统一网关透传的 `/app/magicmail` 前缀由 `middleware.BasePath` 在路由匹配前剥离（不再双注册路由）
- 移除飞牛应用「同时开启 TCP 端口」选项与安装配置向导（已无配置项）

### 新增
- 飞牛应用包同时提供 **x86** 与 **ARM64** 两个版本（`magicmail-<版本>-x86.fpk` / `magicmail-<版本>-arm64.fpk`），本地 `./build_fpk.sh [x86|arm64]` 同步支持

### 修复
- 修复 PWA `manifest.webmanifest` 图标路径未拼 base，导致网关下图标 404
- 修复带网关前缀访问时 401 跳转登录页丢失前缀的问题
- 修复 CI 的 `build-fpk` 复用无 `BASE_URL` 前端产物，导致发布的 FPK 在网关下白屏
- 修复版本更新检测：版本比较方向写反，本地版本高于远端时会误报「发现新版本」
- 修复更新提示里版本号显示异常（`vv1.1.1` / 当前版本空白）：统一去掉 `v` 前缀，改用 JS 注入的版本号（Vite `define` 不会替换 Vue 模板中的标识符）
- 修复更新横幅被顶栏遮挡：横幅层级提升到顶栏之上，并让内容区、侧边栏、Toast 在有横幅时整体下移
- 修复「忽略此版本」无效：忽略的版本持久化到 localStorage，刷新/重启不再重复弹出，出现更高版本时仍会重新提示
- 修复登录页底部版本号显示为 `v0.0.0`：版本号统一从 `src/appVersion.js` 取构建注入值


## [v1.1.1] - 2026-07-07

- 新增Outlook OAuth2支持、一键全部标记已读、邮箱服务商品牌图标、CNB平台Docker镜像构建
- 完善Docker权限管理(PUID/PGID)、纯Go SQLite禁用CGO、飞牛fnOS FPK包构建与多架构Docker流水线
- 修复OAuth2 IMAP认证失败、邮件重复入库、企业微信中文乱码(GBK/GB18030)、UI列表项样式等问题


## [v1.1.0] - 2026-07-01

- 新增批量操作(已读/未读)、每页显示数设置、IMAP IDLE自动降级、账号健康检查
- 修复163邮箱发送及CRLF注入漏洞
- UI响应式优化


## [v1.0.2] - 2026-06-16

- 完善移动端下布局错位问题; 
- 完善webhook推送逻辑和模板变量; 
- 修复未读计数器更新不及时不同步问题


## [v1.0.1] - 2026-06-14

### 安全
- 修复 Docker 构建与 GitHub Release 构建产物未注入生产标记的问题，防止运行时打印 SQL 查询日志导致数据泄露

### 部署 & CI/CD
- Docker 数据目录迁移至 /app/data，更新容器内路径配置
- 新增 Docker Compose 编排文件，支持一键编排部署
- GitHub Actions 新增 Docker 多架构镜像自动构建推送 (linux/amd64 + linux/arm64)
- GitHub Actions 新增多架构交叉编译 Release 自动发布 (Linux/Windows/macOS x86_64/ARM64)
- 部署脚本重构：支持交互菜单、系统自检、镜像加速、macOS LaunchDaemon
- 新增 Windows 平台部署支持


## [v1.0.0] - 2026-06-13

### 🆕 核心功能
- **IMAP 邮件代收**：基于 IMAP4rev1 协议，支持 TLS/STARTTLS 加密连接
- **IMAP IDLE 实时推送**：新邮件秒级到达，不支持 IDLE 的服务器自动降级为轮询
- **SMTP 邮件发送**：支持 HTML 正文、附件上传、抄送/密送
- **POP3 协议支持**：兼容仅支持 POP3 的邮箱服务商
- **多账号管理**：支持添加任意数量的 IMAP/POP3 邮箱账号，独立管理同步状态
- **邮件去重**：基于 Message-ID 全局去重，避免重复存储
- **MIME 解析**：完整 multipart 解析，自动字符集转换（UTF-8 / GBK / ISO-8859-*）

### 🔔 通知系统
- **SSE 实时推送**：Server-Sent Events 服务端推送，前端 < 1 秒刷新（含指数退避重连）
- **Web Push 浏览器推送**：基于 VAPID 协议，页面关闭也能收到桌面通知
- **Webhook 外部通知**：自定义 Header/Body 模板，支持变量占位符

### 📎 附件管理
- **混合缓存模式**：小文件立即缓存 + 大文件懒加载按需下载
- **可配置策略**：缓存阈值、磁盘空间保护、过期自动清理
- **流式传输**：大附件从 IMAP 零内存拷贝输出到浏览器

### 🛡️ 安全认证
- **JWT Token 鉴权**：所有 `/api/v1/*` 接口保护
- **AES-256-GCM 加密**：邮箱密码加密存储
- **CORS 跨域控制**：白名单机制

### 🌐 网络代理
- **HTTP/SOCKS5 代理**：按账号独立配置代理
- **全局生效**：IMAP/POP3 收信和 SMTP 发信均通过代理连接

### ✉️ 草稿箱
- **草稿 CRUD**：保存、编辑、删除草稿
- **批量操作**：批量删除草稿

### 💻 部署方式
- **单二进制部署**：前端嵌入 Go 二进制，纯 Go SQLite 无需 CGO
- **跨平台编译**：Linux / macOS / Windows (amd64/arm64)
- **Docker 支持**：提供 Dockerfile 一键容器化
- **systemd 服务**：内置 Linux systemd 服务配置
- **飞牛 fnOS 应用**：支持一键安装到飞牛 NAS 系统

### 📱 前端特性
- **PWA 支持**：可安装到桌面/手机主屏幕，离线缓存访问
- **深色模式**：跟随系统自动切换浅色/深色主题
- **完全响应式**：手机、平板、PC 全尺寸适配
- **版本更新检测**：启动时自动检查新版本并提示用户
