package default_ctrl

import (
	"NetworkAuth/services"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RootHandler 根路径处理器
// 使用模板渲染服务器信息页面
func RootHandler(c *gin.Context) {
	// 获取设置服务
	settings := services.GetSettingsService()

	// 传递模板数据
	data := map[string]interface{}{
		"Title":         settings.GetString("site_title", "NetworkAuth Server"),
		"Keywords":      settings.GetString("site_keywords", ""),
		"Description":   settings.GetString("site_description", ""),
		"SystemName":    "系统提醒", // 对应 H1
		"WarningText":   "🚫 未授权，拒绝访问",
		"InfoText":      "💬 如有问题，请联系网站管理员",
		"FooterText":    settings.GetString("footer_text", "Copyright © 2026 NetworkAuth. All Rights Reserved."),
		"ICPRecord":     settings.GetString("icp_record", ""),
		"ICPRecordLink": settings.GetString("icp_record_link", "https://beian.miit.gov.cn"),
		"CurrentYear":   time.Now().Year(),
	}

	c.HTML(http.StatusOK, "index.html", data)
}
