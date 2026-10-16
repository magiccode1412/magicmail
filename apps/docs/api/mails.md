# 邮件管理接口

## 邮件列表

```
GET /api/v1/mails
```

支持分页、搜索、筛选。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | integer | 1 | 页码 |
| pageSize | integer | 20 | 每页数量（最大 100）|
| keyword | string | - | 搜索关键词（发件人/主题/正文）|
| accountId | integer | - | 按邮箱 ID 筛选 |
| isRead | boolean | - | 已读/未读筛选 |
| isStarred | boolean | - | 星标筛选 |
| sort | string | `createdAt` | 排序字段 |
| order | string | `desc` | 排序方向（asc / desc）|

**示例**

```bash
curl "http://localhost:8080/api/v1/mails?page=1&pageSize=20&keyword=重要" \
  -H "Authorization: Bearer <token>"
```

**响应**

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": 1024,
        "accountId": 1,
        "messageId": "<abc123@mail.example.com>",
        "from": "sender@example.com",
        "fromName": "张三",
        "to": ["user@example.com"],
        "subject": "项目进度更新",
        "summary": "本周完成了前端页面的开发...",
        "isHtml": true,
        "isRead": false,
        "isStarred": true,
        "hasAttachment": true,
        "createdAt": "2026-06-05T09:30:00Z"
      }
    ],
    "total": 256,
    "page": 1,
    "pageSize": 20
  }
}
```

## 统计数据

```
GET /api/v1/mails/stats
```

返回各邮箱的邮件统计（总数、未读数等）。

## 邮件详情

```
GET /api/v1/mails/:id
```

获取完整邮件内容（含 HTML 正文、完整头信息）。

## 标记已读/未读

```
PUT /api/v1/mails/:id/read
```

**请求体**

```json
{ "isRead": true }
```

## 标记星标

```
PUT /api/v1/mails/:id/star
```

**请求体**

```json
{ "isStarred": true }
```

## 删除邮件

```
DELETE /api/v1/mails/:id
```

从本地数据库中删除邮件记录。

## 批量删除邮件

```
POST /api/v1/mails/batch-delete
```

批量删除多封邮件。

**请求体**

```json
{ "ids": [1, 2, 3] }
```

## 发送邮件

```
POST /api/v1/mails/send
```

通过 SMTP 发送邮件。

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| accountId | integer | 是 | 发件邮箱账号 ID |
| to | string[] | 是 | 收件人列表 |
| cc | string[] | 否 | 抄送列表 |
| bcc | string[] | 否 | 密送列表 |
| subject | string | 是 | 邮件主题 |
| body | string | 是 | HTML 邮件正文 |
| attachments | string[] |否 | 附件文件路径列表 |

## SSE 实时推送流

```
GET /api/v1/mails/stream
```

建立 Server-Sent Events 长连接，实时接收邮件更新事件。需 JWT 认证。

**响应格式**

```
Content-Type: text/event-stream
Cache-Control: no-cache, no-transform
Connection: keep-alive
```

**事件类型**

| 事件名 | 触发时机 | 数据结构 |
|--------|----------|----------|
| `connected` | 连接建立 | `{client_id, server_time, online_count}` |
| `mail.received` | 新邮件到达 | `{account_id, account_email, mail_count, mails[...], timestamp}` |
| `mail.synced` | 邮件同步完成 | `{account_id, account_email, timestamp}` |
| `heartbeat` | 每 15 秒 | `{time}` |

**示例（JavaScript）**

```javascript
const token = 'YOUR_JWT_TOKEN'
const es = new EventSource(`/api/v1/mails/stream?token=${token}`)

es.addEventListener('mail.received', (e) => {
  const data = JSON.parse(e.data)
  console.log(`收到 ${data.mail_count} 封新邮件`)
})
```

## SSE 健康检查

```
GET /api/v1/mails/stream/health
```

检查 SSE 服务是否正常运行。

**响应**

```json
{ "status": "ok", "online": 3, "service": "sse" }
```
