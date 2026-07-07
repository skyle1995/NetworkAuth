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
	if err := db.AutoMigrate(&models.App{}, &models.Card{}, &models.Member{}, &models.Binding{}, &models.API{}, &models.Variable{}, &models.Function{}); err != nil {
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
	if _, err := CheckMemberStatus("APP-1", res.Token); err != nil {
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
	if _, err := CheckMemberStatus("APP-1", res.Token); err == nil {
		t.Fatalf("old token should be invalidated after re-login")
	}
	if _, err := CheckMemberStatus("APP-1", res2.Token); err != nil {
		t.Fatalf("new token should be valid: %v", err)
	}

	// 登出 → 令牌清空，心跳失败
	if err := MemberLogout("APP-1", res2.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := CheckMemberStatus("APP-1", res2.Token); err == nil {
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
	reg, err := AccountRegister("APP-1", "alice", "secret1")
	if err != nil {
		t.Fatalf("AccountRegister: %v", err)
	}
	if reg.Username != "alice" {
		t.Fatalf("unexpected register result: %+v", reg)
	}
	// 重复注册应失败
	if _, err := AccountRegister("APP-1", "alice", "x"); err == nil {
		t.Fatalf("duplicate register should fail")
	}
	// 未充值（已过期）登录应失败
	if _, err := AccountLogin("APP-1", "alice", "secret1", "", "1.2.3.4"); err == nil {
		t.Fatalf("login should fail before recharge (expired)")
	}

	// 用一张卡为账号充值 30 天
	card := models.Card{CardNo: "KM-RC", AppUUID: "APP-1", Duration: 30 * 24 * 60, Status: models.CardStatusUnused}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}
	res, err := RechargeByCard("APP-1", "alice", "KM-RC")
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
	if _, err := RechargeByCard("APP-1", "alice", "KM-RC"); err == nil {
		t.Fatalf("reusing consumed card should fail")
	}

	// 错误密码登录失败
	if _, err := AccountLogin("APP-1", "alice", "wrong", "", "1.2.3.4"); err == nil {
		t.Fatalf("login with wrong password should fail")
	}
	// 充值后正确密码登录成功
	login, err := AccountLogin("APP-1", "alice", "secret1", "", "1.2.3.4")
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
	if _, err := AccountRegister("APP-1", "carl", "pw123456"); err != nil {
		t.Fatalf("register: %v", err)
	}
	card := models.Card{CardNo: "KM-DATA", AppUUID: "APP-1", Duration: 24 * 60, Status: models.CardStatusUnused}
	db.Create(&card)
	if _, err := RechargeByCard("APP-1", "carl", "KM-DATA"); err != nil {
		t.Fatalf("recharge: %v", err)
	}
	login, err := AccountLogin("APP-1", "carl", "pw123456", "", "1.2.3.4")
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
	if _, err := AccountLogin("APP-1", "carl", "newpass1", "", "1.2.3.4"); err != nil {
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

func TestAccountRegisterDisabled(t *testing.T) {
	db := setupPublicTestDB(t)
	// 显式关闭注册（App.RegisterEnabled 带 default:1，需强制置 0）
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").Update("register_enabled", 0)
	if _, err := AccountRegister("APP-1", "bob", "pw"); err == nil {
		t.Fatalf("register should be rejected when disabled")
	}
}
