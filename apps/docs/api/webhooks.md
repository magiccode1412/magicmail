# Webhook 接口

## Webhook 列表

```
GET /api/v1/webhooks
```

返回当前用户创建的所有 Webhook 配置。

## 创建 Webhook

```
POST /api/v1/webhooks
```

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | Webhook 名称 |
| url | string | 是 | 回调 URL（公网可达）|
| headers | object | 否 | 自定义 HTTP Header |
| bodyTemplate | string | 否 | Body 模板（支持变量替换）|
| enabled | boolean | 否 | 是否启用（默认 true）|

**示例**

> 下文示例中使用双花括号作为变量占位符：

```bash
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "name": "飞书通知",
    "url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxx",
    "headers": { "Content-Type": "application/json" },
    "bodyTemplate": "{\"title\":\"📧 新邮件通知 - {{data.account_name}}\",\"content\":\"收到 {{data.mail_count}} 封新邮件\\n来源：{{data.account_name}} <{{data.account_email}}>\\n邮件列表：{{data.mails}}\",\"type\":\"markdown\"}",
    "enabled": true
  }'
```

## Webhook 详情

```
GET /api/v1/webhooks/:id
```

## 更新 Webhook

```
PUT /api/v1/webhooks/:id
```

同创建接口字段，部分更新。

## 删除 Webhook

```
DELETE /api/v1/webhooks/:id
```

## 测试推送

```
POST /api/v1/webhooks/:id/test
```

发送一条模拟新邮件通知，验证 Webhook 是否正常工作。

**响应**

```json
{
  "code": 0,
  "data": {
    "statusCode": 200,
    "responseBody": "{\"ok\":true}",
    "timestamp": "2026-06-05T11:00:00Z"
  }
}
```

## 推送日志

```
GET /api/v1/webhooks/:id/logs
```

查看该 Webhook 的历史推送记录。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | integer | 1 | 页码 |
| pageSize | integer | 20 | 每页数量 |

**响应**

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": 5001,
        "webhookId": 3,
        "mailSubject": "项目会议纪要",
        "mailFrom": "pm@example.com",
        "statusCode": 200,
        "errorMessage": null,
        "createdAt": "2026-06-05T10:30:00Z"
      }
    ],
    "total": 42
  }
}
```
