package middleware

import (
	"errors"
	"net/http"
	"strings"

	"NetworkAuth/models"
	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
)

// ctxAPIKey 上下文中存放已鉴权密钥的键名
const ctxAPIKey = "api_key"

// extractAPIKey 依次从 X-API-Key 头、Authorization: Bearer、query 的 apikey 中取密钥。
// 优先请求头；query 仅为方便 GET 接口传参。
func extractAPIKey(c *gin.Context) string {
	if v := strings.TrimSpace(c.GetHeader("X-API-Key")); v != "" {
		return v
	}
	if auth := strings.TrimSpace(c.GetHeader("Authorization")); auth != "" {
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			return strings.TrimSpace(auth[7:])
		}
		return auth
	}
	if v := strings.TrimSpace(c.Query("apikey")); v != "" {
		return v
	}
	return ""
}

// RequireAPIKey 返回一个要求携带有效密钥且具备指定能力(scope)的中间件。
// 通过后把密钥记录写入上下文，供 handler 取用。
func RequireAPIKey(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, err := services.AuthAPIKey(extractAPIKey(c), scope, c.ClientIP())
		if err != nil {
			status := http.StatusForbidden
			if errors.Is(err, services.ErrAPIKeyMissing) {
				status = http.StatusUnauthorized
			}
			c.JSON(status, gin.H{"code": status, "error": err.Error()})
			c.Abort()
			return
		}
		c.Set(ctxAPIKey, key)
		c.Next()
	}
}

// CurrentAPIKey 从上下文取出已鉴权的密钥（未鉴权时返回 nil）。
func CurrentAPIKey(c *gin.Context) *models.ApiKey {
	if v, ok := c.Get(ctxAPIKey); ok {
		if k, ok := v.(*models.ApiKey); ok {
			return k
		}
	}
	return nil
}
