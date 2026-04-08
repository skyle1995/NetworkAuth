package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"NetworkAuth/utils"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ============================================================================
// 公共函数
// ============================================================================

// ValidateConfig 验证配置
func ValidateConfig() (*AppConfig, error) {
	var config AppConfig

	// 解析配置到结构体
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 验证配置
	if err := ValidateConfigValue(&config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	log.Info("配置内容验证通过")
	return &config, nil
}

// ValidateConfigValue 验证配置
func ValidateConfigValue(config *AppConfig) error {
	// 验证服务器配置
	if err := validateServerConfig(&config.Server); err != nil {
		return fmt.Errorf("服务器配置错误: %w", err)
	}

	// 验证数据库配置
	if err := validateDatabaseConfig(&config.Database); err != nil {
		return fmt.Errorf("数据库配置错误: %w", err)
	}

	// 验证Redis配置
	if err := validateRedisConfig(&config.Redis); err != nil {
		return fmt.Errorf("redis配置错误: %w", err)
	}

	// 验证日志配置
	if err := validateLogConfig(&config.Log); err != nil {
		return fmt.Errorf("日志配置错误: %w", err)
	}

	return nil
}

// validateServerConfig 验证服务器配置
func validateServerConfig(config *ServerConfig) error {
	// 验证主机地址
	if config.Host != "" {
		if ip := net.ParseIP(config.Host); ip == nil && config.Host != "localhost" {
			return fmt.Errorf("无效的主机地址: %s", config.Host)
		}
	}

	// 验证端口
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("无效的端口号: %d，端口号必须在1-65535之间", config.Port)
	}

	return nil
}

// validateDatabaseConfig 验证数据库配置
func validateDatabaseConfig(config *DatabaseConfig) error {
	// 验证数据库类型
	validTypes := []string{"mysql", "sqlite"}
	if !contains(validTypes, config.Type) {
		return fmt.Errorf("不支持的数据库类型: %s，支持的类型: %s", config.Type, strings.Join(validTypes, ", "))
	}

	// 根据类型验证具体配置
	switch config.Type {
	case "mysql":
		return validateMySQLConfig(&config.MySQL)
	case "sqlite":
		return validateSQLiteConfig(&config.SQLite)
	}

	return nil
}

// validateMySQLConfig 验证MySQL配置
func validateMySQLConfig(config *MySQLConfig) error {
	if config.Host == "" {
		return errors.New("MySQL主机地址不能为空")
	}
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("无效的MySQL端口号: %d", config.Port)
	}
	if config.Username == "" {
		return errors.New("MySQL用户名不能为空")
	}
	if config.Database == "" {
		return errors.New("MySQL数据库名不能为空")
	}
	if config.MaxIdleConns < 0 {
		return errors.New("MySQL最大空闲连接数不能为负数")
	}
	if config.MaxOpenConns < 0 {
		return errors.New("MySQL最大打开连接数不能为负数")
	}
	return nil
}

// validateSQLiteConfig 验证SQLite配置
func validateSQLiteConfig(config *SQLiteConfig) error {
	if config.Path == "" {
		return errors.New("SQLite数据库路径不能为空")
	}

	path := config.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(utils.GetRootDir(), path)
	}

	// 检查目录是否存在，不存在则创建
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建SQLite数据库目录失败: %w", err)
		}
	}

	return nil
}

// validateRedisConfig 验证Redis配置
func validateRedisConfig(config *RedisConfig) error {
	if config.Host == "" {
		return errors.New("Redis主机地址不能为空")
	}
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("无效的Redis端口号: %d", config.Port)
	}
	if config.DB < 0 || config.DB > 15 {
		return fmt.Errorf("无效的Redis数据库索引: %d，必须在0-15之间", config.DB)
	}
	return nil
}

// validateLogConfig 验证日志配置
func validateLogConfig(config *LogConfig) error {
	// 验证日志级别
	validLevels := []string{"trace", "debug", "info", "warn", "error", "fatal", "panic"}
	if !contains(validLevels, config.Level) {
		return fmt.Errorf("无效的日志级别: %s，支持的级别: %s", config.Level, strings.Join(validLevels, ", "))
	}

	// 检查日志文件目录（仅当日志文件路径不为空时）
	if config.File != "" {
		path := config.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(utils.GetRootDir(), path)
		}
		dir := filepath.Dir(path)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("创建日志目录失败: %w", err)
			}
		}
	}

	// 验证日志轮转配置
	if config.MaxSize <= 0 {
		return errors.New("日志文件最大大小必须大于0")
	}
	if config.MaxBackups < 0 {
		return errors.New("日志备份文件数量不能为负数")
	}
	if config.MaxAge < 0 {
		return errors.New("日志文件保留天数不能为负数")
	}

	return nil
}

// contains 检查切片是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetConfigValue 获取配置值，支持类型转换和默认值
func GetConfigValue[T any](key string, defaultValue T) T {
	if !viper.IsSet(key) {
		return defaultValue
	}

	value := viper.Get(key)
	if result, ok := value.(T); ok {
		return result
	}

	// 尝试类型转换
	if converted, err := convertValue[T](value); err == nil {
		return converted
	}

	return defaultValue
}

// convertValue 尝试类型转换
func convertValue[T any](value interface{}) (T, error) {
	var zero T
	str := fmt.Sprintf("%v", value)

	switch any(zero).(type) {
	case int:
		if i, err := strconv.Atoi(str); err == nil {
			return any(i).(T), nil
		}
	case string:
		return any(str).(T), nil
	case bool:
		if b, err := strconv.ParseBool(str); err == nil {
			return any(b).(T), nil
		}
	}

	return zero, fmt.Errorf("无法转换类型")
}
