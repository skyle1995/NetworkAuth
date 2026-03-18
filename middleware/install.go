package middleware

import (
	"NetworkAuth/services"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// InstallCheckMiddleware 检查系统是否已安装
func InstallCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 放行静态资源和favicon
		if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/assets/") || path == "/favicon.ico" {
			c.Next()
			return
		}

		// 检查是否为安装相关的路由
		isInstallRoute := path == "/install" || path == "/api/install"

		// 获取系统的安装状态
		// 在没有数据库的时候，GetSettingsService().GetString 会返回默认值 "0"
		isInstalled := services.GetSettingsService().GetString("is_installed", "0") == "1"

		// 如果未安装且当前不是访问安装页面，则重定向到安装页面
		if !isInstalled && !isInstallRoute {
			// 对于 API 请求，返回 JSON 提示
			if strings.HasPrefix(path, "/api/") || strings.Contains(path, "/api/") {
				c.JSON(http.StatusForbidden, gin.H{
					"code": 403,
					"msg":  "系统未初始化，请先完成安装",
				})
				c.Abort()
				return
			}
			c.Redirect(http.StatusTemporaryRedirect, "/install")
			c.Abort()
			return
		}

		// 如果已安装但尝试访问安装页面，则重定向到首页或后台
		if isInstalled && isInstallRoute {
			c.Redirect(http.StatusTemporaryRedirect, "/admin")
			c.Abort()
			return
		}

		c.Next()
	}
}
