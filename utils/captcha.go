package utils

import (
	"time"
)

// CaptchaStore 全局验证码存储器
// 使用 base64Captcha 提供的默认内存存储，确保 admin 和 user 端可以共享验证码状态
var CaptchaStore = NewBoundedCaptchaStore(20000, 10*time.Minute)

// VerifyCaptcha 验证验证码的有效性
// captchaId: 验证码的唯一标识符
// captchaValue: 用户输入的验证码内容
// 返回值: 验证是否通过
// 该函数提供函数级注释，支持大小写不敏感匹配，验证通过后会自动删除验证码（一次性使用，防重放）
func VerifyCaptcha(captchaId, captchaValue string) bool {
	if captchaId == "" || captchaValue == "" {
		return false
	}

	// CaptchaStore.Verify 内部使用 strings.EqualFold 进行大小写不敏感比较，
	// 且 clear=true 会在取出后立即删除该验证码，确保一次性使用、防止重放。
	return CaptchaStore.Verify(captchaId, captchaValue, true)
}
