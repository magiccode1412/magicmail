---
name: cnb-repo-knowledge-base
description: 检索指定仓库的知识库，返回相关的文本片段及其来源
---

# CNB 仓库知识库

检索指定仓库的知识库，返回最相关的文本片段及其来源。

## 执行命令

```bash
cnb knowledge-base query-knowledge-base-get --repo <group>/<repo> --query "<自然语言问题或关键词>" --top-k 5
```

- `--repo`：仓库路径，如 `cnb/feedback`，必填
- `--query`：查询语句，必填
- `--top-k`：返回的片段数量，默认 5
