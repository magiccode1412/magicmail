// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

//go:build !dev

package embedfs

import "embed"

//go:embed all:dist
// DistFS 嵌入前端构建产物
//
// 生产构建前请先执行: cd apps/web && pnpm build（或 npm run build）
var DistFS embed.FS
