package default_ctrl

import (
	"NetworkAuth/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RootHandler 根路径处理器
// 返回服务器信息 JSON
func RootHandler(c *gin.Context) {
	// 获取设置服务
	settings := services.GetSettingsService()

	// 传递数据
	data := gin.H{
		"title":       settings.GetString("site_title", "NetworkAuth Server"),
		"description": settings.GetString("site_description", ""),
		"status":      "running",
		"message":     "NetworkAuth API Server is running",
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": data,
	})
}
