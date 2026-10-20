package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// ============================================================================
// 结构体定义
// ============================================================================

// DevModeConfig 开发模式配置
type DevModeConfig struct {
	// 是否跳过验证码验证（【已废弃】dev_mode 不再影响验证码；
	// 现在只有配置文件中显式开启 security.skip_captcha 才会跳过，字段保留以兼容结构体解析）
	SkipCaptcha bool
	// 是否显示详细错误信息
	ShowDetailedErrors bool
	// 是否启用调试日志
	EnableDebugLog bool
}

// ============================================================================
// 中间件函数
// ============================================================================

// DevModeMiddleware 开发模式中间件
// 统一管理所有开发模式相关的功能
func DevModeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否为开发模式
		if IsDevMode() {
			// 设置开发模式标识到上下文
			c.Set("dev_mode", true)
			c.Set("dev_config", GetDevModeConfig())

			// 设置开发模式相关的响应头
			c.Header("X-Dev-Mode", "true")
		} else {
			c.Set("dev_mode", false)
		}

		c.Next()
	}
}

// ============================================================================
// 公共函数
// ============================================================================

// IsDevMode 检查是否为开发模式
func IsDevMode() bool {
	return viper.GetBool("server.dev_mode")
}

// GetDevModeConfig 获取开发模式配置
func GetDevModeConfig() DevModeConfig {
	if !IsDevMode() {
		return DevModeConfig{}
	}

	return DevModeConfig{
		// 安全修复：开发模式不再跳过登录验证码。历史行为会让「生产环境误留 dev_mode=true」
		// 直接失去验证码防线（攻击日志里只出现「密码错误」、没有「验证码错误」正是此特征）。
		// 需要本地调试时请显式开启配置 security.skip_captcha（启动时会打印严重安全警告）。
		SkipCaptcha:        false,
		ShowDetailedErrors: true, // 开发模式下显示详细错误
		EnableDebugLog:     true, // 开发模式下启用调试日志
	}
}

// IsDevModeFromContext 从上下文中检查是否为开发模式
func IsDevModeFromContext(c *gin.Context) bool {
	if devMode, exists := c.Get("dev_mode"); exists {
		if isDevMode, ok := devMode.(bool); ok {
			return isDevMode
		}
	}
	// 回退到配置检查
	return IsDevMode()
}

// GetDevModeConfigFromContext 从上下文中获取开发模式配置
func GetDevModeConfigFromContext(c *gin.Context) DevModeConfig {
	if config, exists := c.Get("dev_config"); exists {
		if devConfig, ok := config.(DevModeConfig); ok {
			return devConfig
		}
	}
	// 回退到默认配置
	return GetDevModeConfig()
}

// ShouldSkipCaptcha 检查是否应该跳过验证码验证
//
// 只认配置文件中显式开启的 security.skip_captcha（默认 false），
// 与 dev_mode 完全解耦：开发模式遗留在生产环境时验证码依然生效。
func ShouldSkipCaptcha(c *gin.Context) bool {
	_ = c
	return viper.GetBool("security.skip_captcha")
}

// ============================================================================
// 私有函数
// ============================================================================
