# 草稿箱接口

草稿箱支持保存、编辑、删除邮件草稿。

## 草稿列表

```
GET /api/v1/drafts
```

返回当前用户的所有草稿（分页）。

## 保存草稿

```
POST /api/v1/drafts
```

创建新草稿或覆盖已有草稿。

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 否 | 草稿 ID（传入则为更新） |
| accountId | integer | 是 | 发件邮箱账号 ID |
| to | string[] | 是 | 收件人列表 |
| cc | string[] | 否 | 抄送列表 |
| bcc | string[] | 否 | 密送列表 |
| subject | string | 是 | 邮件主题 |
| body | string | 是 | HTML 邮件正文 |
| attachments | string[] |否 | 附件文件路径列表 |

## 草稿详情

```
GET /api/v1/drafts/:id
```

获取指定草稿的完整内容。

## 更新草稿

```
PUT /api/v1/drafts/:id
```

更新已有草稿内容。请求体同「保存草稿」。

## 删除草稿

```
DELETE /api/v1/drafts/:id
```

删除指定草稿。

## 批量删除草稿

```
POST /api/v1/drafts/batch-delete
```

批量删除多个草稿。

**请求体**

```json
{ "ids": [1, 2, 3] }
```
