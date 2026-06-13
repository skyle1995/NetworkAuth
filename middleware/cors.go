package middleware

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// CorsMiddleware 处理跨域请求
// 允许 Vue 等前端分离架构在开发和生产环境下访问后端 API
//
// 安全策略（避免“反射任意来源 + 允许携带凭证”这一危险组合）：
//  1. 配置了 server.cors_allow_origins 白名单：仅放行白名单内来源，并允许携带凭证；
//  2. 未配置白名单但处于开发模式：放行任意来源并允许凭证，方便本地前后端分离调试；
//  3. 未配置白名单且为生产模式：放行任意来源但禁止携带凭证（安全降级），
//     防止恶意站点借用浏览器凭证发起跨域请求。
func CorsMiddleware() gin.HandlerFunc {
	devMode := viper.GetBool("server.dev_mode")

	// 构建来源白名单集合
	allowSet := make(map[string]struct{})
	for _, o := range viper.GetStringSlice("server.cors_allow_origins") {
		if o = strings.TrimSpace(o); o != "" {
			allowSet[o] = struct{}{}
		}
	}

	cfg := cors.Config{
		AllowMethods:  []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-CSRF-Token", "Accept"},
		ExposeHeaders: []string{"Content-Length"},
		MaxAge:        12 * time.Hour,
	}

	switch {
	case len(allowSet) > 0:
		// 显式白名单：仅允许列表内来源，可携带凭证
		cfg.AllowOriginFunc = func(origin string) bool {
			_, ok := allowSet[origin]
			return ok
		}
		cfg.AllowCredentials = true
	case devMode:
		// 开发模式：放行任意来源并允许凭证
		cfg.AllowOriginFunc = func(origin string) bool { return true }
		cfg.AllowCredentials = true
	default:
		// 生产且未配置白名单：放行任意来源但禁止携带凭证（安全降级）
		cfg.AllowOriginFunc = func(origin string) bool { return true }
		cfg.AllowCredentials = false
	}

	return cors.New(cfg)
}
