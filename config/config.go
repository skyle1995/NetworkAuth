package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"NetworkAuth/utils"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var currentConfigFilePath string

// ============================================================================
// 结构体定义
// ============================================================================

// ServerConfig 服务器配置结构体
// 包含服务器运行相关的配置信息
type ServerConfig struct {
	Host             string   `json:"host" mapstructure:"host"`                             // 服务器监听地址
	Port             int      `json:"port" mapstructure:"port"`                             // 服务器监听端口
	Dist             string   `json:"dist" mapstructure:"dist"`                             // 静态文件目录
	DevMode          bool     `json:"dev_mode" mapstructure:"dev_mode"`                     // 开发模式（跳过验证码等）
	AccessLog        bool     `json:"access_log" mapstructure:"access_log"`                 // 是否输出访问日志
	CorsAllowOrigins []string `json:"cors_allow_origins" mapstructure:"cors_allow_origins"` // 允许跨域携带凭证的来源白名单（为空时回退到安全降级策略）
}

// DatabaseConfig 数据库配置结构体
// 包含数据库连接相关的配置信息
type DatabaseConfig struct {
	Type   string       `json:"type" mapstructure:"type"`     // 数据库类型（mysql/sqlite）
	MySQL  MySQLConfig  `json:"mysql" mapstructure:"mysql"`   // MySQL配置
	SQLite SQLiteConfig `json:"sqlite" mapstructure:"sqlite"` // SQLite配置
}

// MySQLConfig MySQL数据库配置结构体
// 包含MySQL数据库连接的详细配置信息
type MySQLConfig struct {
	Host         string `json:"host" mapstructure:"host"`                     // 数据库主机地址
	Port         int    `json:"port" mapstructure:"port"`                     // 数据库端口
	Username     string `json:"username" mapstructure:"username"`             // 数据库用户名
	Password     string `json:"password" mapstructure:"password"`             // 数据库密码
	Database     string `json:"database" mapstructure:"database"`             // 数据库名称
	Charset      string `json:"charset" mapstructure:"charset"`               // 字符集
	MaxIdleConns int    `json:"max_idle_conns" mapstructure:"max_idle_conns"` // 最大空闲连接数
	MaxOpenConns int    `json:"max_open_conns" mapstructure:"max_open_conns"` // 最大打开连接数
}

// SQLiteConfig SQLite数据库配置结构体
// 包含SQLite数据库文件路径配置
type SQLiteConfig struct {
	Path string `json:"path" mapstructure:"path"` // 数据库文件路径
}

// RedisConfig Redis配置结构体
// 包含Redis连接相关的配置信息
type RedisConfig struct {
	Host     string `json:"host" mapstructure:"host"`         // Redis服务器地址
	Port     int    `json:"port" mapstructure:"port"`         // Redis服务器端口
	Password string `json:"password" mapstructure:"password"` // Redis密码
	DB       int    `json:"db" mapstructure:"db"`             // Redis数据库编号
}

// LogConfig 日志配置结构体
// 包含日志记录相关的配置信息
type LogConfig struct {
	Level      string `json:"level" mapstructure:"level"`             // 日志级别
	File       string `json:"file" mapstructure:"file"`               // 日志文件路径
	MaxSize    int    `json:"max_size" mapstructure:"max_size"`       // 单个日志文件最大大小(MB)
	MaxBackups int    `json:"max_backups" mapstructure:"max_backups"` // 保留的旧日志文件数量
	MaxAge     int    `json:"max_age" mapstructure:"max_age"`         // 日志文件保留天数
}

// AppConfig 应用配置结构体
type AppConfig struct {
	Server   ServerConfig   `json:"server" mapstructure:"server"`
	Database DatabaseConfig `json:"database" mapstructure:"database"`
	Redis    RedisConfig    `json:"redis" mapstructure:"redis"`
	Log      LogConfig      `json:"log" mapstructure:"log"`
}

// ============================================================================
// 公共函数
// ============================================================================

// GetDefaultAppConfig 获取默认应用配置
func GetDefaultAppConfig() *AppConfig {
	return &AppConfig{
		Server: ServerConfig{
			Host:             "0.0.0.0",
			Port:             8080,
			Dist:             "",
			DevMode:          false,
			AccessLog:        true,
			CorsAllowOrigins: []string{},
		},
		Database: DatabaseConfig{
			Type: "sqlite",
			MySQL: MySQLConfig{
				Host:         "localhost",
				Port:         3306,
				Username:     "",
				Password:     "",
				Database:     "",
				Charset:      "utf8mb4",
				MaxIdleConns: 10,
				MaxOpenConns: 100,
			},
			SQLite: SQLiteConfig{
				Path: "database.db",
			},
		},
		Redis: RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       0,
		},
		Log: LogConfig{
			Level:      "info",
			File:       "logs/app.log",
			MaxSize:    100,
			MaxBackups: 5,
			MaxAge:     30,
		},
	}
}

// Init 初始化配置文件
func Init(cfgFilePath string) {
	if !filepath.IsAbs(cfgFilePath) {
		if wd, err := os.Getwd(); err == nil {
			candidate := filepath.Clean(filepath.Join(wd, cfgFilePath))
			if _, statErr := os.Stat(candidate); statErr == nil {
				cfgFilePath = candidate
			} else {
				cfgFilePath = filepath.Join(utils.GetRootDir(), cfgFilePath)
			}
		} else {
			cfgFilePath = filepath.Join(utils.GetRootDir(), cfgFilePath)
		}
	}

	currentConfigFilePath = cfgFilePath
	viper.SetConfigFile(cfgFilePath)
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	defaultConfig := GetDefaultAppConfig()
	var needWrite bool
	var configBytes []byte

	// 检查配置文件是否存在
	fileContent, err := os.ReadFile(cfgFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.WithField("file", utils.DisplayPath(filepath.Clean(cfgFilePath))).Info("配置文件不存在，将在本地生成默认配置")
			needWrite = true
		} else {
			log.WithField("err", err).Fatal("读取配置文件失败")
		}
	} else {
		// 尝试解析现有的配置，与默认配置合并，结构不一致则重写
		if err := json.Unmarshal(fileContent, defaultConfig); err != nil {
			log.WithField("err", err).Warn("配置文件解析失败，将使用默认值重写")
			needWrite = true
		} else {
			// 将合并后的配置重新序列化，比对是否需要更新（例如结构体增加了新字段）
			newBytes, _ := json.MarshalIndent(defaultConfig, "", "  ")
			// 简单比较去除空白后的长度或内容
			if !bytes.Equal(bytes.TrimSpace(fileContent), bytes.TrimSpace(newBytes)) {
				needWrite = true
			}
		}
	}

	if needWrite {
		configBytes, err = json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			log.WithField("err", err).Fatal("配置序列化错误")
		}
		if err := os.MkdirAll(filepath.Dir(cfgFilePath), 0755); err != nil {
			log.WithField("err", err).Fatal("创建配置目录失败")
		}
		if err := os.WriteFile(cfgFilePath, configBytes, 0644); err != nil {
			log.WithField("err", err).Fatal("写入配置文件失败")
		}
		if len(fileContent) == 0 {
			log.Info("已成功生成并加载默认配置")
		} else {
			log.Info("已写出更新后的配置结构到文件")
		}
	} else {
		configBytes = fileContent
	}

	// 将配置加载到viper中
	if err := viper.ReadConfig(bytes.NewBuffer(configBytes)); err != nil {
		log.WithField("err", err).Fatal("读取配置到viper失败")
	}

	cleanPath := filepath.Clean(cfgFilePath)
	log.WithField("file", utils.DisplayPath(cleanPath)).Info("使用配置文件")

	// 验证配置
	if _, err := ValidateConfig(); err != nil {
		log.WithFields(
			log.Fields{
				"err": err,
			},
		).Fatal("配置内容验证失败")
	}
}

func SaveConfig(appConfig *AppConfig) error {
	if err := ValidateConfigValue(appConfig); err != nil {
		return err
	}
	if currentConfigFilePath == "" {
		currentConfigFilePath = "config.json"
	}
	if !filepath.IsAbs(currentConfigFilePath) {
		currentConfigFilePath = filepath.Join(utils.GetRootDir(), currentConfigFilePath)
	}
	if err := os.MkdirAll(filepath.Dir(currentConfigFilePath), 0755); err != nil {
		return err
	}
	configBytes, err := json.MarshalIndent(appConfig, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(currentConfigFilePath, configBytes, 0644); err != nil {
		return err
	}
	viper.SetConfigFile(currentConfigFilePath)
	viper.SetConfigType("json")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	syncViperConfig(appConfig)
	return nil
}

// UpdateConfig 更新配置文件
// 接收一个回调函数，在回调函数中修改配置对象，然后保存到文件
func UpdateConfig(updateFn func(*AppConfig)) error {
	// 1. 获取当前配置
	var currentConfig AppConfig
	if err := viper.Unmarshal(&currentConfig); err != nil {
		return err
	}

	// 2. 执行更新回调
	updateFn(&currentConfig)

	return SaveConfig(&currentConfig)
}

func syncViperConfig(currentConfig *AppConfig) {
	viper.Set("server.host", currentConfig.Server.Host)
	viper.Set("server.port", currentConfig.Server.Port)
	viper.Set("server.dist", currentConfig.Server.Dist)
	viper.Set("server.dev_mode", currentConfig.Server.DevMode)
	viper.Set("server.access_log", currentConfig.Server.AccessLog)
	viper.Set("server.cors_allow_origins", currentConfig.Server.CorsAllowOrigins)

	viper.Set("database.type", currentConfig.Database.Type)
	viper.Set("database.mysql.host", currentConfig.Database.MySQL.Host)
	viper.Set("database.mysql.port", currentConfig.Database.MySQL.Port)
	viper.Set("database.mysql.username", currentConfig.Database.MySQL.Username)
	viper.Set("database.mysql.password", currentConfig.Database.MySQL.Password)
	viper.Set("database.mysql.database", currentConfig.Database.MySQL.Database)
	viper.Set("database.mysql.charset", currentConfig.Database.MySQL.Charset)
	viper.Set("database.mysql.max_idle_conns", currentConfig.Database.MySQL.MaxIdleConns)
	viper.Set("database.mysql.max_open_conns", currentConfig.Database.MySQL.MaxOpenConns)
	viper.Set("database.sqlite.path", currentConfig.Database.SQLite.Path)

	viper.Set("redis.host", currentConfig.Redis.Host)
	viper.Set("redis.port", currentConfig.Redis.Port)
	viper.Set("redis.password", currentConfig.Redis.Password)
	viper.Set("redis.db", currentConfig.Redis.DB)

	viper.Set("log.level", currentConfig.Log.Level)
	viper.Set("log.file", currentConfig.Log.File)
	viper.Set("log.max_size", currentConfig.Log.MaxSize)
	viper.Set("log.max_backups", currentConfig.Log.MaxBackups)
	viper.Set("log.max_age", currentConfig.Log.MaxAge)
}
