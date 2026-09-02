---
name: cnb-text-path-converter
description: 将 ISSUE/PR 中的相对路径转为绝对路径
requires:
  bins: [node]
  files: [scripts/convertLink.js]
---

# CNB 文本相对路径转换器

## 使用方式

获取 ISSUE 或合并请求的描述、评论、Review 的原始内容后，执行以下脚本，将所有相对路径替换为绝对路径并输出到标准输出。

```bash
# 执行转换，结果输出到标准输出
node scripts/convertLink.js <在此粘贴原始内容>
```

## 示例

**转换前：**
```
请参考 ./src/utils/helper.ts 中的实现，以及 ../docs/api.md 的说明。
```

**转换后：**
```
请参考 https://cnb.cool/cnb/cool/default-npc-agent/-/git/raw/main/src/utils/helper.ts 中的实现，以及 https://cnb.cool/cnb/cool/default-npc-agent/-/git/raw/main/docs/api.md 的说明。
```
