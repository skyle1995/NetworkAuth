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
	emailCodeTTL             = 10 * time.Minute
	defaultEmailCodeCooldown = 60 // 秒
)

// emailCooldownSeconds 读取后台配置的发送限流秒数（默认60，非正数回退默认）。
func emailCooldownSeconds() int {
	n := GetSettingsService().GetInt("smtp_code_cooldown", defaultEmailCodeCooldown)
	if n <= 0 {
		n = defaultEmailCodeCooldown
	}
	return n
}

// cooldownRemain 读取冷却 key 的剩余秒数（向上取整）；不存在或出错返回0。
func cooldownRemain(ctx context.Context, rdb *redis.Client, key string) int {
	ttl, err := rdb.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		return 0
	}
	return int((ttl + time.Second - 1) / time.Second)
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// IsValidEmail 校验邮箱格式
func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func emailCodeKey(email string) string {
	return fmt.Sprintf("email_code:%s", strings.ToLower(email))
}

func emailCodeCooldownKey(email string) string {
	return fmt.Sprintf("email_code_cd:%s", strings.ToLower(email))
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
	policy := GetAccountPolicy()
	if !policy.RegisterEnabled {
		return nil, errors.New("账号注册已关闭")
	}
	if !policy.EmailVerifyEnabled {
		return nil, errors.New("邮箱验证未开启")
	}
	// 邮箱已注册则不再发码（避免枚举可按需调整，这里直接提示）
	var dup int64
	if err := db.Model(&models.Member{}).Where("username = ?", email).Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, errors.New("该邮箱已注册")
	}

	ctx := context.Background()
	rdb := utils.GetRedis()

	// 频率限制：命中冷却则返回剩余秒数
	cdKey := emailCodeCooldownKey(email)
	if remain := cooldownRemain(ctx, rdb, cdKey); remain > 0 {
		return nil, fmt.Errorf("发送过于频繁，请%d秒后再试", remain)
	}

	code, err := genNumericCode(6)
	if err != nil {
		return nil, err
	}
	cooldown := emailCooldownSeconds()
	if err := storeEmailCode(ctx, rdb, emailCodeKey(email), cdKey, code, cooldown); err != nil {
		return nil, err
	}

	subject := fmt.Sprintf("【%s】注册验证码", app.Name)
	body := fmt.Sprintf(
		`<div style="font-family:sans-serif;font-size:14px;color:#333">`+
			`<p>您正在注册 <b>%s</b>，验证码为：</p>`+
			`<p style="font-size:24px;font-weight:bold;letter-spacing:4px;color:#409eff">%s</p>`+
			`<p style="color:#888">验证码 10 分钟内有效，请勿泄露给他人。</p></div>`,
		app.Name, code)

	if err := SendMail(email, subject, body); err != nil {
		// 发送失败则清掉已存的码，允许立即重试
		clearEmailCode(ctx, rdb, emailCodeKey(email), cdKey, code)
		return nil, errors.New("邮件发送失败: " + err.Error())
	}

	return map[string]any{"message": "验证码已发送，请查收邮箱", "cooldown": cooldown}, nil
}

func emailResetCodeKey(email string) string {
	return fmt.Sprintf("email_reset_code:%s", strings.ToLower(email))
}

func emailResetCooldownKey(email string) string {
	return fmt.Sprintf("email_reset_cd:%s", strings.ToLower(email))
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
	if err := db.Where("username = ?", email).First(&member).Error; err != nil {
		return nil, errors.New("该邮箱未注册")
	}
	if member.Type != models.MemberTypeRegister || member.PasswordSalt == "" {
		return nil, errors.New("该账号不支持找回密码")
	}

	ctx := context.Background()
	rdb := utils.GetRedis()
	cdKey := emailResetCooldownKey(email)
	if remain := cooldownRemain(ctx, rdb, cdKey); remain > 0 {
		return nil, fmt.Errorf("发送过于频繁，请%d秒后再试", remain)
	}

	code, err := genNumericCode(6)
	if err != nil {
		return nil, err
	}
	cooldown := emailCooldownSeconds()
	if err := storeEmailCode(ctx, rdb, emailResetCodeKey(email), cdKey, code, cooldown); err != nil {
		return nil, err
	}

	subject := fmt.Sprintf("【%s】找回密码验证码", app.Name)
	body := fmt.Sprintf(
		`<div style="font-family:sans-serif;font-size:14px;color:#333">`+
			`<p>您正在找回 <b>%s</b> 的账号密码，验证码为：</p>`+
			`<p style="font-size:24px;font-weight:bold;letter-spacing:4px;color:#409eff">%s</p>`+
			`<p style="color:#888">验证码 10 分钟内有效，请勿泄露给他人。若非本人操作请忽略。</p></div>`,
		app.Name, code)

	if err := SendMail(email, subject, body); err != nil {
		clearEmailCode(ctx, rdb, emailResetCodeKey(email), cdKey, code)
		return nil, errors.New("邮件发送失败: " + err.Error())
	}

	return map[string]any{"message": "验证码已发送，请查收邮箱", "cooldown": cooldown}, nil
}

// storeEmailCode 原子设置验证码与发送冷却，并重置新验证码的错误次数。
func storeEmailCode(ctx context.Context, rdb *redis.Client, key, cooldownKey, code string, cooldown int) error {
	const script = `
local remain = redis.call('PTTL', KEYS[2])
if remain > 0 then return math.ceil(remain / 1000) end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('DEL', KEYS[3])
redis.call('SET', KEYS[2], '1', 'EX', ARGV[3])
return 0`
	remain, err := rdb.Eval(ctx, script, []string{key, cooldownKey, key + ":attempts"},
		code, emailCodeTTL.Milliseconds(), cooldown).Int()
	if err != nil {
		return errors.New("验证码存储失败")
	}
	if remain > 0 {
		return fmt.Errorf("发送过于频繁，请%d秒后再试", remain)
	}
	return nil
}

// clearEmailCode 发送失败时仅清除本次验证码，避免删除随后重新发送的验证码。
func clearEmailCode(ctx context.Context, rdb *redis.Client, key, cooldownKey, code string) {
	const script = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1], KEYS[2], KEYS[3])
end
return 0`
	rdb.Eval(ctx, script, []string{key, cooldownKey, key + ":attempts"}, code)
}

// VerifyResetCode 原子校验并消费找回密码验证码，最多允许五次错误尝试。
func VerifyResetCode(email, code string) error {
	return consumeEmailCode(emailResetCodeKey(email), code)
}

// VerifyRegisterCode 原子校验并消费注册验证码，最多允许五次错误尝试。
func VerifyRegisterCode(email, code string) error {
	return consumeEmailCode(emailCodeKey(email), code)
}

// consumeEmailCode 在同一 Redis 原子操作内限制尝试次数并删除匹配验证码。
func consumeEmailCode(key, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("验证码不能为空")
	}
	if !utils.IsRedisAvailable() {
		return errors.New("验证码服务暂不可用")
	}
	ctx := context.Background()
	rdb := utils.GetRedis()
	const script = `
local stored = redis.call('GET', KEYS[1])
if not stored then return -1 end
if tonumber(redis.call('GET', KEYS[2]) or '0') >= 5 then return -3 end
if stored ~= ARGV[1] then
    local attempts = redis.call('INCR', KEYS[2])
    local ttl = redis.call('PTTL', KEYS[1])
    if ttl < 1 then ttl = 600000 end
    redis.call('PEXPIRE', KEYS[2], ttl)
    if attempts >= 5 then return -3 end
    return -2
end
redis.call('DEL', KEYS[1], KEYS[2])
return 1`
	result, err := rdb.Eval(ctx, script, []string{key, key + ":attempts"}, code).Int()
	if err != nil {
		return errors.New("验证码校验失败")
	}
	switch result {
	case 1:
		return nil
	case -1:
		return errors.New("验证码不存在或已过期")
	case -3:
		return errors.New("验证码尝试过多，请重新获取")
	default:
		return errors.New("验证码错误")
	}
}
