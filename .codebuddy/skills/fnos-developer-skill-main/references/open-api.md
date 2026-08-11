# fnOS 开放 API 参考

> 官方资料快照：2026-08-06。当前文档将本批能力标为系统版本 `1.2.0401`、宿主 App 版本 `1.34.0` 起支持。上线前核对最新文档与目标设备。

## 1. 接入层次

### 前端 JS SDK

用于需要用户交互或宿主页面能力的场景：

- 文件/目录选择与授权。
- 打开文件、文件详情、文件管理器、应用设置、外部 URL。
- 读取平台配置。
- 设置标题、离开提示、关闭页面。
- 在受支持的 Web 宿主中监听主题和语言变化。

安装与初始化：

```bash
npm install @trimjs/web-app
```

```ts
import { TrimApp } from '@trimjs/web-app';
const sdk = new TrimApp();
```

使用 JS SDK 时，`manifest` 声明：

```ini
micro_app=true
```

### 后端 API

固定通过本地 Unix Socket 调用：

```text
POST /api/v1/trimapp
Unix Socket: /var/run/trim_open_gateway_apiscope.socket
Content-Type: application/json
Authorization: Bearer <token>
```

Token 来自当前进程环境变量 `TRIM_API_TOKEN`。不要申请、生成、持久化或暴露此 Token。

请求：

```json
{
  "reqId": "string",
  "req": "trim.system.getPlatformConfig",
  "appName": "your-app",
  "data": {}
}
```

响应：

```json
{
  "reqId": "string",
  "code": 0,
  "msg": "",
  "data": {}
}
```

`code=0` 表示业务成功；仍应检查 HTTP 状态码。

## 2. Scope 映射

在 `config/resource` 顶层声明：

```json
{
  "api-scope": [
    "trim.file.userAccess",
    "trim.file.userAcl"
  ]
}
```

| 能力 | Scope |
|---|---|
| 管理员应用共享授权 | `trim.file.sharedAccess` |
| 当前用户个人授权 | `trim.file.userAccess` |
| 当前用户文件 ACL 检查 | `trim.file.userAcl` |
| 内部路径转语义路径 | `trim.file.path` |
| 后端读取平台配置 | `trim.system.getPlatformConfig` |

只声明实际使用的 Scope。声明 Scope 不等于获得任意文件访问权。

## 3. 运行环境分支

SDK 属性：

| 属性 | 含义 |
|---|---|
| `isWeb` | 当前是否 Web 环境；移动 App 内嵌页通常为 `false` |
| `isStandaloneWeb` | 是否独立浏览器页面；为 `true` 时无宿主注入能力 |

授权类能力：

- `isStandaloneWeb=false`：直接调用 `pickSharedFile`、`pickUserFile` 等。
- `isStandaloneWeb=true`：由用户点击触发 `openAppAuth`，使用 `redirectUri` 接收结果。

推荐使用统一网关，让应用页与回调页同域。回调必须：

1. `parseAppAuthCallback(window.location.href)`。
2. 校验业务 `state`。
3. `postMessage` 时使用精确同源目标。
4. 原页面检查 `event.origin` 与消息类型。
5. 保留手动“刷新授权状态”按钮，应对移动浏览器 `window.opener` 缺失或 `window.close()` 失效。

不要在页面加载、定时器或非用户手势回调中自动打开授权页。

`$on` 仅在以下条件使用：

```ts
sdk.isWeb === true && sdk.isStandaloneWeb === false
```

## 4. 平台配置

### 前端

```ts
getPlatformConfig(): Promise<PlatformConfig>
```

返回可能包含：

```ts
interface PlatformConfig {
  theme: 'dark' | 'light';
  language: string;
  systemLanguage?: string;
  appVersion?: string;
  systemVersion: string;
  format: {
    date?: string;
    time?: string;
  };
}
```

页面初始化优先用前端方法。

### 后端

Scope：`trim.system.getPlatformConfig`

```json
{
  "req": "trim.system.getPlatformConfig",
  "appName": "your-app",
  "data": {}
}
```

后端返回系统语言和系统版本，用于后端渲染或兼容逻辑。

## 5. 管理员应用共享授权

适用：管理员为应用配置一批固定目录，内容不按使用用户区分。

限制：

- 只有管理员可操作。
- 只支持目录，不支持文件。

Scope：`trim.file.sharedAccess`

### 前端方法

```ts
pickSharedFile(params?: SharedFilePickerParams)
authorizeSharedFile(path: string)
```

`SharedFilePickerParams`：

```ts
interface SharedFilePickerParams {
  title?: string;
  okText?: string;
  sidebarGroup?: SidebarGroup[];
  creatable?: boolean;
  disabledPaths?: string[];
}
```

### 后端查询

```json
{
  "req": "trim.file.getSharedAccessibleFolders",
  "appName": "your-app",
  "data": {}
}
```

成功数据：

```json
{
  "paths": ["/vol1/1000/data"]
}
```

### 后端删除

```json
{
  "req": "trim.file.delSharedAccessibleFolder",
  "appName": "your-app",
  "data": {
    "path": "/vol1/1000/data"
  }
}
```

普通用户调用共享授权，宿主直调可能返回 `code: 1` 和“仅管理员可进行此操作”；跳转回调可能返回 `access_denied`。

## 6. 用户个人授权

适用：不同用户访问不同目录或文件。建议结合统一网关获取当前用户 UID。

Scope：`trim.file.userAccess`

### 前端选择/授权

```ts
pickUserFile(params?: FilePickerParams)
authorizeUserFile(path: string)
```

```ts
interface FilePickerParams {
  multiple?: boolean;
  directory?: boolean;
  accept?: string[];
  sidebarGroup?: SidebarGroup[];
  title?: string;
  okText?: string;
  creatable?: boolean;
  disabledPaths?: string[];
}
```

目录授权：

- 设置 `directory: true`。
- 目录只支持单选。
- 后端可按 UID 查询目录授权。

文件授权：

- 设置 `directory: false`。
- 可用 `accept: ['.png', '.jpeg']` 限制扩展名。
- 文件授权结果直接使用选择器或授权回调返回的路径。
- 文件授权不会出现在 `getUserAccessibleFolders` 的目录列表中。

### 后端查询目录授权

UID 必须来自可信网关上下文，而不是客户端参数：

```json
{
  "req": "trim.file.getUserAccessibleFolders",
  "appName": "your-app",
  "data": {
    "uid": 1000
  }
}
```

### 后端删除目录授权

```json
{
  "req": "trim.file.delUserAccessibleFolder",
  "appName": "your-app",
  "data": {
    "uid": 1000,
    "path": "/vol1/home/user"
  }
}
```

## 7. 文件 ACL 检查

授权让应用具备访问路径的能力，不代表当前登录用户可读写该路径。向用户返回文件内容或执行写操作前，检查用户 ACL。

Scope：`trim.file.userAcl`

```json
{
  "req": "trim.file.checkUserACL",
  "appName": "your-app",
  "data": {
    "uid": 1000,
    "path": [
      "/vol1/1000/data/test.txt",
      "/vol1/1000/data/demo"
    ]
  }
}
```

`path` 可为字符串或数组。返回：

```json
[
  {
    "path": "/vol1/1000/data/test.txt",
    "readable": true,
    "writable": false,
    "deletable": false
  }
]
```

规则：

- 列表、预览、下载：`readable`。
- 创建/修改：`writable`。
- 删除：`deletable`。
- 路径不存在或无法读取状态时，全部按 `false` 处理。

ACL 检查不替代路径授权范围和应用自己的业务权限。

## 8. 路径转换

页面不要直接展示 `/vol1/...`。

Scope：`trim.file.path`

```json
{
  "req": "trim.file.convertPath",
  "appName": "your-app",
  "data": {
    "path": [
      "/vol1/1000/photo",
      "/vol1/1000/demo.pdf"
    ],
    "language": "zh-CN"
  }
}
```

`language` 必填，并应使用当前界面语言。返回每个原始 `path` 对应的 `semanticPath`。

## 9. 页面路由

以下均为前端 JS SDK，无 Scope：

```ts
openFile(path: string)
showFileDetails(paths: string[], options?: object)
openFileManager(path: string)
openAppSetting()
openURL(url: string, target?: string, features?: string)
```

调用 `openFile`、`showFileDetails`、`openFileManager` 前，应用仍需保证路径来自允许范围，并按业务需要检查 ACL。

`openURL` 在 Web 宿主与移动 WebView 中行为不同，不要假设一定保留原窗口或可自动返回。

## 10. 页面交互

以下均为前端 JS SDK，无 Scope：

```ts
setTitle(title: string)
$on('os/theme', callback)
$on('os/language', callback)
setExitPageTips(params?: { title?: string; content?: string })
close()
```

流程建议：

- 初始化先 `getPlatformConfig()`。
- 仅在受支持 Web 宿主里注册 `$on`。
- 有未保存内容时设置离开提示；保存后调用无参数 `setExitPageTips()` 清除。

## 11. SidebarGroup

```ts
type SidebarGroup =
  | 'myFiles'
  | 'otherShare'
  | 'external'
  | 'remote'
  | 'favorites'
  | 'team';
```

含义依次为我的文件、他人共享、外接存储、远程挂载、收藏、团队空间。

## 12. 错误处理

### JS SDK 常见错误

| code | 含义 | 建议 |
|---:|---|---|
| `0` | 成功 | 正常处理 |
| `1000000` | 服务或内部异常 | 刷新状态后重试 |
| `1000001` | 登录/认证失败 | 重新登录或认证 |
| `1000002` | 管理员权限或 Scope 不足 | 检查权限与 Scope |
| `1000030` | 请求非法或路径不支持 | 检查参数、路径类型、能力状态 |
| `1000300` | 未找到已安装应用 | 确认应用安装并运行 |
| `1000701` | 路径不存在 | 让用户重新选择 |
| `1003103` | 应用权限校验失败 | 检查安装与权限，必要时重装应用 |
| `1003201` | 管理员关闭普通用户授权能力 | 提示需要管理员 |

共享授权的普通用户权限不足还可能以 `code: 1` 或跳转回调 `access_denied` 表示。

### 后端 API 常见错误

| HTTP | code | msg | 排查方向 |
|---:|---:|---|---|
| `200/400` | `200001` | `Invalid Params` | JSON、字段、类型、必填项 |
| `401` | `200004` | `Unauthorized` | 当前进程是否有有效 Token |
| `403` | `200003` | `Forbidden` | Scope 是否声明，Token 是否含 Scope |
| `404` | `200005` | `Not Found` | `req` 拼写、接口注册、系统版本 |
| `200/500` | `200006` | `Internal Error` | 业务模块内部错误，记录 reqId |

处理顺序：HTTP 状态 -> `code` -> `msg` -> `reqId` -> Scope/Token/版本/参数。

## 13. 后端调用安全模板

最低要求：

- Unix Socket 固定为 `/var/run/trim_open_gateway_apiscope.socket`。
- 请求路径固定为 `/api/v1/trimapp`。
- Token 每次从环境读取。
- 不允许前端控制任意 `req`；服务端使用白名单。
- 对 `uid`、路径、语言等参数做类型和边界验证。
- 设置合理超时和响应体大小限制。
- 日志脱敏。

见 `templates/node-trim-api-client.mjs`。

## 官方来源

- https://developer.fnnas.com/api/calling/
- https://developer.fnnas.com/api/platform-config/
- https://developer.fnnas.com/api/authorization/overview/
- https://developer.fnnas.com/api/authorization/shared-access/
- https://developer.fnnas.com/api/authorization/user-access/
- https://developer.fnnas.com/api/authorization/file-acl/
- https://developer.fnnas.com/api/authorization/path-convert/
- https://developer.fnnas.com/api/page/routing/
- https://developer.fnnas.com/api/page/ui/
- https://developer.fnnas.com/api/error-codes/
