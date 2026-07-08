package storage

import "context"

// FileInfo 存储桶中的文件信息
type FileInfo struct {
	Name          string `json:"name"`           // 文件名
	Size          int64  `json:"size"`           // 文件大小 (字节)
	SizeFormatted string `json:"size_formatted"` // 格式化后的文件大小
	ModifiedTime  string `json:"modified_time"`  // 修改时间字符串
	Timestamp     int64  `json:"timestamp"`      // 修改时间戳
	DownloadURL   string `json:"download_url"`   // 下载链接
	RelativePath  string `json:"relative_path"`  // 相对路径
	ETag          string `json:"etag"`           // ETag
}

// VersionInfo 版本目录信息
type VersionInfo struct {
	Version string     `json:"version"` // 版本号（如 "2.0.150"）
	Files   []FileInfo `json:"files"`   // 该版本目录下的文件列表
}

// Storage 对象存储通用接口
type Storage interface {
	// ListFiles 获取指定前缀的文件列表
	// 返回按照修改时间倒序排列的 ZIP 文件列表
	ListFiles(ctx context.Context, prefix string) ([]FileInfo, error)

	// GetObject 获取指定 key 的对象内容
	GetObject(ctx context.Context, key string) ([]byte, error)

	// ScanVersions 扫描 basePrefix 下所有 v*/ 版本目录，返回版本号列表（降序）
	// 如 basePrefix="NetworkAuth/" → 扫描 "NetworkAuth/v2.0.150/"、"NetworkAuth/v2.0.149/" → 返回 ["2.0.150", "2.0.149"]
	ScanVersions(ctx context.Context, basePrefix string) ([]string, error)

	// ListVersionFiles 获取指定版本目录下全部文件
	// basePrefix 如 "NetworkAuth/"，version 如 "2.0.150" → 拼接 "NetworkAuth/v2.0.150/" 读取
	ListVersionFiles(ctx context.Context, basePrefix, version string) ([]FileInfo, error)
}
