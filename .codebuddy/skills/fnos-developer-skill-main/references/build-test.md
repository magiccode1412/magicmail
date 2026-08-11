# fnOS 构建、安装与排错参考

## 1. 工具

### fnpack

用途：创建项目并生成 `.fpk`。

```bash
fnpack create <appname>
fnpack create <appname> --without-ui true
fnpack create <appname> --template docker
fnpack create <appname> --template docker --without-ui true
fnpack build
fnpack build --directory <path>
```

`fnpack` 基础检查包括：

- `manifest`
- `config/privilege`
- `config/resource`
- `ICON.PNG`
- `ICON_256.PNG`
- `app/`
- `cmd/`
- `wizard/`
- 声明 `desktop_uidir` 时对应目录存在

官方工具下载版本会变化，下载前查看最新官方页面。

### appcenter-cli

在 fnOS 设备上使用：

```bash
appcenter-cli install-fpk myapp.fpk
appcenter-cli install-fpk myapp.fpk --env config.env
appcenter-cli install-local
appcenter-cli default-volume
appcenter-cli default-volume 1
appcenter-cli list
appcenter-cli start myapp
appcenter-cli stop myapp
```

手动交互测试优先使用应用中心界面；重复安装、CI 或脚本化测试使用 CLI。

## 2. 本地预检

```bash
python scripts/validate_fnos_project.py /path/to/project
fnpack build --directory /path/to/project
```

检查：

- JSON 文件可解析。
- `manifest` 字段与运行模型一致。
- UI 入口引用文件存在。
- 生命周期脚本存在且可执行。
- 没有明显 Token 泄露、root 过度使用或路径硬编码。
- 架构声明与二进制/镜像一致。

启发式脚本不能验证设备行为、Windows ACL、网关注册、系统依赖安装或宿主 JS SDK。

## 3. 首次安装测试

在干净或专用测试设备上：

1. 选择目标存储卷。
2. 安装 `.fpk`。
3. 完成向导。
4. 检查桌面入口、应用设置和启动状态。
5. 检查进程、端口或 Unix Socket。
6. 执行主流程。
7. 检查 `etc`、`var`、`tmp`、`home`、共享目录的数据落点。
8. 停止、启动、重启设备后复测。

静态 CGI：

- URL 是否为 `/cgi/ThirdParty/{appname}/index.cgi/`。
- `index.cgi` 是否可执行。
- 静态文件是否安装到预期 `target` 子目录。
- 404、非法路径和 MIME 是否正确。

统一网关：

- `gatewayPrefix` 是否注册。
- 服务是否在 `TRIM_APPDEST/{gatewaySocket}` 监听。
- 陈旧 Socket 是否清理。
- 页面和 API 是否在统一前缀下。
- WebSocket 是否能升级连接。
- 网关 Header 是否存在，普通用户与管理员行为是否不同。

## 4. 权限测试

至少测试：

- 普通用户无法执行管理员共享授权。
- 管理员可以添加/删除共享授权目录。
- 用户 A 的个人授权不被用户 B 查询或访问。
- 已授权目录中无读权限的子项不会被返回。
- 写入/删除分别受 `writable`/`deletable` 限制。
- 撤销授权后应用刷新状态并拒绝访问。
- 路径不存在、符号链接逃逸、`..`、编码变体均被拒绝。
- 客户端伪造 UID/管理员标记无效。

## 5. 开放 API 测试

### 正常路径

- 前端 `getPlatformConfig`。
- 宿主环境直接授权。
- 独立浏览器 `openAppAuth` 回调。
- 后端从 `TRIM_API_TOKEN` 调用 Unix Socket。
- 查询授权目录、ACL、路径转换。

### 失败路径

- 缺少 Scope -> `403 / 200003 Forbidden`。
- Token 无效或缺失 -> `401 / 200004 Unauthorized`。
- `req` 拼写错误或版本不支持 -> `404 / 200005 Not Found`。
- 参数类型错误 -> `200001 Invalid Params`。
- 普通用户调用共享授权 -> 权限拒绝或 `access_denied`。
- 回调 `state` 不匹配 -> 应用拒绝结果。
- `window.opener` 不存在 -> 用户可手动刷新授权状态。

## 6. 生命周期测试

### start

- 首次启动成功。
- 重复启动不产生多个实例。
- 依赖缺失时失败并写 `TRIM_TEMP_LOGFILE`。
- 端口占用/Socket 创建失败时清晰报错。

### status

- 运行返回 0。
- 未运行返回 3。
- PID 文件过期时不误报运行。

### stop

- 优雅停止。
- 超时后有受控的强制结束策略。
- 清理 PID 和 Socket。
- 重复停止可安全执行。

### upgrade

- 从每个支持的旧版本升级。
- 配置与数据迁移幂等。
- 失败后可恢复。
- 运行中的应用能按预期停止和重启。

### config

- 修改配置后服务正确重载或重启。
- 非法配置被拒绝，旧配置仍可用。

### uninstall

- 保留数据与删除数据两种选择符合预期。
- 不删除外部共享数据或其他应用数据。

## 7. 常见问题定位

### `fnpack build` 失败

优先检查：

- 必需目录/图标缺失。
- `manifest` 必需字段缺失或格式错误。
- `config/*.json` 非法。
- `desktop_uidir` 指向不存在目录。

### 桌面入口打不开

- 入口 ID/URL 与 `manifest.desktop_applaunchname` 是否一致。
- CGI 的应用名大小写与安装路径是否一致。
- 端口是否监听，协议是否正确。
- 网关 Socket 是否存在且服务正在监听。
- 入口 `allUsers` 与当前用户是否匹配。

### 应用显示运行但服务不可用

- `status` 是否只检查了过期 PID 文件。
- 服务是否启动后立即退出。
- 端口是否只绑定 `127.0.0.1` 或绑定了错误地址。
- Unix Socket 是否创建到错误目录。
- 运行用户是否有访问数据和 Socket 的权限。

### API `Unauthorized`

- 当前进程是否由 fnOS 应用脚本启动并继承 `TRIM_API_TOKEN`。
- 是否在启动后错误地缓存了旧 Token。
- Authorization 是否严格为 `Bearer <token>`。

### API `Forbidden`

- `config/resource` 是否声明精确 Scope。
- 重打包并重装后 Scope 是否生效。
- `req` 对应 Scope 是否正确。

### API `Not Found`

- `req` 拼写。
- 系统版本/宿主 App 版本。
- 接口是否属于当前开放能力。

### 授权成功但无法读文件

依次检查：

1. 这是共享授权、用户目录授权还是一次性文件授权。
2. 路径是否在授权范围。
3. 当前登录 UID 是否可信。
4. `checkUserACL` 是否 `readable=true`。
5. 应用运行用户是否具备底层 ACL。
6. 路径是否已移动、删除或被符号链接替换。

### 主题/语言监听无效

- 当前是否 Web 宿主。
- `sdk.isStandaloneWeb` 是否为 `false`。
- 初始化是否先调用 `getPlatformConfig`。
- 移动 App 内嵌页和独立浏览器不支持 `$on`，需要降级策略。

## 8. 发布前矩阵

至少覆盖：

- 计划支持的 fnOS 版本。
- x86/ARM 中实际声明的平台。
- 管理员与普通用户。
- 浅色/暗色、支持的界面语言。
- 首次安装、覆盖升级、卸载保留、卸载删除。
- 正常磁盘空间与低磁盘空间。
- 依赖在线/离线或不可用。
- 授权存在/撤销/路径删除。
- 设备重启、应用重启。

发布材料通常包括最终 `.fpk`、应用图标、真实界面截图和准确 `manifest`。上架流程可能变化，提交前查看最新官方页面。

## 官方来源

- https://developer.fnnas.com/docs/quick-started/prerequisites/
- https://developer.fnnas.com/docs/quick-started/create-application/
- https://developer.fnnas.com/docs/quick-started/test-application/
- https://developer.fnnas.com/docs/quick-started/publish-application/
- https://developer.fnnas.com/docs/cli/fnpack/
- https://developer.fnnas.com/docs/cli/appcentercli/
