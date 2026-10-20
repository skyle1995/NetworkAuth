package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func newProxyTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := ConfigureTrustedProxies(r); err != nil {
		t.Fatalf("配置受信任代理失败: %v", err)
	}
	r.GET("/ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})
	return r
}

func clientIPOf(t *testing.T, r *gin.Engine, remoteAddr string, headers map[string]string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Body.String()
}

// 未配置受信任代理时，X-Forwarded-For 必须被忽略（防止伪造 IP 绕过限流/污染日志）
func TestClientIPCannotBeSpoofedWithoutTrustedProxies(t *testing.T) {
	viper.Set("server.trusted_proxies", []string{})
	t.Cleanup(func() { viper.Set("server.trusted_proxies", []string{}) })

	r := newProxyTestEngine(t)

	got := clientIPOf(t, r, "203.0.113.50:1234", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
		"X-Real-IP":       "5.6.7.8",
	})
	if got != "203.0.113.50" {
		t.Fatalf("未配置受信任代理时应取 TCP 对端地址，期望 203.0.113.50，实际 %q", got)
	}
}

// 配置了受信任代理后，只采信代理追加的地址（X-Forwarded-For 最右侧的非受信任地址），
// 客户端自带的最左侧伪造地址必须被丢弃。
func TestClientIPUsesTrustedProxyAppendedAddress(t *testing.T) {
	viper.Set("server.trusted_proxies", []string{"127.0.0.1/32"})
	t.Cleanup(func() { viper.Set("server.trusted_proxies", []string{}) })

	r := newProxyTestEngine(t)

	got := clientIPOf(t, r, "127.0.0.1:1234", map[string]string{
		// 客户端伪造了 198.51.100.7，反向代理追加了真实地址 203.0.113.60
		"X-Forwarded-For": "198.51.100.7, 203.0.113.60",
	})
	if got != "203.0.113.60" {
		t.Fatalf("应取代理追加的真实地址 203.0.113.60，实际 %q", got)
	}
}

// 直连对端不在受信任列表时，仍然忽略代理头
func TestClientIPIgnoresHeadersFromUntrustedPeer(t *testing.T) {
	viper.Set("server.trusted_proxies", []string{"127.0.0.1/32"})
	t.Cleanup(func() { viper.Set("server.trusted_proxies", []string{}) })

	r := newProxyTestEngine(t)

	got := clientIPOf(t, r, "203.0.113.70:1234", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	})
	if got != "203.0.113.70" {
		t.Fatalf("非受信任对端应使用 TCP 地址 203.0.113.70，实际 %q", got)
	}
}
