# OAuth2 配置指南

本文档介绍 MagicMail 的 OAuth2 认证系统配置，包括 Azure AD 应用注册、环境变量设置和故障排查。

## 概述

MagicMail 支持 **设备码流 (Device Code Flow)** OAuth2 授权方式，目前优先支持 **Microsoft Outlook / Hotmail** 邮箱的 XOAUTH2 IMAP 登录。

> **为什么需要 OAuth2？**
> 微软自 2022 年起已逐步禁用传统「应用密码」方式的 IMAP/SMTP Basic Auth，
> 要求所有第三方应用使用 OAuth2 进行认证。MagicMail 通过设备码流实现这一过程，
> 无需配置回调 URL，用户只需在浏览器中输入验证码即可完成授权。

## 架构概览

```
┌──────────────┐     设备码请求      ┌───────────┐
│  前端向导 UI │ ──────────────────→ │  后端 API  │
│  (浏览器)    │ ←── 验证码+链接 ─── │  /oauth/*  │
└──────────────┘                     └─────┬─────┘
                                           │
                              ┌────────────┼────────────┐
                              ▼            ▼             ▼
                         ┌────────┐ ┌──────────┐ ┌──────────┐
                         │Microsoft│ │  Google  │ │  Yahoo   │ │ 预留
                         │ Provider│ │ Provider │ │ Provider │
                         └────────┘ └──────────┘ └──────────┘
```

### Client ID 三级优先级

| 优先级 | 来源 | 说明 |
|--------|------|------|
| 1（最高） | 用户在 UI 中自定义 | 高级用户可指定自己的 Client ID |
| 2（中等） | 环境变量 `MAGICMAIL_OAUTH_MICROSOFT_CLIENT_ID` | 部署者可覆盖默认值 |
| 3（兜底） | 开发者硬编码默认值 | 内置在代码中 |

---

## 一、Azure AD 应用注册（获取 Client ID）

### 步骤 1：登录 Azure Portal

1. 打开 [Azure Portal](https://portal.azure.com/)
2. 使用 Microsoft 账号登录

### 步骤 2：注册新应用

1. 导航到 **Microsoft Entra ID** → **应用注册**（或搜索 "App registrations"）
2. 点击 **新注册 (+)**
3. 填写：
   - **名称**: `MagicMail OAuth`（或任意你喜欢的名字）
   - **支持的账户类型**: 选择 **任何组织目录(任何 Microsoft Entra 目录 - 租户)中的帐户和个人 Microsoft 帐户**
     > ⚠️ 必须选择包含「个人 Microsoft 账号」的选项，否则无法授权 @outlook.com / @hotmail.com 个人邮箱
4. 点击 **注册**

### 步骤 3：配置 API 权限

::: warning 必须使用 Office 365 Exchange Online 权限
IMAP/SMTP XOAUTH2 认证**必须**使用 Office 365 Exchange Online 的权限，
**不能**使用 Microsoft Graph 的 `Mail.Read` / `Mail.Send`。
Graph Token 无法用于 IMAP XOAUTH2 认证，会导致 `NO AUTHENTICATE failed` 错误。
:::

1. 在左侧菜单选择 **API 权限** → **添加权限**
2. 选择 **我的组织使用的 API** 标签页
3. 搜索 **Office 365 Exchange Online** 并选择

   > 💡 如果搜索不到，确保应用注册时选择了「任何组织目录中的账户和个人 Microsoft 账户」（common tenant）。

4. 添加以下**委派权限 (Delegated permissions)**：

   | 权限名 | 类型 | 用途 |
   |--------|------|------|
   | `IMAP.AccessAsUser.All` | 委托 | 通过 IMAP 协议访问用户邮箱 |
   | `SMTP.Send` | 委托 | 通过 SMTP 协议发送邮件 |

5. 点击 **添加权限**
6. 再次点击 **添加权限** → **Microsoft API** → **Microsoft Graph** → 勾选 **`offline_access`**（获取 Refresh Token）

::: tip Scope 说明
代码中使用的授权范围（scope）为：
- `https://outlook.office.com/IMAP.AccessAsUser.All`
- `https://outlook.office.com/SMTP.Send`
- `offline_access`

注意：用户委托流程（设备码流）必须使用 `outlook.office.com` 而非 `outlook.office365.com`。
后者仅用于客户端凭据流程（服务主体），详见 [微软官方文档](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth)。
:::

### 步骤 4：获取 Client ID

1. 在应用概览页面，复制 **应用程序(客户端) ID**
2. 这就是你的 Client ID（约 36 个字符的 GUID）

### 步骤 5：配置为移动/桌面端（可选但推荐）

由于我们使用的是设备码流（无需回调 URL），无需额外配置重定向 URI。

但如果 Azure 提示需要配置：
- 进入 **身份验证**
- 添加平台 → 选择 **移动和桌面应用程序**
- 重定向 URL 填写: `https://login.microsoftonline.com/common/oauth2/nativeclient`

---

## 二、部署配置

### 方式 A：环境变量（推荐）

```bash
# 在 .env 文件或启动命令中设置
export MAGICMAIL_OAUTH_MICROSOFT_CLIENT_ID="your-client-id-here"
```

### 方式 B：修改源码默认值

编辑 `apps/server/oauth2/microsoft.go`:

```go
const microsoftDefaultClientID = "YOUR_ACTUAL_CLIENT_ID_HERE"
```

### 方式 C：用户自定义

在前端添加邮箱时，OAuth2 授权区域下方提供「高级选项」折叠面板，用户可填写自定义 Client ID（留空则使用默认值或环境变量配置）。

---

## 三、授权流程演示（用户体验）

### 场景：添加 Outlook 邮箱

```
1. 用户点击「添加账号」→ 弹出服务商选择弹窗
2. 用户点击「Outlook / Hotmail」卡片（带 OAuth2 标识）
3. Step 2 显示：
   - 显示名称输入框
   - 邮箱地址输入框
   - 「开始 OAuth2 授权」按钮
   - 「高级选项」折叠面板（可填写自定义 Client ID）
4. 用户点击授权按钮后：
   - 展开授权面板
   - 显示可点击的超链接：https://microsoft.com/devicelogin（点击直接打开新标签页）
   - 显示大号验证码：ABCD-EFGH（可一键复制）
   - 15分钟倒计时进度条开始
   - 后台每5秒轮询一次
5. 用户操作：
   - 点击链接打开新标签页 → 输入验证码
   - 登录微软账号 → 同意权限
6. 后台检测到授权成功：
   - 显示绿色 ✓ OAuth2 授权成功！
7. 用户点击「下一步」→ 进入 Step 3 高级设置
   - 配置同步模式（仅未读/全部/最近N天）
   - 配置代理（可选）
8. 用户点击「完成」→ 账号创建完成
9. 后续邮件同步自动使用 XOAUTH2 认证
```

---

## 四、故障排查

### Q: 设备码请求失败 — "invalid_scope" (AADSTS70011)

**原因:** Scope 不正确
**解决:**
- 确认使用的是 `https://outlook.office.com/IMAP.AccessAsUser.All`（用户委托流程），而非 `https://outlook.office365.com/`（客户端凭据流程）
- 确认 Azure AD 应用已添加 **Office 365 Exchange Online** 的 `IMAP.AccessAsUser.All` 和 `SMTP.Send` 委派权限
- 确认应用账户类型为「任何组织目录中的账户和个人 Microsoft 账户」（common tenant）

### Q: 设备码请求失败 — "invalid_client"

**原因:** Client ID 无效或未正确配置
**解决:**
- 检查环境变量 `MAGICMAIL_OAUTH_MICROSOFT_CLIENT_ID`
- 确认 Azure AD 应用已启用「个人 Microsoft 账号」支持
- 确认应用未被禁用或删除

### Q: IMAP 认证失败 — "NO AUTHENTICATE failed"

**原因:** Access Token 的 scope 不支持 IMAP XOAUTH2
**解决:**
- 确认 Token 是通过 `outlook.office.com` scope 获取的，而非 Graph API 的 `Mail.Read`
- 如果之前用旧 scope 授权过，需要**删除账号重新走 OAuth2 授权流程**，获取新 scope 的 token
- 确认 Azure AD 应用已添加 Office 365 Exchange Online 的 IMAP 权限

### Q: 用户授权后被提示 "需要管理员同意"

**原因:** 应用需要租户管理员审批权限
**解决:**
- 如果是个人账号（@outlook.com/@hotmail.com），确保注册时选择了正确的账户类型
- 如果是企业账号 (@company.onmicrosoft.com)，请联系管理员授予权限

### Q: Refresh Token 刷新失败

**错误信息:** `RefreshToken 已失效，需要用户重新授权`
**原因:**
- 用户撤销了应用的访问权限
- 密码更改导致 RT 失效
- 超过 90 天未使用（微软 RT 有效期限制）
**解决:** 引导用户重新执行 OAuth2 授权流程

### Q: IMAP XOAUTH2 认证失败

**可能原因:**
1. Access Token 过期且刷新失败
2. IMAP 权限范围不正确
3. 账号被锁定或安全策略阻止

**排查方法:**
```bash
# 查看服务端日志
grep "XOAUTH2" magicmail.log
grep "RefreshToken" magicmail.log
```

### Q: 国内网络无法连接微软 OAuth2 端点

**原因:** GFW 可能干扰
**解决:**
- 确保服务器可以访问 `https://login.microsoftonline.com`
- 如需代理，配置 HTTP_PROXY 环境变量
- Go 的 http.DefaultClient 会自动读取 HTTP_PROXY/HTTPS_PROXY

---

## 五、扩展支持其他邮箱服务商

当前架构预留了 Google Gmail 和 Yahoo 的扩展能力。添加新的 OAuth2 Provider 只需：

1. 在 `apps/server/oauth2/` 创建 `google.go`，实现 `OAuth2Provider` 接口
2. 在 `apps/server/routes/routes.go` 注册路由
3. 在 `apps/web/src/data/providers.js` 更新提供商数据
4. 前端组件自动适配（通过 `oauthRequired` 和 `oauthProvider` 字段）

### 示例：添加 Gmail 支持

```go
// apps/server/oauth2/google.go
package oauth2

type GoogleProvider struct{}

func NewGoogleProvider() *GoogleProvider { return &GoogleProvider{} }
func (p *GoogleProvider) Name() string { return "google" }
func (p *GoogleProvider) DefaultClientID() string { return "YOUR_GOOGLE_CLIENT_ID" }
func (p *GoogleProvider) EnvVarName() string { return "MAGICMAIL_OAUTH_GOOGLE_CLIENT_ID" }
// ... 实现其余接口方法
```

---

## 六、安全注意事项

1. **RefreshToken 加密存储**: 所有 Token 使用 AES-256-GCM 加密后存入 SQLite 数据库
2. **Access Token 不落盘**: AT 仅存在于内存中，进程重启后通过 RT 刷新获取
3. **Client ID 保护**: 默认值不应提交到公开仓库（建议通过环境变量注入）
4. **设备码有效期**: 严格遵守服务端返回的有效期（通常 15 分钟）
5. **轮询间隔**: 遵循微软建议的最小 5 秒间隔，避免被限流

---

## 七：相关文件索引

| 文件 | 说明 |
|------|------|
| `apps/server/oauth2/types.go` | 类型定义 |
| `apps/server/oauth2/provider.go` | 接口 + 注册中心 |
| `apps/server/oauth2/microsoft.go` | Microsoft 实现（scope、端点、Token 刷新） |
| `apps/server/oauth2/xoauth2.go` | XOAUTH2 工具函数 |
| `apps/server/handlers/oauth_handler.go` | API Handler |
| `apps/server/imap/client.go` | IMAP XOAUTH2 认证集成（XOAUTH2Client） |
| `apps/web/src/components/WizardOAuth2Flow.vue` | 前端授权交互 UI |
| `apps/web/src/components/WizardStepConfig.vue` | 配置步骤（含自定义 Client ID 高级选项） |
| `apps/web/src/api/account.js` | 前端 OAuth2 API 函数 |
| `apps/web/src/data/providers.js` | 服务商预设数据 |
