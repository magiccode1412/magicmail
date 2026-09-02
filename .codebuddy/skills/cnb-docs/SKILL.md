---
name: cnb-docs
description: 查询 CNB 平台官方文档，覆盖代码托管、制品库、AI 助手、云原生构建等
---

# CNB 官方文档

自动查询 CNB 云原生构建平台官方文档，提供准确的使用指引和操作说明。

## 文档来源

> 下方 URL 中的 `${CNB_WEB_PROTOCOL}` 和 `${CNB_WEB_HOST}` 为环境变量，使用前先获取实际值再拼接。

CNB 官方文档站点：`${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}`

提供两种文档索引：

| 索引 | URL | 说明 |
|------|-----|------|
| 增量索引 | `${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/llms.txt` | 文档目录索引，包含所有文档的标题和路径链接 |
| 全量文档 | `${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/llms-full.txt` | 包含所有文档的完整内容（体积较大） |

## 文档覆盖范围

CNB 文档涵盖以下主要领域：

- **指南**：创建仓库、Git 认证、迁移工具、角色权限、访问令牌、部署令牌
- **代码托管**：密钥仓库、元数据、ISSUE 模板、分类标签、UI 定制
- **制品库**：Docker、Helm、Maven、npm、PyPI、Cargo、Nuget、Composer、ohpm 等 12 种制品库
- **AI 助手**：AI 助手简介、知识库、MCP Server、Code Wiki
- **云原生开发**：开发环境、端口预览、IDE 客户端（VSCode/Cursor/JetBrains）、容器模式
- **云原生构建**：流水线配置、触发规则、语法手册、构建环境、环境变量、插件、缓存、定时任务、部署流程
- **开发者**：徽章、Open API
- **OAuth 授权**：用户授权、OAuth 应用创建
- **其他**：FAQ、定价、SLA、任务集等

## 术语识别（先查再答）

CNB 平台存在大量与日常用语重合的专有名词，**遇到下列形态的词先按平台概念查文档，禁止凭字面猜测**：

- **点分名称**（`xxx:yyy`）：几乎都是内置任务或平台功能，如 `npc:go`、`cnb:apply`、`cnb:trigger`、`git:auto-merge`、`docker:cache`、`testing:coverage`、`vscode:go`
- **NPC**：CNB 语境下指**自动化角色**（AI 智能助手，可 @ 召唤、可在流水线中执行任务），与游戏 Non-Player Character 无关；`npc:go` 是执行 NPC 任务的内置任务
- 其他与通用词汇重合的平台术语（如 `runner`、`pipeline`、`stage`、`制品库`）

**标准动作**：先查文档索引定位相关文档，读取后再回答；索引未命中时换关键词重试或用全量文档。

## 使用流程

### 第一步：分析用户问题

提取用户问题的关键词，判断想了解哪个领域的文档。

### 第二步：获取文档索引

获取文档目录索引（`llms.txt`）：

```
URL: ${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/llms.txt
提示：列出所有文档的标题和路径链接，帮我找到与「{用户问题的关键词}」相关的文档路径
```

### 第三步：获取具体文档内容

根据索引中匹配到的文档路径，拼接完整 URL 并获取文档内容：

```
URL: ${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}{文档路径}
提示：提取这篇文档的完整内容，保留代码示例和配置说明
```

**注意**：索引路径格式为 `/zh/category/topic.md`。
完整 URL 为 `${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/category/topic.md`。

### 第四步：整理并回答

将获取到的文档内容整理后，根据用户问题：
1. 提取最相关的内容
2. 保留重要的代码示例和配置片段
3. 简明呈现给用户
4. 内容较长时适当精简，并附原文档链接

## 约束速查

### 必须
- 始终基于官方文档回答，不臆造信息；回答时附带文档来源链接

### 建议
- 增量索引未找到时，可尝试全量文档 `${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/llms-full.txt`
- 单个文档无法完全回答时，可获取多个文档综合回答
