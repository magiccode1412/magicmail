# fnOS 应用包与运行模型参考

> 官方资料快照：2026-08-06。版本敏感字段以最新官方文档为准。

## 1. 安装后的目录模型

应用安装后位于 `/var/apps/{appname}`，常见结构：

```text
/var/apps/{appname}
├── cmd/
├── config/
├── manifest
├── ICON.PNG
├── ICON_256.PNG
├── target -> /vol{n}/@appcenter/{appname}
├── etc    -> /vol{n}/@appconf/{appname}
├── var    -> /vol{n}/@appdata/{appname}
├── tmp    -> /vol{n}/@apptemp/{appname}
├── home   -> /vol{n}/@apphome/{appname}
├── meta
├── shares/
└── wizard/
```

语义：

| 目录 | 用途 |
|---|---|
| `target` | 已安装的应用文件与运行资源 |
| `etc` | 配置 |
| `var` | 重启后保留的运行数据 |
| `tmp` | 临时文件 |
| `home` | 应用用户数据 |
| `shares` | `config/resource` 声明的共享目录 |
| `cmd` | 生命周期脚本 |
| `wizard` | 安装、升级、卸载、配置表单 |

不要硬编码安装卷和安装路径。优先使用：

- `TRIM_APPDEST`
- `TRIM_PKGETC`
- `TRIM_PKGVAR`
- `TRIM_PKGTMP`
- `TRIM_PKGHOME`
- `TRIM_PKGMETA`
- `TRIM_APPDEST_VOL`

## 2. manifest

`manifest` 位于包根目录，无扩展名，采用键值格式，不是 JSON。

主要字段：

| 字段 | 含义与约束 |
|---|---|
| `appname` | 应用唯一标识，升级间保持稳定 |
| `version` | 例如 `1.0.0`、`2.1.3-beta` |
| `display_name` | 用户可见名称 |
| `desc` | 应用描述 |
| `source` | 第三方应用使用 `thirdparty` |
| `platform` | `x86`、`arm`、`all`；只有无架构相关二进制时才用 `all` |
| `maintainer` / `maintainer_url` | 开发者信息 |
| `distributor` / `distributor_url` | 可选发布者信息 |
| `os_min_version` / `os_max_version` | 按真实兼容与测试范围声明 |
| `ctl_stop` | 是否显示启动、停止和状态控制 |
| `install_type` | 空值由用户选卷，`root` 安装到系统分区 |
| `install_dep_apps` | 直接依赖；多个依赖用 `:`，最低版本用 `>` |
| `desktop_uidir` | UI 目录，默认 `ui` |
| `desktop_applaunchname` | 应用卡片打开的入口 ID |
| `service_port` | 应用服务端口 |
| `checkport` | 启动前是否检查端口，默认 `true` |
| `disable_authorization_path` | 是否隐藏应用设置中的授权目录设置 |
| `changelog` | 面向用户的更新说明 |
| `micro_app` | 使用开放平台 JS SDK 时设置 `true` |

最小示例：

```ini
appname=MyApp
version=1.0.0
display_name=My App
desc=My fnOS application.
source=thirdparty
platform=all
maintainer=Example Team
maintainer_url=https://example.com
os_min_version=1.2.0
desktop_uidir=ui
desktop_applaunchname=MyApp.main
ctl_stop=false
checkport=false
```

注意：开放 API 文档中的首期能力当前标注最低系统版本 `1.2.0401`、宿主 App 版本 `1.34.0`。使用这些能力时，`os_min_version` 应至少与实际要求一致，并在目标宿主上验证。

## 3. 生命周期脚本

| 脚本 | 时机 |
|---|---|
| `install_init` | 安装应用文件前 |
| `install_callback` | 安装应用文件后 |
| `main` | `start`、`stop`、`status` |
| `upgrade_init` | 升级前 |
| `upgrade_callback` | 升级后 |
| `uninstall_init` | 卸载前 |
| `uninstall_callback` | 卸载清理后 |
| `config_init` | 配置变更应用前 |
| `config_callback` | 配置变更应用后 |

要求：

- 尽量幂等。
- `status`：运行返回 `0`，未运行返回 `3`。
- 普通失败返回非零，常用 `1`。
- 用户可见错误先写 `TRIM_TEMP_LOGFILE`。
- 升级脚本用于数据迁移、配置迁移和兼容检查。
- 卸载逻辑不得擅自删除用户数据。

## 4. 环境变量

### 应用信息

- `TRIM_APPNAME`
- `TRIM_APPVER`
- `TRIM_OLD_APPVER`
- `TRIM_APP_STATUS`

### 用户与权限

- `TRIM_USERNAME` / `TRIM_GROUPNAME`
- `TRIM_UID` / `TRIM_GID`
- `TRIM_RUN_USERNAME` / `TRIM_RUN_GROUPNAME`
- `TRIM_RUN_UID` / `TRIM_RUN_GID`

### 网络与资源

- `TRIM_SERVICE_PORT`
- `TRIM_DATA_SHARE_PATHS`，多个路径以 `:` 分隔
- `TRIM_DATA_ACCESSIBLE_PATHS`，多个路径以 `:` 分隔
- `TRIM_API_TOKEN`

### 日志和安装上下文

- `TRIM_TEMP_LOGFILE`
- `TRIM_TEMP_UPGRADE_FOLDER`
- `TRIM_PKGINST_TEMP_DIR`
- `TRIM_TEMP_TPKFILE`

### 系统上下文

- `TRIM_SYS_VERSION`
- `TRIM_SYS_VERSION_MAJOR`
- `TRIM_SYS_VERSION_MINOR`
- `TRIM_SYS_VERSION_BUILD`
- `TRIM_SYS_ARCH`
- `TRIM_KERNEL_VERSION`
- `TRIM_SYS_MACHINE_ID`
- `TRIM_SYS_LANGUAGE`

向导字段会以同名环境变量提供，不加 `TRIM_` 前缀。

## 5. 权限模型

默认使用包用户：

```json
{
  "defaults": {
    "run-as": "package"
  },
  "username": "myapp_user",
  "groupname": "myapp_group"
}
```

`join-groups` 仅用于真实需要的系统组，例如视频、渲染或设备访问。每增加一个组都扩大访问面。

Root 模式只用于没有更窄方案的特权准备任务。长期服务应尽量降权，例如：

```bash
runuser -u "$TRIM_USERNAME" -- "$TRIM_APPDEST/server/myapp"
```

## 6. config/resource

### 共享数据目录

```json
{
  "data-share": {
    "shares": [
      { "name": "myapp/documents" },
      { "name": "myapp/backups" }
    ]
  }
}
```

共享目录采用 Windows ACL 模型。系统为应用运行用户授予所需 ACL。可通过 `TRIM_DATA_SHARE_PATHS` 或 `/var/apps/myapp/share/` 软链接访问。

仅当其他用户或应用确实需要时设置 `permission.rw` / `permission.ro`。

### 系统链接

```json
{
  "usr-local-linker": {
    "bin": ["bin/myapp-cli"],
    "lib": ["lib/mylib.so"],
    "etc": ["etc/myapp.conf"]
  }
}
```

分别链接到 `/usr/local/bin/`、`/usr/local/lib/`、`/usr/local/etc/`。名称必须避免与系统或其他应用冲突。

### Docker 项目

```json
{
  "docker-project": {
    "projects": [
      {
        "name": "myapp-stack",
        "path": "docker"
      }
    ]
  }
}
```

`path` 相对于 `app`，目录中应包含 `docker-compose.yaml`。

### API Scope

```json
{
  "api-scope": [
    "trim.file.userAccess",
    "trim.file.userAcl"
  ]
}
```

可与其他资源项合并在同一 JSON 对象中。

## 7. 应用入口

入口配置通常位于 `app/ui/config`，入口定义在 `.url` 下。

重要字段：

- `title`
- `icon`
- `type`: `iframe` 或 `url`
- `protocol`
- `port`
- `url`
- `allUsers`
- `fileTypes`
- `noDisplay`
- `control.accessPerm`: `editable`、`readonly`、`hidden`
- `gatewayPrefix`
- `gatewaySocket`

端口入口示例：

```json
{
  ".url": {
    "myapp.main": {
      "title": "My App",
      "icon": "images/icon_{0}.png",
      "type": "iframe",
      "protocol": "http",
      "port": "8080",
      "url": "/",
      "allUsers": true
    }
  }
}
```

文件入口会收到 `path` 查询参数。必须重新验证授权、ACL 和路径边界。

## 8. index.cgi

适合静态页面和轻量请求，不支持 WebSocket。每次请求启动 CGI 进程，不适合高流量、长耗时或复杂 API。

常见路径：

```text
/cgi/ThirdParty/{appname}/index.cgi/
```

安全要求：

- 拒绝目录穿越。
- 仅从预期应用目录提供文件。
- 不直接执行请求参数。
- 不把请求路径拼入 shell 命令。
- 使用真实路径/规范化路径验证最终目标仍在允许根目录中。

## 9. 统一网关

统一网关把 fnOS 域名下的路径转发到应用本地 Unix Socket，并在转发前校验用户会话。

配置示例：

```json
{
  ".url": {
    "myapp.main": {
      "title": "My App",
      "icon": "images/icon_{0}.png",
      "type": "iframe",
      "protocol": "",
      "gatewayPrefix": "/app/myapp",
      "gatewaySocket": "app.sock",
      "url": "/app/myapp",
      "allUsers": true
    }
  }
}
```

转发目标：

```text
/var/apps/myapp/target/app.sock
```

规则：

- `gatewayPrefix` 用 `/app/{appname}` 或稳定子路径。
- 公开路径中避免点号。
- `gatewaySocket` 只填文件名。
- Socket 位于 `target`，脚本用 `TRIM_APPDEST` 定位。
- `protocol` 和 `port` 被忽略。
- HTTP 与 WebSocket 路由保持在声明的前缀下。

可信网关 Header：

- `X-Trim-Userid`
- `X-Trim-Isadmin`
- `X-Trim-Username`

网关只确认登录状态，应用仍负责业务权限。不要信任客户端在请求体、查询参数或 WebSocket 消息中提供的身份字段。

## 10. 用户向导

文件：

- `wizard/install`
- `wizard/upgrade`
- `wizard/uninstall`
- `wizard/config`

每个文件是步骤数组：

```json
[
  {
    "stepTitle": "Setup",
    "items": [
      {
        "type": "text",
        "field": "wizard_port",
        "label": "Service port",
        "rules": [
          { "required": true, "message": "Enter a port" },
          { "pattern": "^[0-9]+$", "message": "Use numbers only" }
        ]
      }
    ]
  }
]
```

字段类型：`text`、`password`、`radio`、`checkbox`、`select`、`switch`、`tips`。

只收集无法安全自动检测或默认处理的必要信息。敏感字段不要放进仓库中的测试环境文件。

## 11. 依赖、运行时和中间件

`install_dep_apps` 只声明直接依赖。依赖检查不递归；多个依赖按官方规定从右到左准备。

示例：

```ini
install_dep_apps=database>2.2.2:cache
```

官方文档示例运行时：

- `python312`
- `nodejs_v22`
- `java-21-openjdk`

调用前将相应 `target/bin` 加入 `PATH`。

官方文档示例中间件：

- `redis`
- `minio`
- `rabbitmq`

使用应用级命名空间，凭据不得硬编码或提交仓库。

## 12. 图标

根目录：

- `ICON.PNG`: 64 x 64 px
- `ICON_256.PNG`: 256 x 256 px

入口图标通常：

- `app/ui/images/icon_64.png`
- `app/ui/images/icon_256.png`

设计要求包括 sRGB、正方形、单文件不超过 1024 KB，64 px 下仍可辨识。

## 官方来源

- https://developer.fnnas.com/docs/core-concepts/framework/
- https://developer.fnnas.com/docs/core-concepts/manifest/
- https://developer.fnnas.com/docs/core-concepts/environment-variables/
- https://developer.fnnas.com/docs/core-concepts/privilege/
- https://developer.fnnas.com/docs/core-concepts/resource/
- https://developer.fnnas.com/docs/core-concepts/app-entry/
- https://developer.fnnas.com/docs/core-concepts/index-cgi/
- https://developer.fnnas.com/docs/core-concepts/gateway-registration/
- https://developer.fnnas.com/docs/core-concepts/wizard/
- https://developer.fnnas.com/docs/core-concepts/dependency/
- https://developer.fnnas.com/docs/core-concepts/middleware/
- https://developer.fnnas.com/docs/core-concepts/runtime/
- https://developer.fnnas.com/docs/core-concepts/icon/
