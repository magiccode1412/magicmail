//go:build windows

package imap

// getDiskFreeSpaceWindows Windows 下暂时跳过磁盘检查，返回一个大值允许写入
// 如需精确检测可引入 golang.org/x/sys/windows 的 GetDiskFreeSpaceExW
func getDiskFreeSpace() (int64, error) {
	return int64(1 << 60), nil
}
