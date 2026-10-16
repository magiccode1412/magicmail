# 附件接口

## 附件列表

```
GET /api/v1/attachments/mail/:mail_id
```

获取指定邮件的所有附件元信息。

**路径参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| mail_id | integer | 邮件 ID |

**响应**

```json
{
  "code": 0,
  "data": [
    {
      "id": 201,
      "filename": "report.pdf",
      "contentType": "application/pdf",
      "size": 1048576
    },
    {
      "id": 202,
      "filename": "data.xlsx",
      "contentType": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
      "size": 524288
    }
  ]
}
```

## 下载附件

```
GET /api/v1/attachments/:id/download
```

以二进制流形式下载附件文件。

**路径参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| id | integer | 附件 ID |

**响应**
- Content-Type: 对应文件的 MIME 类型
- Content-Disposition: attachment; filename="原始文件名"
- Body: 文件二进制内容

**示例（使用 curl 下载）**

```bash
curl -o report.pdf \
  "http://localhost:8080/api/v1/attachments/201/download" \
  -H "Authorization: Bearer <token>"
```
