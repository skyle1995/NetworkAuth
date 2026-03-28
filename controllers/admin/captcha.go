package admin

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"
	"net/http"
	"strings"

	"NetworkAuth/middleware"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 全局变量
// ============================================================================

// 全局验证码存储器
var store = base64Captcha.DefaultMemStore

// ============================================================================
// 辅助函数
// ============================================================================

// secureRandomInt 生成安全的随机整数，范围 [0, max)
func secureRandomInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

// ============================================================================
// API处理器
// ============================================================================

// CaptchaHandler 生成验证码图片
// GET /admin/captcha - 返回验证码图片
func CaptchaHandler(c *gin.Context) {
	// 随机生成4-6位长度
	// 使用crypto/rand生成安全的随机数
	randomNum, err := secureRandomInt(3)
	if err != nil {
		c.String(http.StatusInternalServerError, "生成随机数失败")
		return
	}
	captchaLength := 4 + randomNum // 4-6位随机长度

	// 配置验证码参数 - 使用字母数字混合
	driver := base64Captcha.DriverString{
		Height:          60,
		Width:           200,
		NoiseCount:      0,
		ShowLineOptions: 2 | 4,
		Length:          captchaLength,
		Source:          "ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789", // 混合大小写字母和数字，去除易混淆字符
	}

	// 生成验证码
	captcha := base64Captcha.NewCaptcha(&driver, store)
	id, b64s, _, err := captcha.Generate()
	if err != nil {
		c.String(http.StatusInternalServerError, "生成验证码失败")
		return
	}

	// 将验证码ID存储到Cookie中
	c.SetCookie("captcha_id", id, 300, "/", "", false, true)

	// 设置响应头
	c.Header("Content-Type", "image/png")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// 去掉data:image/png;base64,前缀
	b64s = strings.TrimPrefix(b64s, "data:image/png;base64,")

	imgData, err := base64.StdEncoding.DecodeString(b64s)
	if err != nil {
		c.String(http.StatusInternalServerError, "解码验证码图片失败")
		return
	}

	c.Data(http.StatusOK, "image/png", imgData)
}

// VerifyCaptcha 验证验证码
// 这个函数将在登录处理中被调用
// 支持大小写不敏感匹配
func VerifyCaptcha(c *gin.Context, captchaValue string) bool {
	// 检查是否为开发模式，如果是则跳过验证码验证
	if middleware.ShouldSkipCaptcha(c) {
		return true
	}

	// 从cookie中获取验证码ID
	captchaId, err := c.Cookie("captcha_id")
	if err != nil || captchaId == "" {
		logrus.WithError(err).Warn("验证码验证失败：无法从Cookie获取captcha_id")
		return false
	}
	logrus.Infof("VerifyCaptcha: received captchaId=%s, captchaValue=%s", captchaId, captchaValue)

	// 先尝试原始值验证
	if store.Verify(captchaId, captchaValue, false) {
		// 验证成功后删除验证码
		store.Verify(captchaId, captchaValue, true)
		return true
	}

	// 如果原始值验证失败，尝试小写验证
	if store.Verify(captchaId, strings.ToLower(captchaValue), false) {
		// 验证成功后删除验证码
		store.Verify(captchaId, strings.ToLower(captchaValue), true)
		return true
	}

	// 最后尝试大写验证
	if store.Verify(captchaId, strings.ToUpper(captchaValue), true) {
		return true
	}

	return false
}
