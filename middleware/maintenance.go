package middleware

import (
	"net/http"
	"strings"

	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
)

// MaintenanceMiddleware 维护模式中间件
// 当开启维护模式时，拦截非白名单请求
func MaintenanceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否开启维护模式
		if !services.GetSettingsService().IsMaintenanceMode() {
			c.Next()
			return
		}

		path := c.Request.URL.Path

		// 允许管理员后台相关接口（以便管理员登录关闭维护模式）
		// 包括登录页、登录接口、API接口、CSRF Token等
		if strings.HasPrefix(path, "/api/admin") {
			c.Next()
			return
		}

		// 返回 503 JSON
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"success": false,
			"msg":     "系统正在维护中，请稍后再试",
		})
		c.Abort()
	}
}
