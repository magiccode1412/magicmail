//go:build !windows

package imap

import (
	"syscall"
)

// getDiskFreeSpaceUnix 获取当前磁盘的剩余空间（Linux/macOS）
func getDiskFreeSpace() (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(".", &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}
