package admin

import (
	"encoding/base64"
	"net/http"
	"strings"

	"NetworkAuth/middleware"
	"NetworkAuth/utils"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"github.com/sirupsen/logrus"
)

// CaptchaHandler 生成验证码图片
// GET /admin/captcha - 返回验证码图片
func CaptchaHandler(c *gin.Context) {
	// 配置与 User 端一致，采用较弱的验证码强度以提升正常用户体验
	driver := base64Captcha.DriverString{
		Height:          60,
		Width:           200,
		Length:          4,
		NoiseCount:      20,    // 加点背景噪点干扰
		ShowLineOptions: 2 | 4, // 加点干扰线
		Source:          "ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789",
	}

	// 生成验证码，使用共享的 CaptchaStore
	captcha := base64Captcha.NewCaptcha(&driver, utils.CaptchaStore)
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

	// 调用共享的 VerifyCaptcha
	return utils.VerifyCaptcha(captchaId, captchaValue)
}
