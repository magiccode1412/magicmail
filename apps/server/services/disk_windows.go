//go:build windows

package services

// Windows 下跳过磁盘检查，返回一个大值允许写入
func getDiskFreeSpaceForServices() (int64, error) {
	return int64(1 << 60), nil
}
