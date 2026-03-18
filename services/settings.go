package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"
)

// ============================================================================
// 结构体定义
// ============================================================================

// SettingsService 设置服务
type SettingsService struct {
	mu    sync.RWMutex
	cache map[string]string
}

// ============================================================================
// 全局变量
// ============================================================================

var settingsService *SettingsService
var settingsOnce sync.Once

// ============================================================================
// 公共函数
// ============================================================================

// GetSettingsService 获取设置服务单例
func GetSettingsService() *SettingsService {
	settingsOnce.Do(func() {
		settingsService = &SettingsService{
			cache: make(map[string]string),
		}
		// 初始化时加载所有设置
		settingsService.loadAllSettings()
	})
	return settingsService
}

// ============================================================================
// 私有函数
// ============================================================================

// loadAllSettings 从数据库加载所有设置到缓存
func (s *SettingsService) loadAllSettings() {
	db, err := database.GetDB()
	if err != nil {
		logrus.WithError(err).Error("获取数据库连接失败")
		return
	}
	// 如果数据库未初始化，直接返回，保持缓存为空
	if db == nil {
		return
	}

	var settings []models.Settings
	if err := db.Find(&settings).Error; err != nil {
		logrus.WithError(err).Error("加载设置失败")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, setting := range settings {
		s.cache[setting.Name] = setting.Value
	}

	logrus.WithField("count", len(settings)).Debug("设置缓存加载完成")
}

// Set 设置值（用于测试或运行时更新）
func (s *SettingsService) Set(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[name] = value
}

// GetSettingRealtime 实时获取设置值（优先使用Redis缓存，自动回源数据库）
// 相比 GetString 的内存缓存，此方法能感知其他实例或直接数据库的变更（在Redis TTL过期后）
func (s *SettingsService) GetSettingRealtime(name string) (*models.Settings, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return FindSettingByName(name, db)
}

// GetString 获取字符串类型的设置值
func (s *SettingsService) GetString(name, defaultValue string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if value, exists := s.cache[name]; exists {
		return value
	}
	return defaultValue
}

// GetInt 获取整数类型的设置值
func (s *SettingsService) GetInt(name string, defaultValue int) int {
	strValue := s.GetString(name, "")
	if strValue == "" {
		return defaultValue
	}

	if intValue, err := strconv.Atoi(strValue); err == nil {
		return intValue
	}
	return defaultValue
}

// GetBool 获取布尔类型的设置值
func (s *SettingsService) GetBool(name string, defaultValue bool) bool {
	strValue := s.GetString(name, "")
	if strValue == "" {
		return defaultValue
	}

	return strValue == "1" || strValue == "true"
}

// RefreshCache 刷新设置缓存
func (s *SettingsService) RefreshCache() {
	s.loadAllSettings()
}

// GetSessionTimeout 获取会话超时时间（秒）
func (s *SettingsService) GetSessionTimeout() int {
	return s.GetInt("session_timeout", 3600) // 默认1小时
}

// IsMaintenanceMode 检查是否开启维护模式
func (s *SettingsService) IsMaintenanceMode() bool {
	return s.GetBool("maintenance_mode", false)
}

// GetJWTSecret 获取JWT密钥
func (s *SettingsService) GetJWTSecret() string {
	return s.GetString("jwt_secret", "")
}

// GetEncryptionKey 获取加密密钥
func (s *SettingsService) GetEncryptionKey() string {
	return s.GetString("encryption_key", "")
}

// GetJWTRefresh 获取JWT刷新时间（小时）
func (s *SettingsService) GetJWTRefresh() int {
	return s.GetInt("jwt_refresh", 6)
}

// GetJWTExpire 获取JWT有效期（小时）
func (s *SettingsService) GetJWTExpire() int {
	return s.GetInt("jwt_expire", 24)
}

// GetCookieConfig 获取Cookie配置
func (s *SettingsService) GetCookieConfig() (secure bool, sameSite string, domain string, maxAge int) {
	secure = s.GetBool("cookie_secure", true)
	sameSite = s.GetString("cookie_same_site", "Lax")
	domain = s.GetString("cookie_domain", "")
	maxAge = s.GetInt("cookie_max_age", 86400)
	return
}
