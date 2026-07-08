package selfupdate

import (
	"NetworkAuth/constants"
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"NetworkAuth/utils"
	"NetworkAuth/utils/storage"
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 常量
// ============================================================================

// 自更新配置 settings 键
const (
	settingSelfUpdateType      = "self_update_type"       // 0 不启用 / 1 COS / 2 OSS
	settingSelfUpdateSecretID  = "self_update_secret_id"  // 密钥 ID
	settingSelfUpdateSecretKey = "self_update_secret_key" // 密钥 Key
	settingSelfUpdateRegion    = "self_update_region"     // 区域或 Endpoint
	settingSelfUpdateBucket    = "self_update_bucket"     // 存储桶名称
	settingSelfUpdatePrefix    = "self_update_prefix"     // 路径前缀
	settingSelfUpdateBaseURL   = "self_update_base_url"   // 自定义下载域名

	settingSelfUpdateLastCheckedAt  = "self_update_last_checked_at"  // 最近扫描时间戳
	settingSelfUpdateLastStatusJSON = "self_update_last_status_json" // 最近状态 JSON
	settingSelfUpdatePreparedAt     = "self_update_prepared_at"      // 准备完成时间戳
)

// selfAppName 自更新提取二进制时查找的文件名
const selfAppName = "NetworkAuth"

// selfUpdateAutoCheckInterval 自动检查去重间隔（10 分钟）
const selfUpdateAutoCheckInterval = 10 * time.Minute

// ============================================================================
// 类型定义
// ============================================================================

// SelfUpdateConfig 自更新存储桶配置
type SelfUpdateConfig struct {
	Type      int    `json:"type"`       // 0 不启用 / 1 COS / 2 OSS
	SecretID  string `json:"secret_id"`  // 密钥 ID
	SecretKey string `json:"secret_key"` // 密钥 Key
	Region    string `json:"region"`     // 区域或 Endpoint
	Bucket    string `json:"bucket"`     // 存储桶名称
	Prefix    string `json:"prefix"`     // 路径前缀
	BaseURL   string `json:"base_url"`   // 自定义下载域名
}

// SelfUpdateVersionItem 自更新版本列表中的单个条目
type SelfUpdateVersionItem struct {
	Version       string `json:"version"`        // 版本号
	Size          int64  `json:"size"`           // 匹配平台的包大小
	SizeFormatted string `json:"size_formatted"` // 格式化后的包大小
	SHA256        string `json:"sha256"`         // 包 SHA256
	DownloadURL   string `json:"download_url"`   // 下载链接
	IsNewer       bool   `json:"is_newer"`       // 是否比当前版本新
	IsCurrent     bool   `json:"is_current"`     // 是否为当前版本
}

// SelfUpdateStatus 自更新状态
type SelfUpdateStatus struct {
	Running      bool   `json:"running"`
	CheckedAt    int64  `json:"checked_at"`
	CheckedAtStr string `json:"checked_at_str"`
	LastError    string `json:"last_error"`

	CurrentVersion string `json:"current_version"` // 当前运行版本
	LatestVersion  string `json:"latest_version"`  // 最新云端版本
	VersionsCount  int    `json:"versions_count"`  // 云端版本总数

	// 准备（下载/安装）相关
	Prepared             bool   `json:"prepared"`
	PreparedVersion      string `json:"prepared_version,omitempty"`
	PreparedZip          string `json:"prepared_zip,omitempty"`   // 已下载更新包路径（持久化用于重启后校验，前端展示前经 DisplayPath 脱敏）
	StagingBinary        string `json:"staging_binary,omitempty"` // 已解压二进制路径（持久化用于重启后校验，前端展示前经 DisplayPath 脱敏）
	PrepareError         string `json:"prepare_error,omitempty"`
	AutoReplaceTried     bool   `json:"auto_replace_tried,omitempty"`
	AutoReplaceOK        bool   `json:"auto_replace_ok,omitempty"`
	AutoReplaceError     string `json:"auto_replace_error,omitempty"`
	ScriptShellPath      string `json:"script_shell_path,omitempty"`
	ScriptPowerShellPath string `json:"script_powershell_path,omitempty"`
	DownloadProgress     int    `json:"download_progress"` // 0-100
}

// SelfUpdateManager 自更新管理器单例
type SelfUpdateManager struct {
	mu           sync.Mutex
	running      bool
	last         SelfUpdateStatus
	lastChecked  time.Time
	lastAutoKick time.Time // 上次异步触发时间戳（用于防抖）
	loaded       bool
}

var selfUpdateMgr = &SelfUpdateManager{}

// GetSelfUpdateManager 获取自更新管理器单例
func GetSelfUpdateManager() *SelfUpdateManager {
	return selfUpdateMgr
}

// ============================================================================
// 配置读写
// ============================================================================

// LoadConfig 从 settings 表加载自更新存储桶配置
func (m *SelfUpdateManager) LoadConfig() *SelfUpdateConfig {
	svc := services.GetSettingsService()
	cfg := &SelfUpdateConfig{
		Type:      svc.GetInt(settingSelfUpdateType, 0),
		SecretID:  svc.GetString(settingSelfUpdateSecretID, ""),
		SecretKey: svc.GetString(settingSelfUpdateSecretKey, ""),
		Region:    svc.GetString(settingSelfUpdateRegion, ""),
		Bucket:    svc.GetString(settingSelfUpdateBucket, ""),
		Prefix:    svc.GetString(settingSelfUpdatePrefix, "NetworkAuth/"),
		BaseURL:   svc.GetString(settingSelfUpdateBaseURL, ""),
	}
	return cfg
}

// SaveConfig 保存自更新存储桶配置到 settings 表
func (m *SelfUpdateManager) SaveConfig(cfg *SelfUpdateConfig) error {
	ctx := context.Background()
	updates := map[string]string{
		settingSelfUpdateType:      strconv.Itoa(cfg.Type),
		settingSelfUpdateSecretID:  cfg.SecretID,
		settingSelfUpdateSecretKey: cfg.SecretKey,
		settingSelfUpdateRegion:    cfg.Region,
		settingSelfUpdateBucket:    cfg.Bucket,
		settingSelfUpdatePrefix:    cfg.Prefix,
		settingSelfUpdateBaseURL:   cfg.BaseURL,
	}
	for key, value := range updates {
		if err := selfUpdateSetSettingString(ctx, key, value, ""); err != nil {
			return err
		}
	}
	services.GetSettingsService().RefreshCache()
	return nil
}

// buildSelfUpdateStore 根据配置构建存储实例
func buildSelfUpdateStore(cfg *SelfUpdateConfig) storage.Storage {
	switch cfg.Type {
	case 1:
		s, _ := storage.NewCOSStorage(storage.COSConfig{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
			Region:    cfg.Region,
			Bucket:    cfg.Bucket,
			BaseURL:   cfg.BaseURL,
		})
		return s
	case 2:
		s, _ := storage.NewOSSStorage(storage.OSSConfig{
			AccessKeyID:     cfg.SecretID,
			AccessKeySecret: cfg.SecretKey,
			Endpoint:        cfg.Region,
			Bucket:          cfg.Bucket,
			BaseURL:         cfg.BaseURL,
		})
		return s
	}
	return nil
}

// ============================================================================
// 版本扫描
// ============================================================================

// ScanVersions 扫描存储桶获取版本列表（无版本限制，全部返回）
func (m *SelfUpdateManager) ScanVersions(ctx context.Context) ([]SelfUpdateVersionItem, error) {
	cfg := m.LoadConfig()
	if cfg.Type == 0 {
		return nil, errors.New("未配置存储桶")
	}

	store := buildSelfUpdateStore(cfg)
	if store == nil {
		return nil, errors.New("存储桶配置无效")
	}

	allVersions, err := store.ScanVersions(ctx, cfg.Prefix)
	if err != nil {
		return nil, err
	}
	if len(allVersions) == 0 {
		return nil, nil
	}

	currentVersion := selfUpdateNormalizeVersion(constants.AppVersion)
	var items []SelfUpdateVersionItem

	for _, v := range allVersions {
		isNewer := selfUpdateCompareVersion(currentVersion, v) < 0
		isCurrent := selfUpdateCompareVersion(currentVersion, v) == 0

		item := SelfUpdateVersionItem{
			Version:   v,
			IsNewer:   isNewer,
			IsCurrent: isCurrent,
		}

		// 获取该版本目录下的文件，填充大小和下载链接
		files, err := store.ListVersionFiles(ctx, cfg.Prefix, v)
		if err == nil && len(files) > 0 {
			filtered := selfUpdateFilterByPlatform(files)
			if len(filtered) == 0 {
				filtered = files
			}
			selected := filtered[0]
			item.Size = selected.Size
			item.SizeFormatted = selected.SizeFormatted
			item.DownloadURL = selected.DownloadURL

			// 读取 SHA256
			raw, err := store.GetObject(ctx, selected.RelativePath+".sha256")
			if err == nil {
				fields := strings.Fields(strings.TrimSpace(string(raw)))
				if len(fields) > 0 {
					h := strings.ToLower(strings.TrimSpace(fields[0]))
					if len(h) == 64 {
						item.SHA256 = h
					}
				}
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// selfUpdateFilterByPlatform 按当前运行平台筛选更新包文件
func selfUpdateFilterByPlatform(files []storage.FileInfo) []storage.FileInfo {
	goos := strings.ToLower(runtime.GOOS)
	goarch := strings.ToLower(runtime.GOARCH)

	var matched []storage.FileInfo
	for _, f := range files {
		name := strings.ToLower(f.Name)
		if strings.Contains(name, "-"+goos+"-"+goarch) {
			matched = append(matched, f)
		}
	}
	return matched
}

// ============================================================================
// 检查更新
// ============================================================================

// Check 同步检查更新：扫描存储桶，更新内部状态并持久化
func (m *SelfUpdateManager) Check(ctx context.Context) SelfUpdateStatus {
	m.mu.Lock()
	if m.running {
		st := m.statusLocked()
		m.mu.Unlock()
		return st
	}
	m.running = true
	m.mu.Unlock()

	m.doCheck(ctx)
	return m.GetStatus()
}

// CheckAsync 异步检查更新（带防抖：10 分钟内已有结果或 2 秒内重复触发则跳过）
func (m *SelfUpdateManager) CheckAsync(ctx context.Context) SelfUpdateStatus {
	now := time.Now()

	m.mu.Lock()
	m.ensureLoadedLocked()
	if m.running {
		st := m.statusLocked()
		m.mu.Unlock()
		return st
	}
	if now.Sub(m.lastChecked) < selfUpdateAutoCheckInterval {
		st := m.statusLocked()
		m.mu.Unlock()
		return st
	}
	if !m.lastAutoKick.IsZero() && now.Sub(m.lastAutoKick) < 2*time.Second {
		st := m.statusLocked()
		m.mu.Unlock()
		return st
	}
	m.running = true
	m.lastAutoKick = now
	m.mu.Unlock()

	go func() {
		m.doCheck(context.Background())
	}()
	return m.GetStatus()
}

// CheckForceAsync 异步强制检查更新（绕过 CheckAsync 的节流缓存，手动检查用）
func (m *SelfUpdateManager) CheckForceAsync(ctx context.Context) SelfUpdateStatus {
	m.mu.Lock()
	m.ensureLoadedLocked()
	if m.running {
		st := m.statusLocked()
		m.mu.Unlock()
		return st
	}
	m.running = true
	m.lastAutoKick = time.Now()
	m.mu.Unlock()

	go func() {
		m.doCheck(context.Background())
	}()
	return m.GetStatus()
}

// doCheck 执行实际的检查逻辑（scan + persist）
func (m *SelfUpdateManager) doCheck(ctx context.Context) {
	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()

	items, err := m.ScanVersions(ctx)
	if err != nil {
		m.mu.Lock()
		m.last.LastError = err.Error()
		m.lastChecked = time.Now()
		m.last.VersionsCount = 0
		m.last.LatestVersion = ""
		m.persistLocked(ctx)
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.lastChecked = time.Now()
	m.last.LastError = ""

	// 从版本列表中找出最新版
	if len(items) > 0 {
		m.last.LatestVersion = items[0].Version // 列表已按版本降序，第一项即最新
		m.last.VersionsCount = len(items)
	} else {
		m.last.LatestVersion = ""
		m.last.VersionsCount = 0
	}
	m.persistLocked(ctx)
	m.mu.Unlock()
}

// statusLocked 返回当前状态的快照（需已持有锁）
func (m *SelfUpdateManager) statusLocked() SelfUpdateStatus {
	m.ensureLoadedLocked()
	st := m.last
	st.CurrentVersion = constants.AppVersion
	st.Running = m.running
	if !m.lastChecked.IsZero() {
		st.CheckedAt = m.lastChecked.Unix()
		st.CheckedAtStr = m.lastChecked.Format(time.RFC3339)
	}
	return st
}

// ============================================================================
// 状态管理
// ============================================================================

// GetStatus 获取当前更新状态
func (m *SelfUpdateManager) GetStatus() SelfUpdateStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureLoadedLocked()
	// 实时读取：仅当“已准备”态失效（缓存缺失/已生效）时清理并写回；
	// 不触碰本次会话进行中的失败反馈（PrepareError），以免前端轮询丢失失败原因
	if m.last.Prepared && !m.preparedInstallValidLocked() {
		m.clearPreparedLocked()
		m.persistLocked(context.Background())
	}
	st := m.last
	st.CurrentVersion = constants.AppVersion
	st.PreparedZip = utils.DisplayPath(st.PreparedZip)
	st.StagingBinary = utils.DisplayPath(st.StagingBinary)
	st.ScriptShellPath = utils.DisplayPath(st.ScriptShellPath)
	st.ScriptPowerShellPath = utils.DisplayPath(st.ScriptPowerShellPath)
	st.Running = m.running
	if !m.lastChecked.IsZero() {
		st.CheckedAt = m.lastChecked.Unix()
		st.CheckedAtStr = m.lastChecked.Format(time.RFC3339)
	}
	return st
}

// ensureLoadedLocked 从数据库加载状态（首次访问时）
func (m *SelfUpdateManager) ensureLoadedLocked() {
	if m.loaded {
		return
	}
	m.loaded = true

	svc := services.GetSettingsService()
	checkedAtStr := strings.TrimSpace(svc.GetString(settingSelfUpdateLastCheckedAt, ""))
	if checkedAtStr != "" {
		if ts, err := strconv.ParseInt(checkedAtStr, 10, 64); err == nil && ts > 0 {
			m.lastChecked = time.Unix(ts, 0)
		}
	}

	raw := strings.TrimSpace(svc.GetString(settingSelfUpdateLastStatusJSON, ""))
	if raw == "" {
		return
	}

	var st SelfUpdateStatus
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return
	}
	st.Running = false
	st.CheckedAt = 0
	st.CheckedAtStr = ""

	m.last = st
	// 重启加载：仅保留“有效且待安装”的准备态；
	// 失败/进行中/已生效/缓存缺失等残留一律清理并写回，避免重启后仍显示旧的“准备状态/下载失败”
	if !m.preparedInstallValidLocked() && m.hasAnyPrepareStateLocked() {
		m.clearPreparedLocked()
		m.persistLocked(context.Background())
	}
}

// clearPreparedLocked 清空所有准备/安装/下载相关的状态字段（需已持有锁）
func (m *SelfUpdateManager) clearPreparedLocked() {
	m.last.Prepared = false
	m.last.PreparedVersion = ""
	m.last.PreparedZip = ""
	m.last.StagingBinary = ""
	m.last.PrepareError = ""
	m.last.AutoReplaceTried = false
	m.last.AutoReplaceOK = false
	m.last.AutoReplaceError = ""
	m.last.ScriptShellPath = ""
	m.last.ScriptPowerShellPath = ""
	m.last.DownloadProgress = 0
}

// preparedInstallValidLocked 判断当前是否持有一个“有效且待安装”的准备态（需已持有锁）
// 有效条件：已准备完成、准备版本不等于当前运行版本（即尚未生效）、且本地缓存文件仍存在。
func (m *SelfUpdateManager) preparedInstallValidLocked() bool {
	if !m.last.Prepared {
		return false
	}
	// 准备版本已等于当前运行版本：更新已生效，不再算待安装
	if m.last.PreparedVersion != "" &&
		selfUpdateCompareVersion(m.last.PreparedVersion, constants.AppVersion) == 0 {
		return false
	}
	// staging 二进制或更新包仍存在才算有效
	if m.last.StagingBinary != "" {
		if _, err := os.Stat(m.last.StagingBinary); err == nil {
			return true
		}
	}
	if m.last.PreparedZip != "" {
		if _, err := os.Stat(m.last.PreparedZip); err == nil {
			return true
		}
	}
	return false
}

// hasAnyPrepareStateLocked 是否存在任何准备/失败/进度残留字段（需已持有锁）
func (m *SelfUpdateManager) hasAnyPrepareStateLocked() bool {
	return m.last.Prepared ||
		m.last.PreparedVersion != "" ||
		m.last.PrepareError != "" ||
		m.last.PreparedZip != "" ||
		m.last.StagingBinary != "" ||
		m.last.AutoReplaceTried ||
		m.last.AutoReplaceOK ||
		m.last.AutoReplaceError != "" ||
		m.last.ScriptShellPath != "" ||
		m.last.ScriptPowerShellPath != "" ||
		m.last.DownloadProgress != 0
}

// persistLocked 持久化状态到数据库
func (m *SelfUpdateManager) persistLocked(ctx context.Context) {
	ts := int64(0)
	if !m.lastChecked.IsZero() {
		ts = m.lastChecked.Unix()
	}
	_ = selfUpdateSetSettingString(ctx, settingSelfUpdateLastCheckedAt, strconv.FormatInt(ts, 10), "自更新最近扫描时间戳")

	b, err := json.Marshal(m.last)
	if err != nil {
		return
	}
	_ = selfUpdateSetSettingString(ctx, settingSelfUpdateLastStatusJSON, string(b), "自更新最近状态")
}

// ============================================================================
// 辅助函数
// ============================================================================

// selfUpdateSetSettingString 写入设置到数据库
func selfUpdateSetSettingString(ctx context.Context, name, value, description string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	var setting models.Settings
	result := db.WithContext(ctx).Where("name = ?", name).First(&setting)
	if result.Error != nil {
		setting = models.Settings{
			Name:        name,
			Value:       value,
			Description: description,
		}
		return db.WithContext(ctx).Create(&setting).Error
	}

	return db.WithContext(ctx).Model(&setting).Update("value", value).Error
}

// selfUpdateNormalizeVersion 标准化版本号字符串
func selfUpdateNormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// selfUpdateCompareVersion 比较两个版本号
// 返回值: 1 表示 a > b，-1 表示 a < b，0 表示相等
func selfUpdateCompareVersion(a, b string) int {
	aa := selfUpdateNormalizeVersion(a)
	bb := selfUpdateNormalizeVersion(b)
	if aa == "" && bb == "" {
		return 0
	}

	parseParts := func(s string) []int {
		if s == "" {
			return nil
		}
		parts := strings.Split(s, ".")
		result := make([]int, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				result = append(result, 0)
				continue
			}
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

	x := parseParts(aa)
	y := parseParts(bb)
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
