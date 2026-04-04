package utils

import (
	"strings"

	"github.com/mojocn/base64Captcha"
)

// CaptchaStore 全局验证码存储器
// 使用 base64Captcha 提供的默认内存存储，确保 admin 和 user 端可以共享验证码状态
var CaptchaStore = base64Captcha.DefaultMemStore

// VerifyCaptcha 验证验证码的有效性
// captchaId: 验证码的唯一标识符
// captchaValue: 用户输入的验证码内容
// 返回值: 验证是否通过
// 该函数提供函数级注释，支持大小写不敏感匹配，验证通过后会自动删除验证码
func VerifyCaptcha(captchaId, captchaValue string) bool {
	if captchaId == "" || captchaValue == "" {
		return false
	}

	// 使用 switch 进行连续逻辑判断，尝试不同的大小写组合
	switch {
	case CaptchaStore.Verify(captchaId, captchaValue, true):
		// 原始值匹配成功
		return true
	case CaptchaStore.Verify(captchaId, strings.ToLower(captchaValue), true):
		// 小写匹配成功
		return true
	case CaptchaStore.Verify(captchaId, strings.ToUpper(captchaValue), true):
		// 大写匹配成功
		return true
	default:
		// 匹配失败
		return false
	}
}
