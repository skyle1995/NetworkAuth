package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/utils"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ============================================================================
// 注册邮箱验证码
// ============================================================================
//
// 验证码存于 Redis，10 分钟有效；同一邮箱 60 秒内只能发一次。
// 依赖系统 SMTP 配置发送邮件（见 mail.go）。

const (
	emailCodeTTL      = 10 * time.Minute
	emailCodeCooldown = 60 * time.Second
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// IsValidEmail 校验邮箱格式
func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func emailCodeKey(appUUID, email string) string {
	return fmt.Sprintf("email_code:%s:%s", appUUID, strings.ToLower(email))
}

func emailCodeCooldownKey(appUUID, email string) string {
	return fmt.Sprintf("email_code_cd:%s:%s", appUUID, strings.ToLower(email))
}

// genNumericCode 生成 n 位数字验证码
func genNumericCode(n int) (string, error) {
	var b strings.Builder
	for i := 0; i < n; i++ {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteString(d.String())
	}
	return b.String(), nil
}

// SendRegisterCode 发送注册验证码到指定邮箱。
func SendRegisterCode(appUUID, email string) (any, error) {
	appUUID = strings.TrimSpace(appUUID)
	email = strings.TrimSpace(email)
	if !IsValidEmail(email) {
		return nil, errors.New("邮箱格式不正确")
	}
	if !utils.IsRedisAvailable() {
		return nil, errors.New("验证码服务暂不可用")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	if app.RegisterEnabled != 1 {
		return nil, errors.New("该应用未开启账号注册")
	}
	if app.EmailVerifyEnabled != 1 {
		return nil, errors.New("该应用未开启邮箱验证")
	}
	// 邮箱已注册则不再发码（避免枚举可按需调整，这里直接提示）
	var dup int64
	if err := db.Model(&models.Member{}).Where("app_uuid = ? AND username = ?", app.UUID, email).Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, errors.New("该邮箱已注册")
	}

	ctx := context.Background()
	rdb := utils.GetRedis()

	// 频率限制
	if exists, _ := rdb.Exists(ctx, emailCodeCooldownKey(appUUID, email)).Result(); exists > 0 {
		return nil, errors.New("发送过于频繁，请稍后再试")
	}

	code, err := genNumericCode(6)
	if err != nil {
		return nil, err
	}
	if err := rdb.Set(ctx, emailCodeKey(appUUID, email), code, emailCodeTTL).Err(); err != nil {
		return nil, errors.New("验证码存储失败")
	}
	rdb.Set(ctx, emailCodeCooldownKey(appUUID, email), "1", emailCodeCooldown)

	subject := fmt.Sprintf("【%s】注册验证码", app.Name)
	body := fmt.Sprintf(
		`<div style="font-family:sans-serif;font-size:14px;color:#333">`+
			`<p>您正在注册 <b>%s</b>，验证码为：</p>`+
			`<p style="font-size:24px;font-weight:bold;letter-spacing:4px;color:#409eff">%s</p>`+
			`<p style="color:#888">验证码 10 分钟内有效，请勿泄露给他人。</p></div>`,
		app.Name, code)

	if err := SendMail(email, subject, body); err != nil {
		// 发送失败则清掉已存的码，允许立即重试
		rdb.Del(ctx, emailCodeKey(appUUID, email), emailCodeCooldownKey(appUUID, email))
		return nil, errors.New("邮件发送失败: " + err.Error())
	}

	return map[string]any{"message": "验证码已发送，请查收邮箱"}, nil
}

func emailResetCodeKey(appUUID, email string) string {
	return fmt.Sprintf("email_reset_code:%s:%s", appUUID, strings.ToLower(email))
}

func emailResetCooldownKey(appUUID, email string) string {
	return fmt.Sprintf("email_reset_cd:%s:%s", appUUID, strings.ToLower(email))
}

// SendResetCode 发送找回密码验证码到指定邮箱（邮箱必须为本应用已注册账号）。
func SendResetCode(appUUID, email string) (any, error) {
	appUUID = strings.TrimSpace(appUUID)
	email = strings.TrimSpace(email)
	if !IsValidEmail(email) {
		return nil, errors.New("邮箱格式不正确")
	}
	if !utils.IsRedisAvailable() {
		return nil, errors.New("验证码服务暂不可用")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	// 找回密码要求邮箱为已注册的注册型账号
	var member models.Member
	if err := db.Where("app_uuid = ? AND username = ?", app.UUID, email).First(&member).Error; err != nil {
		return nil, errors.New("该邮箱未注册")
	}
	if member.Type != models.MemberTypeRegister || member.PasswordSalt == "" {
		return nil, errors.New("该账号不支持找回密码")
	}

	ctx := context.Background()
	rdb := utils.GetRedis()
	if exists, _ := rdb.Exists(ctx, emailResetCooldownKey(appUUID, email)).Result(); exists > 0 {
		return nil, errors.New("发送过于频繁，请稍后再试")
	}

	code, err := genNumericCode(6)
	if err != nil {
		return nil, err
	}
	if err := rdb.Set(ctx, emailResetCodeKey(appUUID, email), code, emailCodeTTL).Err(); err != nil {
		return nil, errors.New("验证码存储失败")
	}
	rdb.Set(ctx, emailResetCooldownKey(appUUID, email), "1", emailCodeCooldown)

	subject := fmt.Sprintf("【%s】找回密码验证码", app.Name)
	body := fmt.Sprintf(
		`<div style="font-family:sans-serif;font-size:14px;color:#333">`+
			`<p>您正在找回 <b>%s</b> 的账号密码，验证码为：</p>`+
			`<p style="font-size:24px;font-weight:bold;letter-spacing:4px;color:#409eff">%s</p>`+
			`<p style="color:#888">验证码 10 分钟内有效，请勿泄露给他人。若非本人操作请忽略。</p></div>`,
		app.Name, code)

	if err := SendMail(email, subject, body); err != nil {
		rdb.Del(ctx, emailResetCodeKey(appUUID, email), emailResetCooldownKey(appUUID, email))
		return nil, errors.New("邮件发送失败: " + err.Error())
	}

	return map[string]any{"message": "验证码已发送，请查收邮箱"}, nil
}

// VerifyResetCode 校验找回密码验证码；成功后删除，防止复用。
func VerifyResetCode(appUUID, email, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("验证码不能为空")
	}
	if !utils.IsRedisAvailable() {
		return errors.New("验证码服务暂不可用")
	}
	ctx := context.Background()
	rdb := utils.GetRedis()
	key := emailResetCodeKey(appUUID, email)
	stored, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return errors.New("验证码不存在或已过期")
	}
	if err != nil {
		return errors.New("验证码校验失败")
	}
	if stored != code {
		return errors.New("验证码错误")
	}
	rdb.Del(ctx, key)
	return nil
}

// VerifyRegisterCode 校验注册验证码；成功后删除，防止复用。
func VerifyRegisterCode(appUUID, email, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("验证码不能为空")
	}
	if !utils.IsRedisAvailable() {
		return errors.New("验证码服务暂不可用")
	}
	ctx := context.Background()
	rdb := utils.GetRedis()
	key := emailCodeKey(appUUID, email)
	stored, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return errors.New("验证码不存在或已过期")
	}
	if err != nil {
		return errors.New("验证码校验失败")
	}
	if stored != code {
		return errors.New("验证码错误")
	}
	rdb.Del(ctx, key)
	return nil
}
