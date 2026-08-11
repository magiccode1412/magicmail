# fnOS Developer Skill

这是一个面向大模型的飞牛 fnOS 开发 Skill，资料依据：

- https://developer.fnnas.com/docs/guide/
- https://developer.fnnas.com/api/overview/

资料快照日期：2026-08-06。

## 目录

```text
fnos-developer-skill/
├── SKILL.md
├── README.md
├── references/
│   ├── package-model.md
│   ├── open-api.md
│   ├── security-review.md
│   └── build-test.md
├── templates/
│   ├── node-trim-api-client.mjs
│   ├── frontend-auth-flow.ts
│   ├── cmd-main.sh
│   ├── manifest.template
│   ├── privilege.package.json
│   ├── resource.user-files.json
│   ├── ui.gateway.json
│   └── ui.cgi.json
└── scripts/
    └── validate_fnos_project.py
```

## 使用

将整个目录放入支持文件夹式 Skills 的大模型环境，并让模型加载 `SKILL.md`。模型应按任务需要读取 `references/`，而不是一次性加载所有资料。

审查已有工程：

```bash
python scripts/validate_fnos_project.py /path/to/fnos-project
```

输出 JSON：

```bash
python scripts/validate_fnos_project.py /path/to/fnos-project --json
```

校验脚本只做启发式检查。正式验证仍需：

```bash
fnpack build --directory /path/to/fnos-project
```

并在专用 fnOS 测试设备上安装、升级、权限和异常测试。

## 更新

飞牛开放平台仍可能新增能力或调整版本要求。更新本 Skill 时，优先核对官方文档中的：

- `fnpack`/`appcenter-cli` 当前版本与命令。
- `manifest`、资源和向导字段。
- 开放 API 方法、Scope、最低系统版本与宿主 App 版本。
- 错误码和上架流程。
