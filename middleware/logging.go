package middleware

import (
	"time"

	"NetworkAuth/utils/logger"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ============================================================================
// 结构体定义
// ============================================================================

// LoggingMiddleware 日志记录中间件结构体
// 用于记录HTTP请求的详细信息，包括方法、路径、状态码和响应时间
type LoggingMiddleware struct {
	logger *logger.Logger
}

// ============================================================================
// 构造函数
// ============================================================================

// NewLoggingMiddleware 创建新的日志记录中间件实例
func NewLoggingMiddleware(logger *logger.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

// ============================================================================
// 中间件函数
// ============================================================================

// Handler 返回Gin中间件函数，用于记录HTTP请求日志
// 记录格式参考了更灵活的 NetworkAuth 实现，支持配置开关和日志级别检查
func (lm *LoggingMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否启用了访问日志
		if !viper.GetBool("server.access_log") {
			c.Next()
			return
		}

		// 如果日志级别不是Debug或更高（Trace），则不记录访问日志
		// 避免在Info级别输出过多的访问日志干扰正常业务日志
		if lm.logger.Level < logrus.DebugLevel {
			c.Next()
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 计算响应时间
		duration := time.Since(start)

		if raw != "" {
			path = path + "?" + raw
		}

		// 记录请求日志
		lm.logger.LogRequestWithHeaders(
			c.Request.Method,
			path,
			c.ClientIP(), // 使用 Gin 内置的方法获取 IP
			c.Writer.Status(),
			duration,
			c.Errors.ByType(gin.ErrorTypePrivate).String(),
			c.Request.UserAgent(),
		)
	}
}

// ============================================================================
// 公共函数
// ============================================================================

// WrapHandler 创建Gin日志中间件
// 使用全局日志记录器创建日志中间件
func WrapHandler() gin.HandlerFunc {
	log := logger.GetLogger()
	middleware := NewLoggingMiddleware(log)
	return middleware.Handler()
}
