package server

import (
	"NetworkAuth/controllers/install"

	"github.com/gin-gonic/gin"
)

// RegisterInstallRoutes 注册安装相关的路由
func RegisterInstallRoutes(r *gin.Engine) {
	// 安装向导页面
	r.GET("/install", install.InstallPageHandler)

	// 提交安装表单
	r.POST("/api/install", install.InstallSubmitHandler)
}
