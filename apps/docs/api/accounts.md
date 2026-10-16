# 邮箱管理接口

## 邮箱列表

```
GET /api/v1/accounts
```

返回已添加的所有 IMAP 邮箱账号（密码字段已脱敏）。

**响应示例**

```json
{
  "code": 0,
  "data": [
    {
      "id": 1,
      "name": "工作邮箱",
      "email": "user@example.com",
      "host": "imap.example.com",
      "port": 993,
      "username": "user@example.com",
      "status": "active",
      "lastSyncAt": "2026-06-05T10:00:00Z",
      "mailCount": 256
    }
  ]
}
```

## 邮箱详情

```
GET /api/v1/accounts/:id
```

获取单个邮箱账号的完整信息。

**路径参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| id | integer | 邮箱 ID |

## 创建邮箱

```
POST /api/v1/accounts
```

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 显示名称 |
| email | string | 否 | 邮箱地址（留空时自动复用 username）|
| host | string | 是 | IMAP 主机地址 |
| port | integer | 是 | IMAP 端口（通常 993 或 143）|
| username | string | 是 | IMAP 登录用户名 |
| password | string | 是 | IMAP 密码 |
| smtp_host | string | 否 | SMTP 主机地址 |
| smtp_port | integer | 否 | SMTP 端口 |

**示例**

```bash
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "name": "QQ邮箱",
    "email": "user@qq.com",
    "host": "imap.qq.com",
    "port": 993,
    "username": "user@qq.com",
    "password": "authorization-code"
  }'
```

## 更新邮箱

```
PUT /api/v1/accounts/:id
```

同创建接口字段，传入需要修改的字段即可（部分更新）。

## 删除邮箱

```
DELETE /api/v1/accounts/:id
```

删除指定邮箱及其关联的所有邮件数据。

## 测试连接

```
POST /api/v1/accounts/test-connection
```

验证 IMAP 连接参数是否正确，不保存数据。

**请求体** — 同创建邮箱的字段。

**响应**

```json
{ "code": 0, "data": { "success": true, "message": "连接成功" } }
```

## 手动同步

```
POST /api/v1/accounts/:id/sync
```

触发指定邮箱立即执行一次 IMAP 同步。
