# API 概览

Magicmail 提供完整的 RESTful API，基础路径为 `/api/v1`。

> 除 `/auth/*` 接口外，所有接口均需在请求头中携带 **JWT Token**：
>
> ```
> Authorization: Bearer <your_jwt_token>
> ```

## 基础信息

| 项目 | 值 |
|------|-----|
| Base URL | `http://localhost:8080/api/v1` |
| 认证方式 | JWT Bearer Token |
| 数据格式 | JSON (Content-Type: application/json) |
| 字符编码 | UTF-8 |

## 通用响应格式

### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

### 错误响应

```json
{
  "code": 40001,
  "message": "参数错误：邮箱地址不能为空"
}
```

### 分页列表响应

```json
{
  "code": 0,
  "data": {
    "items": [ ... ],
    "total": 128,
    "page": 1,
    "pageSize": 20
  }
}
```

## 接口一览

| 分组 | 说明 | 链接 |
|------|------|------|
| 认证 | 注册 / 登录 / 状态检查 | [查看 →](/api/auth) |
| 邮箱管理 | IMAP 账号 CRUD + 启停 | [查看 →](/api/accounts) |
| 邮件管理 | 邮件 CRUD / 发信 / 批量删除 / SSE 推送流 | [查看 →](/api/mails) |
| 草稿箱 | 草稿 CRUD / 批量删除 | [查看 →](/api/drafts) |
| 附件 | 附件列表与下载（含懒加载） | [查看 →](/api/attachments) |
| Webhook | Webhook 管理 | [查看 →](/api/webhooks) |
| SSE 实时推送 | Server-Sent Events 邮件更新流 | [查看 →](/api/sse) |
| Web Push 浏览器推送 | VAPID 订阅 / 取消订阅 / 测试推送 | [查看 →](/api/push) |

## 健康检查

无需认证：

```bash
curl http://localhost:8080/health
```

```json
{ "status": "ok" }
```
