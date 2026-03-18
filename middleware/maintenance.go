package middleware

import (
	"net/http"
	"strings"

	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
)

// MaintenanceMiddleware 维护模式中间件
// 当开启维护模式时，拦截非白名单请求
func MaintenanceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否开启维护模式
		if !services.GetSettingsService().IsMaintenanceMode() {
			c.Next()
			return
		}

		// 白名单检查（路径前缀匹配）
		path := c.Request.URL.Path

		// 1. 允许静态资源
		if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/assets/") || path == "/favicon.ico" {
			c.Next()
			return
		}

		// 2. 允许管理员后台相关接口（以便管理员登录关闭维护模式）
		// 包括登录页、登录接口、API接口、CSRF Token等
		if strings.HasPrefix(path, "/admin") {
			c.Next()
			return
		}

		// 3. 检查请求类型
		// AJAX/JSON 请求返回 503 JSON
		accept := c.GetHeader("Accept")
		xrw := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Requested-With")))
		if strings.Contains(accept, "application/json") || xrw == "xmlhttprequest" || strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    503,
				"success": false,
				"msg":     "系统正在维护中，请稍后再试",
			})
			c.Abort()
			return
		}

		// 4. 普通页面请求渲染维护页面
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusServiceUnavailable)
		c.Writer.WriteString(maintenanceHTML)
		c.Abort()
	}
}

// 简单的维护页面 HTML
const maintenanceHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>系统维护中</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background-color: #f0f2f5;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            color: #333;
        }
        .container {
            text-align: center;
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.1);
            max-width: 500px;
            width: 90%;
        }
        h1 { font-size: 24px; margin-bottom: 16px; color: #1890ff; }
        p { font-size: 16px; color: #666; line-height: 1.6; }
        .icon { font-size: 64px; margin-bottom: 24px; color: #faad14; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">⚠️</div>
        <h1>系统维护中</h1>
        <p>为了提供更好的服务，系统正在进行升级维护。<br>请稍后访问，给您带来的不便敬请谅解。</p>
    </div>
</body>
</html>`
