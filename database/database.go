package database

import (
	"NetworkAuth/utils"
	"context"
	"fmt"
	"log"
	"os"
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
		initErr = performInit()
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
	// 如果已有连接，尝试关闭它
	if dbInstance != nil {
		if healthCheckCancel != nil {
			healthCheckCancel()
			healthCheckCancel = nil
		}
		if sqlDB, err := dbInstance.DB(); err == nil {
			sqlDB.Close()
		}
	}
	dbInstance = nil

	// 重新执行初始化逻辑（不经过 once.Do）
	return dbInstance, performInit()
}

func performInit() error {
	// 检查是否已经有配置文件（通过检查文件是否存在）
	configFile := viper.ConfigFileUsed()
	// 如果 viper 没有使用配置文件（可能是因为没找到文件而使用了默认配置），
	// 或者配置文件路径为空，我们应该假设处于未安装状态。
	// 但 viper.ConfigFileUsed() 在 ReadInConfig 成功后会返回文件名。
	// 如果 ReadInConfig 失败（因为文件不存在），viper 可能会返回空或者我们在 config.go 中设置的路径。

	// 在 config.go 中，如果文件不存在，我们加载了默认配置但没有写文件。
	// 此时 viper.ConfigFileUsed() 可能是空的或者我们设置的路径。
	// 让我们检查该路径对应的文件是否存在。

	if configFile == "" {
		configFile = "config.json"
	}

	_, err := os.Stat(configFile)
	isConfigExists := !os.IsNotExist(err)

	// 如果配置文件不存在，说明还没有经过安装初始化，暂时不连接数据库
	if !isConfigExists {
		logrus.Info("尚未初始化配置，跳过数据库连接")
		return nil
	}

	var initErr error
	dbType := viper.GetString("database.type")
	switch dbType {
	case "mysql":
		initErr = initMySQL()
	default:
		initErr = initSQLite()
	}

	// 如果数据库初始化成功，配置连接池和启动健康检查
	if initErr == nil && dbInstance != nil {
		// 加载数据库配置
		var configPrefix string
		if dbType == "mysql" {
			configPrefix = "database.mysql"
		} else {
			configPrefix = "database.sqlite"
		}

		dbConfig := utils.LoadDatabaseConfig(configPrefix)

		// 验证配置
		if err := utils.ValidateDatabaseConfig(dbConfig); err != nil {
			logrus.WithError(err).Warn("数据库配置验证失败，使用默认配置")
			dbConfig = utils.GetDefaultDatabaseConfig()
		}

		// 配置连接池
		if err := utils.ConfigureConnectionPool(dbInstance, dbConfig); err != nil {
			logrus.WithError(err).Error("配置数据库连接池失败")
		}

		// 启动健康检查
		healthCheckCancel = utils.StartHealthCheck(dbInstance, dbConfig)
	}
	return initErr
}

// SetDB 设置全局 *gorm.DB 实例（用于测试）
func SetDB(db *gorm.DB) {
	dbInstance = db
}

// ============================================================================
// 私有函数
// ============================================================================

// initSQLite 初始化 SQLite 数据库
// 使用 viper 中的 database.sqlite.path 作为数据库文件路径
func initSQLite() error {
	path := viper.GetString("database.sqlite.path")
	if path == "" {
		path = "./recharge.db"
	}
	dsn := fmt.Sprintf("file:%s?cache=shared&_busy_timeout=5000&_fk=1", path)
	var logLevel gLogger.LogLevel
	switch viper.GetString("logger.level") {
	case "debug":
		logLevel = gLogger.Info
	case "error":
		logLevel = gLogger.Error
	default:
		logLevel = gLogger.Warn
	}
	gl := gLogger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), gLogger.Config{SlowThreshold: 2 * time.Second, LogLevel: logLevel, IgnoreRecordNotFoundError: true, Colorful: false})
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gl})
	if err != nil {
		logrus.WithError(err).Error("SQLite 初始化失败")
		return err
	}

	// SQLite 连接池配置（SQLite 对连接池支持有限，但仍可设置基本参数）
	if sqlDB, err := db.DB(); err == nil {
		// SQLite 通常使用单连接，但可以设置一些基本参数
		sqlDB.SetMaxOpenConns(1) // SQLite 建议使用单连接
		sqlDB.SetMaxIdleConns(1)
	}

	dbInstance = db
	logrus.WithField("path", path).Info("SQLite 连接已建立")
	return nil
}

// initMySQL 初始化 MySQL 数据库
// 从 viper 读取 database.mysql.* 配置构建 DSN
func initMySQL() error {
	host := viper.GetString("database.mysql.host")
	port := viper.GetInt("database.mysql.port")
	user := viper.GetString("database.mysql.username")
	pass := viper.GetString("database.mysql.password")
	dbname := viper.GetString("database.mysql.database")
	charset := viper.GetString("database.mysql.charset")
	if charset == "" {
		charset = "utf8mb4"
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local", user, pass, host, port, dbname, charset)
	var logLevel gLogger.LogLevel
	switch viper.GetString("logger.level") {
	case "debug":
		logLevel = gLogger.Info
	case "error":
		logLevel = gLogger.Error
	default:
		logLevel = gLogger.Warn
	}
	gl := gLogger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), gLogger.Config{SlowThreshold: 2 * time.Second, LogLevel: logLevel, IgnoreRecordNotFoundError: true, Colorful: false})
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: gl})
	if err != nil {
		logrus.WithError(err).Error("MySQL 初始化失败")
		return err
	}

	dbInstance = db
	logrus.WithField("host", host).WithField("database", dbname).Info("MySQL 连接已建立")
	return nil
}
