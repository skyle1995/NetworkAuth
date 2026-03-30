package middleware

import (
	"NetworkAuth/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// InstallCheckMiddleware 检查系统是否已安装
func InstallCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		isInstallRoute := path == "/api/install" || path == "/api/install/"

		// 获取系统的安装状态
		isInstalledStr := services.GetSettingsService().GetString("is_installed", "0")
		isInstalled := isInstalledStr == "1"

		// 如果未安装且是 API 请求但不是安装接口，则返回 403 JSON
		// 如果是前端页面请求，不在此处拦截，交由前端 Vue Router 拦截并跳转至安装页
		if !isInstalled && !isInstallRoute && len(path) >= 4 && path[:4] == "/api" {
			c.JSON(http.StatusForbidden, gin.H{
				"code": 403,
				"msg":  "系统未初始化，请先完成安装",
			})
			c.Abort()
			return
		}

		// 如果已安装但尝试访问安装接口，则返回 403 JSON
		if isInstalled && isInstallRoute {
			c.JSON(http.StatusForbidden, gin.H{
				"code": 403,
				"msg":  "系统已安装，请勿重复初始化",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
