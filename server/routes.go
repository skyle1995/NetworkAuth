package server

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 聚合注册所有路由
func RegisterRoutes(r *gin.Engine) {
	// 所有路由基于 /api
	apiGroup := r.Group("/api")

	RegisterInstallRoutes(apiGroup)
	RegisterDefaultRoutes(apiGroup)
	RegisterAdminRoutes(apiGroup)
}
