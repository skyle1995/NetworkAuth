package server

import (
	defaultctrl "NetworkAuth/controllers/default"
	"NetworkAuth/middleware"
	"time"

	"github.com/gin-gonic/gin"
)

// RegisterDefaultRoutes 注册默认路由
// 包含根路径、健康检查、API信息等基础端点
func RegisterDefaultRoutes(rg *gin.RouterGroup) {
	homeGroup := rg.Group("/home")

	// 根路径 (限制：每分钟最多 60 次请求，防止 CC)
	homeGroup.GET("", middleware.RateLimit(60, time.Minute), defaultctrl.RootHandler)
}
