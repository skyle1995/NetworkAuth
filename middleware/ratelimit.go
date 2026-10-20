package middleware

import (
	"NetworkAuth/utils"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 限流中间件
//
// 设计要点（安全修复）：
//  1. Redis 可用时使用 Redis 固定窗口计数（多实例共享）。
//  2. Redis 不可用（未配置/连接失败/执行超时）时回退到进程内内存限流，
//     绝不能像旧实现那样直接放行——否则「没装 Redis」等于所有限流全部关闭。
//  3. 限流 key 使用 ClientIP()，其可信度依赖 server.trusted_proxies 配置：
//     未配置受信任代理时 ClientIP 为 TCP 对端地址，无法通过伪造请求头绕过。
// ============================================================================

const (
	// redisRateLimitTimeout 单次 Redis 计数操作的超时，避免 Redis 卡顿拖慢登录接口
	redisRateLimitTimeout = 500 * time.Millisecond
	// memoryRateLimitMaxEntries 内存兜底限流的 key 上限，防止被海量伪造 IP 撑爆内存
	memoryRateLimitMaxEntries = 100000
)

// rateBucket 内存固定窗口计数器
type rateBucket struct {
	count   int64
	resetAt time.Time
}

// memoryRateLimiter 进程内固定窗口限流器（Redis 不可用时的兜底实现）
type memoryRateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*rateBucket
	maxEntries  int
	lastCleanup time.Time
}

var fallbackRateLimiter = newMemoryRateLimiter(memoryRateLimitMaxEntries)

func newMemoryRateLimiter(maxEntries int) *memoryRateLimiter {
	return &memoryRateLimiter{
		buckets:    make(map[string]*rateBucket),
		maxEntries: maxEntries,
	}
}

// incr 自增 key 的窗口计数并返回当前计数（窗口过期则重新计时）
func (m *memoryRateLimiter) incr(key string, window time.Duration) int64 {
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupLocked(now)

	b := m.buckets[key]
	if b == nil || now.After(b.resetAt) {
		b = &rateBucket{count: 0, resetAt: now.Add(window)}
		m.buckets[key] = b
	}
	b.count++
	return b.count
}

// cleanupLocked 定期清理过期 bucket；极端情况下整体回收，避免内存无限增长
func (m *memoryRateLimiter) cleanupLocked(now time.Time) {
	if now.Sub(m.lastCleanup) < time.Minute && len(m.buckets) <= m.maxEntries {
		return
	}
	m.lastCleanup = now

	for k, b := range m.buckets {
		if now.After(b.resetAt) {
			delete(m.buckets, k)
		}
	}

	if m.maxEntries > 0 && len(m.buckets) > m.maxEntries {
		// 说明遭遇了海量随机 key（通常是伪造 IP 的扫描器）；
		// 清空计数只会短暂放宽限流，但能避免进程 OOM。
		m.buckets = make(map[string]*rateBucket)
	}
}

// RateLimit 固定窗口限流中间件
//
// limit: 时间窗口内允许的最大请求数（<=0 表示不限流）
// window: 时间窗口大小
//
// Redis 不可用时自动降级为进程内限流（fail-secure），而不是放行。
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit <= 0 || window <= 0 {
			c.Next()
			return
		}

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		key := fmt.Sprintf("ratelimit:%s:%s", path, c.ClientIP())

		count, ok := redisIncr(key, window)
		if !ok {
			count = fallbackRateLimiter.incr(key, window)
		}

		if count > int64(limit) {
			c.Header("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
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

// redisIncr 尝试通过 Redis 做窗口计数；返回 (计数, 是否成功)。
// 任何错误（未配置 Redis、连接失败、超时）都返回 ok=false，由调用方回退到内存限流。
func redisIncr(key string, window time.Duration) (int64, bool) {
	client := utils.GetRedis()
	if client == nil {
		return 0, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisRateLimitTimeout)
	defer cancel()

	count, err := client.Incr(ctx, key).Result()
	if err != nil {
		return 0, false
	}
	if count == 1 {
		// 首次计数时设置窗口过期；失败不影响本次计数（下一次会重新设置）
		_ = client.Expire(ctx, key, window).Err()
	}
	return count, true
}
