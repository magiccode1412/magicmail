# 发布流程

本文档描述 Magicmail 从开发到发布的完整流程，以及其中几个容易踩坑的机制（tag 归属校验、cnb → GitHub 同步、Release 正文来源、`version.json` 部署时机）。

## 分支模型

| 分支 | 角色 | 说明 |
|------|------|------|
| `dev` | 日常开发 | 持续合入，随时可能包含未完成的功能 |
| `main` | **发布冻结窗口**（等价于 release branch） | 只在发版时接收 `dev` 的合并，合并即进入冻结 |

文档站不通过分支发布，而是由 GitHub Actions 直接上传构建产物部署（见[文档站部署](#文档站部署)），
仓库里没有也不需要 `gh-pages` 分支。

**为什么 `main` 是 release branch 而不是普通主干**：`main` 上的提交历史远短于 `dev`，只在发版时才推进。这意味着「把 `dev` 合并进 `main`」这个动作本身就完成了**代码冻结** —— 之后 `dev` 可以继续开发下一个版本，不会污染当前发布窗口。这正是 release branch 的核心作用，所以本项目不再单独切 `release/x.y` 分支。

代码仓库的主副本在 **cnb**（`origin` 指向 `cnb.cool`），GitHub 是由 cnb 单向同步出去的镜像。所有开发和打 tag 都在 cnb 侧进行。

打 tag 后**两个平台各自独立发版**，互不依赖：cnb 由 `.cnb/release.yml` 发布 Release，GitHub 由 `release.yml`（Actions）发布。任一侧流水线挂了，另一侧的版本产物仍然完整。

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
        │                                ▼
        │                       观察期：招募试用 + 自己挂机
        │                       （关键：沉默 ≠ 通过，见步骤 3）
        │                                │
        │                         有问题：修复提交直接进 main，
        │                                 打 v1.2.0-rc.2 …
        │                                │
        │                         Go ▼（No-Go 转 rc.2）
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

推送后 cnb 的 `tag_push` 事件会并行触发两条流水线：

1. `.cnb/release.yml` —— 构建并发布 **cnb Release**（约 10 分钟，产物 8 个）
2. `.cnb.yml` 的 `sync_tag_to_github` —— 把 tag 同步到 GitHub，GitHub Actions 随即开始构建（约 1 分钟开始，Release 约 5 分钟完成，Docker 多架构约 20 分钟）

版本号带 `-` 后缀会被自动标记为 **Pre-release**，不会进入 Latest，也不会产生 Docker 浮动标签。

出问题就直接在 `main` 上提交修复，然后打 `v1.2.0-rc.2`，依此类推。

打完 rc 即进入观察期，见下一步。

### 3. rc 观察期（决定「能不能发正式版」）

rc 到正式 tag 之间是一段**观察期**。它的意义不是「等着别出事」，而是「主动去发现问题」——
这段时间的价值完全取决于**有没有真人在跑 rc**，原因见[沉默不等于通过](#沉默不等于通过)。

观察期多长取决于改动量：

| 改动规模 | 建议观察期 |
|---|---|
| 纯文档 / 依赖升级 | 无需 rc，可直接发正式版 |
| 单点 bug 修复 | rc 产物验收通过即可发 |
| 涉及认证、长连接、定时轮询等「长时间运行才暴露」的改动 | ≥ 1 天，且必须覆盖下方必测场景 |

**必测场景**（按本次实际改动范围选取）：

- 从**上一个正式版**完整升级到 rc 一次，而不是只做全新安装 —— 升级路径有独享的坑：旧数据迁移、token 失效、前端缓存
- 三种部署形态各自验证：飞牛统一网关 / Docker / 独立部署
- 挂机 ≥ 2 小时，观察日志中的重连、限流、鉴权失败
- 改动涉及定时轮询或长连接时，需跨过至少一个完整周期（token 过期、心跳间隔）

**Go / No-Go 判据**

Go（发正式版），需**全部**满足：

- 至少 1 个真实实例跑 rc 超过 12 小时无异常，**或**维护者自己把上述必测场景全量跑通
- 上一个正式版 → rc 的升级路径验证通过
- 三种部署形态均已验证（或本次改动明确只影响其中一种）
- `git log --oneline v<版本>-rc.N..origin/main` **输出为空**

最后一条是最硬的证据，它确认观察期内 `main` 上没有进过新提交。若 `main` 已有新提交，
正式 tag 就必须打在新的 tip 上，且那些修复同样要验过 —— 此时「等了一天没发现问题」的结论不再成立。

No-Go（转 rc.N+1），**任一**命中：

- 出现收不到信、被服务商限流、长连接反复重连
- 任一形态登录后 API 仍报鉴权失败
- 升级路径走不通（如升级后无法登录，且不是预期中的「需重新登录一次」）

#### 沉默不等于通过

观察期最容易踩的坑，是把「没有收到反馈」当成「rc 通过了」。

更新检测的唯一来源是已部署的 `version.json`（见步骤 6），而它在正式发布前**不会更新**；
rc 又是 Pre-release，不会出现在 Releases 的 Latest 中。结果是：**线上用户根本不知道有 rc 存在**。

此时「一天没有用户反馈」是必然事件，信息量为零 —— 你拿不到否定信号，是因为**根本没有采样**，
而不是因为 rc 通过了。

所以观察期有两件事，缺一不可：

1. **主动找人跑**：在 QQ 群 / QQ 频道招募试用者，写明这是预发布版、**升级后需要重新登录**
   （提前说明，否则会被当成 bug 报上来），并列出希望覆盖的邮箱类型与部署形态。
   **飞牛统一网关形态优先** —— 历史经验里改动最集中的就是它
2. **自己挂机跑**：只有时间能测出来的问题（限流、半开连接、token 过期自愈）必须靠自己跑满一个周期

顺带提醒：用户遇到「升级后让我重新登录」，第一反应是重新登录完事，**不会来报 issue**；
而「收不到信」往往要隔几小时才发现。被动等反馈的信噪比极低。

### 4. 打正式版标签

观察期判据通过（Go）后，在 `main` 上打正式标签：

```bash
git tag -a v1.2.0 -m "release: v1.2.0" -m "本次变更摘要"
git push origin v1.2.0
```

### 5. 验收

见下方[验收清单](#验收清单)。

### 6. 部署 `version.json`（必须最后一步）

`version.json` 是更新检测的**唯一**来源（前端 `useUpdateCheck` 只比对它的 `latest` 字段，与 GitHub Release 无关）。部署到 EdgeOne Pages 后，所有线上实例会立刻弹出更新提示。

```bash
# 确认 Release 与镜像都就绪后再部署
cat version.json    # latest / releaseDate / downloadUrl
```

**必须等 Docker 镜像推送完成后再部署**。否则用户看到「发现新版本 v1.2.0」，去 `docker pull …:latest` 却仍拉到旧的 1.1.1 镜像（应用内下载走 GitHub Release 不受影响，但 Docker 用户会踩空）。

### 7. 回灌 `dev`

```bash
git checkout dev
git merge --ff-only main
git push origin dev
```

## 关键机制

### tag 必须打在 `main` 上

三处会用 `git merge-base --is-ancestor HEAD origin/main` 强制校验，**打在 `dev` 上会直接红叉**：`release.yml` 的 `guard` job、`docker-publish.yml` 的「校验 tag 位置」步骤、以及 cnb 侧的 `scripts/release-guard.sh`。

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

> **cnb 分支配置的覆盖语义（重要）**：一旦为某个分支单独配置了 `push`，`$` 兜底分支下的 `push` 就**不再**对该分支生效。
> `dev` 已经单独配置了 `push`（同步 + 构建校验），所以任何与 `dev` 推送相关的流水线都必须写在 `dev:` 下，写在 `$:` 下不会执行 —— 曾经因此导致 `dev` 的改动不再同步到 GitHub。

### 双平台各自发版

tag 到达两个平台后，两侧**各自**完成「构建 → Release → 上传产物」，没有产物互传：

| 平台 | 流水线 | Release 产物 |
|------|--------|--------------|
| cnb | `.cnb/release.yml`（`$.tag_push`） | `git:release` 建 Release，`cnbcool/attachments` 上传附件 |
| GitHub | `.github/workflows/release.yml`（`tags: v*`） | `softprops/action-gh-release` 一步完成 |

所以上传下载渠道有两条，任一平台故障都不影响另一平台的可用性。

cnb 侧与 GitHub 侧的几个实现差异，都是平台能力不同导致的：

| 点 | GitHub | cnb |
|----|--------|-----|
| 发布与上传附件 | 一个 action 搞定 | `git:release` **不支持上传附件**，必须再配一个 `cnbcool/attachments` 任务，且顺序不能反（先建 Release 再传附件） |
| 非发版 tag | `tags: ['v*']` 在事件层就过滤掉 | 事件无法按 tag 名过滤，改由 `scripts/release-guard.sh` 用退出码 **78** 结束（任务成功但中断流水线），不会显示成失败 |
| 预发布标记 | `prerelease: ${{ contains(version, '-') }}` 表达式 | `preRelease` 只接受布尔字面量，变量替换会被判成字符串，因此拆成两个 stage 用 `if` 二选一 |
| 并行构建 | job matrix（跨机器） | 同流水线内用 `jobs` 对象并行，共享工作区；`go:embed` 目录会互相覆盖，所以每个平台先 `cp -r server` 出独立副本 |

配套脚本都在 `scripts/` 下，前缀 `release-*`，与 `verify-build.sh` 用同一份构建环境（`.ci/Dockerfile`）：

| 脚本 | 作用 |
|------|------|
| `release-guard.sh` | 校验版本号规范 + tag 必须在 `main` 上；输出 `version` / `prerelease` / `latest` |
| `rebuild-guard.sh` | 手动重建（按钮）入口校验：版本号必须已有同名 tag + 当前 HEAD 在 `main`；输出 `version` / `prerelease` / `latest` |
| `release-frontend.sh` | 构建两遍前端：默认 base → `build/web-default`，网关 base → `build/web-gateway` |
| `release-binary.sh` | 交叉编译单个平台（并行 job 调用） |
| `release-fpk.sh` | 打包单个架构的 FPK（并行 job 调用） |
| `release-notes.sh` | 生成 Release 正文 → `release-notes.md` |

### 不升版本号重建产物

版本刚发布就发现产物有问题，`main` 上补了修复提交，但**不想升版本号**时，按原版本号重新构建并覆盖同名 Release：

| 平台 | 操作 |
|------|------|
| cnb | `main` 分支详情页 → 自定义按钮 → **重建最新版本** → 版本号留空（取 `version.json` 的 `latest`），也可手工填 `v1.2.0` |
| GitHub | Actions → Build & Release → Run workflow → 填同一个版本号 |

两侧**没有联动**：cnb 的按钮只重建 cnb 的产物，GitHub 侧要另外手动重跑。只做一边会出现两个平台产物不一致（用户下载渠道主要在 GitHub，见[双平台各自发版](#双平台各自发版)）。

cnb 侧是 `.cnb/release.yml` 的 `web_trigger_rebuild_release` 流水线，与 `tag_push` 共用同一套构建 stage，差别只在入口校验和 Release 的创建方式：

- 版本号来自按钮输入或 `version.json`，`git:release` 必须显式传 `tag`（`tag_push` 下自动取当前 tag）
- Release 已存在，用 `overlying: false`（默认，先删后建）覆盖，旧附件随 Release 一起清掉，不会同名堆积
- 入口校验 `scripts/rebuild-guard.sh`：版本号必须已有同名 tag（不能凭空建版本）、当前 HEAD 必须在 `main` 上；重建的不是 `version.json` 里的 `latest` 时，不会抢 Latest 标记

按钮只在 `main` 分支显示（`.cnb/web_trigger.yml` 的 `reg: ^main$`），和[tag 必须打在 `main` 上](#tag-必须打在-main-上)是同一条约束。

### dev 分支持续构建校验

`dev` 每次 push 会跑 `build-verify` 流水线（`.cnb.yml` 的 `dev.push`），执行 `scripts/verify-build.sh`：

- 前端构建
- 6 个目标平台交叉编译
- 飞牛 FPK（x86 / arm64）打包，并校验网关前缀与二进制架构

它不发 Release、不推镜像、不产生任何对外产物，目的只是把「发版时才会做的事」提前做一遍。其中 FPK 的两项校验（`/app/magicmail` 前缀、二进制架构与 `manifest` 声明一致）只有真正打出 FPK 才能验证，靠单元测试和前端构建都拦不住 —— 在合并进 `main` 之前发现，比等到发 rc 标签再发现要早得多。

构建环境由 `.ci/Dockerfile` 预装（Node 22 + pnpm 10 + Go 1.25 + fnpack + file），脚本也可以在本地直接运行：

```bash
bash scripts/verify-build.sh
```

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

> GitHub 与 cnb 的 Release **都要看**。产物是两个平台各自构建的，一边成功不代表另一边也成功了。

### cnb Release

- [ ] `tag_push` 流水线 `release` 成功（非 `v*` tag 会显示「跳过」而非失败）
- [ ] 标记正确：正式版进入 Latest，rc 标记为预发布
- [ ] 附件 8 个：6 个平台二进制 + `magicmail-<版本>-x86.fpk` + `magicmail-<版本>-arm64.fpk`
- [ ] 正文包含完整的「安全 / 重构 / 新增 / 修复」分段，而不是 `- 本次无详细变更记录`

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
去 cnb 看 `tag_push` 的 `sync_tag_to_github` 流水线。未生效时向 `main` 推一个提交触发全量同步。

**cnb 的 `release` 流水线显示跳过/没跑**
非 `v*` 形式的 tag（如 `test-1`）会被 `release-guard.sh` 以退出码 78 结束，这是预期行为，不会标红。
如果 `v*` tag 也没跑，检查 `.cnb.yml` 顶部的 `include: - .cnb/release.yml` 是否还在 —— `tag_push` 只能挂在 `$` 兜底分支下，写在具体分支名下不会触发。

**想重发同一个版本号的产物，又不想升版本号**
在 cnb 的 `main` 分支详情页点「重建最新版本」按钮，GitHub 侧在 Actions 手动重跑 Build & Release。两侧都要做，否则产物不一致。详见[不升版本号重建产物](#不升版本号重建产物)。

**Release 正文只有一条「本次无详细变更记录」**
`docs/guide/changelog.md` 里缺少 `## [vX.Y.Z]` 条目，或标题格式不匹配（必须是 `## [v1.2.0]` 这种方括号包裹、带 `v` 前缀的写法）。

**`latest` 镜像没更新**
只有正式版 tag 才会推送 `latest`/`1.2`/`1`。如果构建成功但标签缺失，用 `workflow_dispatch` 手动重跑并填写 `image_tag`。

**用户反馈「发现新版本」但下载链接 404**
`version.json` 部署早于 Release 发布。部署前先确认 `downloadUrl` 指向的 Release 页面已存在。

**FPK 装不上，提示版本问题**
`fnapp/manifest` 的 `version` 没跟着升，或与目标设备上已安装版本相同导致拒绝覆盖。

## 文档站部署

文档有两个发布渠道：

| 渠道 | 定位 | 触发方式 |
|------|------|----------|
| EdgeOne | 主用 | 关联仓库后自动部署，日常无需干预 |
| GitHub Pages | 保底 | `deploy.yml` 在 `main` 分支有 `docs/**` 变更时自动部署 |

GitHub Pages 走官方 Pages Action（`configure-pages` → `upload-pages-artifact` → `deploy-pages`），
直接上传构建产物，**不使用 `gh-pages` 分支**。

> **前置条件**：仓库首次启用时需到 Settings → Pages → Source 选择 **GitHub Actions**。
> 否则 `Build Docs` 会成功，但 `Deploy` job 直接失败（不会执行任何步骤）。

`deploy.yml` 只在 `main` 分支触发，在 `dev` 上改文档不会发布，需先合并到 `main`。
也可以手动触发：Actions → Deploy Docs to GitHub Pages → Run workflow。

### 文档渠道：正式版 / 开发版

EdgeOne 按推送分支分别构建，`main` 出正式版、`dev` 出预览版，两边域名不同。
为了让两套文档一眼可分，构建时会判定**渠道**（`stable` / `dev`），`dev` 渠道额外带上：

| 表现 | `dev` | `stable` |
|------|-------|----------|
| 站点标题 | Magicmail 开发版 | Magicmail |
| 页面标题后缀 | `（开发版）` | 无 |
| 顶部横幅 | 显示（可关闭，按版本记忆） | 无 |
| 「更多」菜单 | 首项为「查看正式版文档 →」 | 无 |
| `robots` meta | `noindex,nofollow` | 无 |

> `noindex` 是必须的：预览站对外可访问，否则未发布的 changelog 会先被搜索引擎收录。

判定逻辑在 `docs/.vitepress/channel.ts`，优先级从高到低：

1. `DOCS_CHANNEL` 环境变量显式指定（`dev` / `stable`）
2. CI 分支变量（`GITHUB_REF_NAME` / `CNB_BRANCH` / 各 Pages 平台的分支变量）
3. `.git/HEAD` 当前分支名
4. 兜底推断：`docs/guide/changelog.md` 顶格版本领先 `version.json` 的 `latest` → `dev`
5. 都拿不到 → `stable`（保守默认，正式站不会误挂「开发版」标识）

构建日志会打印判定结果，排查时看这两行：

```
[DOCS_CHANNEL] dev (source: CNB_BRANCH=dev)
[DOCS_STABLE] version=v1.2.0 url=https://160621.xyz/magicmail
```

**EdgeOne 侧配置**：在预览环境的构建变量里加 `DOCS_CHANNEL=dev` 即可显式指定；
不加也能靠第 3、4 级推断出来，加了只是更明确。正式环境同理设 `DOCS_CHANNEL=stable`。
GitHub Pages 已在 `deploy.yml` 的 `env` 中固定为 `stable`（该工作流只在 `main` 触发）。

另外可用 `DOCS_STABLE_URL` 覆盖横幅指向的正式版文档地址（默认 `https://160621.xyz/magicmail`）。

**本地预览**两种渠道：

```bash
cd docs

pnpm dev                      # 按当前分支判定（在 dev 分支上就是开发版）
DOCS_CHANNEL=stable pnpm dev  # 强制按正式版预览
```
