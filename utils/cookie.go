package utils

import (
	"net/http"
	"time"
)

// ============================================================================
// Cookie创建函数
// ============================================================================

// CreateSecureCookie 创建安全的Cookie
// name: Cookie名称
// value: Cookie值
// maxAge: 过期时间（秒），0表示会话Cookie，-1表示立即过期
// domain: Cookie域名
// secure: 是否只在HTTPS下发送
// sameSiteStr: SameSite属性（Strict/Lax/None）
func CreateSecureCookie(name, value string, maxAge int, domain string, secure bool, sameSiteStr string) *http.Cookie {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   maxAge,
	}

	// 设置安全属性
	if secure {
		cookie.Secure = true
	}

	// 设置SameSite属性
	switch sameSiteStr {
	case "Strict":
		cookie.SameSite = http.SameSiteStrictMode
	case "Lax":
		cookie.SameSite = http.SameSiteLaxMode
	case "None":
		cookie.SameSite = http.SameSiteNoneMode
		// SameSite=None 必须配合 Secure=true 使用
		cookie.Secure = true
	default:
		cookie.SameSite = http.SameSiteStrictMode
	}

	// 设置Domain
	if domain != "" {
		cookie.Domain = domain
	}

	// 如果maxAge > 0，设置Expires时间
	if maxAge > 0 {
		cookie.Expires = time.Now().Add(time.Duration(maxAge) * time.Second)
	} else if maxAge == -1 {
		// 立即过期
		cookie.Expires = time.Unix(0, 0)
	}

	return cookie
}

// CreateSessionCookie 创建会话Cookie（浏览器关闭时过期）
func CreateSessionCookie(name, value string, domain string, secure bool, sameSiteStr string) *http.Cookie {
	return CreateSecureCookie(name, value, 0, domain, secure, sameSiteStr)
}

// CreateExpiredCookie 创建立即过期的Cookie（用于清理）
func CreateExpiredCookie(name string, domain string) *http.Cookie {
	return CreateSecureCookie(name, "", -1, domain, false, "Lax")
}
