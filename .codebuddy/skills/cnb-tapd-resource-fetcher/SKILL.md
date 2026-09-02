---
name: cnb-tapd-resource-fetcher
description: 获取 TAPD 资源（需求/缺陷/任务/迭代），支持从 TAPD 链接自动解析
---

# 获取 TAPD 资源

从用户提供的 TAPD 链接中提取参数，调用 API 获取资源数据。

## 1. 解析链接

格式：`https://www.tapd.cn/tapd_fe/{workspace_id}/{type}/detail/{id}`

- `type`: story / bug / task / iteration

## 2. 请求

```bash
curl -s "$CNB_API_ENDPOINT/tapd/reference/{workspace_id}/{type}/{id}" \
  -H "Accept: application/vnd.cnb.api+json" \
  -H "Authorization: Bearer $CNB_TOKEN"
```

多个链接时逐个请求，汇总后统一展示。

## 3. 响应处理

- HTTP 2xx 且 `data` 非空 → 返回 `data` 内容
- HTTP 2xx 但 `data` 为 `[]` → 提示"未找到该资源，请检查资源 ID"
- HTTP >= 300 → 按 `errcode` 提示用户（返回 `status` 和 `trace`）：

| errcode | 提示 |
|---------|------|
| `2000076` | 项目未启用 TAPD 集成，请联系管理员开启 |
| `2000073` | 未绑定 TAPD 账号，请前往个人设置绑定 |
| `2000074` | TAPD 授权已过期，请重新绑定 |
| `2000075` | 无权限或资源 ID 错误，请检查链接 |
| `401` | 请先登录 CNB 平台 |
| `403` / `404` | 无权限或资源不存在 |
| 其他 | 请求失败，请重试或联系管理员并提供 trace |

注意：不要向用户暴露错误堆栈等内部细节。
