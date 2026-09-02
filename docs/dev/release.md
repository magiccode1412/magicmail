# 发布流程

本文档描述 Magicmail 从开发到发布的完整流程，以及其中几个容易踩坑的机制（tag 归属校验、cnb → GitHub 同步、Release 正文来源、`version.json` 部署时机）。

## 分支模型

| 分支 | 角色 | 说明 |
|------|------|------|
| `dev` | 日常开发 | 持续合入，随时可能包含未完成的功能 |
| `main` | **发布冻结窗口**（等价于 release branch） | 只在发版时接收 `dev` 的合并，合并即进入冻结 |
| `gh-pages` | 文档站 | `deploy.yml` 自动部署，不需要手动操作 |

**为什么 `main` 是 release branch 而不是普通主干**：`main` 上的提交历史远短于 `dev`，只在发版时才推进。这意味着「把 `dev` 合并进 `main`」这个动作本身就完成了**代码冻结** —— 之后 `dev` 可以继续开发下一个版本，不会污染当前发布窗口。这正是 release branch 的核心作用，所以本项目不再单独切 `release/x.y` 分支。

代码仓库的主副本在 **cnb**（`origin` 指向 `cnb.cool`），GitHub 是由 cnb 单向同步出去的镜像。所有开发和打 tag 都在 cnb 侧进行。

## 版本号管理

版本号散落在四处，发布前必须保持一致：

| 文件 | 字段 | 用途 | `version.js` 是否自动更新 |
|------|------|------|--------------------------|
| `web/package.json` | `version` | 前端 `__APP_VERSION__`，页面显示与更新检测比对 | ✅ |
| `version.json` | `latest` | 更新检测的远端版本号，部署到 EdgeOne Pages | ✅ |
| `docs/guide/changelog.md` | `## [vX.Y.Z]` | **Release 正文的唯一来源** | ✅ 只加标题，内容需手填 |
| `fnapp/manifest` | `version` | FPK 包内版本，飞牛应用中心识别 | ❌ **必须手动改** |

> **注意**：`node version.js` 不会更新 `fnapp/manifest`。升版本后请手动同步，否则会出现「文件名是 `magicmail-1.2.0-x86.fpk`、包内版本却是 1.1.1」的不一致，飞牛应用中心会拒绝覆盖安装。

### 升版本号

```bash
node version.js minor "新增飞牛 ARM64 安装包"   # 或 major / patch
node version.js minor --dry-run                 # 预览不写入
node version.js --current                       # 查看当前版本
```

执行后会更新上表前三处。随后**必须**手工完成两件事：

1. 把 `fnapp/manifest` 的 `version` 改成同一个版本号
2. 把 `docs/guide/changelog.md` 里新增的 `## [vX.Y.Z]` 条目内容补全（分类写「安全 / 重构 / 新增 / 修复」）

`version.js` 最后会提示 `git tag` 和 `git push --tags`，**按本文档的流程走时忽略它的提示** —— tag 要打在 `main` 上而不是当前分支，且 `version.json` 必须最后部署。

## 发布步骤

```
  dev 分支                          main 分支（冻结窗口）
  ─────────                         ────────────────────
  升版本号 + 补 changelog
        │
        │  git commit
        ▼
  node version.js minor ──────►  git checkout main
  改 fnapp/manifest                git merge dev        ← 代码冻结
        │                          git push origin main
        │                                │
        │                                ▼
        │                       打 tag v1.2.0-rc.1
        │                                │
        │                                ▼
        │                       验收 rc（产物/FPK/镜像/正文）
        │                                │
        │                         有问题：修复提交直接进 main，
        │                                 打 v1.2.0-rc.2 …
        │                                │
        │                         通过 ▼
        │                       打 tag v1.2.0
        │                                │
        │                                ▼
        │                       验收正式版
        │                                │
        │                                ▼
        │                       部署 version.json   ← 必须最后一步
        │                                │
        ◄─────── 回灌 dev ───────────────┘
        git merge --ff-only main
```

### 1. 合并 `dev` → `main`（进入冻结窗口）

```bash
git checkout main
git merge dev                 # 有冲突以 dev 为准：git merge -X theirs dev
git push origin main
```

合并即冻结。此后到正式发布之间，`main` 只接收**稳定性修复**，新功能一律留在 `dev`。

### 2. 在 `main` 上打 rc 标签

```bash
git tag -a v1.2.0-rc.1 -m "release: v1.2.0-rc.1"
git push origin v1.2.0-rc.1
```

推送后 cnb 的 `tag_push` 流水线会把 tag 同步到 GitHub，GitHub Actions 随即开始构建（约 1 分钟开始，Release 约 5 分钟完成，Docker 多架构约 20 分钟）。

版本号带 `-` 后缀会被自动标记为 **Pre-release**，不会进入 Latest，也不会产生 Docker 浮动标签。

出问题就直接在 `main` 上提交修复，然后打 `v1.2.0-rc.2`，依此类推。

### 3. 打正式版标签

rc 验收通过后，在 `main` 上打正式标签：

```bash
git tag -a v1.2.0 -m "release: v1.2.0" -m "本次变更摘要"
git push origin v1.2.0
```

### 4. 验收

见下方[验收清单](#验收清单)。

### 5. 部署 `version.json`（必须最后一步）

`version.json` 是更新检测的**唯一**来源（前端 `useUpdateCheck` 只比对它的 `latest` 字段，与 GitHub Release 无关）。部署到 EdgeOne Pages 后，所有线上实例会立刻弹出更新提示。

```bash
# 确认 Release 与镜像都就绪后再部署
cat version.json    # latest / releaseDate / downloadUrl
```

**必须等 Docker 镜像推送完成后再部署**。否则用户看到「发现新版本 v1.2.0」，去 `docker pull …:latest` 却仍拉到旧的 1.1.1 镜像（应用内下载走 GitHub Release 不受影响，但 Docker 用户会踩空）。

### 6. 回灌 `dev`

```bash
git checkout dev
git merge --ff-only main
git push origin dev
```

## 关键机制

### tag 必须打在 `main` 上

`release.yml` 的 `guard` job 和 `docker-publish.yml` 的「校验 tag 位置」步骤会用 `git merge-base --is-ancestor HEAD origin/main` 强制校验，**打在 `dev` 上会直接红叉**。

原因：rc 的语义是「功能冻结、只收稳定性修复的候选版本」。`dev` 的定义是持续合入，在 `dev` 上打 rc 等于宣布「当前这个随时在变的状态就是发布候选」，测试者拿不到可复现的锚点，也说不清 rc.2 相对 rc.1 到底改了什么。

手动触发（`workflow_dispatch`）补发时，必须显式填写版本号，且只能在 `main` 分支执行。

### tag 同步链路（cnb → GitHub）

```
cnb 打 tag ──► cnb tag_push 流水线 ──► GitHub 收到 tag ──► GitHub Actions 构建
             (tencentcom/git-sync)
```

两个曾经出问题的点，都已修复：

- `git-sync` 的 `push_tags` 默认为 `false`，只同步分支不同步标签 → 已在 `.cnb.yml` 显式开启
- cnb 的 `push` 事件**只在分支推送时触发**，推 tag 不触发 → `.cnb.yml` 已单独配置 `tag_push` 事件

如果推了 tag 但 GitHub 长时间没反应，先去 cnb 看 `tag_push` 流水线是否成功；作为兜底，向 `main` 推任意一个提交也会触发全量同步（会带上所有 tag）。

### Release 正文来自 `changelog.md`

正文由 `release.yml` 从 `docs/guide/changelog.md` 中提取 `## [vX.Y.Z]` 到下一个 `## [` 之间的内容。

**不用 `git log` 自动生成的原因**：`main` 只在发版时接收 `dev` 合并，历史很短。`git describe --tags HEAD^` 取到的是 merge commit 的第一个父，此时 `dev` 上的几十个提交对它不可达，自动生成的 changelog 只有一两条。

因此：**`changelog.md` 里对应版本的条目没写，Release 正文就是空的**（会回退成 `- 本次无详细变更记录`）。升版本后请务必补全。

### Docker 浮动标签

| 标签 | 正式版 `v1.2.0` | 预发布 `v1.2.0-rc.1` |
|------|----------------|---------------------|
| `1.2.0` / `1.2.0-rc.1` | ✅ | ✅ |
| `1.2` | ✅ | ❌ |
| `1` | ✅ | ❌ |
| `latest` | ✅ | ❌ |

由 `docker-publish.yml` 中的 `is_release` 判断控制：只有「tag 推送且版本号不含 `-`」才生成浮动标签，避免预发布把 `latest` 指向不稳定镜像。

## 验收清单

### GitHub Release

- [ ] 标记正确：正式版 `prerelease = false`，rc 为 `true`
- [ ] 产物 8 个：6 个平台二进制 + `magicmail-<版本>-x86.fpk` + `magicmail-<版本>-arm64.fpk`
- [ ] 正文包含完整的「安全 / 重构 / 新增 / 修复」分段，而不是 `- 本次无详细变更记录`
- [ ] Releases 列表中正式版位于 Latest

### Docker 镜像（正式版）

- [ ] Docker Hub 与 ghcr.io 均出现 `1.2.0`、`1.2`、`1`、`latest` 四个标签
- [ ] `latest` 的更新时间是今天，而不是停留在上个版本

### 飞牛 FPK

- [ ] 两个架构包都已产出，且**包内版本与文件名版本一致**（`fnapp/manifest` 的 `version` 已同步）
- [ ] 实机安装后通过网关访问不白屏（CI 会校验 `/app/magicmail` 前缀，但实机验证更稳妥）

## 常见问题

**打了 tag，GitHub Actions 没触发**
去 cnb 看 `tag_push` 流水线。未生效时向 `main` 推一个提交触发全量同步。

**Release 正文只有一条「本次无详细变更记录」**
`docs/guide/changelog.md` 里缺少 `## [vX.Y.Z]` 条目，或标题格式不匹配（必须是 `## [v1.2.0]` 这种方括号包裹、带 `v` 前缀的写法）。

**`latest` 镜像没更新**
只有正式版 tag 才会推送 `latest`/`1.2`/`1`。如果构建成功但标签缺失，用 `workflow_dispatch` 手动重跑并填写 `image_tag`。

**用户反馈「发现新版本」但下载链接 404**
`version.json` 部署早于 Release 发布。部署前先确认 `downloadUrl` 指向的 Release 页面已存在。

**FPK 装不上，提示版本问题**
`fnapp/manifest` 的 `version` 没跟着升，或与目标设备上已安装版本相同导致拒绝覆盖。

## 文档站部署

`deploy.yml` 在 **`main` 分支**的 `docs/**` 变更时自动部署。在 `dev` 上修改文档不会发布到线上，需先合并到 `main`。
