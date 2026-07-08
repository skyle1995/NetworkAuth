package storage

import (
	"fmt"
	"math"
)

// FormatBytes 格式化文件大小
// 输入字节数，返回带有单位 (B, KB, MB, GB, TB) 的格式化字符串
func FormatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	pow := math.Floor(math.Log(float64(bytes)) / math.Log(1024))
	if pow > float64(len(units)-1) {
		pow = float64(len(units) - 1)
	}
	size := float64(bytes) / math.Pow(1024, pow)
	return fmt.Sprintf("%.2f %s", size, units[int(pow)])
}
