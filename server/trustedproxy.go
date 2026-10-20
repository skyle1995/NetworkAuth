package server

import (
	"fmt"

	"NetworkAuth/config"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ConfigureTrustedProxies 配置 Gin 的受信任反向代理列表，决定 ClientIP() 如何解析客户端地址。
//
// 安全背景：
//   - Gin 默认把「所有 IP」都视为受信任代理（0.0.0.0/0、::/0），并且从 X-Forwarded-For
//     的最左侧取值；任何客户端只要自带 X-Forwarded-For 头就能伪造自己的 IP。
//   - 本项目所有基于 IP 的限流（登录、验证码、业务接口）以及登录日志都依赖 ClientIP()，
//     一旦 IP 可伪造，限流就会被「每个请求换一个 IP」轻易绕过，日志也会被污染。
//
// 策略：
//   - 未配置 server.trusted_proxies 时：调用 SetTrustedProxies(nil)，
//     彻底禁用代理头解析，客户端 IP 只能取 TCP 对端地址（不可伪造）。
//   - 配置了 server.trusted_proxies（如 Nginx/CDN 的出口地址）时：仅当直连对端属于
//     这些地址才解析 X-Forwarded-For，并从右向左选取第一个非受信任代理的地址。
//
// 注意：若服务部署在 Nginx/CDN 之后却未配置该白名单，所有请求的 ClientIP 都会变成
// 反向代理自身的地址，IP 级限流会退化为「全局共享额度」，因此必须二选一正确配置。
func ConfigureTrustedProxies(r *gin.Engine) error {
	trusted := loadTrustedProxies()

	if len(trusted) == 0 {
		if err := r.SetTrustedProxies(nil); err != nil {
			return fmt.Errorf("禁用受信任代理失败: %w", err)
		}
		logrus.Warn("[安全] 未配置 server.trusted_proxies：已禁用 X-Forwarded-For/X-Real-IP 解析，客户端 IP 取 TCP 对端地址（可防止伪造 IP 绕过限流）。若部署在 Nginx/CDN 之后，请把代理出口地址加入白名单，否则所有请求会被识别为同一个 IP。")
		return nil
	}

	if err := r.SetTrustedProxies(trusted); err != nil {
		return fmt.Errorf("server.trusted_proxies 配置无效（需为 IP 或 CIDR，如 127.0.0.1/32）: %w", err)
	}
	logrus.WithField("trusted_proxies", trusted).Info("[安全] 已启用受信任反向代理：客户端 IP 将从 X-Forwarded-For 解析（仅信任白名单内代理追加的地址）")
	return nil
}

// loadTrustedProxies 读取受信任代理白名单：优先取已解析的配置结构，回退到 viper
func loadTrustedProxies() []string {
	if cfg := config.GetRuntimeConfig(); cfg != nil {
		return normalizeTrustedProxies(cfg.Server.TrustedProxies)
	}
	return normalizeTrustedProxies(viper.GetStringSlice("server.trusted_proxies"))
}

// normalizeTrustedProxies 清理空白项，避免因配置中的空字符串导致启动失败
func normalizeTrustedProxies(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
