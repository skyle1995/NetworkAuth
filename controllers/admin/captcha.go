package admin

import (
	"encoding/base64"
	"net/http"
	"strings"

	"NetworkAuth/middleware"
	"NetworkAuth/services"
	"NetworkAuth/utils"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"github.com/sirupsen/logrus"
)

// 验证码类型
const (
	captchaTypeImage = "image" // 字符(图形)验证码
	captchaTypeSlide = "slide" // 滑动拼图验证码
	captchaTypeClick = "click" // 点击文字验证码
)

// GetCaptchaType 读取当前验证码类型设置（默认滑块）。非法值回退为滑块。
func GetCaptchaType() string {
	t := services.GetSettingsService().GetString("captcha_type", captchaTypeSlide)
	if t != captchaTypeImage && t != captchaTypeSlide && t != captchaTypeClick {
		return captchaTypeSlide
	}
	return t
}

// CaptchaTypeHandler 返回当前验证码类型（公开，登录页据此决定渲染字符或滑块）
// GET /admin/captcha/type -> {"code":0,"data":{"type":"slide"}}
func CaptchaTypeHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": gin.H{"type": GetCaptchaType()}})
}

// SlideCaptchaHandler 生成滑动拼图验证码
// GET /admin/captcha/slide -> {code,data:{id,master_image,tile_image,tile_x,tile_y,...}}
func SlideCaptchaHandler(c *gin.Context) {
	data, err := utils.GenerateSlideCaptcha()
	if err != nil {
		logrus.WithError(err).Error("生成滑动验证码失败")
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "生成滑动验证码失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": data})
}

// SlideCaptchaVerifyHandler 校验滑动拼图落点，通过则下发一次性令牌
// POST /admin/captcha/slide/verify {id,x} -> {code,data:{token}}
func SlideCaptchaVerifyHandler(c *gin.Context) {
	var body struct {
		ID string `json:"id"`
		X  int    `json:"x"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	if !utils.VerifySlideCaptcha(body.ID, body.X) {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "验证未通过，请重试"})
		return
	}
	token := utils.IssueSlideToken()
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": gin.H{"token": token}})
}

// ClickCaptchaHandler 生成点击文字验证码
// GET /admin/captcha/click -> {code,data:{id,master_image,thumb_image,...,dot_count}}
func ClickCaptchaHandler(c *gin.Context) {
	data, err := utils.GenerateClickCaptcha()
	if err != nil {
		logrus.WithError(err).Error("生成点击文字验证码失败")
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "生成点击文字验证码失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": data})
}

// ClickCaptchaVerifyHandler 校验有序点击点，通过则下发一次性令牌
// POST /admin/captcha/click/verify {id,points:[{x,y}...]} -> {code,data:{token}}
func ClickCaptchaVerifyHandler(c *gin.Context) {
	var body struct {
		ID     string             `json:"id"`
		Points []utils.ClickPoint `json:"points"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	if !utils.VerifyClickCaptcha(body.ID, body.Points) {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "验证未通过，请重试"})
		return
	}
	token := utils.IssueSlideToken() // 与滑块共用一次性令牌
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": gin.H{"token": token}})
}

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

// VerifyCaptcha 登录时校验验证码：按当前验证码类型分支
//   - slide：消费前置滑块校验签发的一次性令牌 slideToken
//   - image：从 cookie 取 captcha_id + 用户输入 imageValue 校验（大小写不敏感）
//
// 开发模式(dev_mode)下一律跳过。
func VerifyCaptcha(c *gin.Context, imageValue, slideToken string) bool {
	// 检查是否为开发模式，如果是则跳过验证码验证
	if middleware.ShouldSkipCaptcha(c) {
		return true
	}

	// 滑动拼图 / 点击文字：登录仅需消费前置校验签发的一次性通过令牌
	if t := GetCaptchaType(); t == captchaTypeSlide || t == captchaTypeClick {
		return utils.ConsumeSlideToken(slideToken)
	}

	// 字符(图形)验证码：从 cookie 中获取验证码 ID
	captchaId, err := c.Cookie("captcha_id")
	if err != nil || captchaId == "" {
		logrus.WithError(err).Warn("验证码验证失败：无法从Cookie获取captcha_id")
		return false
	}
	return utils.VerifyCaptcha(captchaId, imageValue)
}
