package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// COSConfig 腾讯云 COS 配置信息
type COSConfig struct {
	SecretID  string // API 密钥 ID
	SecretKey string // API 密钥 Key
	Region    string // 区域，例如: ap-guangzhou
	Bucket    string // 存储桶名称
	BaseURL   string // 可选，自定义域名，如果为空将使用默认的 COS 域名
}

type cosStorage struct {
	client  *cos.Client
	baseURL string
}

// NewCOSStorage 创建腾讯云 COS 存储实例
// 输入参数：
//   - cfg: 腾讯云 COS 配置信息（包含密钥、区域、桶名、自定义下载域名）
//
// 返回值：
//   - Storage: 存储实例（实现 Storage 接口）
//   - error: 创建失败原因
func NewCOSStorage(cfg COSConfig) (Storage, error) {
	// API 访问域名固定使用 COS 官方桶域名，避免自定义下载域名（例如 CDN 域名）不支持列举 API 导致无法获取文件列表
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("解析 COS BucketURL 失败: %w", err)
	}

	downloadBaseURL := cfg.BaseURL
	if strings.TrimSpace(downloadBaseURL) == "" {
		downloadBaseURL = bucketURL.String()
	}
	downloadBaseURL = strings.TrimSuffix(downloadBaseURL, "/")

	b := &cos.BaseURL{BucketURL: bucketURL}
	client := cos.NewClient(b, &http.Client{
		Timeout: 30 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &cosStorage{
		client:  client,
		baseURL: downloadBaseURL,
	}, nil
}

// ListFiles 获取指定前缀的文件列表，仅返回 zip 文件，按修改时间降序排列
func (s *cosStorage) ListFiles(ctx context.Context, prefix string) ([]FileInfo, error) {
	opt := &cos.BucketGetOptions{
		Prefix:  prefix,
		MaxKeys: 1000,
	}

	v, _, err := s.client.Bucket.Get(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("获取 COS 文件列表失败: %w", err)
	}

	var files []FileInfo
	for _, content := range v.Contents {
		// 仅过滤 zip 文件
		if strings.ToLower(filepath.Ext(content.Key)) == ".zip" {
			modTime, err := time.Parse(time.RFC3339, content.LastModified)
			var timestamp int64
			var timeStr string
			if err == nil {
				timestamp = modTime.Unix()
				timeStr = modTime.Format(time.DateTime)
			} else {
				timestamp = 0
				timeStr = content.LastModified
			}

			files = append(files, FileInfo{
				Name:          filepath.Base(content.Key),
				Size:          int64(content.Size),
				SizeFormatted: FormatBytes(int64(content.Size)),
				ModifiedTime:  timeStr,
				Timestamp:     timestamp,
				DownloadURL:   fmt.Sprintf("%s/%s", s.baseURL, content.Key),
				RelativePath:  content.Key,
				ETag:          strings.Trim(content.ETag, `"`),
			})
		}
	}

	// 按照修改时间降序排列
	sort.Slice(files, func(i, j int) bool {
		return files[i].Timestamp > files[j].Timestamp
	})

	return files, nil
}

func (s *cosStorage) GetObject(ctx context.Context, key string) ([]byte, error) {
	resp, err := s.client.Object.Get(ctx, key, nil)
	if err != nil {
		return nil, fmt.Errorf("获取 COS 对象失败: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 COS 对象失败: %w", err)
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
func (s *cosStorage) ScanVersions(ctx context.Context, basePrefix string) ([]string, error) {
	opt := &cos.BucketGetOptions{
		Prefix:    basePrefix,
		MaxKeys:   1000,
		Delimiter: "/",
	}

	v, _, err := s.client.Bucket.Get(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("扫描 COS 版本目录失败: %w", err)
	}

	var versions []string
	for _, prefix := range v.CommonPrefixes {
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
func (s *cosStorage) ListVersionFiles(ctx context.Context, basePrefix, version string) ([]FileInfo, error) {
	prefix := basePrefix + "v" + version + "/"
	return s.ListFiles(ctx, prefix)
}

// compareVersionStrings 比较两个版本号字符串
// 返回值: 1 表示 a > b，-1 表示 a < b，0 表示相等
func compareVersionStrings(a, b string) int {
	parseParts := func(v string) []int {
		parts := strings.Split(v, ".")
		result := make([]int, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			n := 0
			for i := 0; i < len(p); i++ {
				ch := p[i]
				if ch < '0' || ch > '9' {
					break
				}
				n = n*10 + int(ch-'0')
			}
			result = append(result, n)
		}
		return result
	}

	x := parseParts(a)
	y := parseParts(b)
	maxLen := len(x)
	if len(y) > maxLen {
		maxLen = len(y)
	}

	for i := 0; i < maxLen; i++ {
		xi, yi := 0, 0
		if i < len(x) {
			xi = x[i]
		}
		if i < len(y) {
			yi = y[i]
		}
		if xi > yi {
			return 1
		}
		if xi < yi {
			return -1
		}
	}
	return 0
}
