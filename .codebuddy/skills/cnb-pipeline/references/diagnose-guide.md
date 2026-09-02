# CI 流水线诊断指南

两种模式：**🔧 失败诊断**（根因分析 + 修复）｜**⚡ 性能优化**（瓶颈分析 + 加速）。
失败 + 慢并存时，先诊断失败，再附耗时分析。

依赖 [cnb-api] skill：查询构建状态、下载日志。

---

## Step 1：确定构建 sn（可选）

默认不传 `--sn`，CLI 自动定位失败构建。需显式指定时，从 `cnb pulls check-status` 对应检查项的 `target_url` 末段取 sn（形如 `cnb-xxx-xxxxxxxxx`），勿用 `context` 字段。

其他来源：

| 用户输入 | 获取方式 |
|---------|---------|
| 构建链接 | URL 中 `/-/build/logs/{sn}` 片段 |
| 已知 sn | 直接使用 |
| 历史构建 | `cnb build get-build-logs --repo ... --sourceRef ...` 查询后挑选 |

---

## 🔧 失败诊断

```bash
cnb pulls get-ci-logs           # 自动定位失败构建
```

CLI 自动：查询状态 → 定位失败 Stage → 获取详细日志（无日志时降级到全量 + base64 解码）→ 输出耗时概览。

**失败类型判定**：

| 类型 | 特征关键词 | 处理 |
|------|-----------|------|
| 🔴 代码 | `error:` `FAILED` `test failed` `eslint` `TypeError` `SyntaxError` `npm ERR!` `ERESOLVE` | 定位错误文件/行号，关联最近提交 |
| 🟡 配置 | `image not found` `pull image failed` `invalid configuration` `env not set` `permission denied` `OOM` | 读取 `.cnb.yml` / `.ci/` 分析配置 |
| 🔵 外部服务 | `ETIMEDOUT` `ECONNREFUSED` `ENOTFOUND` `502/503/504` `rate limit` `SSL` | 指出故障服务，建议重试 |

---

## ⚡ 性能优化

通过 `cnb build --help` / `cnb pulls --help` 探索可用命令，获取构建的 Stage 耗时数据和慢 Stage 日志。

分析要点：耗时 >5min、占比 >50%、无依赖可并行。

**常见瓶颈**：

| 优化点 | 日志特征 |
|--------|---------|
| 依赖安装慢 | `added X packages in Ys`（>60s）、无 `cache hit` |
| 缺少缓存 | 每次重新下载依赖 |
| 镜像拉取慢 | `Pulling from`、`Status: Downloaded` |
| 测试执行慢 | `Test Suites:`、`Duration` |
| 串行可并行 | 多个无依赖 Stage 串行 |
| 重复工作 | 多处相同 install/build |

结合 `.cnb.yml` / `.ci/` 分析并行空间、缓存配置、镜像选择。

---

## 报告输出

- **失败诊断**：构建编号、失败类型、Pipeline/Stage 状态表、根因（含关键日志）、修复建议。
- **性能优化**：构建编号、总耗时、Stage 耗时排行、瓶颈分析、按收益排序的优化建议（附预估节省时间与 `.cnb.yml` 修改片段）。

---

## 注意事项

- 多 Stage 失败：优先分析**最先失败**的（根因）
- 多类型并存：代码 > 配置 > 外部服务
- 长日志：关注**末尾**错误信息
- 不暴露 Token / 密码等敏感信息
- 优化建议要具体可操作，优先最大收益方案
- **修复后禁止轮询 CI 状态**：推送修复代码后立即结束，禁止 `sleep`、循环查询等任何等待操作，CI 失败会自动唤起 NPC 继续处理
