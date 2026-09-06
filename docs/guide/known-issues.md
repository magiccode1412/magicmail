# 已知问题

记录用户反馈的问题及处理进度。


## 问题列表

### 点击「立即同步」无反应，终端日志完全静默

- **状态**：✅ 已修复未发布
- **记录时间**：2026-09-06
- **问题描述**：添加企业微信邮箱（腾讯企业邮）后点击「立即同步」，接口返回 `200`，但此后终端不再输出任何日志，前端一直停留在同步中。典型日志片段：

  ```
  2026/09/06 13:15:34 /workspace/server/services/account_service.go:337
  [0.180ms] [rows:1] SELECT * FROM `mail_accounts` WHERE user_id = 1 AND `mail_accounts`.`id` = 1 ...
  13:15:34 | 200 |     647.375µs | 127.0.0.1 | POST | /api/v1/accounts/1/sync | -
  ```

- **排查过程**（两个关键判据）：
  1. 请求仅耗时 **647µs** 就返回 200 —— 说明 `AccountService.TriggerSync` 只是投递了一个唤醒信号便返回，同步在后台异步执行，接口本身没有阻塞，符合预期；
  2. 此后**一条 SQL 都没有** —— 而 `syncOnce()` 的第一步就是 `db.First(&fresh, account.ID)`（必然产生一条 GORM 日志）。没有这条日志 ⇒ `syncOnce` 根本没被调用 ⇒ **Worker 当时不在任何监听 `wakeCh` 的 select 上**。

  由此把范围从「同步逻辑有问题」缩小到「唤醒信号没送达 / 送达了但 Worker 正在阻塞 IO 中」。
- **根因分析**（三种可能，前两种与协议无关）：
  1. **Worker 正卡在一次同步里**（最常见）：`syncOnce` 全程没有任何日志，而企业邮邮箱邮件量大时，`fetcher.syncMailbox` 会先 FETCH 全部 envelope，再对**每封**新邮件单独 FETCH 正文与附件，首次全量同步可持续数十分钟。期间手动同步信号只能在缓冲为 1 的 `wakeCh` 中排队，等本轮结束后才执行，且界面无任何进度反馈；
  2. **Worker 已退出但仍残留在 Worker 表中**：`Run()` 返回后 `WorkerPool.workers` 不会自动清理该项，`WakeWorker` 只判断「表中是否存在」便返回 `true`，`Wake()` 投递的信号进入**无人消费**的通道 —— 表现为永久无响应、零日志；
  3. **POP3 账号被服务端吊住**：`pop3` 客户端此前完全没有超时（`net.Dial` 无 `Timeout`、`textproto` 读取无 deadline），服务端建立 TCP 后不响应时读取会永久阻塞，Worker 卡死在 `syncOnce` 内既不报错也不退出。
- **修复方案**：
  1. **唤醒可靠性**：`AccountWorker` 新增 `doneCh`，`Run()` 返回时关闭；`WakeWorker` 投递前先做存活判定，识别到已退出的 Worker 即从表中移除并返回 `false`，让 `TriggerSync` 回退为 `RestartWorker`（原来只能干等）；
  2. **唤醒可观测**：`Run()` 主循环中三个唤醒分支（IDLE / 轮询 / IDLE 退避）全部补日志 `🔔 收到手动同步请求`，`TriggerSync` 区分两条路径输出 `🔔 已唤醒 Worker 执行同步` 与 `🔔 账号无运行中的 Worker，回退为重启同步`。**该日志是否出现，是判断信号有没有送达 Worker 的分水岭**；
  3. **同步过程可观测**：`syncOnce` 增加 `🔄 开始同步` / `🏁 同步结束（含耗时）`；`fetcher` 的 FETCH 循环每 50 封输出 `⏳ 同步进度 (邮箱): 已扫描 x/y 封，新增 z 封`，大邮箱首次同步不再像卡死；
  4. **停止信号不再丢失**：`stopCh` 由无缓冲改为缓冲 1。原实现是无缓冲通道 + 非阻塞发送，Worker 正忙（如正在 `syncOnce`）时停止信号会被 `default` 分支直接丢弃，导致重启后新旧两个 Worker 并存、竞态同步；
  5. **POP3 全链路超时**：建连与 TLS 握手 30s、普通命令 60s、`LIST` 全量列表 2min、单封邮件下载 5min，任何挂起都会在可预期时间内变成一条明确的错误日志。顺带将 `fmt.Sprintf("%s:%d", host, port)` 改为 `net.JoinHostPort()`，修复 IPv6 主机下地址非法的问题。
- **涉及文件**：
  - `server/imap/worker.go` — `doneCh` 存活判定与表项清理、唤醒日志、`syncOnce` 起止日志、`stopCh` 缓冲
  - `server/imap/fetcher.go` — FETCH 循环进度日志
  - `server/pop3/client.go` — 建连/命令/下载超时、IPv6 地址拼接
  - `server/services/account_service.go` — `TriggerSync` 唤醒/重启路径日志
- **复现验证**：重新编译后点击同步，正常应依次出现：

  ```
  🔔 已唤醒 Worker 执行同步: xxx@xx (account_id=1)
  🔔 收到手动同步请求: xxx@xx
  🔄 开始同步 (xxx@xx) [手动触发]
  ⏳ 同步进度 (xxx@xx): 已扫描 50/832 封，新增 12 封
  🏁 同步结束 (xxx@xx) [手动触发]: 耗时 3m12s
  ```

  判定方式：
  - 只有第一行 ⇒ Worker 卡住或已死；再看 `🔄 开始同步` 与 `🏁 同步结束` 是否成对 —— 有开始无结束即卡在连接/认证/拉取某一步；
  - 出现 `♻️ 检测到已退出的 Worker，已从表中移除` ⇒ 命中根因 2，现已自动重启恢复；
  - 停在 `📬 开始同步 xxx: 模式=..., 收件箱共 N 封邮件` ⇒ 命中根因 1，属正常的首次全量同步（之后为增量），等进度日志推进即可。
- **附注**：IMAP 路径下 `go-imap/v2` 自带命令级读超时（普通响应 30s、`literal` 5min、`IDLE` 期间为 0），因此不会永久挂起；POP3 无此保护，是本次补超时的重点。

### 189.cn 等邮箱历史邮件无法通过 IMAP 收取（服务商限制）

- **状态**：🚫 服务商限制，客户端无解
- **记录时间**：2026-09-03
- **问题描述**：天翼邮箱（189.cn，Coremail/21cn 实现）添加成功后应用内一直为空，但网页端能看到大量历史邮件（如中国电信广东分公司的账单邮件）。应用日志显示 `✅ IMAP 认证成功` 紧跟着 `📭 收件箱为空`，认证本身没有问题。
- **排查过程**（可复用于任何"收不到信"的账号，工具用法见 [IMAP 诊断工具](/dev/imap-diagnostics)）：
  1. `go run ./cmd/imapdiag -host imap.189.cn -user <账号> -pass <授权码> -all`；
  2. 三条**独立路径**计数一致为 0：`STATUS INBOX` 的 `MESSAGES=0`、`SELECT` 的 `EXISTS=0`、`UID SEARCH ALL` 返回空集合——三路一致可排除协议解析问题；
  3. `-all` 扫描全部 9 个目录（INBOX / 草稿箱 / 已发送 / 垃圾箱 / 已删除 / 广告文件夹 / 我的账单 / 我的发票 / 官方活动）**全部为 0**，说明邮件也不是被归类到了别的文件夹；
  4. 从外部邮箱发一封测试邮件后立刻复查：`EXISTS=1`，且该邮件的 **UID=115**——一封刚到达的邮件却拿到 115，证明服务器上存在过 114 个更早的 UID，只是不向 IMAP 暴露（这是定性证据）；
  5. 在网页端把历史邮件**转发给自己**后应用能正常收到——确认 IMAP 链路与代码均无问题。
- **根因**：服务商策略。**IMAP/POP3 只投递"开通服务之后"新到达的邮件，历史邮件不对第三方客户端开放**。这是运营商邮箱的常见做法，客户端在协议层无法绕过。
- **变通方案**：网页端把需要的历史邮件转发给自己，转发件会作为全新邮件进入 IMAP 并被正常收取。仅限个别重要邮件，批量操作不现实。
- **涉及文件**：无（非代码问题）。诊断工具见 `server/cmd/imapdiag/main.go`。
- **附注**：该服务器 `SELECT` 返回的 `UIDNEXT` 恒为 `0`（违反 RFC 3501，规范要求 ≥1），`UIDVALIDITY` 也是 1、2、3… 这类小整数。业务代码不读取这两个字段，故不受影响。

### IMAP/POP3 邮件缺失 Message-ID 时被反复重复入库

- **状态**：✅ 已修复未发布
- **记录时间**：2026-07-06
- **问题描述**：当邮件没有真实的 `Message-ID` 头部时（如阿里云通知类邮件），同一封邮件在每次 IMAP/POP3 同步时都会被当作新邮件重复入库。数据库中可看到同一封邮件出现数十条记录（`subject`/`from`/`sent_at`/`message_uid` 完全一致，但 `id` 和 `created_at` 不同），并重复触发 `mail.received` webhook。
- **根因分析**：
  1. 旧版 fallback 逻辑在邮件缺失 `Message-ID` 时使用 `time.Now()` 生成伪唯一 ID：`<auto-{uid}-{timestamp}@proxy>`。由于时间戳每次同步都不同，生成的 `message_id` 每次都不一样，导致去重查询 `WHERE message_id = ? AND account_id = ?` 永远匹配不到已有记录；
  2. 去重查询未包含 `folder` 字段，同一 `Message-ID` 在 inbox/sent 等不同文件夹中可能产生冲突；
  3. 数据库层面 `message_id` 字段上的全局唯一索引过于宽泛，会阻止不同账号收到相同 `Message-ID` 的正常邮件。
- **修复方案**：
  1. **稳定 fallback Message-ID**：不再使用 `time.Now()`，改为基于 `account_id + folder + uid`（IMAP）或 `account_id + seq`（POP3）生成稳定标识，确保同一封邮件每次同步生成相同的 `message_id`；
  2. **去重查询增加 folder**：`WHERE message_id = ? AND account_id = ? AND folder = ?`，避免不同文件夹间的误判；
  3. **索引调整**：将 `message_id` 从 `uniqueIndex`（全局唯一）改为普通 `index`，新增 `message_uid` 索引提升查询性能；
  4. **数据库迁移自动清理**：启动时自动执行 `cleanupDuplicateMails()`，删除历史重复记录（按 `account_id + folder + message_uid` 去重，保留最早入库的一条），将旧格式 `<auto-*@proxy>` 的 `message_id` 更新为新稳定格式，移除过宽的旧唯一索引，创建复合唯一索引 `idx_mails_account_folder_uid`。
- **涉及文件**：
  - `server/imap/fetcher.go` — IMAP fallback Message-ID 生成 + 去重查询
  - `server/pop3/fetcher.go` — POP3 fallback Message-ID 生成 + 去重查询 + Folder 字段
  - `server/models/mail.go` — 索引调整
  - `server/database/database.go` — 迁移后清理函数 `cleanupDuplicateMails()`
- **影响范围**：IMAP 和 POP3 协议均受影响；使用阿里云、部分企业邮箱等不发 `Message-ID` 的邮件时尤为明显。修复后历史重复数据在下次启动时自动清理，后续同步不再产生重复。

### 企业微信邮件中文乱码 (GBK/GB18030 编码)

- **状态**：✅ 已修复未发布
- **记录时间**：2026-07-02
- **问题描述**：接收企业微信邮箱（`@exmail.weixin.qq.com`）的邮件时，邮件正文中的中文字符显示为乱码（如 `浣犲ソ锛岀粏鐣岀箒鏄�`），而标题和发件人/收件人显示正常。该问题仅影响使用 `gb18030`/`gbk` 字符集编码的非 UTF-8 邮件。
- **根因分析**：
  1. Go 标准库 `mime.WordDecoder` 默认只支持 UTF-8，无法解码 RFC 2047 头部中声明的 GBK/GB18030 字符集；
  2. `go-message` 库的全局 `message.CharsetReader` 虽然被正确调用并返回了 GBK 解码器，但其**内部对 CharsetReader 返回值的处理存在缺陷**，导致双重编码（Mojibake）：原始 GBK 字节 → 正确解码为 UTF-8 → 又被当作 Latin-1 读入 → 再次编码为 UTF-8 = 最终乱码；
  3. 含大量 ASCII HTML 标签的正文在自动检测时会被误判为有效 UTF-8，跳过了手动解码步骤。
- **修复方案**：
  1. **全局 CharsetReader 改为透传**：将 `message.CharsetReader` 设为始终返回原始输入流，阻止 `go-message` 自行解码 body；
  2. **自定义解码函数 `decodeTextContentWithCharset()`**：根据 MIME 头部声明的 charset 参数精确选择解码器（`simplifiedchinese.GBK.NewDecoder()`），一次性正确转换为目标 UTF-8 字符串；
  3. **头部字段解码**：使用带 `CharsetReader` 回调的 `mime.WordDecoder` 解码 Subject、From/To 显示名等 RFC 2047 编码头部；
  4. **Charset 值清理**：所有读取 charset 的位置均添加 `strings.Trim(charset, "\"' \t")`，处理 MIME 声明中可能包裹的引号。
- **涉及文件**：
  - `server/main.go` — 全局 `message.CharsetReader` 设置（透传模式）
  - `server/imap/fetcher.go` — IMAP 邮件正文/头部解码逻辑
  - `server/pop3/fetcher.go` — POP3 邮件正文/头部解码逻辑
  - `server/services/attachment_service.go` — 附件名 RFC 2047 解码
- **依赖新增**：`golang.org/x/text v0.21.0`（`simplifiedchinese.GBK` 解码器）

### 163/126 等网易邮箱无法发送邮件

- **状态**：✅ 已修复并于v1.1.0发布
- **记录时间**：2026-07-01
- **问题描述**：使用 163、126 等网易邮箱时，虽然可以正常收信（IMAP），但无法发送邮件（SMTP）。原因是用户在添加账号时通常只填写 IMAP 服务器地址，系统未能正确推断对应的 SMTP 服务器地址和端口，导致 SMTP 连接失败。
- **修复方案**：
  1. 新增常见邮箱服务商的 SMTP 服务器映射表（`smtpHostMap`），支持 163、126、QQ、新浪、阿里云、Gmail、Yahoo、Outlook 等主流邮箱；
  2. 新增 SMTP 端口映射表（`smtpPortMap`），针对不同服务商使用正确的端口（如网易系使用 465 SSL/TLS，Gmail/Outlook 使用 587 STARTTLS）；
  3. 实现 `inferSMTPHost()` 函数，根据 IMAP 地址自动推断 SMTP 服务器地址（优先匹配已知服务商，否则将 `imap.` 替换为 `smtp.`）；
  4. 实现 `DefaultSmtpPort()` 函数，根据 SMTP 服务器地址返回正确的默认端口。
- **涉及文件**：`server/models/mail_account.go`

### IMAP IDLE 不兼容部分邮件服务器

- **状态**：✅ 已修复（v1.1.0 发布，2026-09-03 进一步增强，未发布）
- **记录时间**：2026-07-01（2026-09-03 更新）
- **问题描述**：部分邮件服务器不支持 IMAP IDLE 命令（RFC 2177），当 Worker 尝试使用 IDLE 实时监听时会报错并反复重试，导致日志大量错误信息，且每次重启都会重新尝试。
- **修复方案**：
  1. 新增 IDLE 支持服务器白名单（`idleVerifiedServers`），仅对 Gmail、Yahoo、Outlook、QQ 等已验证服务器启用 IDLE；
  2. 新增运行时学习机制（`idleLearnedUnsupported`），首次遇到不支持的 IDLE 错误（包含 "BAD"、"not support"、"not allowed" 关键字）时，自动将该服务器标记为不支持并加入全局黑名单；
  3. 后续同服务器的其他 Worker 可共享黑名单信息，避免重复尝试。
- **涉及文件**：`server/imap/worker.go`
- **备注**：未知服务器仍会首次尝试 IDLE，符合渐进式兼容策略。失败后的处理已分为两类：
  - 服务器明确返回不支持（`BAD` / `not support` / `not allowed`）：写入全局黑名单，同服务器的其他账号一并降级为轮询；
  - 网络抖动等瞬时故障：**指数退避重连**（30s → 60s → 120s，上限 15 分钟），连续失败 4 次后才降级为轮询，且只影响当前 Worker、不写入全局黑名单（避免误伤同服务器的其他账号）。
- **补充（2026-09-03 增强）**：v1.1.0 的判定方式（白名单 + 错误文本匹配）在 189.cn 这类服务器上失效——它不会回 `BAD`，而是返回违反 RFC 2177 的畸形响应，错误文本命中不了黑名单，只能被当成"瞬时故障"走指数退避，白试 4 次（约 3.5 分钟内 4 次无效登录）才降级，且黑名单在进程重启后清空。现改为三层判定：
  1. **能力探测（第一道防线）**：认证后检查 `CAPABILITY` 是否声明 `IDLE`（RFC 2177 要求支持者必须声明）。189.cn 能力集仅 `IMAP4 IMAP4rev1 ID XLIST XAPPLEPUSHSERVICE`，无 IDLE，一次判定即转轮询。注意必须**在认证之后**读取——部分服务器登录前后能力集不同；
  2. **协议解析错误识别**：`imapwire` / `cannot read tag` / `expected atom` 等解析失败属**确定性失败**（重试多少次都一样），与"明确不支持"同等对待，一次失败即入全局黑名单；
  3. **静态域名黑名单（兜底）**：收拾"声明支持但响应不合规"的服务器（如部分 Coremail 部署），连首次尝试都不做。
- **诊断工具**：`go run ./cmd/imapdiag -host <服务器> -user <账号> -pass <密码>` 可直接报告服务器是否声明 IDLE，完整用法见 [IMAP 诊断工具](/dev/imap-diagnostics)。

### SMTP 邮件头部 CRLF 注入漏洞 (CWE-93)

- **状态**：✅ 已修复并于v1.1.0发布
- **记录时间**：2026-06-26
- **安全等级**：中等（CVSS ≈ 5.3）
- **问题描述**：发送邮件时，`buildMessage` 函数直接将用户输入的收件人地址、主题等字段拼接到 RFC822 邮件头部中，未对 `\r`、`\n` 等控制字符进行过滤。攻击者可在收件人或主题字段中注入换行符，从而注入额外的 SMTP 头部或篡改邮件内容。
- **修复方案**：
  1. 新增 `sanitizeHeaderValue` 和 `sanitizeEmailAddr` 函数，在构建邮件头部时清洗所有用户输入，移除控制字符；
  2. 在 `Send` handler 入口处增加输入校验，检测并拒绝包含 `\r\n` 的字段值（纵深防御）。
- **涉及文件**：`server/smtp/client.go`, `server/handlers/mail_handler.go`
- **参考**：同类漏洞 (CVE form-data v4.0.5) — 本项目虽不依赖该库，但存在相同的攻击面

### 密码解密失败导致账号列表查询异常

- **状态**：✅ 已修复并于v1.1.0发布
- **记录时间**：2026-06-26
- **问题描述**：当某个邮箱账号的密码加密数据损坏或密钥不匹配时，`AfterFind` 钩子中的解密失败会阻断整个账号列表查询接口（500 错误），导致用户无法查看任何账号。
- **修复方案**：
  1. 解密失败时仅记录警告日志并清空密码字段，不再返回错误阻断查询；
  2. 新增 `AccountListDTO` 专用列表查询模型，避免列表场景触发 `AfterFind` 解密逻辑；
  3. 新增账号健康检查接口 `/api/v1/accounts/health`，便于排查异常账号。
- **涉及文件**：`server/models/mail_account.go`, `server/services/account_service.go`, `server/handlers/account_handler.go`, `server/services/health_check_service.go`, `web/src/stores/accountStore.js`

### 163/126 网易邮箱登录失败 (Unsafe Login)

- **状态**：✅ 已修复并于v1.1.0发布
- **记录时间**：2026-06-25
- **问题描述**：使用 163、126 等网易邮箱时，IMAP 登录返回 "SELECT Unsafe Login" 错误，导致无法正常收信。原因是网易邮箱要求客户端在登录前必须发送 ID 命令声明身份（符合 RFC 2971 规范）。
- **修复方案**：在 IMAP 登录前主动发送 ID 命令，声明客户端信息（Name: MagicMail, Version: 1.0.0, Vendor: MagicCode）。若服务器不支持 ID 命令则仅记录日志，不阻塞登录流程。
- **涉及文件**：`server/imap/client.go`

### 邮箱管理页面中等宽度下信息与按钮重叠

- **状态**：✅ 已修复并于v1.1.0发布
- **记录时间**：2026-06-26
- **问题描述**：在 768px ~ 900px 宽度区间，邮箱管理页面的桌面端 Grid 布局会导致邮箱地址信息与右侧操作按钮（编辑、同步、删除）发生重叠，影响使用体验。
- **修复方案**：将 `AccountManage.vue` 的响应式断点从 `768px` 调整为 `900px`，使中等屏幕更早切换到卡片布局。
- **涉及文件**：`web/src/views/AccountManage.vue`
