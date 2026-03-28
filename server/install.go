package server

import (
	"NetworkAuth/controllers/install"

	"github.com/gin-gonic/gin"
)

// RegisterInstallRoutes 注册安装相关的路由
func RegisterInstallRoutes(rg *gin.RouterGroup) {
	installGroup := rg.Group("/install")

	// 提交安装表单
	installGroup.POST("", install.InstallSubmitHandler)
}
