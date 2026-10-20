package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// 回归测试：dev_mode 不得再跳过登录验证码（生产误配 dev_mode=true 时验证码必须仍然生效），
// 只有显式开启 security.skip_captcha 才允许跳过。
func TestShouldSkipCaptchaRequiresExplicitFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	t.Cleanup(viper.Reset)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	viper.Set("server.dev_mode", true)
	viper.Set("security.skip_captcha", false)
	if ShouldSkipCaptcha(c) {
		t.Fatal("dev_mode=true 时不应跳过验证码")
	}

	viper.Set("security.skip_captcha", true)
	if !ShouldSkipCaptcha(c) {
		t.Fatal("显式开启 security.skip_captcha 时应跳过验证码")
	}

	viper.Set("security.skip_captcha", false)
	viper.Set("server.dev_mode", false)
	if ShouldSkipCaptcha(c) {
		t.Fatal("默认配置下不应跳过验证码")
	}
}
