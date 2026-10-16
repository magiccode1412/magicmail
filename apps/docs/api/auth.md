# 认证接口

## 用户注册

```
POST /api/v1/auth/register
```

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名 |
| password | string | 是 | 密码（至少 6 位）|

**示例**

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'
```

## 用户登录

```
POST /api/v1/auth/login
```

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名 |
| password | string | 是 | 密码 |

**响应示例**

```json
{
  "code": 0,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": 1,
      "username": "admin"
    }
  }
}
```

::: tip 后续请求
登录后获得的 `token` 需要在后续请求的 Header 中携带：
`Authorization: Bearer eyJhbGciOiJIUzI1NiIs...`
:::

## 检查登录状态

```
GET /api/v1/auth/status
```

需携带 JWT Token。

**响应示例**

```json
{
  "code": 0,
  "data": {
    "loggedIn": true,
    "user": { "id": 1, "username": "admin" }
  }
}
```
