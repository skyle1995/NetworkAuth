package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newRateLimitTestEngine(path string, limit int, window time.Duration) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET(path, RateLimit(limit, window), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})
	return r
}

func hit(r *gin.Engine, path, remoteAddr string) int {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// Redis 不可用时应回退到内存限流（fail-secure），而不是直接放行
func TestRateLimitFallsBackToMemoryWhenRedisUnavailable(t *testing.T) {
	if _, ok := redisIncr("ratelimit:test:probe", time.Minute); ok {
		t.Skip("当前环境 Redis 可用，跳过内存兜底测试")
	}

	const path = "/api/test/memory-fallback"
	r := newRateLimitTestEngine(path, 3, time.Minute)

	for i := 1; i <= 3; i++ {
		if code := hit(r, path, "192.0.2.10:1234"); code != http.StatusOK {
			t.Fatalf("第 %d 次请求应放行，实际状态码 %d", i, code)
		}
	}
	if code := hit(r, path, "192.0.2.10:1234"); code != http.StatusTooManyRequests {
		t.Fatalf("超过阈值的请求应返回 429，实际状态码 %d", code)
	}
}

// 不同客户端 IP 使用各自的独立配额
func TestRateLimitIsolatesByClientIP(t *testing.T) {
	const path = "/api/test/ip-isolation"
	r := newRateLimitTestEngine(path, 1, time.Minute)

	if code := hit(r, path, "192.0.2.20:1234"); code != http.StatusOK {
		t.Fatalf("IP-A 第 1 次请求应放行，实际 %d", code)
	}
	if code := hit(r, path, "192.0.2.20:1234"); code != http.StatusTooManyRequests {
		t.Fatalf("IP-A 第 2 次请求应被限流，实际 %d", code)
	}
	if code := hit(r, path, "192.0.2.21:1234"); code != http.StatusOK {
		t.Fatalf("IP-B 使用独立配额应放行，实际 %d", code)
	}
}

// 内存限流器窗口计数与过期语义
func TestMemoryRateLimiterWindowSemantics(t *testing.T) {
	m := newMemoryRateLimiter(10)
	key := "ratelimit:unit-test"

	for i := 1; i <= 5; i++ {
		if got := m.incr(key, time.Minute); got != int64(i) {
			t.Fatalf("第 %d 次计数应为 %d，实际 %d", i, i, got)
		}
	}

	shortWindow := newMemoryRateLimiter(10)
	if got := shortWindow.incr("k", time.Millisecond); got != 1 {
		t.Fatalf("首次计数应为 1，实际 %d", got)
	}
	time.Sleep(5 * time.Millisecond)
	if got := shortWindow.incr("k", time.Millisecond); got != 1 {
		t.Fatalf("窗口过期后应重新计数为 1，实际 %d", got)
	}
}
