package utils

import (
	"fmt"
	"testing"
	"time"
)

// 一次性令牌必须绑定签发时的 IP + User-Agent + 验证码类型，且只能使用一次
func TestSlideTokenBindsClientTypeAndIsSingleUse(t *testing.T) {
	const ip, ua = "203.0.113.7", "Mozilla/5.0 test-agent"

	token := IssueSlideToken(ip, ua, "slide")
	if token == "" {
		t.Fatal("签发令牌失败")
	}
	if !ConsumeSlideToken(token, ip, ua, "slide") {
		t.Fatal("同 IP + 同 UA + 同类型的令牌应校验通过")
	}
	if ConsumeSlideToken(token, ip, ua, "slide") {
		t.Fatal("令牌是一次性的，重复消费必须失败")
	}

	// 不同验证码类型不能混用（回归测试：历史上 slide 令牌可用于 click 模式的登录）
	token = IssueSlideToken(ip, ua, "slide")
	if ConsumeSlideToken(token, ip, ua, "click") {
		t.Fatal("slide 令牌不得用于 click 登录（验证码类型混淆漏洞）")
	}

	token = IssueSlideToken(ip, ua, "click")
	if ConsumeSlideToken(token, ip, ua, "slide") {
		t.Fatal("click 令牌不得用于 slide 登录")
	}

	// 换 IP 无法使用
	token = IssueSlideToken(ip, ua, "slide")
	if ConsumeSlideToken(token, "198.51.100.9", ua, "slide") {
		t.Fatal("跨 IP 搬运的令牌必须失败")
	}

	// 换 UA 无法使用
	token = IssueSlideToken(ip, ua, "slide")
	if ConsumeSlideToken(token, ip, "Mozilla/5.0 other-agent", "slide") {
		t.Fatal("UA 不一致的令牌必须失败")
	}

	// 空令牌 / 空类型 / 空签发类型都必须失败
	if ConsumeSlideToken("", ip, ua, "slide") {
		t.Fatal("空令牌必须失败")
	}
	if ConsumeSlideToken(token, ip, ua, "") {
		t.Fatal("期望类型为空时必须失败")
	}
	if IssueSlideToken(ip, ua, "") != "" {
		t.Fatal("签发类型为空时不应签发令牌")
	}
}

// 超过有效期的令牌必须失效
func TestSlideTokenRejectsExpired(t *testing.T) {
	const ip, ua = "203.0.113.8", "Mozilla/5.0 expired-agent"

	token := "test-expired-token"
	raw := fmt.Sprintf(`{"ip":%q,"ua":%q,"type":"slide","ts":%d}`,
		ip, fingerprintUA(ua), time.Now().Add(-slideTokenTTL-time.Minute).Unix())
	if err := CaptchaStore.Set(slideTokenPrefix+token, raw); err != nil {
		t.Fatalf("写入测试令牌失败: %v", err)
	}

	if ConsumeSlideToken(token, ip, ua, "slide") {
		t.Fatal("超过有效期的令牌必须失败")
	}
}

// 未绑定信息的旧格式令牌（"1"）必须视为无效，防止历史实现被利用
func TestSlideTokenRejectsLegacyFormat(t *testing.T) {
	const ip, ua = "203.0.113.9", "Mozilla/5.0 legacy-agent"

	token := "test-legacy-token"
	if err := CaptchaStore.Set(slideTokenPrefix+token, "1"); err != nil {
		t.Fatalf("写入测试令牌失败: %v", err)
	}
	if ConsumeSlideToken(token, ip, ua, "slide") {
		t.Fatal("旧格式无绑定信息的令牌必须失败")
	}
}
