package middleware

import (
	"NetworkAuth/web"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// ============================================================================
// 结构体定义
// ============================================================================

// DevModeConfig 开发模式配置
type DevModeConfig struct {
	// 是否启用模板热重载
	EnableTemplateReload bool
	// 是否跳过验证码验证
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
func DevModeMiddleware(engine *gin.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否为开发模式
		if IsDevMode() {
			// 设置开发模式标识到上下文
			c.Set("dev_mode", true)
			c.Set("dev_config", GetDevModeConfig())

			// 如果启用了模板热重载，则重新加载模板
			config := GetDevModeConfig()
			if config.EnableTemplateReload {
				reloadTemplates(engine)
			}

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
		EnableTemplateReload: true, // 开发模式下默认启用模板热重载
		SkipCaptcha:          true, // 开发模式下默认跳过验证码
		ShowDetailedErrors:   true, // 开发模式下显示详细错误
		EnableDebugLog:       true, // 开发模式下启用调试日志
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
func ShouldSkipCaptcha(c *gin.Context) bool {
	config := GetDevModeConfigFromContext(c)
	return config.SkipCaptcha
}

// ============================================================================
// 私有函数
// ============================================================================

// reloadTemplates 重新加载模板（内部函数）
func reloadTemplates(engine *gin.Engine) {
	if tmpl, err := web.ParseTemplates(); err == nil {
		engine.SetHTMLTemplate(tmpl)
	}
}
