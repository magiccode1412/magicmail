# 注册 Microsoft Azure AD 应用

本教程将指导你完成 Azure AD 应用注册，使 MagicMail 能够通过 OAuth2 协议连接 Outlook / Hotmail 邮箱。

## 为什么需要注册？

:::: info 背景
微软自 2022 年起逐步禁用了 IMAP/SMTP 的传统密码认证方式（Basic Auth），
要求所有第三方应用必须使用 **OAuth2** 进行身份验证。

MagicMail 采用**设备码流 (Device Code Flow)** 实现这一过程，
无需配置回调 URL，用户只需在浏览器输入验证码即可完成授权。
::::

## 前置准备

- 一个 Microsoft 账号（支持 `@outlook.com`、`@hotmail.com`、`@live.com` 等个人邮箱）
- 可访问 [Azure Portal](https://portal.azure.com/) 的浏览器

---

## 步骤一：登录 Azure Portal

1. 打开 [Azure Portal](https://portal.azure.com/)
2. 使用你的 Microsoft 账号登录

> 如果你没有 Azure 订阅也没关系——注册应用是**完全免费**的，
> 不需要付费订阅或信用卡。

---

## 步骤二：创建应用注册

### 2.1 进入应用注册页面

有两种方式进入：

**方式 A — 通过搜索：**
- 点击顶部的 **🔍 搜索栏**
- 输入 `App registrations`
- 点击搜索结果中的 **应用注册**

**方式 B — 通过菜单导航：**
1. 左侧菜单找到 **Microsoft Entra ID**（旧称 Azure Active Directory）
2. 展开 **管理** 分组
3. 点击 **应用注册**

### 2.2 注册新应用

点击页面上方的 **+ 新注册** 按钮，填写以下信息：

| 字段 | 填写内容 | 说明 |
|------|----------|------|
| **名称** | `MagicMail OAuth` | 随意填写，用于标识你的应用 |
| **支持的账户类型** | 见下方详解 | ⚠️ 关键选项 |
| **重定向 URI (可选)** | 留空 | 设备码流不需要 |

:::: warning ⚠️ 关键：选择正确的账户类型
点击「支持的账户类型」下拉框后，必须选择包含**个人 Microsoft 账号**的选项：

```
✅ 正确选项：
"任何组织目录(任何 Microsoft Entra 目录 - 租户)中的帐户和个人 Microsoft 帐户"

❌ 错误选项：
"仅限此组织目录中的帐户"
```

如果选错了，你将无法使用 `@outlook.com` / `@hotmail.com` 个人邮箱进行授权！
::::

### 2.3 完成注册

确认信息无误后，点击页面底部的 **注册** 按钮。

注册成功后，你会看到应用的**概览面板**，其中包含我们需要的 **应用程序(客户端) ID**。

---

## 步骤三：配置 API 权限

### 3.1 添加权限

1. 在左侧菜单中点击 **API 权限**
2. 点击 **+ 添加权限** 按钮
3. 选择 **我的组织使用的 API** 标签页
4. 搜索并选择 **Office 365 Exchange Online**

::: warning 必须使用 Office 365 Exchange Online
IMAP/SMTP XOAUTH2 认证**必须**使用 Office 365 Exchange Online 的权限，
**不能**使用 Microsoft Graph 的 `Mail.Read` / `Mail.Send`。
Graph Token 无法用于 IMAP XOAUTH2 认证，会导致 `NO AUTHENTICATE failed` 错误。
:::

### 3.2 选择所需权限

在权限列表中，勾选以下 **委派权限 (Delegated permissions)**：

| 权限名称 | 说明 |
|----------|------|
| `IMAP.AccessAsUser.All` | 通过 IMAP 协议访问用户邮箱 |
| `SMTP.Send` | 通过 SMTP 协议以用户身份发送邮件 |
| `offline_access` | 获取 Refresh Token，用于长期访问（需在 Microsoft Graph 下单独添加） |

选中后，点击页面底部的 **添加权限** 按钮。

### 3.3 授予管理员同意（个人账号可跳过）

如果你使用的是**个人 Microsoft 账号**注册的应用，这一步通常不需要操作。

对于**工作/学校账号**，可能需要管理员同意：

1. 在 **API 权限** 页面
2. 如果状态列显示 **未授予...**，点击 **代表 <租户名> 授予管理员同意**
3. 确认授权操作

---

## 步骤四：获取客户端 ID

1. 回到应用的 **概览** 页面（左侧菜单顶部）
2. 找到 **应用程序(客户端) ID)** 字段
3. 点击右侧的 **复制** 图标，保存这个值

这就是 MagicMail 所需的 **Client ID**，格式类似：

```
a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

---

## 步骤五：（可选但推荐）配置平台类型

虽然设备码流不依赖重定向 URI，但建议配置以避免 Azure 的警告提示：

1. 左侧菜单点击 **身份验证**
2. 在「平台配置」区域点击 **+ 添加平台**
3. 选择 **移动和桌面应用程序**
4. 勾选 **通过使用系统浏览器向最终用户发出请求...**
5. 重定向 URI 会自动填充为：
   ```
   https://login.microsoftonline.com/common/oauth2/nativeclient
   ```
6. 点击 **配置**

---

## 步骤六：将 Client ID 配置到 MagicMail

获取 Client ID 后，有以下三种方式将其提供给 MagicMail：

### 方式 A：环境变量（推荐）

```bash
export MAGICMAIL_OAUTH_MICROSOFT_CLIENT_ID="a1b2c3d4-e5f6-7890-abcd-ef1234567890"
```

适用于 Docker 部署或生产环境，在 `.env` 文件或启动命令中设置即可。

### 方式 B：前端自定义（按需）

在添加 Outlook 邮箱时，OAuth2 授权区域提供「自定义 Client ID」高级选项。
适合需要为不同账号使用不同 Azure 应用的场景。

### 方式 C：修改源码默认值

编辑服务端源码 `apps/server/oauth2/microsoft.go`，修改常量：

```go
const microsoftDefaultClientID = "你的实际 Client ID"
```

仅适合自用部署，不建议分发包含硬编码 Client ID 的构建产物。

---

## 验证配置

完成以上所有步骤后，你可以验证 OAuth2 功能是否正常工作：

1. 启动 MagicMail 服务
2. 进入 Web UI → 设置 → 邮箱管理 → 添加账号
3. 选择 **Outlook / Hotmail** 服务商卡片（带有 🔒 OAuth2 标识）
4. 输入邮箱地址和显示名称
5. 点击 **开始 OAuth2 授权** 按钮
6. 按照屏幕指引在浏览器中完成授权

如果一切正常，你应该会看到绿色的 ✓ 授权成功提示！

---

## 故障排查

<details>
<summary><strong>设备码请求失败："invalid_client"</strong></summary>

**可能原因及解决方案：**

| 原因 | 解决方法 |
|------|----------|
| Client ID 复制不完整或有空格 | 重新复制，确保无多余字符 |
| 环境变量未生效 | 重启服务，检查 `.env` 文件语法 |
| 应用账户类型错误 | 返回步骤 2.2 确认选择了「个人 Microsoft 账号」 |
| Azure 应用被禁用或删除 | 重新创建应用注册 |
</details>

<details>
<summary><strong>授权时提示"需要管理员同意"</strong></summary>

**个人账号 (@outlook/@hotmail)：**
- 确认注册时选择的账户类型包含「个人 Microsoft 账号」

**企业/学校账号 (@company.onmicrosoft.com)：**
- 联系你的 IT 管理员授予权限
- 或让管理员在 Azure AD 中批准该应用的权限请求
</details>

<details>
<summary><strong>IMAP XOAUTH2 认证失败</strong></summary>

**排查步骤：**

```bash
# 1. 查看服务端日志中的 OAuth2 相关输出
grep -E "(XOAUTH2|RefreshToken|OAuth)" magicmail.log

# 2. 确认已添加 Mail.Read 和 Mail.Send 权限（步骤 3.2）
#    缺少权限会导致认证被拒

# 3. 检查 Access Token 是否过期
#    Token 有效期通常 1 小时，过期后会自动刷新
```

常见原因：
- Access Token 过期且 Refresh Token 刷新失败
- 用户撤销了应用访问权限（需重新授权）
- 超过 90 天未使用导致 RT 失效
</details>

<details>
<summary><strong>国内网络无法连接微软 OAuth2 端点</strong></summary>

MagicMail 的 HTTP 客户端会自动读取系统代理环境变量：

```bash
# 设置代理（如需要）
export HTTPS_PROXY=http://your-proxy:port
export HTTP_PROXY=http://your-proxy:port
```

或者在前端添加邮箱时，为该账号单独配置 HTTP 代理（见[邮箱管理 > HTTP 代理](/guide/accounts#http-代理)）。
</details>

---

## 安全最佳实践

| 建议 | 说明 |
|------|------|
| 🔒 不要公开 Client ID | 虽然 Client ID 本身不是机密，但避免提交到公开 Git 仓库 |
| 🔄 定期审查权限 | 在 Azure Portal 定期检查应用的 API 权限是否仍然必要 |
| 📝 记录应用用途 | 在应用描述中注明用途，方便后续审计 |
| ⚠️ 不共享 RefreshToken | RT 是加密存储在本地数据库中的，不要导出分享 |

---

## 相关文档

- [邮箱管理 - 添加 Outlook 账号](/guide/accounts)
- [环境变量完整参考](/config/environment)
- [OAuth2 架构技术文档](/oauth2-setup-guide)
