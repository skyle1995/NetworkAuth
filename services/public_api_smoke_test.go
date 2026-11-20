package services

import (
	"strconv"
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/utils/encrypt"
	b64 "encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setPolicy 测试辅助：把旧版「应用级账号策略字段」写入全局 settings。
// 一号通后这些策略是全站配置，测试沿用旧键名以减少改动。
func setPolicy(t *testing.T, kv map[string]interface{}) {
	t.Helper()
	mapping := map[string]string{
		"register_enabled":              models.SettingAccountRegisterEnabled,
		"email_verify_enabled":          models.SettingAccountEmailVerifyEnabled,
		"card_register_enabled":         models.SettingAccountCardRegisterEnabled,
		"register_limit_enabled":        models.SettingAccountRegisterLimitEnabled,
		"register_device_limit_enabled": models.SettingAccountRegisterDeviceLimitEnabled,
		"register_limit_time":           models.SettingAccountRegisterLimitTime,
		"register_count":                models.SettingAccountRegisterCount,
		"card_login_enabled":            models.SettingAccountCardLoginEnabled,
		"recharge_enabled":              models.SettingAccountRechargeEnabled,
		"trial_enabled":                 models.SettingAccountTrialEnabled,
		"trial_limit_time":              models.SettingAccountTrialLimitTime,
		"trial_claim_mode":              models.SettingAccountTrialClaimMode,
	}
	items := map[string]string{}
	for k, v := range kv {
		// 试用额度：旧字段 trial_duration 语义为「按当时模式发放」，现拆成点数与时长两项
		if k == "trial_duration" {
			items[models.SettingAccountTrialDurationMinutes] = fmt.Sprintf("%v", v)
			items[models.SettingAccountTrialPoints] = fmt.Sprintf("%v", v)
			continue
		}
		nk, ok := mapping[k]
		if !ok {
			continue
		}
		items[nk] = fmt.Sprintf("%v", v)
	}
	if err := SaveAccountPolicy(items); err != nil {
		t.Fatalf("setPolicy: %v", err)
	}
}
// setupPublicTestDB 建库并植入一个已启用、开启卡密登录的应用
// ============================================================================
// 二级令牌测试辅助
// ============================================================================

// activateForTest 用账号令牌激活应用（等价客户端调 70 应用激活）。
// 返回 (激活信息, 错误)，便于测试断言「登录通过但激活被拒」这类场景。
func activateForTest(t *testing.T, appUUID, accountToken, fingerprint string) (*ActivationInfo, error) {
	t.Helper()
	db, err := database.GetDB()
	if err != nil {
		t.Fatalf("get db: %v", err)
	}
	acct, err := AuthAccountToken(db, accountToken)
	if err != nil {
		return nil, err
	}
	var app models.App
	if err := db.Where("uuid = ?", appUUID).First(&app).Error; err != nil {
		return nil, err
	}
	if fingerprint == "" {
		fingerprint = "FP-TEST"
	}
	// 沿用登录时记录的版本与设备名，与生产路径（AuthForApp 隐式激活）保持一致
	return Activate(db, acct, &app, fingerprint, acct.IP, acct.Version, acct.DeviceName)
}

// mustDB / mustAccountToken / mustApp 供需要精确控制 IP 等参数的激活用例使用
func mustDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.GetDB()
	if err != nil {
		t.Fatalf("get db: %v", err)
	}
	return db
}

func mustAccountToken(t *testing.T, token string) *models.MemberAccountToken {
	t.Helper()
	acct, err := AuthAccountToken(mustDB(t), token)
	if err != nil {
		t.Fatalf("auth account token: %v", err)
	}
	return acct
}

func mustApp(t *testing.T, appUUID string) *models.App {
	t.Helper()
	var app models.App
	if err := mustDB(t).Where("uuid = ?", appUUID).First(&app).Error; err != nil {
		t.Fatalf("load app %s: %v", appUUID, err)
	}
	return &app
}
// loadActivation 按激活令牌取激活记录（测试辅助）
func loadActivation(t *testing.T, db *gorm.DB, token string) models.AppActivation {
	t.Helper()
	var a models.AppActivation
	if err := db.Where("token = ?", token).First(&a).Error; err != nil {
		t.Fatalf("load activation: %v", err)
	}
	return a
}

// mustMemberUUID 按用户名取账号 UUID（测试辅助）
func mustMemberUUID(t *testing.T, db *gorm.DB, username string) string {
	t.Helper()
	var m models.Member
	if err := db.Where("username = ?", username).First(&m).Error; err != nil {
		t.Fatalf("load member %s: %v", username, err)
	}
	return m.UUID
}
// mustActivate 激活失败直接终止
// appTokenOf 用账号令牌激活，返回激活令牌。
// 新协议下应用级接口（41-45/53/54）只认激活令牌，测试里统一走这里换。
func appTokenOf(t *testing.T, appUUID, accountToken, fingerprint string) string {
	t.Helper()
	return mustActivate(t, appUUID, accountToken, fingerprint).Token
}

func mustActivate(t *testing.T, appUUID, accountToken, fingerprint string) *ActivationInfo {
	t.Helper()
	info, err := activateForTest(t, appUUID, accountToken, fingerprint)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	return info
}
func setupPublicTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.App{}, &models.Card{}, &models.Member{}, &models.Binding{}, &models.API{}, &models.Variable{}, &models.Function{}, &models.AppActivation{}, &models.CardPackage{}, &models.MemberLevel{}, &models.AppLevelPoint{}, &models.Settings{}, &models.MemberAccountToken{}, &models.AppVersion{}, &models.AppActivation{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	seedTestQuota(db)
	// 一号通：默认「计时门禁 + 不限等级 + 不扣点」，等价于旧版会员模式。
	// 需要其它组合的用例自行改三个开关（见 setBilling* 辅助）。
	app := models.App{
		UUID:              "APP-1",
		Name:              "测试应用",
		Secret:            "SECRET",
		Status:            1,
		BillTimeEnabled:   1,
		BillPointsEnabled: 0,
		LevelGateEnabled:  1,
		RequiredLevel:     1,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("seed app: %v", err)
	}
	database.SetDB(db)
	// 每个用例独立库，但 SettingsService 是单例缓存，必须重置，否则读到上一个用例的配置
	ResetSettingsService()
	return db
}

// seedAppVersion 往版本列表插一条记录。
// 版本号、更新方式、下载地址、更新公告都只存在 app_versions 里，apps 表不再有这些列。
func seedAppVersion(t *testing.T, db *gorm.DB, appUUID, version string, downloadType int, url, changelog string, enabled bool) {
	t.Helper()
	v := models.AppVersion{
		AppUUID:      appUUID,
		Version:      version,
		DownloadType: downloadType,
		DownloadURL:  url,
		Changelog:    changelog,
		Enabled:      enabled,
		PublishedAt:  time.Now().UnixMilli(),
	}
	if err := db.Create(&v).Error; err != nil {
		t.Fatalf("seed version %s: %v", version, err)
	}
	// 模型上 download_type 带 default:2，零值插入会被 GORM 替换成默认值，
	// 需要 0（不启用更新）时单独写一次
	if downloadType == models.DownloadTypeDisabled {
		if err := db.Model(&models.AppVersion{}).Where("id = ?", v.ID).
			Update("download_type", models.DownloadTypeDisabled).Error; err != nil {
			t.Fatalf("force download_type=0 for %s: %v", version, err)
		}
	}
}

func TestAPICodecRoundTrip(t *testing.T) {
	// RC4 密钥（16 进制串）
	rc4Key, _ := encrypt.GenerateRC4Key(8)
	rc4Hex := hex.EncodeToString(rc4Key)
	// 易加密密钥（逗号分隔整数）
	easyKey, _, _ := encrypt.GenerateEasyKey()
	easyStr := encrypt.FormatKeyAsString(easyKey)
	// RSA 密钥对（PEM）
	rsaPub, rsaPriv, err := encrypt.GenerateRSAKeyPairPEM(2048)
	if err != nil {
		t.Fatalf("gen rsa: %v", err)
	}

	cases := []struct {
		name string
		api  *models.API
	}{
		{
			name: "None",
			api:  &models.API{SubmitAlgorithm: models.AlgorithmNone, ReturnAlgorithm: models.AlgorithmNone},
		},
		{
			name: "RC4",
			api: &models.API{
				SubmitAlgorithm: models.AlgorithmRC4, SubmitPrivateKey: rc4Hex,
				ReturnAlgorithm: models.AlgorithmRC4, ReturnPrivateKey: rc4Hex,
			},
		},
		{
			name: "Easy",
			api: &models.API{
				SubmitAlgorithm: models.AlgorithmEasy, SubmitPrivateKey: easyStr,
				ReturnAlgorithm: models.AlgorithmEasy, ReturnPrivateKey: easyStr,
			},
		},
		{
			name: "RSA",
			api: &models.API{
				SubmitAlgorithm: models.AlgorithmRSA, SubmitPrivateKey: rsaPriv,
				ReturnAlgorithm: models.AlgorithmRSA, ReturnPublicKey: rsaPub,
			},
		},
	}

	plain := `{"card":"KM-ABCD-1234","machine_code":"MC-9"}`
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			codec := NewAPICodec(tc.api)
			cipher, err := codec.EncryptResponse(plain)
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}
			if tc.name != "None" && cipher == plain {
				t.Fatalf("ciphertext should differ from plaintext")
			}
			got, err := codec.DecryptRequest(cipher)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if got != plain {
				t.Fatalf("round trip mismatch:\n want %q\n got  %q", plain, got)
			}
		})
	}
}

func TestCardLoginActivateStatusLogout(t *testing.T) {
	db := setupPublicTestDB(t)

	// 植入一张未使用的登录卡（1 天）
	card := models.Card{
		CardNo: "KM-TESTCARD", Duration: 24 * 60, Status: models.CardStatusUnused,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}

	// 首次登录 → 激活并创建卡密账号
	res, err := CardLogin("APP-1", "KM-TESTCARD", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("first CardLogin: %v", err)
	}
	if res.Token == "" || res.Type != models.MemberTypeCard {
		t.Fatalf("unexpected login result: %+v", res)
	}
	// 卡应被核销
	var reloaded models.Card
	db.First(&reloaded, card.ID)
	if reloaded.Status != models.CardStatusUsed || reloaded.UsedByMember == "" {
		t.Fatalf("card not marked used: %+v", reloaded)
	}
	// 自动创建了卡密账号
	var memberCount int64
	db.Model(&models.Member{}).Where("username = ?", "KM-TESTCARD").Count(&memberCount)
	if memberCount != 1 {
		t.Fatalf("expected 1 member created, got %v", memberCount)
	}

	// 心跳应通过
	if _, err := CheckMemberStatus("APP-1", appTokenOf(t, "APP-1", res.Token, "FP-AUTO")); err != nil {
		t.Fatalf("status check should pass: %v", err)
	}

	// 再次登录（已使用卡）→ 二级令牌模型：每次登录发新的**账号令牌**，
	// 但旧令牌不会被吊销（账号令牌可多设备并存），两者都应可用。
	// 「同一台设备重复进入同一应用不重复扣费」由激活的幂等保证（见 TestActivationIdempotent）。
	res2, err := CardLogin("APP-1", "KM-TESTCARD", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("second CardLogin: %v", err)
	}
	if res2.Token == res.Token {
		t.Fatalf("re-login should issue a new token")
	}
	if _, err := CheckMemberStatus("APP-1", appTokenOf(t, "APP-1", res.Token, "FP-AUTO")); err != nil {
		t.Fatalf("first account token should still be valid: %v", err)
	}
	if _, err := CheckMemberStatus("APP-1", appTokenOf(t, "APP-1", res2.Token, "FP-AUTO2")); err != nil {
		t.Fatalf("new token should be valid: %v", err)
	}

	// 登出 → 令牌清空，心跳失败
	if err := MemberLogout("APP-1", res2.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, _, _, err := AuthForApp(db, "APP-1", res2.Token); err == nil {
		t.Fatalf("account token should be invalid after logout")
	}
}

func TestCardLoginFrozenRejected(t *testing.T) {
	db := setupPublicTestDB(t)
	card := models.Card{
		CardNo: "KM-FROZEN", Duration: 60, Status: models.CardStatusFrozen,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}
	if _, err := CardLogin("APP-1", "KM-FROZEN", "", "1.2.3.4", "1.0.0", ""); err == nil {
		t.Fatalf("frozen card login should be rejected")
	}
}

func TestAccountRegisterLoginRecharge(t *testing.T) {
	db := setupPublicTestDB(t)
	// 开启注册与充值（全局账号策略）
	setPolicy(t, map[string]interface{}{"register_enabled": 1, "recharge_enabled": 1})

	// 注册（无试用 → 注册即过期，不返回令牌）
	reg, err := AccountRegister("APP-1", "alice@test.com", "secret1", "", "", "1.2.3.4", "")
	if err != nil {
		t.Fatalf("AccountRegister: %v", err)
	}
	if reg.Username != "alice@test.com" {
		t.Fatalf("unexpected register result: %+v", reg)
	}
	// 重复注册应失败
	if _, err := AccountRegister("APP-1", "alice@test.com", "x", "", "", "1.2.3.4", ""); err == nil {
		t.Fatalf("duplicate register should fail")
	}
	// 未充值（已过期）：二级令牌下「登录」只做身份认证，应当成功；
	// 但「激活」要过会员模式的有效期门禁，应当被拒。
	expiredLogin, err := AccountLogin("APP-1", "alice@test.com", "secret1", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("login should succeed before recharge (identity only): %v", err)
	}
	if _, err := activateForTest(t, "APP-1", expiredLogin.Token, "FP-EXP"); err == nil {
		t.Fatalf("activation should be rejected before recharge (expired)")
	}

	// 用一张卡为账号充值 30 天
	card := models.Card{CardNo: "KM-RC", Duration: 30 * 24 * 60, Status: models.CardStatusUnused}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}
	res, err := RechargeByCard("APP-1", "alice@test.com", "KM-RC")
	if err != nil {
		t.Fatalf("RechargeByCard: %v", err)
	}
	if res.ExpiredAt.Before(time.Now().Add(29 * 24 * time.Hour)) {
		t.Fatalf("recharge did not extend expiry: %v", res.ExpiredAt)
	}
	// 卡应被核销
	var reloaded models.Card
	db.First(&reloaded, card.ID)
	if reloaded.Status != models.CardStatusUsed {
		t.Fatalf("recharge card not marked used")
	}
	// 重复用同一张卡充值应失败
	if _, err := RechargeByCard("APP-1", "alice@test.com", "KM-RC"); err == nil {
		t.Fatalf("reusing consumed card should fail")
	}

	// 错误密码登录失败
	if _, err := AccountLogin("APP-1", "alice@test.com", "wrong", "", "1.2.3.4", "1.0.0", ""); err == nil {
		t.Fatalf("login with wrong password should fail")
	}
	// 充值后正确密码登录成功
	login, err := AccountLogin("APP-1", "alice@test.com", "secret1", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("AccountLogin after recharge: %v", err)
	}
	if login.Token == "" {
		t.Fatalf("login token empty")
	}

	// 到期查询应可用
	if _, err := GetMemberExpiry("APP-1", login.Token); err != nil {
		t.Fatalf("GetMemberExpiry: %v", err)
	}
}

func TestCardRegister(t *testing.T) {
	db := setupPublicTestDB(t)
	// 开启注册 + 卡密注册（全局账号策略；计费模式默认会员模式）
	setPolicy(t, map[string]interface{}{"register_enabled": 1, "card_register_enabled": 1})

	// 未提交卡密 → 拒绝
	if _, err := AccountRegister("APP-1", "cr1@test.com", "pw123456", "", "", "1.2.3.4", ""); err == nil {
		t.Fatalf("register without card should be rejected when card register on")
	}
	// 不存在的卡 → 拒绝
	if _, err := AccountRegister("APP-1", "cr1@test.com", "pw123456", "", "KM-NOPE", "1.2.3.4", ""); err == nil {
		t.Fatalf("register with unknown card should be rejected")
	}

	// 冻结卡 → 拒绝
	frozen := models.Card{CardNo: "KM-CRFROZEN", Duration: 24 * 60, Status: models.CardStatusFrozen}
	db.Create(&frozen)
	if _, err := AccountRegister("APP-1", "cr1@test.com", "pw123456", "", "KM-CRFROZEN", "1.2.3.4", ""); err == nil {
		t.Fatalf("register with frozen card should be rejected")
	}

	// 有效未用卡（30 天）→ 注册成功，账号获得约 30 天到期，卡被核销
	card := models.Card{CardNo: "KM-CRREG", Duration: 30 * 24 * 60, Status: models.CardStatusUnused}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}
	reg, err := AccountRegister("APP-1", "cr1@test.com", "pw123456", "", "KM-CRREG", "1.2.3.4", "")
	if err != nil {
		t.Fatalf("card register should succeed: %v", err)
	}
	if reg.ExpiredAt.Before(time.Now().Add(29 * 24 * time.Hour)) {
		t.Fatalf("card register did not grant card duration: %v", reg.ExpiredAt)
	}
	// 卡应被核销并记录去向
	var reloaded models.Card
	db.First(&reloaded, card.ID)
	if reloaded.Status != models.CardStatusUsed {
		t.Fatalf("register card not marked used")
	}
	// 注册即带时长 → 可直接登录
	if _, err := AccountLogin("APP-1", "cr1@test.com", "pw123456", "", "1.2.3.4", "1.0.0", ""); err != nil {
		t.Fatalf("login right after card register should succeed: %v", err)
	}

	// 同一张卡不能被第二个账号复用
	if _, err := AccountRegister("APP-1", "cr2@test.com", "pw123456", "", "KM-CRREG", "1.2.3.4", ""); err == nil {
		t.Fatalf("reusing consumed card for register should fail")
	}
	// 复用失败时不应残留 cr2 账号（事务回滚）
	var leaked int64
	db.Model(&models.Member{}).Where("username = ?", "cr2@test.com").Count(&leaked)
	if leaked != 0 {
		t.Fatalf("failed card register should not create member, found %v", leaked)
	}
}

func TestCardRegisterPointsMode(t *testing.T) {
	db := setupPublicTestDB(t)
	// 卡密注册：注册即按卡面值发放点数（与计费模式无关，面值有哪个发哪个）
	setPolicy(t, map[string]interface{}{"register_enabled": 1, "card_register_enabled": 1})
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").
		Updates(map[string]interface{}{"billing_mode": models.BillingModePoints, "bill_points_enabled": 1, "bill_time_enabled": 0})

	card := models.Card{CardNo: "KM-CRPTS", Points: 500, Status: models.CardStatusUnused}
	db.Create(&card)
	reg, err := AccountRegister("APP-1", "pts@test.com", "pw123456", "", "KM-CRPTS", "1.2.3.4", "")
	if err != nil {
		t.Fatalf("points-mode card register should succeed: %v", err)
	}
	if reg.Points != 500 {
		t.Fatalf("card points not granted, got %v", reg.Points)
	}
}

func TestBackfillRegisterMachineOnLogin(t *testing.T) {
	db := setupPublicTestDB(t)

	// 后台建号：register_machine 为空，给足时长使其可登录
	m, err := CreateMember("bf@test.com", "pw123456", 24*60, 0, "")
	if err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	if m.RegisterMachine != "" {
		t.Fatalf("precondition: register_machine should be empty, got %q", m.RegisterMachine)
	}

	// 首次带设备码登录 → 回填为注册设备
	if _, err := AccountLogin("APP-1", "bf@test.com", "pw123456", "MC-FIRST", "1.2.3.4", "1.0.0", ""); err != nil {
		t.Fatalf("login: %v", err)
	}
	var after models.Member
	db.Where("username = ?", "bf@test.com").First(&after)
	if after.RegisterMachine != "MC-FIRST" {
		t.Fatalf("register_machine not backfilled, got %q", after.RegisterMachine)
	}

	// 再次用不同设备码登录 → 已有注册设备，不覆盖
	if _, err := AccountLogin("APP-1", "bf@test.com", "pw123456", "MC-SECOND", "1.2.3.4", "1.0.0", ""); err != nil {
		t.Fatalf("login2: %v", err)
	}
	db.Where("username = ?", "bf@test.com").First(&after)
	if after.RegisterMachine != "MC-FIRST" {
		t.Fatalf("register_machine should not be overwritten, got %q", after.RegisterMachine)
	}

	// 不带设备码登录不应把注册设备清空（另建一个无注册设备的号验证）
	if _, err := CreateMember("bf2@test.com", "pw123456", 24*60, 0, ""); err != nil {
		t.Fatalf("CreateMember2: %v", err)
	}
	if _, err := AccountLogin("APP-1", "bf2@test.com", "pw123456", "", "1.2.3.4", "1.0.0", ""); err != nil {
		t.Fatalf("login3: %v", err)
	}
	var after2 models.Member
	db.Where("username = ?", "bf2@test.com").First(&after2)
	if after2.RegisterMachine != "" {
		t.Fatalf("empty machine code should not backfill, got %q", after2.RegisterMachine)
	}
}

func TestFreeModeUsableWhenExpired(t *testing.T) {
	db := setupPublicTestDB(t)
	// 切换为免费模式
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("free_mode", 1)

	// 建号后置为已过期
	m, err := CreateMember("free@test.com", "pw123456", 1, 0, "")
	if err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	past := time.Now().Add(-24 * time.Hour)
	if err := db.Model(&models.Member{}).Where("uuid = ?", m.UUID).Update("expired_at", past).Error; err != nil {
		t.Fatalf("force expire: %v", err)
	}

	// 免费模式：过期账号仍可登录
	res, err := AccountLogin("APP-1", "free@test.com", "pw123456", "MC-1", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("free-mode login should succeed even when expired: %v", err)
	}

	// 免费模式：走默认扣费路径（no_charge=false）心跳也不扣费、不因过期拒绝
	if _, err := CheckMemberStatus("APP-1", appTokenOf(t, "APP-1", res.Token, "FP-AUTO")); err != nil {
		t.Fatalf("free-mode heartbeat should be usable when expired: %v", err)
	}

	// 对照：切回会员模式后，同一过期账号登录仍成功，但激活应被拒
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{"free_mode": 0, "billing_mode": models.BillingModeMember, "bill_time_enabled": 1, "bill_points_enabled": 0, "level_gate_enabled": 1})
	memLogin, err := AccountLogin("APP-1", "free@test.com", "pw123456", "MC-1", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("login should still succeed (identity only): %v", err)
	}
	if _, err := activateForTest(t, "APP-1", memLogin.Token, "MC-1"); err == nil {
		t.Fatalf("member-mode expired account should be rejected at activation")
	}
}


func TestLoginVersionUpdate(t *testing.T) {
	db := setupPublicTestDB(t)
	seedAppVersion(t, db, "APP-1", "2.0.0", models.DownloadTypeFree, "http://x/app", "修复若干问题", true)
	card := models.Card{CardNo: "KM-VER", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)

	// 旧版本登录 → 需更新，返回更新信息
	res, err := CardLogin("APP-1", "KM-VER", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.Update == nil {
		t.Fatalf("update info should be returned when download_type != 0")
	}
	if !res.Update.NeedUpdate || res.Update.DownloadType != models.DownloadTypeFree ||
		res.Update.LatestVersion != "2.0.0" || res.Update.DownloadURL != "http://x/app" {
		t.Fatalf("unexpected update info: %+v", res.Update)
	}

	// 新版本登录 → 仍返回 update 对象，但 need_update=false
	res2, err := CardLogin("APP-1", "KM-VER", "", "1.2.3.4", "2.0.0", "")
	if err != nil {
		t.Fatalf("login2: %v", err)
	}
	if res2.Update == nil || res2.Update.NeedUpdate {
		t.Fatalf("up-to-date client should not need update: %+v", res2.Update)
	}

	// 停用全部版本 → 没有可更新版本，登录不带 update 信息
	db.Model(&models.AppVersion{}).Where("app_uuid = ?", "APP-1").Update("enabled", false)
	res3, err := CardLogin("APP-1", "KM-VER", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("login3: %v", err)
	}
	if res3.Update != nil {
		t.Fatalf("no update info expected when download_type=0, got %+v", res3.Update)
	}
}

func TestForceUpdateRejectsOldClient(t *testing.T) {
	db := setupPublicTestDB(t)
	seedAppVersion(t, db, "APP-1", "2.0.0", models.DownloadTypeForce, "http://x/app", "必须升级", true)
	card := models.Card{CardNo: "KM-FU", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)

	// 强制更新 + 版本过旧 → 拒绝登录，返回带更新信息的 ForceUpdateError
	_, err := CardLogin("APP-1", "KM-FU", "", "1.2.3.4", "1.0.0", "")
	if err == nil {
		t.Fatalf("force update should reject old client login")
	}
	var fue *ForceUpdateError
	if !errors.As(err, &fue) {
		t.Fatalf("expected ForceUpdateError, got %T: %v", err, err)
	}
	if fue.Update.LatestVersion != "2.0.0" || fue.Update.DownloadURL != "http://x/app" ||
		fue.Update.DownloadType != models.DownloadTypeForce {
		t.Fatalf("unexpected update info: %+v", fue.Update)
	}
	// 关键：拒绝时不得核销卡、不得建号、不得开会话
	var reloaded models.Card
	db.First(&reloaded, card.ID)
	if reloaded.Status != models.CardStatusUnused {
		t.Fatalf("card must NOT be consumed on force-update rejection, status=%v", reloaded.Status)
	}
	var members, sessions int64
	db.Model(&models.Member{}).Count(&members)
	db.Model(&models.AppActivation{}).Count(&sessions)
	if members != 0 || sessions != 0 {
		t.Fatalf("no member/session should be created on rejection, members=%v sessions=%v", members, sessions)
	}

	// 版本达标 → 正常登录、下发令牌
	res, err := CardLogin("APP-1", "KM-FU", "", "1.2.3.4", "2.0.0", "")
	if err != nil {
		t.Fatalf("up-to-date client should log in: %v", err)
	}
	if res.Token == "" {
		t.Fatalf("expected token on success")
	}
	if res.Update == nil || res.Update.NeedUpdate {
		t.Fatalf("up-to-date force-update app should return update object with need_update=false: %+v", res.Update)
	}
}

func TestLoginRequiresVersion(t *testing.T) {
	db := setupPublicTestDB(t)
	card := models.Card{CardNo: "KM-NV", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)

	// 空版本 → 拒绝登录
	if _, err := CardLogin("APP-1", "KM-NV", "", "1.2.3.4", "", ""); err == nil {
		t.Fatalf("login without version should be rejected")
	}
	// 带版本 → 成功；版本记录在**激活**上（登录只发账号令牌，不建会话）
	res, err := CardLogin("APP-1", "KM-NV", "", "1.2.3.4", "3.1.4", "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	info := mustActivate(t, "APP-1", res.Token, "MC-NV")
	var act models.AppActivation
	if err := db.Where("token = ?", info.Token).First(&act).Error; err != nil {
		t.Fatalf("load activation: %v", err)
	}
	if act.Version != "3.1.4" {
		t.Fatalf("activation should record client version, got %q", act.Version)
	}
}

func TestCardPackageSnapshot(t *testing.T) {
	db := setupPublicTestDB(t)
	pkg := models.CardPackage{
		Name: "月卡", Type: models.PackageTypeTime,
		Duration: 30 * 24 * 60, Price: 1000, Status: 1,
	}
	db.Create(&pkg)

	cards, _, err := BatchCreateCards("SNAP", 12, 1, pkg.UUID, "")
	if err != nil {
		t.Fatalf("BatchCreateCards: %v", err)
	}
	if cards[0].Duration != 30*24*60 || cards[0].Price != 1000 || cards[0].PackageUUID != pkg.UUID {
		t.Fatalf("card should snapshot package value/price: %+v", cards[0])
	}

	// 套餐改面值与售价 → 已售出的卡不受影响
	db.Model(&models.CardPackage{}).Where("uuid = ?", pkg.UUID).
		Updates(map[string]interface{}{"duration": 1, "price": 9999})
	var c models.Card
	db.Where("card_no = ?", cards[0].CardNo).First(&c)
	if c.Duration != 30*24*60 || c.Price != 1000 {
		t.Fatalf("snapshot polluted by package change: duration=%v price=%v", c.Duration, c.Price)
	}

	// 一号通：卡密全站通用，套餐面值不再与应用计费模式绑定。
	// 点数套餐可直接制卡，面值快照进卡。
	ptPkg := models.CardPackage{
		Name: "100点", Type: models.PackageTypePoints,
		Points: 100, Price: 500, Status: 1,
	}
	db.Create(&ptPkg)
	ptsCards, _, err := BatchCreateCards("X", 12, 1, ptPkg.UUID, "")
	if err != nil {
		t.Fatalf("points package should be usable: %v", err)
	}
	if ptsCards[0].Points != 100 || ptsCards[0].Price != 500 {
		t.Fatalf("points card snapshot wrong: points=%v price=%v", ptsCards[0].Points, ptsCards[0].Price)
	}

	// 面值为空的套餐应被拒绝（避免制出废卡）
	if _, _, err := BatchCreateCards("Y", 12, 1, "", ""); err == nil {
		t.Fatalf("empty package uuid should be rejected")
	}
}

func TestMemberLevelRebateAndUpgrade(t *testing.T) {
	db := setupPublicTestDB(t)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"billing_mode":       models.BillingModePoints,
			"bill_points_enabled": 1,
			"bill_time_enabled":   0,
		"points_per_login": 0, // 登录不扣点，便于断言面值
	})
	pkg := models.CardPackage{
		Name: "100点", Type: models.PackageTypePoints,
		Points: 100, Price: 1000, Status: 1,
	}
	db.Create(&pkg)
	lv := models.MemberLevel{
		Name: "白银", Level: 2, Threshold: 1000, RebateRate: 10, Status: 1,
	}
	db.Create(&lv)

	cards, _, err := BatchCreateCards("PK", 12, 2, pkg.UUID, "")
	if err != nil {
		t.Fatalf("BatchCreateCards: %v", err)
	}

	// 卡1 激活：新号无等级 → 按原面值 100 点；累充 1000 → 升级白银
	res, err := CardLogin("APP-1", cards[0].CardNo, "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	if res.Points != 100 {
		t.Fatalf("new account should get face value 100 (no rebate), got %v", res.Points)
	}
	var m models.Member
	db.Where("username = ?", cards[0].CardNo).First(&m)
	if m.TotalRecharge != 1000 || m.LevelUUID != lv.UUID {
		t.Fatalf("should accumulate 1000 and upgrade to silver, got total=%v level=%q", m.TotalRecharge, m.LevelUUID)
	}

	// 卡2 充值：白银返利 10% → 发 110 点，余额 100+110=210；累充 2000
	st, err := RechargeByCard("APP-1", cards[0].CardNo, cards[1].CardNo)
	if err != nil {
		t.Fatalf("RechargeByCard: %v", err)
	}
	if st.Points != 210 {
		t.Fatalf("silver 10%% rebate should grant 110 (total 210), got %v", st.Points)
	}
	db.Where("username = ?", cards[0].CardNo).First(&m)
	if m.TotalRecharge != 2000 {
		t.Fatalf("total recharge want 2000, got %v", m.TotalRecharge)
	}

	// 登录返回应带累计充值与会员等级信息
	relog, err := CardLogin("APP-1", cards[0].CardNo, "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("relogin: %v", err)
	}
	if relog.TotalRecharge != 2000 || relog.LevelName != "白银" || relog.Level != 2 || relog.RebateRate != 10 {
		t.Fatalf("login should return recharge/level info, got total=%v level=%v name=%q rebate=%v",
			relog.TotalRecharge, relog.Level, relog.LevelName, relog.RebateRate)
	}

	// 心跳也应带等级信息；中途管理员改等级 → 下次心跳同步到最新，避免客户端用旧权限
	hb1, err := CheckMemberStatus("APP-1", appTokenOf(t, "APP-1", relog.Token, "FP-AUTO3"))
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if hb1.Level != 2 || hb1.LevelName != "白银" {
		t.Fatalf("heartbeat should carry level info, got level=%v name=%q", hb1.Level, hb1.LevelName)
	}
	// 管理员把该等级权限调高
	db.Model(&models.MemberLevel{}).Where("uuid = ?", lv.UUID).Update("level", 5)
	hb2, err := CheckMemberStatus("APP-1", appTokenOf(t, "APP-1", relog.Token, "FP-AUTO3"))
	if err != nil {
		t.Fatalf("heartbeat2: %v", err)
	}
	if hb2.Level != 5 {
		t.Fatalf("heartbeat should reflect updated level 5, got %v", hb2.Level)
	}
}

func TestUpdateMemberProfileRecalibratesLevel(t *testing.T) {
	db := setupPublicTestDB(t)
	silver := models.MemberLevel{
		Name: "白银", Threshold: 1000, RebateRate: 10, Status: 1,
	}
	gold := models.MemberLevel{
		Name: "黄金", Threshold: 5000, RebateRate: 20, Status: 1,
	}
	db.Create(&silver)
	db.Create(&gold)

	m, err := CreateMember("tr@test.com", "pw123456", 24*60, 0, "")
	if err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	// 初始：无等级记录 = 默认档
	if m.TotalRecharge != 0 || m.LevelUUID != "" {
		t.Fatalf("new member should start at default level, got total=%v level=%q", m.TotalRecharge, m.LevelUUID)
	}

	setRecharge := func(v int) (*models.Member, error) {
		return UpdateMemberProfile(m.ID, MemberProfileUpdate{TotalRecharge: &v})
	}

	// 手动改累充到黄金门槛 → 升到黄金
	got, err := setRecharge(5000)
	if err != nil {
		t.Fatalf("UpdateMemberProfile: %v", err)
	}
	if got.TotalRecharge != 5000 || got.LevelUUID != gold.UUID {
		t.Fatalf("should calibrate to gold, got total=%v level=%q", got.TotalRecharge, got.LevelUUID)
	}

	// 手动改低到白银区间 → 相应降级为白银（手动改写按新值校准）
	got, err = setRecharge(1200)
	if err != nil {
		t.Fatalf("UpdateMemberProfile down: %v", err)
	}
	if got.LevelUUID != silver.UUID {
		t.Fatalf("lowering recharge should calibrate down to silver, got %q", got.LevelUUID)
	}

	// 改为 0 → 回到默认档（无等级）
	got, err = setRecharge(0)
	if err != nil {
		t.Fatalf("UpdateMemberProfile zero: %v", err)
	}
	if got.LevelUUID != "" {
		t.Fatalf("zero recharge should fall back to free level, got %q", got.LevelUUID)
	}

	// 负数拒绝
	if _, err := setRecharge(-1); err == nil {
		t.Fatalf("negative recharge should be rejected")
	}

	// 编辑点数与备注：不传的字段不动（累充仍为 0）
	pts, remark := 66.0, "vip客户"
	got, err = UpdateMemberProfile(m.ID, MemberProfileUpdate{Points: &pts, Remark: &remark})
	if err != nil {
		t.Fatalf("UpdateMemberProfile points: %v", err)
	}
	if got.Points != 66 || got.Remark != "vip客户" || got.TotalRecharge != 0 {
		t.Fatalf("partial update wrong: points=%s remark=%q total=%v", strconv.FormatFloat(got.Points, 'f', -1, 64), got.Remark, got.TotalRecharge)
	}

	// 设为永久
	perm := true
	got, err = UpdateMemberProfile(m.ID, MemberProfileUpdate{Permanent: &perm})
	if err != nil {
		t.Fatalf("UpdateMemberProfile permanent: %v", err)
	}
	if !got.ExpiredAt.Equal(models.PermanentTime) {
		t.Fatalf("should be permanent, got %v", got.ExpiredAt)
	}
}

func TestRegisterLimitByIP(t *testing.T) {
	setupPublicTestDB(t)
	setPolicy(t, map[string]interface{}{
		"register_enabled":       1,
		"register_limit_enabled": 1,
		"register_limit_time":    1,
		"register_count":         1,
	})

	if _, err := AccountRegister("APP-1", "limit1@test.com", "pw123456", "", "", "8.8.8.8", ""); err != nil {
		t.Fatalf("first register should pass: %v", err)
	}
	if _, err := AccountRegister("APP-1", "limit2@test.com", "pw123456", "", "", "8.8.8.8", ""); err == nil {
		t.Fatalf("second register from same IP should be rejected")
	}
	if _, err := AccountRegister("APP-1", "limit3@test.com", "pw123456", "", "", "8.8.4.4", ""); err != nil {
		t.Fatalf("register from another IP should pass: %v", err)
	}
}

func TestRegisterLimitByDevice(t *testing.T) {
	setupPublicTestDB(t)
	setPolicy(t, map[string]interface{}{
		"register_enabled":              1,
		"register_device_limit_enabled": 1,
		"register_limit_time":           1,
		"register_count":                1,
	})

	// 开启设备限制但未提交设备码 → 拒绝
	if _, err := AccountRegister("APP-1", "d0@test.com", "pw123456", "", "", "1.1.1.1", ""); err == nil {
		t.Fatalf("register without machine code should be rejected when device limit on")
	}
	// 同一设备第一次通过、第二次被拦（不同 IP 也拦，证明是按设备而非 IP）
	if _, err := AccountRegister("APP-1", "d1@test.com", "pw123456", "", "", "1.1.1.1", "MC-AAA"); err != nil {
		t.Fatalf("first register on device should pass: %v", err)
	}
	if _, err := AccountRegister("APP-1", "d2@test.com", "pw123456", "", "", "9.9.9.9", "MC-AAA"); err == nil {
		t.Fatalf("second register on same device (different IP) should be rejected")
	}
	// 换设备可继续
	if _, err := AccountRegister("APP-1", "d3@test.com", "pw123456", "", "", "1.1.1.1", "MC-BBB"); err != nil {
		t.Fatalf("register on another device should pass: %v", err)
	}
}

func TestClaimTrialLimits(t *testing.T) {
	setupPublicTestDB(t)
	setPolicy(t, map[string]interface{}{
		"register_enabled": 1,
		"trial_enabled":    1,
		"trial_limit_time": 1,
		"trial_duration":   60,
	})

	reg, err := AccountRegister("APP-1", "trial@test.com", "pw123456", "", "", "1.2.3.4", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if reg.ExpiredAt.After(time.Now().Add(5 * time.Minute)) {
		t.Fatalf("register should not auto grant trial, got %v", reg.ExpiredAt)
	}
	claimed, err := ClaimTrial("APP-1", "trial@test.com", "pw123456")
	if err != nil {
		t.Fatalf("ClaimTrial: %v", err)
	}
	if claimed.ExpiredAt.Before(time.Now().Add(55 * time.Minute)) {
		t.Fatalf("trial did not extend expiry enough: %v", claimed.ExpiredAt)
	}
	if _, err := ClaimTrial("APP-1", "trial@test.com", "pw123456"); err == nil {
		t.Fatalf("second permanent trial claim should be rejected")
	}
}

func TestClaimTrialRequiresExhausted(t *testing.T) {
	db := setupPublicTestDB(t)
	setPolicy(t, map[string]interface{}{
		"register_enabled": 1, "trial_enabled": 1, "trial_limit_time": 0, "trial_duration": 60,
		"trial_claim_mode": models.TrialClaimExhaustedOnly,
	})

	// 时长模式：未到期不能领
	if _, err := AccountRegister("APP-1", "act@test.com", "pw123456", "", "", "1.2.3.4", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	db.Model(&models.Member{}).Where("username = ?", "act@test.com").
		Update("expired_at", time.Now().Add(24*time.Hour))
	if _, err := ClaimTrial("APP-1", "act@test.com", "pw123456"); err == nil {
		t.Fatalf("active (not expired) account should NOT claim trial")
	}
	// 到期后可领
	db.Model(&models.Member{}).Where("username = ?", "act@test.com").
		Update("expired_at", time.Now().Add(-time.Hour))
	if _, err := ClaimTrial("APP-1", "act@test.com", "pw123456"); err != nil {
		t.Fatalf("expired account should claim trial: %v", err)
	}

	// 一号通：账号有余额即视为「仍可用」，不能领试用（与计费模式无关）
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{"billing_mode": models.BillingModePoints, "bill_points_enabled": 1, "bill_time_enabled": 0})
	if _, err := AccountRegister("APP-1", "pt@test.com", "pw123456", "", "", "1.2.3.4", ""); err != nil {
		t.Fatalf("register pt: %v", err)
	}
	db.Model(&models.Member{}).Where("username = ?", "pt@test.com").Update("points", 5)
	if _, err := ClaimTrial("APP-1", "pt@test.com", "pw123456"); err == nil {
		t.Fatalf("account with points should NOT claim trial")
	}
	db.Model(&models.Member{}).Where("username = ?", "pt@test.com").Update("points", 0)
	if _, err := ClaimTrial("APP-1", "pt@test.com", "pw123456"); err != nil {
		t.Fatalf("zero-point account should claim trial: %v", err)
	}
}

func TestClaimTrialUnlimitedAllowsActive(t *testing.T) {
	db := setupPublicTestDB(t)
	setPolicy(t, map[string]interface{}{
		"register_enabled": 1, "trial_enabled": 1, "trial_limit_time": 0,
		"trial_duration": 60, "trial_claim_mode": models.TrialClaimUnlimited,
	})
	if _, err := AccountRegister("APP-1", "unl@test.com", "pw123456", "", "", "1.2.3.4", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	// 未到期账号，在「无限制」方案下仍可领取
	db.Model(&models.Member{}).Where("username = ?", "unl@test.com").
		Update("expired_at", time.Now().Add(24*time.Hour))
	if _, err := ClaimTrial("APP-1", "unl@test.com", "pw123456"); err != nil {
		t.Fatalf("unlimited mode should allow active account to claim: %v", err)
	}
}

func TestDataInterfacesAndChangePassword(t *testing.T) {
	db := setupPublicTestDB(t)
	setPolicy(t, map[string]interface{}{"register_enabled": 1, "recharge_enabled": 1})
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").
		Update("app_data", base64Encode("APPDATA-XYZ"))

	// 植入全站变量与函数
	if err := db.Create(&models.Variable{Alias: "server_url", Data: "https://x"}).Error; err != nil {
		t.Fatalf("seed variable: %v", err)
	}
	if err := db.Create(&models.Function{Alias: "checkVip", Code: "return true"}).Error; err != nil {
		t.Fatalf("seed function: %v", err)
	}

	// 注册 + 充值 + 登录，拿到有效令牌
	if _, err := AccountRegister("APP-1", "carl@test.com", "pw123456", "", "", "1.2.3.4", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	card := models.Card{CardNo: "KM-DATA", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	if _, err := RechargeByCard("APP-1", "carl@test.com", "KM-DATA"); err != nil {
		t.Fatalf("recharge: %v", err)
	}
	login, err := AccountLogin("APP-1", "carl@test.com", "pw123456", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	token := login.Token

	// 程序数据
	if _, err := GetAppData("APP-1", appTokenOf(t, "APP-1", token, "FP-AUTO4")); err != nil {
		t.Fatalf("GetAppData: %v", err)
	}
	// 变量
	if _, err := GetVariable("APP-1", appTokenOf(t, "APP-1", token, "FP-AUTO4"), "server_url"); err != nil {
		t.Fatalf("GetVariable: %v", err)
	}
	// 全局函数
	if _, err := GetFunction("APP-1", appTokenOf(t, "APP-1", token, "FP-AUTO4"), "checkVip"); err != nil {
		t.Fatalf("GetFunction: %v", err)
	}
	// 不存在的变量
	if _, err := GetVariable("APP-1", appTokenOf(t, "APP-1", token, "FP-AUTO4"), "nope"); err == nil {
		t.Fatalf("missing variable should error")
	}
	// 无效令牌
	if _, err := GetAppData("APP-1", "badtoken"); err == nil {
		t.Fatalf("invalid token should error")
	}

	// 改密：旧密码错误应失败
	if _, err := ChangeMemberPassword("APP-1", token, "wrong", "newpass1"); err == nil {
		t.Fatalf("wrong old password should fail")
	}
	// 改密成功
	if _, err := ChangeMemberPassword("APP-1", token, "pw123456", "newpass1"); err != nil {
		t.Fatalf("ChangeMemberPassword: %v", err)
	}
	// 改密后旧令牌失效：账号令牌应查不到
	if _, _, _, err := AuthForApp(db, "APP-1", token); err == nil {
		t.Fatalf("account token should be invalidated after password change")
	}
	// 用新密码可再次登录
	if _, err := AccountLogin("APP-1", "carl@test.com", "newpass1", "", "1.2.3.4", "1.0.0", ""); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}

func TestChangePasswordRejectedForCardAccount(t *testing.T) {
	db := setupPublicTestDB(t)
	card := models.Card{CardNo: "KM-CARDPWD", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	login, err := CardLogin("APP-1", "KM-CARDPWD", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	if _, err := ChangeMemberPassword("APP-1", login.Token, "x", "y123456"); err == nil {
		t.Fatalf("card account should not allow password change")
	}
}

func base64Encode(s string) string {
	return b64.StdEncoding.EncodeToString([]byte(s))
}

func TestRebindMachineLimitsAndDeduct(t *testing.T) {
	db := setupPublicTestDB(t)
	// 免费转绑次数现在是账号级配额：免费 1 次，第 2 次起每次扣 60 分钟
	setMachineVerify(db, 1)
	setTestQuota(db, 1, 20, 1)
	setRebindWindow(db, 1) // 1=永久（窗口已账号级）
	// 绑定与转绑已下沉账号策略：开启校验、允许转绑、最多 2 次、超免费扣 60 分钟
	// 设备：开、免费 1 次、永久窗口、最多 2 次、超免费扣 60；IP：开、宽松
	setMachineVerify(db, 1)
	setRebindPolicy(db, 1, 1, 1, 1, 2, 60, 2, 1, 0, 0)

	// 卡密登录（带机器码），账号有 10 天
	card := models.Card{CardNo: "KM-RB", Duration: 10 * 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	login, err := CardLogin("APP-1", "KM-RB", "MC-OLD", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	before := loadMemberByToken(t, db, login.Token).ExpiredAt

	// 第 1 次转绑（免费，不扣时），绑定应替换为 MC-1（卡密账号用卡号鉴权，无需令牌）
	if _, err := Rebind("APP-1", "KM-RB", "", "MC-1", "", "", "1.2.3.4"); err != nil {
		t.Fatalf("rebind 1: %v", err)
	}
	m := loadMemberByToken(t, db, login.Token)
	if m.MachineRebindUsed != 1 {
		t.Fatalf("used should be 1, got %v", m.MachineRebindUsed)
	}
	if !m.ExpiredAt.Equal(before) {
		t.Fatalf("free rebind should not deduct time")
	}
	var bindCount int64
	db.Model(&models.Binding{}).Where("member_uuid = ? AND type = ?", m.UUID, models.BindingTypeMachine).Count(&bindCount)
	if bindCount != 1 {
		t.Fatalf("expected single machine binding after rebind, got %v", bindCount)
	}

	// 第 2 次转绑（超免费 → 扣 60 分钟）
	if _, err := Rebind("APP-1", "KM-RB", "", "MC-2", "", "", "1.2.3.4"); err != nil {
		t.Fatalf("rebind 2: %v", err)
	}
	m2 := loadMemberByToken(t, db, login.Token)
	if diff := before.Sub(m2.ExpiredAt) - time.Hour; diff > time.Second || diff < -time.Second {
		t.Fatalf("second rebind should deduct 60min, diff %v", diff)
	}

	// 第 3 次转绑 → 超过 max(2) 次上限，拒绝
	if _, err := Rebind("APP-1", "KM-RB", "", "MC-3", "", "", "1.2.3.4"); err == nil {
		t.Fatalf("third rebind should exceed limit")
	}
}

func TestRebindSameDeviceIsNoOp(t *testing.T) {
	db := setupPublicTestDB(t)
	setMachineVerify(db, 1)
	setRebindPolicy(db, 1, 1, 1, 1, 5, 60, 2, 1, 0, 0)
	card := models.Card{CardNo: "KM-SAME", Duration: 10 * 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	// 二级令牌：绑定发生在激活时，故登录后必须激活一次才能建立初始设备绑定
	loginSame, err := CardLogin("APP-1", "KM-SAME", "MC-A", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	mustActivate(t, "APP-1", loginSame.Token, "MC-A")

	load := func() models.Member {
		var m models.Member
		db.Where("username = ?", "KM-SAME").First(&m)
		return m
	}
	before := load().ExpiredAt

	// 换绑到同一设备 MC-A → 幂等 no-op：不计次、不扣时
	if _, err := Rebind("APP-1", "KM-SAME", "", "MC-A", "", "", "1.2.3.4"); err != nil {
		t.Fatalf("rebind same device: %v", err)
	}
	if m := load(); m.MachineRebindUsed != 0 {
		t.Fatalf("same-device rebind should not consume count, got %v", m.MachineRebindUsed)
	}
	if m := load(); !m.ExpiredAt.Equal(before) {
		t.Fatalf("same-device rebind should not deduct time")
	}

	// 换绑到新设备 MC-B → 正常计次并扣时
	if _, err := Rebind("APP-1", "KM-SAME", "", "MC-B", "", "", "1.2.3.4"); err != nil {
		t.Fatalf("rebind new device: %v", err)
	}
	if m := load(); m.MachineRebindUsed != 1 {
		t.Fatalf("new-device rebind should count once, got %v", m.MachineRebindUsed)
	}
}

func TestMultiDeviceRebind(t *testing.T) {
	db := setupPublicTestDB(t)
	// 设备数是账号级配额（不再取 app.multi_open_count）
	setMachineVerify(db, 1)
	setTestQuota(db, 2, 20, 20)
	// 绑定与转绑已下沉账号策略
	setMachineVerify(db, 1)
	setRebindPolicy(db, 1, 1, 1, 1, 0, 0, 2, 1, 0, 0)
	card := models.Card{CardNo: "KM-MD", Duration: 30 * 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)

	// 登录设备 A（带设备名）
	loginA, err := CardLogin("APP-1", "KM-MD", "MC-A", "1.1.1.1", "1.0.0", "iPhone 15")
	if err != nil {
		t.Fatalf("login A: %v", err)
	}
	mustActivate(t, "APP-1", loginA.Token, "MC-A")
	var member models.Member
	db.Where("username = ?", "KM-MD").First(&member)

	// 未满(1/2) → 转绑新设备 B 是「新增」，不替换
	if _, err := Rebind("APP-1", "KM-MD", "", "MC-B", "", "Pixel 8", "1.1.1.1"); err != nil {
		t.Fatalf("add device B: %v", err)
	}
	countBindings := func() int64 {
		var c int64
		db.Model(&models.Binding{}).Where("member_uuid = ? AND type = ?", member.UUID, models.BindingTypeMachine).Count(&c)
		return c
	}
	if countBindings() != 2 {
		t.Fatalf("should have 2 device bindings, got %v", countBindings())
	}

	// 不带设备码 → 返回设备列表（含设备号/名称/时间）
	res, err := Rebind("APP-1", "KM-MD", "", "", "", "", "1.1.1.1")
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	devices := res.(map[string]any)["devices"].([]BoundDevice)
	if len(devices) != 2 {
		t.Fatalf("device list should have 2, got %v", len(devices))
	}
	names := map[string]string{}
	for _, d := range devices {
		names[d.MachineCode] = d.DeviceName
		if d.BoundAt == "" {
			t.Fatalf("device %s missing bound_at", d.MachineCode)
		}
	}
	if names["MC-A"] != "iPhone 15" || names["MC-B"] != "Pixel 8" {
		t.Fatalf("device names wrong: %+v", names)
	}

	// 已满(2/2) 且未指定要替换的设备 → 拒绝
	if _, err := Rebind("APP-1", "KM-MD", "", "MC-C", "", "iPad", "1.1.1.1"); err == nil {
		t.Fatalf("full without replace target should be rejected")
	}

	// 指定替换 MC-A → A 被换成 C
	if _, err := Rebind("APP-1", "KM-MD", "", "MC-C", "MC-A", "iPad", "1.1.1.1"); err != nil {
		t.Fatalf("replace MC-A with MC-C: %v", err)
	}
	var vals []string
	db.Model(&models.Binding{}).Where("member_uuid = ? AND type = ?", member.UUID, models.BindingTypeMachine).
		Order("value").Pluck("value", &vals)
	if len(vals) != 2 || vals[0] != "MC-B" || vals[1] != "MC-C" {
		t.Fatalf("after replace expect [MC-B MC-C], got %v", vals)
	}

	// 替换不存在的设备 → 拒绝
	if _, err := Rebind("APP-1", "KM-MD", "", "MC-D", "MC-ZZZ", "", "1.1.1.1"); err == nil {
		t.Fatalf("replacing nonexistent device should be rejected")
	}
}

func TestDefaultLevelDisplay(t *testing.T) {
	db := setupPublicTestDB(t)
	card := models.Card{CardNo: "KM-DL", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)

	// 没有 level=1 等级 → 默认档：level=1、名称「默认会员」
	res, err := CardLogin("APP-1", "KM-DL", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.Level != 1 || res.LevelName != "默认会员" || res.LevelColor != "#909399" {
		t.Fatalf("default account should be level=1 / 默认会员 / 灰, got level=%v name=%q color=%q",
			res.Level, res.LevelName, res.LevelColor)
	}

	// 配置一个 level=1 的等级「青铜」并指定颜色（门槛很高，账号不会被分配到它）→ 默认档名称/颜色随之
	db.Create(&models.MemberLevel{Name: "青铜", Level: 1, Color: "#CD7F32", Threshold: 999999, Status: 1})
	res2, err := CardLogin("APP-1", "KM-DL", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("login2: %v", err)
	}
	if res2.Level != 1 || res2.LevelName != "青铜" || res2.LevelColor != "#CD7F32" {
		t.Fatalf("default level should follow level=1 config, got level=%v name=%q color=%q",
			res2.Level, res2.LevelName, res2.LevelColor)
	}
}

func TestLevelExtraConcurrent(t *testing.T) {
	db := setupPublicTestDB(t)
	// 并发数是账号级：基础值取账号策略，等级只做加量，应用配置不再参与
	setMachineVerify(db, 1)
	setTestQuota(db, 20, 1, 20)
	lv := models.MemberLevel{Name: "黄金", Threshold: 0, ExtraConcurrent: 2, Status: 1}
	db.Create(&lv)

	m, err := CreateMember("vip@test.com", "pw123456", 24*60, 0, "")
	if err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	// 建号即落到默认档（threshold=0 的「黄金」）：基础 1 + 等级额外 2 = 3
	if got := effectiveMaxConcurrent(db, m); got != 3 {
		t.Fatalf("new account should land on default level (base 1 + extra 2 = 3), got %v", got)
	}
	// 清空等级 → 回落策略基础值
	db.Model(&models.Member{}).Where("id = ?", m.ID).Update("level_uuid", "")
	db.First(m, m.ID)
	if got := effectiveMaxConcurrent(db, m); got != 1 {
		t.Fatalf("no level → concurrent should be strategy value 1, got %v", got)
	}
}

func TestIPVerifyBindingAndRebind(t *testing.T) {
	db := setupPublicTestDB(t)
	// IP 绑定与设备绑定共用同一个账号级上限（账号最多记几个来源地址）
	setMachineVerify(db, 1)
	setTestQuota(db, 1, 20, 20)
	// IP 校验与转绑已下沉账号策略：开启精确 IP 校验；本用例只开 IP 转绑
	setIPVerify(db, 1)
	setRebindPolicy(db, 0, 1, 0, 1, 0, 0, 2, 1, 2, 0)

	card := models.Card{CardNo: "KM-IP", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	// 二级令牌：IP 绑定校验发生在**激活**时（登录只做身份认证）
	ipLogin1, err := CardLogin("APP-1", "KM-IP", "MC-IP", "10.0.0.1", "1.0.0", "")
	if err != nil {
		t.Fatalf("first IP login should pass: %v", err)
	}
	mustActivate(t, "APP-1", ipLogin1.Token, "MC-IP")
	if _, err := activateForTest(t, "APP-1", ipLogin1.Token, "MC-IP"); err != nil {
		t.Fatalf("same IP re-activation should pass: %v", err)
	}
	// 换 IP 再次激活 → 应被 IP 绑定拒绝
	ipLogin2, err := CardLogin("APP-1", "KM-IP", "MC-IP", "10.0.0.2", "1.0.0", "")
	if err != nil {
		t.Fatalf("login from other IP should still pass (identity only): %v", err)
	}
	if _, err := Activate(mustDB(t), mustAccountToken(t, ipLogin2.Token), mustApp(t, "APP-1"), "MC-IP", "10.0.0.2", "1.0.0", ""); err == nil {
		t.Fatalf("activation from unbound IP should be rejected")
	}
	// 死循环验证：新 IP 激活被拒，但凭卡号转绑无需令牌，转绑后即可激活
	if _, err := Rebind("APP-1", "KM-IP", "", "", "", "", "10.0.0.2"); err != nil {
		t.Fatalf("Rebind IP: %v", err)
	}
	if _, err := Activate(mustDB(t), mustAccountToken(t, ipLogin2.Token), mustApp(t, "APP-1"), "MC-IP", "10.0.0.2", "1.0.0", ""); err != nil {
		t.Fatalf("activation from rebound IP should pass: %v", err)
	}
	// 转绑后旧 IP 再激活应被拒（登录本身仍成功，因为不校验绑定）
	oldIPLogin, err := CardLogin("APP-1", "KM-IP", "MC-IP", "10.0.0.1", "1.0.0", "")
	if err != nil {
		t.Fatalf("login should still succeed: %v", err)
	}
	if _, err := Activate(mustDB(t), mustAccountToken(t, oldIPLogin.Token), mustApp(t, "APP-1"), "MC-IP", "10.0.0.1", "1.0.0", ""); err == nil {
		t.Fatalf("activation from old IP should be rejected after rebind")
	}
}

func TestRebindDisabledRejected(t *testing.T) {
	db := setupPublicTestDB(t)
	setRebindPolicy(db, 0, 0, 1, 1, 0, 0, 2, 1, 0, 0) // 设备与 IP 转绑都关闭
	card := models.Card{CardNo: "KM-NOREBIND", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	if _, err := CardLogin("APP-1", "KM-NOREBIND", "", "1.2.3.4", "1.0.0", ""); err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	if _, err := Rebind("APP-1", "KM-NOREBIND", "", "MC-X", "", "", "1.2.3.4"); err == nil {
		t.Fatalf("rebind should be rejected when disabled")
	}
}

func TestVersionAndCardInfo(t *testing.T) {
	db := setupPublicTestDB(t)
	seedAppVersion(t, db, "APP-1", "1.2.0", models.DownloadTypeForce, "https://dl", "版本说明", true)

	// 旧版本需要更新
	r, err := CheckVersion("APP-1", "1.1.0")
	if err != nil {
		t.Fatalf("CheckVersion: %v", err)
	}
	if need := r.(map[string]any)["need_update"].(bool); !need {
		t.Fatalf("1.1.0 < 1.2.0 should need update")
	}
	// 同版本不需要更新
	r2, _ := CheckVersion("APP-1", "1.2.0")
	if need := r2.(map[string]any)["need_update"].(bool); need {
		t.Fatalf("same version should not need update")
	}

	// 卡密信息
	card := models.Card{CardNo: "KM-INFO", Duration: 60, Status: models.CardStatusUnused}
	db.Create(&card)
	infoApp := &models.App{UUID: "APP-1"}
	info, err := GetCardInfo(infoApp, "KM-INFO")
	if err != nil {
		t.Fatalf("GetCardInfo: %v", err)
	}
	if info.(map[string]any)["status"].(int) != models.CardStatusUnused {
		t.Fatalf("unexpected card status")
	}
	if _, err := GetCardInfo(infoApp, "NOPE"); err == nil {
		t.Fatalf("missing card should error")
	}
}

func TestExecuteRemoteFunction(t *testing.T) {
	db := setupPublicTestDB(t)
	// 一个用参数计算的函数：返回 a+b 与一个标记
	if err := db.Create(&models.Function{
		Alias:   "calc",
		Code:    "return { sum: params.a + params.b, vip: params.a > 10 };",
	}).Error; err != nil {
		t.Fatalf("seed calc: %v", err)
	}
	// Function.Number 为毫秒时间戳且唯一，隔开 2ms 避免同毫秒冲突
	time.Sleep(2 * time.Millisecond)
	// 死循环函数：验证超时中断
	if err := db.Create(&models.Function{
		Alias:   "loop",
		Code:    "while(true){}",
	}).Error; err != nil {
		t.Fatalf("seed loop: %v", err)
	}

	// 登录拿有效令牌
	card := models.Card{CardNo: "KM-FN", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	login, err := CardLogin("APP-1", "KM-FN", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// 执行 calc(a=20,b=5) → sum=25, vip=true
	res, err := ExecuteFunction("APP-1", appTokenOf(t, "APP-1", login.Token, "FP-EF1"), "calc",
		map[string]any{"a": 20, "b": 5})
	if err != nil {
		t.Fatalf("ExecuteFunction: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	if toFloat(m["sum"]) != 25 {
		t.Fatalf("want sum=25, got %v", m["sum"])
	}
	if m["vip"] != true {
		t.Fatalf("want vip=true, got %v", m["vip"])
	}

	// 未登录令牌应被拒
	if _, err := ExecuteFunction("APP-1", "bad", "calc", nil); err == nil {
		t.Fatalf("invalid token should fail")
	}
	// 死循环应超时返回错误（不挂起）
	if _, err := ExecuteFunction("APP-1", login.Token, "loop", nil); err == nil {
		t.Fatalf("infinite loop should be interrupted with error")
	}
	// 不存在的函数
	if _, err := ExecuteFunction("APP-1", login.Token, "nope", nil); err == nil {
		t.Fatalf("missing function should fail")
	}
}

// 验证沙箱内只读辅助函数 getUser()/getApp() 可用，且不泄露 secret/password。
func TestRemoteFunctionReadOnlyHelpers(t *testing.T) {
	db := setupPublicTestDB(t)
	if err := db.Create(&models.Function{
		Alias:   "whoami",
		Code: "var u=getUser(); var a=getApp();" +
			"return { name: u.username, app: a.uuid, leaked: (a.secret!==undefined)||(u.password!==undefined) };",
	}).Error; err != nil {
		t.Fatalf("seed whoami: %v", err)
	}

	card := models.Card{CardNo: "KM-WHO", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	login, err := CardLogin("APP-1", "KM-WHO", "", "9.9.9.9", "1.0.0", "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	res, err := ExecuteFunction("APP-1", appTokenOf(t, "APP-1", login.Token, "FP-EF2"), "whoami", nil)
	if err != nil {
		t.Fatalf("ExecuteFunction: %v", err)
	}
	m := res.(map[string]any)
	if m["app"] != "APP-1" {
		t.Fatalf("want app=APP-1, got %v", m["app"])
	}
	if m["name"] != "KM-WHO" {
		t.Fatalf("want name=KM-WHO, got %v", m["name"])
	}
	if m["leaked"] != false {
		t.Fatalf("secret/password must not be exposed to sandbox, got leaked=%v", m["leaked"])
	}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	default:
		return -1
	}
}

func TestSessionCleanupSweep(t *testing.T) {
	db := setupPublicTestDB(t)
	delete(lastSessionSweep, "APP-1") // 确保本次会执行清理
	// 自动离线时长设为 10 分钟，用于验证按 OfflineTimeout 清理
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("offline_timeout", 10)

	now := time.Now()
	// 租约已过期的激活：应被扫描回收
	db.Create(&models.AppActivation{
		Token: "stale", MemberUUID: "m1", AppUUID: "APP-1",
		LastActiveAt: now.Add(-20 * time.Minute), LeaseExpiresAt: now.Add(-10 * time.Minute),
	})
	// 租约有效的激活：保留
	db.Create(&models.AppActivation{
		Token: "fresh", MemberUUID: "m2", AppUUID: "APP-1",
		LastActiveAt: now, LeaseExpiresAt: now.Add(30 * time.Minute),
	})
	// 孤儿激活（应用不存在）：连同记录一起清掉
	db.Create(&models.AppActivation{
		Token: "orphan", MemberUUID: "m3", AppUUID: "GONE-APP",
		LastActiveAt: now, LeaseExpiresAt: now.Add(30 * time.Minute),
	})

	sweepSessions()

	exists := func(token string) bool {
		var n int64
		db.Model(&models.AppActivation{}).Where("token = ? AND deactivated_at IS NULL", token).Count(&n)
		return n > 0
	}
	if exists("stale") {
		t.Fatalf("stale session should be swept")
	}
	if !exists("fresh") {
		t.Fatalf("fresh session should be kept")
	}
	if exists("orphan") {
		t.Fatalf("orphan session (deleted app) should be swept")
	}
}

func TestOpenSignVerify(t *testing.T) {
	secret := "APPSECRET123"
	ts := time.Now().Unix()
	sign := SignOpenRequest("APP-1", 10, "CIPHERDATA", ts, secret)

	// 正确签名通过
	if err := VerifyOpenSign("APP-1", 10, "CIPHERDATA", ts, sign, secret); err != nil {
		t.Fatalf("valid sign should pass: %v", err)
	}
	// 错误密钥失败
	if err := VerifyOpenSign("APP-1", 10, "CIPHERDATA", ts, sign, "WRONG"); err == nil {
		t.Fatalf("wrong secret should fail")
	}
	// 篡改数据失败
	if err := VerifyOpenSign("APP-1", 10, "TAMPERED", ts, sign, secret); err == nil {
		t.Fatalf("tampered data should fail")
	}
	// 过期时间戳失败
	old := time.Now().Add(-10 * time.Minute).Unix()
	oldSign := SignOpenRequest("APP-1", 10, "CIPHERDATA", old, secret)
	if err := VerifyOpenSign("APP-1", 10, "CIPHERDATA", old, oldSign, secret); err == nil {
		t.Fatalf("expired timestamp should fail")
	}
	// 缺少签名失败
	if err := VerifyOpenSign("APP-1", 10, "CIPHERDATA", ts, "", secret); err == nil {
		t.Fatalf("missing sign should fail")
	}
}

func TestMultiOpenSessions(t *testing.T) {
	db := setupPublicTestDB(t)
	// 并发 2（账号级，按设备计），非顶号
	setMachineVerify(db, 1)
	setTestQuota(db, 20, 2, 20)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("machine_verify", 1)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").
		Update("login_type", 1)
	card := models.Card{CardNo: "KM-MO", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)

	// 二级令牌：并发名额按「激活」计数（app_activations），不是登录会话
	sessionCount := func() int64 {
		return ActiveActivationCount(db, mustMemberUUID(t, db, "KM-MO"), "APP-1")
	}

	// 前两次「登录 + 激活」成功（2 个占用）
	login1, err := CardLogin("APP-1", "KM-MO", "", "1.1.1.1", "1.0.0", "")
	if err != nil {
		t.Fatalf("login1: %v", err)
	}
	mustActivate(t, "APP-1", login1.Token, "MC-1")
	login2, err := CardLogin("APP-1", "KM-MO", "", "1.1.1.2", "1.0.0", "")
	if err != nil {
		t.Fatalf("login2: %v", err)
	}
	mustActivate(t, "APP-1", login2.Token, "MC-2")
	if sessionCount() != 2 {
		t.Fatalf("expected 2 activations, got %v", sessionCount())
	}
	// 第三次（非顶号）应被拒，占用数不变
	login3, err := CardLogin("APP-1", "KM-MO", "", "1.1.1.3", "1.0.0", "")
	if err != nil {
		t.Fatalf("login3 should succeed (identity only): %v", err)
	}
	if _, err := activateForTest(t, "APP-1", login3.Token, "MC-3"); err == nil {
		t.Fatalf("third activation should be rejected (non-preempt, limit 2)")
	}
	if sessionCount() != 2 {
		t.Fatalf("rejected activation should not take a slot, got %v", sessionCount())
	}

	// 切换为顶号：新激活成功且占用数仍为 2（踢掉了最早的一个）
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("login_type", 0)
	login4, err := CardLogin("APP-1", "KM-MO", "", "1.1.1.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("login4: %v", err)
	}
	info4 := mustActivate(t, "APP-1", login4.Token, "MC-4")
	if sessionCount() != 2 {
		t.Fatalf("preemption should keep activation count at limit, got %v", sessionCount())
	}
	// 新激活有效
	if _, err := CheckMemberStatus("APP-1", info4.Token); err != nil {
		t.Fatalf("new activation should be valid: %v", err)
	}
}

func TestMultiOpenScopeMachine(t *testing.T) {
	db := setupPublicTestDB(t)
	// 单设备范围 + 并发 1（账号级，按设备计）+ 非顶号
	setMachineVerify(db, 1)
	setTestQuota(db, 20, 1, 20)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("machine_verify", 1)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("login_type", 1)
	card := models.Card{CardNo: "KM-MS", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)

	sessCount := func() int64 {
		return ActiveActivationCount(db, mustMemberUUID(t, db, "KM-MS"), "APP-1")
	}

	// 机器 A 登录 + 激活 → 1 个占用
	loginA, err := CardLogin("APP-1", "KM-MS", "MC-A", "1.1.1.1", "1.0.0", "")
	if err != nil {
		t.Fatalf("login A: %v", err)
	}
	mustActivate(t, "APP-1", loginA.Token, "MC-A")
	if sessCount() != 1 {
		t.Fatalf("expected 1 activation, got %v", sessCount())
	}
	// 同机器 A 再登录 + 激活 → 幂等复用同一激活，占用数不增
	loginA2, err := CardLogin("APP-1", "KM-MS", "MC-A", "2.2.2.2", "1.0.0", "")
	if err != nil {
		t.Fatalf("re-login A: %v", err)
	}
	infoA2 := mustActivate(t, "APP-1", loginA2.Token, "MC-A")
	if !infoA2.Reused {
		t.Fatalf("same fingerprint should reuse activation")
	}
	if sessCount() != 1 {
		t.Fatalf("same machine re-activation should stay 1, got %v", sessCount())
	}
	// 不同机器 B（非顶号）→ 超出，激活被拒
	loginB, err := CardLogin("APP-1", "KM-MS", "MC-B", "3.3.3.3", "1.0.0", "")
	if err != nil {
		t.Fatalf("login B should succeed (identity only): %v", err)
	}
	if _, err := activateForTest(t, "APP-1", loginB.Token, "MC-B"); err == nil {
		t.Fatalf("second machine should be rejected (non-preempt, count=1)")
	}

	// 切顶号 → 机器 B 激活成功，踢掉机器 A
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("login_type", 0)
	loginB2, err := CardLogin("APP-1", "KM-MS", "MC-B", "3.3.3.3", "1.0.0", "")
	if err != nil {
		t.Fatalf("preempt login B: %v", err)
	}
	mustActivate(t, "APP-1", loginB2.Token, "MC-B")
	if sessCount() != 1 {
		t.Fatalf("preemption should keep 1 open, got %v", sessCount())
	}
	var a models.AppActivation
	db.Where("app_uuid = ? AND deactivated_at IS NULL", "APP-1").First(&a)
	if a.Fingerprint != "MC-B" {
		t.Fatalf("surviving open should be MC-B, got %s", a.Fingerprint)
	}
}

func TestRiskControl(t *testing.T) {
	db := setupPublicTestDB(t)
	card := models.Card{CardNo: "KM-RISK", Duration: 10 * 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	login, err := CardLogin("APP-1", "KM-RISK", "", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("CardLogin: %v", err)
	}

	// 扣时 1 天
	if _, err := RiskDeductMember("APP-1", "KM-RISK", 24*60); err != nil {
		t.Fatalf("RiskDeductMember: %v", err)
	}

	// 封停 → 会话失效 + 状态封停
	if _, err := RiskDisableMember("APP-1", "KM-RISK"); err != nil {
		t.Fatalf("RiskDisableMember: %v", err)
	}
	if _, err := CheckMemberStatus("APP-1", login.Token); err == nil {
		t.Fatalf("session should be killed after disable")
	}
	var m models.Member
	db.Where("username = ?", "KM-RISK").First(&m)
	if m.Status != models.MemberStatusDisabled {
		t.Fatalf("member should be disabled, got status %v", m.Status)
	}

	// 拉黑
	if _, err := RiskBlacklistMember("APP-1", "KM-RISK"); err != nil {
		t.Fatalf("RiskBlacklistMember: %v", err)
	}
	db.Where("username = ?", "KM-RISK").First(&m)
	if m.Status != models.MemberStatusBlack {
		t.Fatalf("member should be blacklisted, got status %v", m.Status)
	}

	// 不存在的用户
	if _, err := RiskDisableMember("APP-1", "NOBODY"); err == nil {
		t.Fatalf("disabling nonexistent user should fail")
	}
}

func TestEmailValidationAndRegister(t *testing.T) {
	for _, ok := range []string{"a@b.com", "user.name+tag@sub.domain.cn"} {
		if !IsValidEmail(ok) {
			t.Fatalf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "notanemail", "a@b", "a b@c.com", "@x.com"} {
		if IsValidEmail(bad) {
			t.Fatalf("%q should be invalid", bad)
		}
	}

	db := setupPublicTestDB(t)
	setPolicy(t, map[string]interface{}{"register_enabled": 1})

	// 非法邮箱注册被拒
	if _, err := AccountRegister("APP-1", "notanemail", "pw123456", "", "", "1.2.3.4", ""); err == nil {
		t.Fatalf("register with invalid email should fail")
	}

	// 开启邮箱验证后，无有效验证码注册应失败（测试环境无 Redis → 验证码服务不可用亦属拒绝）
	setPolicy(t, map[string]interface{}{"email_verify_enabled": 1})
	if _, err := AccountRegister("APP-1", "eve@test.com", "pw123456", "000000", "", "1.2.3.4", ""); err == nil {
		t.Fatalf("register should fail without valid email code when verification enabled")
	}

	// 关闭验证：邮箱注册账号 username=email 且 Email 字段落库
	setPolicy(t, map[string]interface{}{"email_verify_enabled": 0})
	if _, err := AccountRegister("APP-1", "frank@test.com", "pw123456", "", "", "1.2.3.4", ""); err != nil {
		t.Fatalf("register should succeed with verification off: %v", err)
	}
	var reg models.Member
	db.Where("username = ?", "frank@test.com").First(&reg)
	if reg.Email != "frank@test.com" {
		t.Fatalf("email field not stored, got %q", reg.Email)
	}
}

func TestPointsPerCountMode(t *testing.T) {
	db := setupPublicTestDB(t)
	// 点数模式 + 按次(登录扣1点) + 开启充值
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"billing_mode":       models.BillingModePoints,
			"bill_points_enabled": 1,
			"bill_time_enabled":   0,
		"points_charge_mode": models.PointsChargePerCount,
		"points_per_login":   1,
	})
	// 面值 3 点的卡
	card := models.Card{CardNo: "KM-PC", Points: 3, Status: models.CardStatusUnused}
	db.Create(&card)

	// 卡密登录 → 账号获得 3 点（登录本身不扣点）
	res, err := CardLogin("APP-1", "KM-PC", "MC-PC", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	if res.Points != 3 {
		t.Fatalf("login should not charge: want 3 points, got %v", res.Points)
	}
	// 激活 → 按次扣 1 点 → 余 2
	actRes := mustActivate(t, "APP-1", res.Token, "MC-PC")
	if actRes.Points != 2 {
		t.Fatalf("activation should charge 1 point (3->2), got %v", actRes.Points)
	}

	// 显式功能扣点：用**激活令牌**（新协议下的正确用法）：扣 1 → 余 1
	st, err := DeductPoints("APP-1", actRes.Token, 1)
	if err != nil {
		t.Fatalf("DeductPoints: %v", err)
	}
	if st.Points != 1 {
		t.Fatalf("want 1 point after deduct, got %v", st.Points)
	}
	// 扣点超额 → 拒绝
	if _, err := DeductPoints("APP-1", actRes.Token, 5); err == nil {
		t.Fatalf("over-deduct should fail")
	}

	// 充值：加 10 点 → 余 11
	rc := models.Card{CardNo: "KM-PC-RC", Points: 10, Status: models.CardStatusUnused}
	db.Create(&rc)
	rst, err := RechargeByCard("APP-1", "KM-PC", "KM-PC-RC")
	if err != nil {
		t.Fatalf("recharge: %v", err)
	}
	if rst.Points != 11 {
		t.Fatalf("want 11 points after recharge, got %v", rst.Points)
	}
}

func TestPointsPerCountLoginRejectedWhenEmpty(t *testing.T) {
	db := setupPublicTestDB(t)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"billing_mode":       models.BillingModePoints,
			"bill_points_enabled": 1,
			"bill_time_enabled":   0,
		"points_charge_mode": models.PointsChargePerCount,
		"points_per_login":   1,
	})
	// 1 点卡：首登扣光
	card := models.Card{CardNo: "KM-PC1", Points: 1, Status: models.CardStatusUnused}
	db.Create(&card)
	res, err := CardLogin("APP-1", "KM-PC1", "MC-PC1", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("first login: %v", err)
	}
	// 首次激活扣光 1 点
	first := mustActivate(t, "APP-1", res.Token, "MC-PC1")
	if first.Points != 0 {
		t.Fatalf("first activation should consume the only point, got %v", first.Points)
	}
	// 余额 0 时换一台设备再激活 → 点数不足被拒
	if _, err := activateForTest(t, "APP-1", res.Token, "MC-PC2"); err == nil {
		t.Fatalf("activation with 0 points should fail")
	}
	// 同设备重复激活则是幂等复用，不扣费也不报错
	if again, err := activateForTest(t, "APP-1", res.Token, "MC-PC1"); err != nil || !again.Reused {
		t.Fatalf("re-activation on same fingerprint should be reused, err=%v", err)
	}
}

func TestPointsPerTimeMode(t *testing.T) {
	db := setupPublicTestDB(t)
	// 点数模式 + 按时：每 60 分钟扣 1 点
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"billing_mode":       models.BillingModePoints,
			"bill_points_enabled": 1,
			"bill_time_enabled":   0,
		"points_charge_mode":    models.PointsChargePerTime,
		"points_per_period":     1,
		"points_period_minutes": 60,
	})
	card := models.Card{CardNo: "KM-PT", Points: 2, Status: models.CardStatusUnused}
	db.Create(&card)

	// 登录 → 不扣点（身份认证）
	res, err := CardLogin("APP-1", "KM-PT", "MC-PT", "1.2.3.4", "1.0.0", "")
	if err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	if res.Points != 2 {
		t.Fatalf("login should not charge, want 2 points, got %v", res.Points)
	}

	// 激活（FROM_CREATION）→ 预扣一个周期：余 1 点，周期结束在约 60 分钟后
	actRes := mustActivate(t, "APP-1", res.Token, "MC-PT")
	if actRes.Points != 1 {
		t.Fatalf("activation should pre-bill 1 point (2->1), got %v", actRes.Points)
	}
	m := loadMemberByToken(t, db, res.Token)
	act := loadActivation(t, db, actRes.Token)
	if act.PeriodEnd == nil || act.PeriodEnd.Before(time.Now().Add(59*time.Minute)) {
		t.Fatalf("paid window should be ~60min ahead on activation, got %v", act.PeriodEnd)
	}

	// 心跳仍在窗口内 → 不再扣点
	if st, _ := CheckMemberStatus("APP-1", actRes.Token); st.Points != 1 {
		t.Fatalf("heartbeat within window should not deduct, got %v", st.Points)
	}
	// 周期已过：心跳按应用配置结算，续扣 1 点 → 0
	db.Model(&models.AppActivation{}).Where("token = ?", actRes.Token).
		Update("period_end", time.Now().Add(-time.Minute))
	st, err := CheckMemberStatus("APP-1", actRes.Token)
	if err != nil {
		t.Fatalf("renew heartbeat: %v", err)
	}
	if st.Points != 0 {
		t.Fatalf("expired window should renew and deduct to 0, got %v", st.Points)
	}
	_ = m
	_ = m

	// 再次周期过期且余额 0 → 默认心跳续费失败，激活被置死
	db.Model(&models.AppActivation{}).Where("token = ?", actRes.Token).
		Update("period_end", time.Now().Add(-time.Minute))
	if _, err := CheckMemberStatus("APP-1", actRes.Token); err == nil {
		t.Fatalf("should be unusable when window expired and points exhausted")
	}
	// 续费失败后激活已置死 → 下次心跳直接报授权失效
	if _, err := CheckMemberStatus("APP-1", actRes.Token); err == nil {
		t.Fatalf("activation should stay dead after billing failure")
	}
}

func loadMemberByToken(t *testing.T, db *gorm.DB, token string) models.Member {
	t.Helper()
	// 二级令牌：先按激活令牌找，再按账号令牌找，最后回退旧会话（兼容残留用例）
	var memberUUID string
	var act models.AppActivation
	if err := db.Where("token = ?", token).First(&act).Error; err == nil {
		memberUUID = act.MemberUUID
	} else {
		var acct models.MemberAccountToken
		if err := db.Where("token = ?", token).First(&acct).Error; err == nil {
			memberUUID = acct.MemberUUID
		} else {
			var session models.AppActivation
			if err := db.Where("token = ?", token).First(&session).Error; err != nil {
				t.Fatalf("load token %s: %v", token, err)
			}
			memberUUID = session.MemberUUID
		}
	}
	var m models.Member
	if err := db.Where("uuid = ?", memberUUID).First(&m).Error; err != nil {
		t.Fatalf("load member: %v", err)
	}
	return m
}

func TestAccountRegisterDisabled(t *testing.T) {
	setupPublicTestDB(t)
	// 显式关闭注册（全局账号策略）
	setPolicy(t, map[string]interface{}{"register_enabled": 0})
	if _, err := AccountRegister("APP-1", "bob@test.com", "pw", "", "", "1.2.3.4", ""); err == nil {
		t.Fatalf("register should be rejected when disabled")
	}
}

func TestManualTopLoginKickSession(t *testing.T) {
	db := setupPublicTestDB(t)
	setMachineVerify(db, 1)
	setTestQuota(db, 20, 1, 20)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("machine_verify", 1)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("login_type", models.LoginTypeManual)
	card := models.Card{CardNo: "KM-MANUAL", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)

	// 第一次「登录 + 激活」占满唯一名额
	loginA, err := CardLogin("APP-1", "KM-MANUAL", "MC-A", "1.1.1.1", "1.0.0", "PC-A")
	if err != nil {
		t.Fatalf("first login: %v", err)
	}
	mustActivate(t, "APP-1", loginA.Token, "MC-A")

	// 第二次激活（不同设备）→ 手动顶号满员，返回激活列表
	loginB, err := CardLogin("APP-1", "KM-MANUAL", "MC-B", "1.1.1.2", "1.0.0", "PC-B")
	if err != nil {
		t.Fatalf("second login should succeed (identity only): %v", err)
	}
	_, err = activateForTest(t, "APP-1", loginB.Token, "MC-B")
	var sfe *SessionsFullError
	if !errors.As(err, &sfe) {
		t.Fatalf("second activation should return SessionsFullError, got %v", err)
	}
	if len(sfe.Sessions) != 1 {
		t.Fatalf("expected 1 online activation in list, got %v", len(sfe.Sessions))
	}
	kickID := sfe.Sessions[0].ID

	// 用另一个账号的凭据去踢不属于它的激活应失败（防越权）
	otherCard := models.Card{CardNo: "KM-OTHER", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&otherCard)
	otherLogin, err := CardLogin("APP-1", "KM-OTHER", "MC-C", "1.1.1.3", "1.0.0", "PC-C")
	if err != nil {
		t.Fatalf("other login: %v", err)
	}
	mustActivate(t, "APP-1", otherLogin.Token, "MC-C")
	if err := KickSession("APP-1", "KM-OTHER", "", kickID, "1.1.1.3"); err == nil {
		t.Fatalf("should not kick another account's activation")
	}

	// 凭本人卡号踢掉指定激活
	if err := KickSession("APP-1", "KM-MANUAL", "", kickID, "1.1.1.1"); err != nil {
		t.Fatalf("kick own activation: %v", err)
	}

	// 踢掉后重新激活应成功
	if _, err := activateForTest(t, "APP-1", loginB.Token, "MC-B"); err != nil {
		t.Fatalf("reactivation after kick: %v", err)
	}
}
// TestCheckVersionByPlugin 插件自检更新：必须比「插件自己的版本」，而不是宿主应用的版本。
//
// 回归点：type 3 曾经只认请求签名对应的应用，插件把自身版本塞进去等于和宿主比大小，
// 后台给插件配的版本号完全读不到，导致「版本相同/更高却提示更新」。
func TestCheckVersionByPlugin(t *testing.T) {
	db := setupPublicTestDB(t)

	// 宿主应用：3.0.1，强制更新
	seedAppVersion(t, db, "APP-1", "3.0.1", models.DownloadTypeForce, "http://x/host", "宿主更新", true)

	// 插件应用：com.example.esp，最高版本 2.1.0，先设成「不启用更新」
	pluginID := "com.example.esp"
	pluginApp := models.App{
		UUID: "APP-PLUGIN", Name: "演示插件", Secret: "SECRET-P", Status: 1, PluginID: &pluginID,
	}
	if err := db.Create(&pluginApp).Error; err != nil {
		t.Fatalf("seed plugin app: %v", err)
	}
	seedAppVersion(t, db, pluginApp.UUID, "2.1.0", models.DownloadTypeDisabled, "", "插件更新", true)

	// ① 不传 plugin_id：沿用旧语义，比宿主应用（3.0.1）→ 2.1.0 必须提示更新（就是旧 bug 的表现）
	legacy, err := CheckVersion("APP-1", "2.1.0")
	if err != nil {
		t.Fatalf("CheckVersion: %v", err)
	}
	if need := legacy.(map[string]any)["need_update"].(bool); !need {
		t.Fatalf("legacy check should compare against host app (3.0.1) and need update")
	}

	// ② 传 plugin_id：比插件自身（2.1.0）→ 相同版本不需更新
	byPlugin, err := CheckVersionByPlugin(pluginID, "2.1.0")
	if err != nil {
		t.Fatalf("CheckVersionByPlugin: %v", err)
	}
	m := byPlugin.(map[string]any)
	if m["plugin_found"] != true {
		t.Fatalf("plugin should be found: %+v", m)
	}
	if m["latest_version"] != "2.1.0" {
		t.Fatalf("latest_version should be plugin's 2.1.0, got %v", m["latest_version"])
	}
	if m["need_update"].(bool) {
		t.Fatalf("same version must not need update: %+v", m)
	}

	// ③ 低版本插件客户端：dt=0（不启用更新）→ 仍不提示
	low, _ := CheckVersionByPlugin(pluginID, "1.0.0")
	if low.(map[string]any)["need_update"].(bool) {
		t.Fatalf("download_type=0 must not ask for update: %+v", low)
	}

	// ④ 改成自由更新 → 低版本提示，最新版不提示
	db.Model(&models.AppVersion{}).Where("app_uuid = ?", pluginApp.UUID).
		Update("download_type", models.DownloadTypeFree)
	free, _ := CheckVersionByPlugin(pluginID, "1.0.0")
	fm := free.(map[string]any)
	if !fm["need_update"].(bool) || fm["download_type"] != models.DownloadTypeFree ||
		fm["download_url"] != "" || fm["update_announcement"] != "插件更新" {
		t.Fatalf("free update payload unexpected: %+v", fm)
	}
	same, _ := CheckVersionByPlugin(pluginID, "2.1.0")
	if same.(map[string]any)["need_update"].(bool) {
		t.Fatalf("up-to-date plugin must not need update: %+v", same)
	}

	// ⑤ 后台没登记这个插件 → 正常的「无更新」结果，不报错
	unknown, err := CheckVersionByPlugin("com.not.registered", "1.0.0")
	if err != nil {
		t.Fatalf("unknown plugin should not error: %v", err)
	}
	um := unknown.(map[string]any)
	if um["plugin_found"] != false || um["need_update"].(bool) || um["download_type"] != models.DownloadTypeDisabled {
		t.Fatalf("unknown plugin should return no-update payload: %+v", um)
	}
}