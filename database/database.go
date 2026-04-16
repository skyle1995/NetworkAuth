package database

import (
	appconfig "NetworkAuth/config"
	"NetworkAuth/utils"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gLogger "gorm.io/gorm/logger"
)

// ============================================================================
// 全局变量
// ============================================================================

var (
	// dbInstance 全局 *gorm.DB 实例，使用单例确保全局复用
	dbInstance *gorm.DB
	// healthCheckCancel 数据库健康检查取消函数
	healthCheckCancel context.CancelFunc
	// once 确保初始化只执行一次
	once sync.Once
)

// ============================================================================
// 公共函数
// ============================================================================

// Init 初始化数据库连接（根据配置自动选择驱动）
// - 默认使用 SQLite（github.com/glebarez/sqlite）
// - 生产环境支持 MySQL（gorm.io/driver/mysql）
func Init() (*gorm.DB, error) {
	var initErr error
	once.Do(func() {
		initErr = performInitFromViper()
	})
	return dbInstance, initErr
}

// GetDB 获取全局 *gorm.DB 实例
// 如果未初始化，会尝试初始化一次
func GetDB() (*gorm.DB, error) {
	if dbInstance != nil {
		return dbInstance, nil
	}
	return Init()
}

// ReInit 重新初始化数据库连接
// 用于在修改配置后重新连接数据库
func ReInit() (*gorm.DB, error) {
	closeCurrentDB()

	// 在 ReInit 时，强制从 viper 重新读取配置并连接，忽略"系统尚未安装"的检查
	// 因为这是安装过程触发的
	var cfg appconfig.AppConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := performInitWithConfig(&cfg); err != nil {
		return nil, err
	}

	if dbInstance == nil {
		return nil, fmt.Errorf("数据库实例初始化后为空")
	}

	return dbInstance, nil
}

func InitWithAppConfig(cfg *appconfig.AppConfig) (*gorm.DB, error) {
	closeCurrentDB()
	if err := performInitWithConfig(cfg); err != nil {
		return nil, err
	}
	return dbInstance, nil
}

func performInitFromViper() error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = "config.json"
	}

	// 从 viper 中读取配置
	var cfg appconfig.AppConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return err
	}

	// 检查数据库类型，如果文件或配置不存在，说明系统尚未安装，跳过数据库连接
	switch cfg.Database.Type {
	case "sqlite":
		dbPath := cfg.Database.SQLite.Path
		if dbPath == "" {
			dbPath = "database.db"
		}
		if !filepath.IsAbs(dbPath) {
			dbPath = filepath.Join(utils.GetRootDir(), dbPath)
		}
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			logrus.Info("SQLite 数据库文件不存在，系统尚未安装，跳过数据库连接")
			return nil
		}
	case "mysql":
		// 只有在明确配置了 host 并且不是安装请求时才去连接 MySQL
		// 我们通过检查是否已有有效配置来判断，比如检查 database 是否为空
		if cfg.Database.MySQL.Database == "" {
			logrus.Info("MySQL 数据库名称未配置，说明系统尚未安装，跳过数据库连接")
			return nil
		}
	}

	return performInitWithConfig(&cfg)
}

func performInitWithConfig(cfg *appconfig.AppConfig) error {
	if cfg == nil {
		return fmt.Errorf("应用配置不能为空")
	}
	if err := appconfig.ValidateConfigValue(cfg); err != nil {
		return err
	}
	var initErr error
	switch cfg.Database.Type {
	case "mysql":
		initErr = initMySQL(&cfg.Database.MySQL, cfg.Log.Level)
		if initErr != nil {
			logrus.WithError(initErr).Error("MySQL 数据库连接失败，请检查配置或重新安装")
			// 既然 MySQL 连不上，说明系统无法正常工作，直接返回错误，由外层决定是否退出
			return initErr
		}
	default:
		initErr = initSQLite(&cfg.Database.SQLite, cfg.Log.Level)
	}
	if initErr != nil || dbInstance == nil {
		return initErr
	}
	dbConfig := buildPoolConfig(cfg)
	if err := utils.ValidateDatabaseConfig(dbConfig); err != nil {
		logrus.WithError(err).Warn("数据库配置验证失败，使用默认配置")
		dbConfig = utils.GetDefaultDatabaseConfig()
	}
	if err := utils.ConfigureConnectionPool(dbInstance, dbConfig); err != nil {
		logrus.WithError(err).Error("配置数据库连接池失败")
	}
	healthCheckCancel = utils.StartHealthCheck(dbInstance, dbConfig)
	return nil
}

// SetDB 设置全局 *gorm.DB 实例（用于测试）
func SetDB(db *gorm.DB) {
	dbInstance = db
}

// ============================================================================
// 私有函数
// ============================================================================

func closeCurrentDB() {
	if healthCheckCancel != nil {
		healthCheckCancel()
		healthCheckCancel = nil
	}
	if dbInstance != nil {
		if sqlDB, err := dbInstance.DB(); err == nil {
			sqlDB.Close()
		}
	}
	dbInstance = nil
}

func buildPoolConfig(cfg *appconfig.AppConfig) *utils.DatabaseConfig {
	dbConfig := utils.GetDefaultDatabaseConfig()
	if cfg.Database.Type == "mysql" {
		if cfg.Database.MySQL.MaxIdleConns > 0 {
			dbConfig.MaxIdleConns = cfg.Database.MySQL.MaxIdleConns
		}
		if cfg.Database.MySQL.MaxOpenConns > 0 {
			dbConfig.MaxOpenConns = cfg.Database.MySQL.MaxOpenConns
		}
		return dbConfig
	}
	dbConfig.MaxIdleConns = 1
	dbConfig.MaxOpenConns = 1
	return dbConfig
}

func buildGormLogger(level string) gLogger.Interface {
	var logLevel gLogger.LogLevel
	switch level {
	case "debug":
		logLevel = gLogger.Info
	case "error":
		logLevel = gLogger.Error
	default:
		logLevel = gLogger.Warn
	}
	return gLogger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), gLogger.Config{SlowThreshold: 2 * time.Second, LogLevel: logLevel, IgnoreRecordNotFoundError: true, Colorful: false})
}

func initSQLite(sqliteConfig *appconfig.SQLiteConfig, logLevel string) error {
	path := sqliteConfig.Path
	if path == "" {
		path = "database.db"
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(utils.GetRootDir(), path)
	}
	dsn := fmt.Sprintf("file:%s?cache=shared&_busy_timeout=5000&_fk=1", path)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: buildGormLogger(logLevel)})
	if err != nil {
		logrus.WithError(err).Error("SQLite 初始化失败")
		return err
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}
	dbInstance = db
	logrus.WithField("path", utils.DisplayPath(path)).Info("SQLite 连接已建立")
	return nil
}

func initMySQL(mysqlConfig *appconfig.MySQLConfig, logLevel string) error {
	host := mysqlConfig.Host
	port := mysqlConfig.Port
	user := mysqlConfig.Username
	pass := mysqlConfig.Password
	dbname := mysqlConfig.Database
	charset := mysqlConfig.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local", user, pass, host, port, dbname, charset)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: buildGormLogger(logLevel)})
	if err != nil {
		logrus.WithError(err).Error("MySQL 初始化失败")
		return err
	}
	dbInstance = db
	logrus.WithField("host", host).WithField("database", dbname).Info("MySQL 连接已建立")
	return nil
}
