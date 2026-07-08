package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSSConfig 阿里云 OSS 配置信息
type OSSConfig struct {
	AccessKeyID     string // 访问密钥 ID
	AccessKeySecret string // 访问密钥 Secret
	Endpoint        string // 访问域名，例如: oss-cn-hangzhou.aliyuncs.com
	Bucket          string // 存储桶名称
	BaseURL         string // 可选，自定义下载域名，如果为空将使用默认 Bucket 域名
}

type ossStorage struct {
	bucket  *oss.Bucket
	baseURL string
}

// NewOSSStorage 创建阿里云 OSS 存储实例
// 实现了 Storage 接口
func NewOSSStorage(cfg OSSConfig) (Storage, error) {
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("创建 OSS 客户端失败: %w", err)
	}

	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("获取 OSS Bucket 失败: %w", err)
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		// 清理 Endpoint 协议前缀以拼接
		endpoint := strings.TrimPrefix(cfg.Endpoint, "http://")
		endpoint = strings.TrimPrefix(endpoint, "https://")
		baseURL = fmt.Sprintf("https://%s.%s", cfg.Bucket, endpoint)
	}

	return &ossStorage{
		bucket:  bucket,
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}, nil
}

// ListFiles 获取指定前缀的文件列表，仅返回 zip 文件，按修改时间降序排列
func (s *ossStorage) ListFiles(ctx context.Context, prefix string) ([]FileInfo, error) {
	// 列出文件
	lsRes, err := s.bucket.ListObjects(oss.Prefix(prefix), oss.MaxKeys(1000))
	if err != nil {
		return nil, fmt.Errorf("获取 OSS 文件列表失败: %w", err)
	}

	var files []FileInfo
	for _, object := range lsRes.Objects {
		if strings.ToLower(filepath.Ext(object.Key)) == ".zip" {
			timestamp := object.LastModified.Unix()
			timeStr := object.LastModified.Format(time.DateTime)

			files = append(files, FileInfo{
				Name:          filepath.Base(object.Key),
				Size:          object.Size,
				SizeFormatted: FormatBytes(object.Size),
				ModifiedTime:  timeStr,
				Timestamp:     timestamp,
				DownloadURL:   fmt.Sprintf("%s/%s", s.baseURL, object.Key),
				RelativePath:  object.Key,
				ETag:          strings.Trim(object.ETag, `"`),
			})
		}
	}

	// 按照修改时间降序排列
	sort.Slice(files, func(i, j int) bool {
		return files[i].Timestamp > files[j].Timestamp
	})

	return files, nil
}

func (s *ossStorage) GetObject(ctx context.Context, key string) ([]byte, error) {
	rc, err := s.bucket.GetObject(key)
	if err != nil {
		return nil, fmt.Errorf("获取 OSS 对象失败: %w", err)
	}
	defer rc.Close()

	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("读取 OSS 对象失败: %w", err)
	}
	return b, nil
}

// ScanVersions 扫描 basePrefix 下所有 v*/ 版本目录，返回版本号列表（降序）
// 输入参数：
//   - ctx: 上下文
//   - basePrefix: 基础前缀，如 "NetworkAuth/"
//
// 返回值：
//   - []string: 版本号列表，如 ["2.0.150", "2.0.149"]（降序）
//   - error: 扫描失败原因
func (s *ossStorage) ScanVersions(ctx context.Context, basePrefix string) ([]string, error) {
	lsRes, err := s.bucket.ListObjects(oss.Prefix(basePrefix), oss.Delimiter("/"), oss.MaxKeys(1000))
	if err != nil {
		return nil, fmt.Errorf("扫描 OSS 版本目录失败: %w", err)
	}

	var versions []string
	for _, prefix := range lsRes.CommonPrefixes {
		// prefix 如 "NetworkAuth/v2.0.150/"
		dirName := strings.TrimPrefix(prefix, basePrefix)
		dirName = strings.TrimSuffix(dirName, "/")
		// 提取 "v2.0.150" 中的版本号 "2.0.150"
		if strings.HasPrefix(dirName, "v") {
			version := strings.TrimPrefix(dirName, "v")
			if version != "" {
				versions = append(versions, version)
			}
		}
	}

	// 按版本号降序排列
	sort.Slice(versions, func(i, j int) bool {
		return compareVersionStrings(versions[i], versions[j]) > 0
	})

	return versions, nil
}

// ListVersionFiles 获取指定版本目录下全部文件
// 输入参数：
//   - ctx: 上下文
//   - basePrefix: 基础前缀，如 "NetworkAuth/"
//   - version: 版本号，如 "2.0.150"
//
// 返回值：
//   - []FileInfo: 该版本目录下的文件列表
//   - error: 获取失败原因
func (s *ossStorage) ListVersionFiles(ctx context.Context, basePrefix, version string) ([]FileInfo, error) {
	prefix := basePrefix + "v" + version + "/"
	return s.ListFiles(ctx, prefix)
}
