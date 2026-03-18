package server

import (
	default_ctrl "NetworkAuth/controllers/default"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 路由注册函数
// ============================================================================

// RegisterDefaultRoutes 注册默认路由
// 只包含根路径，用于默认主页功能
func RegisterDefaultRoutes(r *gin.Engine) {
	// 根路径 - 主页
	r.GET("/", default_ctrl.RootHandler)
}
