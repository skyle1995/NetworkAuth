package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/utils/encrypt"
	b64 "encoding/base64"
	"encoding/hex"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupPublicTestDB 建库并植入一个已启用、开启卡密登录的应用
func setupPublicTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.App{}, &models.Card{}, &models.Member{}, &models.Binding{}, &models.API{}, &models.Variable{}, &models.Function{}, &models.MemberSession{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	app := models.App{
		UUID:             "APP-1",
		Name:             "测试应用",
		Secret:           "SECRET",
		Status:           1,
		CardLoginEnabled: 1,
		MultiOpenCount:   1,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("seed app: %v", err)
	}
	database.SetDB(db)
	return db
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
		CardNo: "KM-TESTCARD", AppUUID: "APP-1",
		Duration: 24 * 60, Status: models.CardStatusUnused,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}

	// 首次登录 → 激活并创建卡密账号
	res, err := CardLogin("APP-1", "KM-TESTCARD", "", "1.2.3.4")
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
		t.Fatalf("expected 1 member created, got %d", memberCount)
	}

	// 心跳应通过
	if _, err := CheckMemberStatus("APP-1", res.Token, true); err != nil {
		t.Fatalf("status check should pass: %v", err)
	}

	// 再次登录（已使用卡）→ 顶号：旧令牌失效，新令牌有效
	res2, err := CardLogin("APP-1", "KM-TESTCARD", "", "1.2.3.4")
	if err != nil {
		t.Fatalf("second CardLogin: %v", err)
	}
	if res2.Token == res.Token {
		t.Fatalf("re-login should issue a new token")
	}
	if _, err := CheckMemberStatus("APP-1", res.Token, true); err == nil {
		t.Fatalf("old token should be invalidated after re-login")
	}
	if _, err := CheckMemberStatus("APP-1", res2.Token, true); err != nil {
		t.Fatalf("new token should be valid: %v", err)
	}

	// 登出 → 令牌清空，心跳失败
	if err := MemberLogout("APP-1", res2.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := CheckMemberStatus("APP-1", res2.Token, true); err == nil {
		t.Fatalf("status check should fail after logout")
	}
}

func TestCardLoginFrozenRejected(t *testing.T) {
	db := setupPublicTestDB(t)
	card := models.Card{
		CardNo: "KM-FROZEN", AppUUID: "APP-1",
		Duration: 60, Status: models.CardStatusFrozen,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}
	if _, err := CardLogin("APP-1", "KM-FROZEN", "", "1.2.3.4"); err == nil {
		t.Fatalf("frozen card login should be rejected")
	}
}

func TestAccountRegisterLoginRecharge(t *testing.T) {
	db := setupPublicTestDB(t)
	// 开启注册与充值
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").
		Updates(map[string]interface{}{"register_enabled": 1, "recharge_enabled": 1})

	// 注册（无试用 → 注册即过期，不返回令牌）
	reg, err := AccountRegister("APP-1", "alice@test.com", "secret1", "", "1.2.3.4", "")
	if err != nil {
		t.Fatalf("AccountRegister: %v", err)
	}
	if reg.Username != "alice@test.com" {
		t.Fatalf("unexpected register result: %+v", reg)
	}
	// 重复注册应失败
	if _, err := AccountRegister("APP-1", "alice@test.com", "x", "", "1.2.3.4", ""); err == nil {
		t.Fatalf("duplicate register should fail")
	}
	// 未充值（已过期）登录应失败
	if _, err := AccountLogin("APP-1", "alice@test.com", "secret1", "", "1.2.3.4"); err == nil {
		t.Fatalf("login should fail before recharge (expired)")
	}

	// 用一张卡为账号充值 30 天
	card := models.Card{CardNo: "KM-RC", AppUUID: "APP-1", Duration: 30 * 24 * 60, Status: models.CardStatusUnused}
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
	if _, err := AccountLogin("APP-1", "alice@test.com", "wrong", "", "1.2.3.4"); err == nil {
		t.Fatalf("login with wrong password should fail")
	}
	// 充值后正确密码登录成功
	login, err := AccountLogin("APP-1", "alice@test.com", "secret1", "", "1.2.3.4")
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

func TestRegisterLimitByIP(t *testing.T) {
	db := setupPublicTestDB(t)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"register_enabled":       1,
		"register_limit_enabled": 1,
		"register_limit_time":    1,
		"register_count":         1,
	})

	if _, err := AccountRegister("APP-1", "limit1@test.com", "pw123456", "", "8.8.8.8", ""); err != nil {
		t.Fatalf("first register should pass: %v", err)
	}
	if _, err := AccountRegister("APP-1", "limit2@test.com", "pw123456", "", "8.8.8.8", ""); err == nil {
		t.Fatalf("second register from same IP should be rejected")
	}
	if _, err := AccountRegister("APP-1", "limit3@test.com", "pw123456", "", "8.8.4.4", ""); err != nil {
		t.Fatalf("register from another IP should pass: %v", err)
	}
}

func TestRegisterLimitByDevice(t *testing.T) {
	db := setupPublicTestDB(t)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"register_enabled":              1,
		"register_device_limit_enabled": 1,
		"register_limit_time":           1,
		"register_count":                1,
	})

	// 开启设备限制但未提交设备码 → 拒绝
	if _, err := AccountRegister("APP-1", "d0@test.com", "pw123456", "", "1.1.1.1", ""); err == nil {
		t.Fatalf("register without machine code should be rejected when device limit on")
	}
	// 同一设备第一次通过、第二次被拦（不同 IP 也拦，证明是按设备而非 IP）
	if _, err := AccountRegister("APP-1", "d1@test.com", "pw123456", "", "1.1.1.1", "MC-AAA"); err != nil {
		t.Fatalf("first register on device should pass: %v", err)
	}
	if _, err := AccountRegister("APP-1", "d2@test.com", "pw123456", "", "9.9.9.9", "MC-AAA"); err == nil {
		t.Fatalf("second register on same device (different IP) should be rejected")
	}
	// 换设备可继续
	if _, err := AccountRegister("APP-1", "d3@test.com", "pw123456", "", "1.1.1.1", "MC-BBB"); err != nil {
		t.Fatalf("register on another device should pass: %v", err)
	}
}

func TestClaimTrialLimits(t *testing.T) {
	db := setupPublicTestDB(t)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"register_enabled": 1,
		"trial_enabled":    1,
		"trial_limit_time": 1,
		"trial_duration":   60,
	})

	reg, err := AccountRegister("APP-1", "trial@test.com", "pw123456", "", "1.2.3.4", "")
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

func TestDataInterfacesAndChangePassword(t *testing.T) {
	db := setupPublicTestDB(t)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").
		Updates(map[string]interface{}{
			"register_enabled": 1, "recharge_enabled": 1,
			"app_data": base64Encode("APPDATA-XYZ"),
		})

	// 植入应用变量与全局函数
	if err := db.Create(&models.Variable{Alias: "server_url", AppUUID: "APP-1", Data: "https://x"}).Error; err != nil {
		t.Fatalf("seed variable: %v", err)
	}
	if err := db.Create(&models.Function{Alias: "checkVip", AppUUID: "0", Code: "return true"}).Error; err != nil {
		t.Fatalf("seed function: %v", err)
	}

	// 注册 + 充值 + 登录，拿到有效令牌
	if _, err := AccountRegister("APP-1", "carl@test.com", "pw123456", "", "1.2.3.4", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	card := models.Card{CardNo: "KM-DATA", AppUUID: "APP-1", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	if _, err := RechargeByCard("APP-1", "carl@test.com", "KM-DATA"); err != nil {
		t.Fatalf("recharge: %v", err)
	}
	login, err := AccountLogin("APP-1", "carl@test.com", "pw123456", "", "1.2.3.4")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	token := login.Token

	// 程序数据
	if _, err := GetAppData("APP-1", token); err != nil {
		t.Fatalf("GetAppData: %v", err)
	}
	// 变量
	if _, err := GetVariable("APP-1", token, "server_url"); err != nil {
		t.Fatalf("GetVariable: %v", err)
	}
	// 全局函数
	if _, err := GetFunction("APP-1", token, "checkVip"); err != nil {
		t.Fatalf("GetFunction: %v", err)
	}
	// 不存在的变量
	if _, err := GetVariable("APP-1", token, "nope"); err == nil {
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
	// 改密后旧令牌失效
	if _, err := GetAppData("APP-1", token); err == nil {
		t.Fatalf("token should be invalidated after password change")
	}
	// 用新密码可再次登录
	if _, err := AccountLogin("APP-1", "carl@test.com", "newpass1", "", "1.2.3.4"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}

func TestChangePasswordRejectedForCardAccount(t *testing.T) {
	db := setupPublicTestDB(t)
	card := models.Card{CardNo: "KM-CARDPWD", AppUUID: "APP-1", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	login, err := CardLogin("APP-1", "KM-CARDPWD", "", "1.2.3.4")
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
	// 开启机器验证与机器码转绑：永久限制、免费 1 次、每次扣 60 分钟
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"machine_verify":         1,
		"machine_rebind_enabled": 1,
		"machine_rebind_limit":   1, // 永久
		"machine_free_count":     1,
		"machine_rebind_count":   2, // 最多 2 次
		"machine_rebind_deduct":  60,
	})

	// 卡密登录（带机器码），账号有 10 天
	card := models.Card{CardNo: "KM-RB", AppUUID: "APP-1", Duration: 10 * 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	login, err := CardLogin("APP-1", "KM-RB", "MC-OLD", "1.2.3.4")
	if err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	before := loadMemberByToken(t, db, login.Token).ExpiredAt

	// 第 1 次转绑（免费，不扣时），绑定应替换为 MC-1（卡密账号用卡号鉴权，无需令牌）
	if _, err := Rebind("APP-1", "KM-RB", "", "MC-1", "1.2.3.4"); err != nil {
		t.Fatalf("rebind 1: %v", err)
	}
	m := loadMemberByToken(t, db, login.Token)
	if m.MachineRebindUsed != 1 {
		t.Fatalf("used should be 1, got %d", m.MachineRebindUsed)
	}
	if !m.ExpiredAt.Equal(before) {
		t.Fatalf("free rebind should not deduct time")
	}
	var bindCount int64
	db.Model(&models.Binding{}).Where("member_uuid = ? AND type = ?", m.UUID, models.BindingTypeMachine).Count(&bindCount)
	if bindCount != 1 {
		t.Fatalf("expected single machine binding after rebind, got %d", bindCount)
	}

	// 第 2 次转绑（超免费 → 扣 60 分钟）
	if _, err := Rebind("APP-1", "KM-RB", "", "MC-2", "1.2.3.4"); err != nil {
		t.Fatalf("rebind 2: %v", err)
	}
	m2 := loadMemberByToken(t, db, login.Token)
	if diff := before.Sub(m2.ExpiredAt) - time.Hour; diff > time.Second || diff < -time.Second {
		t.Fatalf("second rebind should deduct 60min, diff %v", diff)
	}

	// 第 3 次转绑 → 超过 max(2) 次上限，拒绝
	if _, err := Rebind("APP-1", "KM-RB", "", "MC-3", "1.2.3.4"); err == nil {
		t.Fatalf("third rebind should exceed limit")
	}
}

func TestRebindSameDeviceIsNoOp(t *testing.T) {
	db := setupPublicTestDB(t)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"machine_verify":         1,
		"machine_rebind_enabled": 1,
		"machine_rebind_limit":   1, // 永久
		"machine_free_count":     0, // 无免费次数：若真转绑必扣时
		"machine_rebind_count":   5,
		"machine_rebind_deduct":  60,
	})
	card := models.Card{CardNo: "KM-SAME", AppUUID: "APP-1", Duration: 10 * 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	if _, err := CardLogin("APP-1", "KM-SAME", "MC-A", "1.2.3.4"); err != nil {
		t.Fatalf("CardLogin: %v", err)
	}

	load := func() models.Member {
		var m models.Member
		db.Where("app_uuid = ? AND username = ?", "APP-1", "KM-SAME").First(&m)
		return m
	}
	before := load().ExpiredAt

	// 换绑到同一设备 MC-A → 幂等 no-op：不计次、不扣时
	if _, err := Rebind("APP-1", "KM-SAME", "", "MC-A", "1.2.3.4"); err != nil {
		t.Fatalf("rebind same device: %v", err)
	}
	if m := load(); m.MachineRebindUsed != 0 {
		t.Fatalf("same-device rebind should not consume count, got %d", m.MachineRebindUsed)
	}
	if m := load(); !m.ExpiredAt.Equal(before) {
		t.Fatalf("same-device rebind should not deduct time")
	}

	// 换绑到新设备 MC-B → 正常计次并扣时
	if _, err := Rebind("APP-1", "KM-SAME", "", "MC-B", "1.2.3.4"); err != nil {
		t.Fatalf("rebind new device: %v", err)
	}
	if m := load(); m.MachineRebindUsed != 1 {
		t.Fatalf("new-device rebind should count once, got %d", m.MachineRebindUsed)
	}
}

func TestIPVerifyBindingAndRebind(t *testing.T) {
	db := setupPublicTestDB(t)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"ip_verify":         1,
		"ip_rebind_enabled": 1,
		"ip_rebind_limit":   1,
		"ip_free_count":     1,
		"ip_rebind_count":   2,
	})

	card := models.Card{CardNo: "KM-IP", AppUUID: "APP-1", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	if _, err := CardLogin("APP-1", "KM-IP", "", "10.0.0.1"); err != nil {
		t.Fatalf("first IP login should bind IP: %v", err)
	}
	if _, err := CardLogin("APP-1", "KM-IP", "", "10.0.0.2"); err == nil {
		t.Fatalf("login from unbound IP should be rejected")
	}
	// 死循环验证：新 IP 登录被拒，但凭卡号转绑无需令牌，转绑后即可登录
	if _, err := Rebind("APP-1", "KM-IP", "", "", "10.0.0.2"); err != nil {
		t.Fatalf("Rebind IP: %v", err)
	}
	if _, err := CardLogin("APP-1", "KM-IP", "", "10.0.0.2"); err != nil {
		t.Fatalf("login from rebound IP should pass: %v", err)
	}
	if _, err := CardLogin("APP-1", "KM-IP", "", "10.0.0.1"); err == nil {
		t.Fatalf("old IP should be rejected after rebind")
	}
}

func TestRebindDisabledRejected(t *testing.T) {
	db := setupPublicTestDB(t)
	card := models.Card{CardNo: "KM-NOREBIND", AppUUID: "APP-1", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	if _, err := CardLogin("APP-1", "KM-NOREBIND", "", "1.2.3.4"); err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	if _, err := Rebind("APP-1", "KM-NOREBIND", "", "MC-X", "1.2.3.4"); err == nil {
		t.Fatalf("rebind should be rejected when disabled")
	}
}

func TestVersionAndCardInfo(t *testing.T) {
	db := setupPublicTestDB(t)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"version": "1.2.0", "download_url": "https://dl", "download_type": 1,
	})

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
	card := models.Card{CardNo: "KM-INFO", AppUUID: "APP-1", Duration: 60, Status: models.CardStatusUnused}
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
		AppUUID: "APP-1",
		Code:    "return { sum: params.a + params.b, vip: params.a > 10 };",
	}).Error; err != nil {
		t.Fatalf("seed calc: %v", err)
	}
	// Function.Number 为毫秒时间戳且唯一，隔开 2ms 避免同毫秒冲突
	time.Sleep(2 * time.Millisecond)
	// 死循环函数：验证超时中断
	if err := db.Create(&models.Function{
		Alias:   "loop",
		AppUUID: "APP-1",
		Code:    "while(true){}",
	}).Error; err != nil {
		t.Fatalf("seed loop: %v", err)
	}

	// 登录拿有效令牌
	card := models.Card{CardNo: "KM-FN", AppUUID: "APP-1", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	login, err := CardLogin("APP-1", "KM-FN", "", "1.2.3.4")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// 执行 calc(a=20,b=5) → sum=25, vip=true
	res, err := ExecuteFunction("APP-1", login.Token, "calc",
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
		AppUUID: "APP-1",
		Code: "var u=getUser(); var a=getApp();" +
			"return { name: u.username, app: a.uuid, leaked: (a.secret!==undefined)||(u.password!==undefined) };",
	}).Error; err != nil {
		t.Fatalf("seed whoami: %v", err)
	}

	card := models.Card{CardNo: "KM-WHO", AppUUID: "APP-1", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	login, err := CardLogin("APP-1", "KM-WHO", "", "9.9.9.9")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	res, err := ExecuteFunction("APP-1", login.Token, "whoami", nil)
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
	// 失效会话（超过 OfflineTimeout=10 分钟未心跳）
	db.Create(&models.MemberSession{
		Token: "stale", MemberUUID: "m1", AppUUID: "APP-1",
		LastActiveAt: now.Add(-20 * time.Minute),
	})
	// 活跃会话
	db.Create(&models.MemberSession{
		Token: "fresh", MemberUUID: "m2", AppUUID: "APP-1", LastActiveAt: now,
	})
	// 孤儿会话（应用不存在）
	db.Create(&models.MemberSession{
		Token: "orphan", MemberUUID: "m3", AppUUID: "GONE-APP", LastActiveAt: now,
	})

	sweepSessions()

	exists := func(token string) bool {
		var n int64
		db.Model(&models.MemberSession{}).Where("token = ?", token).Count(&n)
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
	// 多开 2，非顶号
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").
		Updates(map[string]interface{}{"multi_open_count": 2, "login_type": 1})
	card := models.Card{CardNo: "KM-MO", AppUUID: "APP-1", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)

	sessionCount := func() int64 {
		var n int64
		db.Model(&models.MemberSession{}).Count(&n)
		return n
	}

	// 前两次登录成功（2 个会话）
	if _, err := CardLogin("APP-1", "KM-MO", "", "1.1.1.1"); err != nil {
		t.Fatalf("login1: %v", err)
	}
	if _, err := CardLogin("APP-1", "KM-MO", "", "1.1.1.2"); err != nil {
		t.Fatalf("login2: %v", err)
	}
	if sessionCount() != 2 {
		t.Fatalf("expected 2 sessions, got %d", sessionCount())
	}
	// 第三次（非顶号）应被拒，会话数不变
	if _, err := CardLogin("APP-1", "KM-MO", "", "1.1.1.3"); err == nil {
		t.Fatalf("third login should be rejected (non-preempt, limit 2)")
	}
	if sessionCount() != 2 {
		t.Fatalf("rejected login should not create a session, got %d", sessionCount())
	}

	// 切换为顶号：新登录成功且会话数仍为 2（踢掉了最早的一个）
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("login_type", 0)
	l4, err := CardLogin("APP-1", "KM-MO", "", "1.1.1.4")
	if err != nil {
		t.Fatalf("preempt login should succeed: %v", err)
	}
	if sessionCount() != 2 {
		t.Fatalf("preemption should keep session count at limit, got %d", sessionCount())
	}
	// 新会话有效
	if _, err := CheckMemberStatus("APP-1", l4.Token, true); err != nil {
		t.Fatalf("new session should be valid: %v", err)
	}
}

func TestMultiOpenScopeMachine(t *testing.T) {
	db := setupPublicTestDB(t)
	// 单设备范围 + 多开1 + 非顶号
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"multi_open_scope": models.MultiOpenScopeMachine,
		"multi_open_count": 1,
		"login_type":       1,
	})
	card := models.Card{CardNo: "KM-MS", AppUUID: "APP-1", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)

	sessCount := func() int64 {
		var n int64
		db.Model(&models.MemberSession{}).Count(&n)
		return n
	}

	// 机器 A 登录 → 1 个开
	if _, err := CardLogin("APP-1", "KM-MS", "MC-A", "1.1.1.1"); err != nil {
		t.Fatalf("login A: %v", err)
	}
	if sessCount() != 1 {
		t.Fatalf("expected 1 session, got %d", sessCount())
	}
	// 同机器 A 再登录 → 仍是同一个开，会话数不增
	if _, err := CardLogin("APP-1", "KM-MS", "MC-A", "2.2.2.2"); err != nil {
		t.Fatalf("re-login A: %v", err)
	}
	if sessCount() != 1 {
		t.Fatalf("same machine re-login should stay 1 session, got %d", sessCount())
	}
	// 不同机器 B（非顶号）→ 超出，拒绝
	if _, err := CardLogin("APP-1", "KM-MS", "MC-B", "3.3.3.3"); err == nil {
		t.Fatalf("second machine should be rejected (non-preempt, scope=machine, count=1)")
	}

	// 切顶号 → 机器 B 登录成功，踢掉机器 A
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("login_type", 0)
	if _, err := CardLogin("APP-1", "KM-MS", "MC-B", "3.3.3.3"); err != nil {
		t.Fatalf("preempt login B: %v", err)
	}
	if sessCount() != 1 {
		t.Fatalf("preemption should keep 1 open, got %d sessions", sessCount())
	}
	var s models.MemberSession
	db.First(&s)
	if s.MachineCode != "MC-B" {
		t.Fatalf("surviving open should be MC-B, got %s", s.MachineCode)
	}
}

func TestRiskControl(t *testing.T) {
	db := setupPublicTestDB(t)
	card := models.Card{CardNo: "KM-RISK", AppUUID: "APP-1", Duration: 10 * 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	login, err := CardLogin("APP-1", "KM-RISK", "", "1.2.3.4")
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
	if _, err := CheckMemberStatus("APP-1", login.Token, true); err == nil {
		t.Fatalf("session should be killed after disable")
	}
	var m models.Member
	db.Where("username = ?", "KM-RISK").First(&m)
	if m.Status != models.MemberStatusDisabled {
		t.Fatalf("member should be disabled, got status %d", m.Status)
	}

	// 拉黑
	if _, err := RiskBlacklistMember("APP-1", "KM-RISK"); err != nil {
		t.Fatalf("RiskBlacklistMember: %v", err)
	}
	db.Where("username = ?", "KM-RISK").First(&m)
	if m.Status != models.MemberStatusBlack {
		t.Fatalf("member should be blacklisted, got status %d", m.Status)
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
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("register_enabled", 1)

	// 非法邮箱注册被拒
	if _, err := AccountRegister("APP-1", "notanemail", "pw123456", "", "1.2.3.4", ""); err == nil {
		t.Fatalf("register with invalid email should fail")
	}

	// 开启邮箱验证后，无有效验证码注册应失败（测试环境无 Redis → 验证码服务不可用亦属拒绝）
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("email_verify_enabled", 1)
	if _, err := AccountRegister("APP-1", "eve@test.com", "pw123456", "000000", "1.2.3.4", ""); err == nil {
		t.Fatalf("register should fail without valid email code when verification enabled")
	}

	// 关闭验证：邮箱注册账号 username=email 且 Email 字段落库
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("email_verify_enabled", 0)
	if _, err := AccountRegister("APP-1", "frank@test.com", "pw123456", "", "1.2.3.4", ""); err != nil {
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
		"operation_mode":     models.OperationModePoints,
		"points_charge_mode": models.PointsChargePerCount,
		"points_per_login":   1,
		"recharge_enabled":   1,
	})
	// 面值 3 点的卡
	card := models.Card{CardNo: "KM-PC", AppUUID: "APP-1", Points: 3, Status: models.CardStatusUnused}
	db.Create(&card)

	// 卡密登录 → 激活 3 点，登录扣 1 → 余 2
	res, err := CardLogin("APP-1", "KM-PC", "", "1.2.3.4")
	if err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	if res.Mode != models.OperationModePoints || res.Points != 2 {
		t.Fatalf("want points mode with 2 points, got mode=%d points=%d", res.Mode, res.Points)
	}

	// 显式功能扣点：扣 1 → 余 1
	st, err := DeductPoints("APP-1", res.Token, 1)
	if err != nil {
		t.Fatalf("DeductPoints: %v", err)
	}
	if st.Points != 1 {
		t.Fatalf("want 1 point after deduct, got %d", st.Points)
	}
	// 扣点超额 → 拒绝
	if _, err := DeductPoints("APP-1", res.Token, 5); err == nil {
		t.Fatalf("over-deduct should fail")
	}

	// 充值：加 10 点 → 余 11
	rc := models.Card{CardNo: "KM-PC-RC", AppUUID: "APP-1", Points: 10, Status: models.CardStatusUnused}
	db.Create(&rc)
	rst, err := RechargeByCard("APP-1", "KM-PC", "KM-PC-RC")
	if err != nil {
		t.Fatalf("recharge: %v", err)
	}
	if rst.Points != 11 {
		t.Fatalf("want 11 points after recharge, got %d", rst.Points)
	}
}

func TestPointsPerCountLoginRejectedWhenEmpty(t *testing.T) {
	db := setupPublicTestDB(t)
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"operation_mode":     models.OperationModePoints,
		"points_charge_mode": models.PointsChargePerCount,
		"points_per_login":   1,
	})
	// 1 点卡：首登扣光
	card := models.Card{CardNo: "KM-PC1", AppUUID: "APP-1", Points: 1, Status: models.CardStatusUnused}
	db.Create(&card)
	res, err := CardLogin("APP-1", "KM-PC1", "", "1.2.3.4")
	if err != nil {
		t.Fatalf("first login: %v", err)
	}
	_ = MemberLogout("APP-1", res.Token)
	// 余额 0 再登录 → 点数不足
	if _, err := CardLogin("APP-1", "KM-PC1", "", "1.2.3.4"); err == nil {
		t.Fatalf("login with 0 points should fail")
	}
}

func TestPointsPerTimeMode(t *testing.T) {
	db := setupPublicTestDB(t)
	// 点数模式 + 按时：每 60 分钟扣 1 点
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Updates(map[string]interface{}{
		"operation_mode":        models.OperationModePoints,
		"points_charge_mode":    models.PointsChargePerTime,
		"points_per_period":     1,
		"points_period_minutes": 60,
	})
	card := models.Card{CardNo: "KM-PT", AppUUID: "APP-1", Points: 2, Status: models.CardStatusUnused}
	db.Create(&card)

	// 登录 → 预扣一个周期：余 1 点，到期在约 60 分钟后
	res, err := CardLogin("APP-1", "KM-PT", "", "1.2.3.4")
	if err != nil {
		t.Fatalf("CardLogin: %v", err)
	}
	if res.Points != 1 {
		t.Fatalf("first login should pre-bill 1 point (2->1), got %d", res.Points)
	}
	m := loadMemberByToken(t, db, res.Token)
	if m.ExpiredAt.Before(time.Now().Add(59 * time.Minute)) {
		t.Fatalf("paid window should be ~60min ahead, got %v", m.ExpiredAt)
	}

	// 心跳仍在窗口内 → 不再扣点
	if st, _ := CheckMemberStatus("APP-1", res.Token, true); st.Points != 1 {
		t.Fatalf("heartbeat within window should not deduct, got %d", st.Points)
	}

	// 模拟窗口已过：把到期时间拨到过去，心跳应续扣 1 点 → 0
	db.Model(&models.Member{}).Where("uuid = ?", m.UUID).
		Update("expired_at", time.Now().Add(-time.Minute))
	st, err := CheckMemberStatus("APP-1", res.Token, true)
	if err != nil {
		t.Fatalf("renew heartbeat: %v", err)
	}
	if st.Points != 0 {
		t.Fatalf("expired window should renew and deduct to 0, got %d", st.Points)
	}

	// 再次窗口过期且余额 0 → 不可用
	db.Model(&models.Member{}).Where("uuid = ?", m.UUID).
		Update("expired_at", time.Now().Add(-time.Minute))
	if _, err := CheckMemberStatus("APP-1", res.Token, true); err == nil {
		t.Fatalf("should be unusable when window expired and points exhausted")
	}
}

func loadMemberByToken(t *testing.T, db *gorm.DB, token string) models.Member {
	t.Helper()
	var session models.MemberSession
	if err := db.Where("token = ?", token).First(&session).Error; err != nil {
		t.Fatalf("load session by token: %v", err)
	}
	var m models.Member
	if err := db.Where("uuid = ?", session.MemberUUID).First(&m).Error; err != nil {
		t.Fatalf("load member: %v", err)
	}
	return m
}

func TestAccountRegisterDisabled(t *testing.T) {
	db := setupPublicTestDB(t)
	// 显式关闭注册（App.RegisterEnabled 带 default:1，需强制置 0）
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("register_enabled", 0)
	if _, err := AccountRegister("APP-1", "bob@test.com", "pw", "", "1.2.3.4", ""); err == nil {
		t.Fatalf("register should be rejected when disabled")
	}
}
