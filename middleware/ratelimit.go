package middleware

import (
	"NetworkAuth/utils"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimit 基于 Redis 的简单固定窗口限流中间件
// limit: 时间窗口内允许的最大请求数
// window: 时间窗口大小
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		client := utils.GetRedis()
		if client == nil {
			// 如果 Redis 未配置或不可用，则放行（降级处理）
			c.Next()
			return
		}

		ip := c.ClientIP()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// 构建 Redis Key，按 IP 和接口路径限制
		key := fmt.Sprintf("ratelimit:%s:%s", path, ip)
		ctx := context.Background()

		// 使用 INCR 增加计数
		count, err := client.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}

		// 如果是第一次访问，设置过期时间
		if count == 1 {
			client.Expire(ctx, key, window)
		}

		if count > int64(limit) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}