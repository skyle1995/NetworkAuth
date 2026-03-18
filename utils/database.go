package utils

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// ============================================================================
// 结构体定义
// ============================================================================

// DatabaseConfig 数据库连接池配置结构体
// 用于配置数据库连接池的各项参数，包括连接池大小、生命周期管理和健康检查等
type DatabaseConfig struct {
	// 连接池配置
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`     // 最大空闲连接数
	MaxOpenConns    int           `mapstructure:"max_open_conns"`     // 最大打开连接数
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`  // 连接最大生存时间
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"` // 连接最大空闲时间

	// 健康检查配置
	PingTimeout         time.Duration `mapstructure:"ping_timeout"`          // Ping超时时间
	HealthCheckInterval time.Duration `mapstructure:"health_check_interval"` // 健康检查间隔
}

// ============================================================================
// 配置函数
// ============================================================================

// GetDefaultDatabaseConfig 获取默认数据库配置
// 返回一个包含合理默认值的数据库配置实例
func GetDefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		MaxIdleConns:        10,               // 默认最大空闲连接数
		MaxOpenConns:        100,              // 默认最大打开连接数
		ConnMaxLifetime:     30 * time.Minute, // 连接最大生存时间30分钟
		ConnMaxIdleTime:     10 * time.Minute, // 连接最大空闲时间10分钟
		PingTimeout:         5 * time.Second,  // Ping超时5秒
		HealthCheckInterval: 30 * time.Second, // 健康检查间隔30秒
	}
}

// LoadDatabaseConfig 从配置文件加载数据库配置
// 使用指定的前缀从viper配置中读取数据库配置，如果配置项不存在则使用默认值
func LoadDatabaseConfig(prefix string) *DatabaseConfig {
	config := GetDefaultDatabaseConfig()

	// 从viper读取配置，如果不存在则使用默认值
	if viper.IsSet(prefix + ".max_idle_conns") {
		config.MaxIdleConns = viper.GetInt(prefix + ".max_idle_conns")
	}
	if viper.IsSet(prefix + ".max_open_conns") {
		config.MaxOpenConns = viper.GetInt(prefix + ".max_open_conns")
	}
	if viper.IsSet(prefix + ".conn_max_lifetime") {
		config.ConnMaxLifetime = viper.GetDuration(prefix + ".conn_max_lifetime")
	}
	if viper.IsSet(prefix + ".conn_max_idle_time") {
		config.ConnMaxIdleTime = viper.GetDuration(prefix + ".conn_max_idle_time")
	}
	if viper.IsSet(prefix + ".ping_timeout") {
		config.PingTimeout = viper.GetDuration(prefix + ".ping_timeout")
	}
	if viper.IsSet(prefix + ".health_check_interval") {
		config.HealthCheckInterval = viper.GetDuration(prefix + ".health_check_interval")
	}

	return config
}

// ConfigureConnectionPool 配置数据库连接池
// 根据提供的配置参数设置GORM数据库的连接池属性
func ConfigureConnectionPool(db *gorm.DB, config *DatabaseConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取底层数据库连接失败: %w", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// LogInfo("数据库连接池配置完成", map[string]interface{}{
	// 	"max_idle_conns":     config.MaxIdleConns,
	// 	"max_open_conns":     config.MaxOpenConns,
	// 	"conn_max_lifetime":  config.ConnMaxLifetime,
	// 	"conn_max_idle_time": config.ConnMaxIdleTime,
	// })

	return nil
}

// ============================================================================
// 全局变量
// ============================================================================

var (
	// redisClient 全局Redis客户端
	redisClient *redis.Client
	// redisOnce 确保只初始化一次
	redisOnce sync.Once
	// redisAvailable 标记Redis是否可用
	redisAvailable bool
)

// ============================================================================
// Redis函数
// ============================================================================

// RedisLogger 自定义Redis日志记录器
// 仅在Debug级别输出Redis内部日志
type RedisLogger struct{}

func (l *RedisLogger) Printf(ctx context.Context, format string, v ...interface{}) {
	if logrus.GetLevel() >= logrus.DebugLevel {
		logrus.Debugf(format, v...)
	}
}

// InitRedis 初始化Redis客户端（仅在配置存在时尝试连接）
// - 从 viper 读取 redis.* 配置
// - 如果连接失败，则标记为不可用，不影响主流程
func InitRedis() {
	redisOnce.Do(func() {
		// 设置自定义日志记录器，避免在Info级别输出大量连接错误日志
		redis.SetLogger(&RedisLogger{})

		host := viper.GetString("redis.host")
		port := viper.GetInt("redis.port")
		if host == "" || port == 0 {
			logrus.Info("未配置Redis或配置不完整，跳过初始化")
			redisAvailable = false
			return
		}
		addr := fmt.Sprintf("%s:%d", host, port)
		redisClient = redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: viper.GetString("redis.password"),
			DB:       viper.GetInt("redis.db"),
		})
		// 健康检查
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logrus.WithError(err).Warn("Redis初始化失败，标记为不可用")
			redisAvailable = false
			return
		}
		redisAvailable = true
		logrus.WithField("addr", addr).Info("Redis 连接已建立")
	})
}

// GetRedis 获取全局Redis客户端，可能返回nil（当不可用时）
func GetRedis() *redis.Client {
	if redisClient == nil {
		InitRedis()
	}
	if !redisAvailable {
		return nil
	}
	return redisClient
}

// IsRedisAvailable 判断Redis是否可用
func IsRedisAvailable() bool {
	if redisClient == nil {
		InitRedis()
	}
	return redisAvailable
}

// RedisGetOrSet 通用Redis缓存获取或设置函数（基于JSON序列化）
// - ctx: 上下文
// - key: 缓存键
// - ttl: 过期时间
// - loader: 当缓存不存在时的加载函数（一般执行数据库查询）
// 返回：目标对象指针和错误
func RedisGetOrSet[T any](ctx context.Context, key string, ttl time.Duration, loader func() (*T, error)) (*T, error) {
	// 如果Redis不可用则直接调用加载函数
	if !IsRedisAvailable() {
		return loader()
	}
	client := GetRedis()
	if client == nil {
		return loader()
	}

	// 先尝试从缓存读取
	data, err := client.Get(ctx, key).Bytes()
	if err == nil {
		var out T
		if uerr := json.Unmarshal(data, &out); uerr == nil {
			return &out, nil
		}
		// 反序列化失败时视为未命中，继续加载
		logrus.WithError(err).WithField("key", key).Warn("Redis缓存反序列化失败，回退到loader")
	} else if err != redis.Nil {
		// 非空且非不存在的错误，记录告警但不中断
		logrus.WithError(err).WithField("key", key).Warn("读取Redis缓存失败")
	}

	// 加载数据
	val, lerr := loader()
	if lerr != nil {
		return nil, lerr
	}
	if val == nil {
		return nil, nil
	}

	// 写回缓存（错误不影响主流程）
	if b, merr := json.Marshal(val); merr == nil {
		if serr := client.Set(ctx, key, b, ttl).Err(); serr != nil {
			logrus.WithError(serr).WithField("key", key).Warn("写入Redis缓存失败")
		}
	}
	return val, nil
}

// RedisDel 删除一个或多个Redis键（当Redis不可用时静默返回）
// - ctx: 上下文
// - keys: 需要删除的键名
func RedisDel(ctx context.Context, keys ...string) error {
	// 如果Redis不可用则直接返回
	if !IsRedisAvailable() {
		return nil
	}
	client := GetRedis()
	if client == nil {
		return nil
	}
	if len(keys) == 0 {
		return nil
	}
	if _, err := client.Del(ctx, keys...).Result(); err != nil {
		logrus.WithError(err).WithField("keys", keys).Warn("删除Redis键失败")
		return err
	}
	return nil
}

// PingDatabase 检查数据库连接健康状态
// 使用指定的超时时间ping数据库以验证连接是否正常
func PingDatabase(db *gorm.DB, timeout time.Duration) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取底层数据库连接失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return sqlDB.PingContext(ctx)
}

// GetConnectionStats 获取数据库连接池统计信息
// 返回当前数据库连接池的详细统计数据，包括连接数、等待时间等
func GetConnectionStats(db *gorm.DB) (*sql.DBStats, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层数据库连接失败: %w", err)
	}

	stats := sqlDB.Stats()
	return &stats, nil
}

// LogConnectionStats 记录数据库连接池统计信息
// 获取并记录数据库连接池的统计信息到日志中，用于监控和调试
func LogConnectionStats(db *gorm.DB) {
	stats, err := GetConnectionStats(db)
	if err != nil {
		LogError("获取数据库连接池统计信息失败", err, nil)
		return
	}

	LogInfo("数据库连接池统计", map[string]interface{}{
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration,
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	})
}

// StartHealthCheck 启动数据库健康检查
// 启动一个后台goroutine定期检查数据库连接健康状态
// 只在健康检查失败时输出错误日志，正常情况下不输出日志
// 返回一个 CancelFunc 用于停止健康检查
func StartHealthCheck(db *gorm.DB, config *DatabaseConfig) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(config.HealthCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := PingDatabase(db, config.PingTimeout); err != nil {
					// 只在健康检查失败时输出错误日志
					LogError("数据库健康检查失败", err, map[string]interface{}{
						"ping_timeout": config.PingTimeout,
					})
				}

				// 记录连接池统计信息（仅在调试模式下）
				if logrus.GetLevel() == logrus.DebugLevel {
					LogConnectionStats(db)
				}
			}
		}
	}()

	// LogInfo("数据库健康检查已启动", map[string]interface{}{
	// 	"check_interval": config.HealthCheckInterval,
	// 	"ping_timeout":   config.PingTimeout,
	// })

	return cancel
}

// ValidateDatabaseConfig 验证数据库配置参数
// 检查数据库配置参数的有效性，确保所有参数都在合理范围内
func ValidateDatabaseConfig(config *DatabaseConfig) error {
	if config.MaxIdleConns < 0 {
		return fmt.Errorf("最大空闲连接数不能为负数: %d", config.MaxIdleConns)
	}
	if config.MaxOpenConns < 0 {
		return fmt.Errorf("最大打开连接数不能为负数: %d", config.MaxOpenConns)
	}
	if config.MaxIdleConns > config.MaxOpenConns && config.MaxOpenConns > 0 {
		return fmt.Errorf("最大空闲连接数(%d)不能大于最大打开连接数(%d)", config.MaxIdleConns, config.MaxOpenConns)
	}
	if config.ConnMaxLifetime < 0 {
		return fmt.Errorf("连接最大生存时间不能为负数: %v", config.ConnMaxLifetime)
	}
	if config.ConnMaxIdleTime < 0 {
		return fmt.Errorf("连接最大空闲时间不能为负数: %v", config.ConnMaxIdleTime)
	}
	if config.PingTimeout <= 0 {
		return fmt.Errorf("Ping超时时间必须大于0: %v", config.PingTimeout)
	}
	if config.HealthCheckInterval <= 0 {
		return fmt.Errorf("健康检查间隔必须大于0: %v", config.HealthCheckInterval)
	}

	return nil
}
