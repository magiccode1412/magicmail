# Webhook 通知

Webhook 允许在新邮件到达时自动推送 HTTP 通知到外部服务，适用于：

- 即时消息推送（钉钉、飞书、企业微信、Slack）
- 自动化工流集成（Zapier、n8n、Make）
- 自定义业务逻辑处理

## 创建 Webhook

1. 进入 **设置中心** → **Webhook 管理**
2. 点击 **新建 Webhook**
3. 配置以下参数：

| 字段 | 说明 |
|------|------|
| 名称 | Webhook 显示名称 |
| URL | 回调地址（需公网可达） |
| 自定义 Header | 额外的 HTTP 请求头（如 Authorization）|
| Body 模板 | 请求体模板，支持变量替换 |

## 变量模板

Body 模板中可以使用以下变量：

| 变量 | 说明 |
|------|------|
| <span v-pre>`{{event}}`</span> | 事件名称（如 `mail.received`） |
| <span v-pre>`{{timestamp}}`</span> | 触发时间（Unix 时间戳字符串） |
| <span v-pre>`{{data.subject}}`</span> | 邮件主题 |
| <span v-pre>`{{data.from}}`</span> | 发件人地址 |
| <span v-pre>`{{data.to}}`</span> | 收件人地址 |
| <span v-pre>`{{data.cc}}`</span> | 抄送地址 |
| <span v-pre>`{{data.sent_at}}`</span> | 邮件发送时间 |
| <span v-pre>`{{data.preview}}`</span> | 正文预览（前 200 字） |
| <span v-pre>`{{data.text_body}}`</span> | 纯文本正文（完整） |
| <span v-pre>`{{data.html_body}}`</span> | HTML 正文（完整） |
| <span v-pre>`{{data.account_id}}`</span> | 邮箱账号 ID |
| <span v-pre>`{{data.account_email}}`</span> | 邮箱地址 |
| <span v-pre>`{{data.account_name}}`</span> | 邮箱显示名称 |
| <span v-pre>`{{data.protocol}}`</span> | 协议类型（imap / pop3） |

::: tip 每封独立触发
每收到一封新邮件，会独立触发一次 Webhook 推送。模板中可直接使用扁平化的单封邮件字段。
:::

### 示例：[魔法推送](https://github.com/magiccode1412/magicpush)

```json
{
  "title": "📧 {{data.subject}}",
  "content": "发件： {{data.from}}\n收件：{{data.to}}\n时间：{{data.sent_at}}",
  "type": "text"
}
```

## 测试与管理

- **测试推送**：发送模拟通知验证配置是否生效
- **查看日志**：查看每次推送的状态码和响应内容
- **启用/禁用**：临时开关某个 Webhook
- **删除**：移除不再需要的 Webhook

::: warning 公网访问
Webhook URL 必须是公网可达的地址。本地开发可使用 ngrok、frp 等内网穿透工具暴露服务。
:::
