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

// rejectCaptchaTypeMismatch 当请求的验证码类型与后台当前配置不一致时直接拒绝。
//
// 安全目的：历史实现中 slide / click 两种验证码共用同一套「一次性登录令牌」，
// 且两个签发接口始终可用。管理员把类型切换为更强的 click 后，攻击者仍可调用
// slide 接口（滑块更容易被脚本识别/爆破）换取登录令牌，使 click 配置形同虚设。
// 现在：登录令牌绑定验证码类型，且未启用的验证码接口不再签发/下发题目。
func rejectCaptchaTypeMismatch(c *gin.Context, want string) bool {
	if GetCaptchaType() == want {
		return false
	}
	c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "当前验证码类型未启用"})
	return true
}

// SlideCaptchaHandler 生成滑动拼图验证码
// GET /admin/captcha/slide -> {code,data:{id,master_image,tile_image,tile_x,tile_y,...}}
func SlideCaptchaHandler(c *gin.Context) {
	if rejectCaptchaTypeMismatch(c, captchaTypeSlide) {
		return
	}
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
	if rejectCaptchaTypeMismatch(c, captchaTypeSlide) {
		return
	}
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
	token := utils.IssueSlideToken(c.ClientIP(), c.Request.UserAgent(), captchaTypeSlide)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": gin.H{"token": token}})
}

// ClickCaptchaHandler 生成点击文字验证码
// GET /admin/captcha/click -> {code,data:{id,master_image,thumb_image,...,dot_count}}
func ClickCaptchaHandler(c *gin.Context) {
	if rejectCaptchaTypeMismatch(c, captchaTypeClick) {
		return
	}
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
	if rejectCaptchaTypeMismatch(c, captchaTypeClick) {
		return
	}
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
	// 与滑块共用令牌存储，但令牌会记录签发类型（click），登录侧只接受与当前配置类型一致的令牌
	token := utils.IssueSlideToken(c.ClientIP(), c.Request.UserAgent(), captchaTypeClick)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": gin.H{"token": token}})
}

// CaptchaHandler 生成验证码图片
// GET /admin/captcha - 返回验证码图片
func CaptchaHandler(c *gin.Context) {
	if rejectCaptchaTypeMismatch(c, captchaTypeImage) {
		return
	}
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
// 仅当配置文件中显式开启 security.skip_captcha（默认 false，启动时会打印严重警告）才跳过；
// server.dev_mode 不再跳过验证码，避免开发配置遗留到生产后验证码完全失效。
func VerifyCaptcha(c *gin.Context, imageValue, slideToken string) bool {
	if middleware.ShouldSkipCaptcha(c) {
		return true
	}

	// 滑动拼图 / 点击文字：登录仅需消费前置校验签发的一次性通过令牌
	// 令牌绑定签发时的 IP + User-Agent，且 5 分钟内有效、一次性使用
	if t := GetCaptchaType(); t == captchaTypeSlide || t == captchaTypeClick {
		// 令牌类型必须与当前配置一致，防止攻击者用未启用的弱验证码接口刷出的令牌登录
		return utils.ConsumeSlideToken(slideToken, c.ClientIP(), c.Request.UserAgent(), t)
	}

	// 字符(图形)验证码：从 cookie 中获取验证码 ID
	captchaId, err := c.Cookie("captcha_id")
	if err != nil || captchaId == "" {
		logrus.WithError(err).Warn("验证码验证失败：无法从Cookie获取captcha_id")
		return false
	}
	return utils.VerifyCaptcha(captchaId, imageValue)
}
