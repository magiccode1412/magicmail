//go:build !windows

package services

import (
	"os"
	"syscall"
)

func getDiskFreeSpaceForServices() (int64, error) {
	var stat syscall.Statfs_t
	wd, _ := os.Getwd()
	if err := syscall.Statfs(wd, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}
