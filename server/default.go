package server

import (
	defaultctrl "NetworkAuth/controllers/default"

	"github.com/gin-gonic/gin"
)

// RegisterDefaultRoutes 注册默认路由
// 包含根路径、健康检查、API信息等基础端点
func RegisterDefaultRoutes(rg *gin.RouterGroup) {
	homeGroup := rg.Group("/home")

	// 根路径
	homeGroup.GET("", defaultctrl.RootHandler)
}
