// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

//go:build dev

package embedfs

import "io/fs"

// DistFS 开发模式占位符
//
// 开发环境下不嵌入前端产物，路由层会自动降级到 Vite 代理或磁盘读取。
var DistFS = func() fs.FS {
	// 返回一个空的 fs.FS，isEmbedded() 检测会失败并降级
	return &emptyFS{}
}()

type emptyFS struct{}

func (*emptyFS) Open(name string) (fs.File, error) { return nil, fs.ErrNotExist }
