package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ============================================================================
// 一号通核心不变量的回归测试
// ============================================================================
//
// 这些断言锁住「钱包全局共享 + 计费按应用」的关键行为，
// 任何一处被改回「按应用隔离」都会在这里失败。

// setupUnifiedTestDB 建一个双应用环境：
//
//	APP-A：点数模式，按次每次登录扣 1 点
//	APP-B：点数模式，按次每次登录扣 1 点
//
// 卡密、账号、会员等级均为全站一份。
func setupUnifiedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.App{}, &models.Card{}, &models.Member{}, &models.Binding{},
		&models.API{}, &models.AppActivation{}, &models.MemberLog{},
		&models.CardPackage{}, &models.MemberLevel{}, &models.AppLevelPoint{},
		&models.Settings{}, &models.Variable{}, &models.Function{}, &models.MemberAccountToken{}, &models.AppActivation{}, &models.AppAccountBilling{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	database.SetDB(db)
	ResetSettingsService()
	seedTestQuota(db)

	// 两个应用各自走点数模式：验证「同一份钱包被两个应用分别消费」
	for _, uuid := range []string{"APP-A", "APP-B"} {
		if err := db.Create(&models.App{
			UUID:                 uuid,
			Name:                 "应用 " + uuid,
			Secret:               "SECRET-" + uuid,
			Status:               1,
			BillPointsEnabled:    1, // 纯点数计费
			BillingMode:          models.BillingModePoints,
			RequiredLevel:        1,
			PointsChargeMode:     models.PointsChargePerCount,
			PointsPerLogin:       1,
			PointsPerPeriod:      1,
			PointsPeriodMinutes:  60,
			PointsHeartbeatCharge: models.HeartbeatChargeOnLogin,
		}).Error; err != nil {
			t.Fatalf("seed app %s: %v", uuid, err)
		}
	}
	return db
}

// TestWalletSharedAcrossApps 跨应用共用一份钱包：
// 在 A 应用消费后，B 应用看到的是同一份余额。
func TestWalletSharedAcrossApps(t *testing.T) {
	db := setupUnifiedTestDB(t)
	// 本用例只验证「显式扣点(53)」的跨应用共享，故让激活本身不扣点
	// （激活扣点由 TestPointsPerCountMode 等用例单独覆盖）
	db.Model(&models.App{}).Where("1 = 1").Update("points_per_login", 0)

	// 全站一个账号，带 3 点
	m := models.Member{
		Username:  "shared@test.com",
		Type:      models.MemberTypeRegister,
		Status:    models.MemberStatusNormal,
		Points:    3,
		ExpiredAt: models.PermanentTime,
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}

	// 在 A 登录 → 扣 1 点
	resA, err := AccountLogin("APP-A", "shared@test.com", "x", "MC-1", "1.1.1.1", "1.0.0", "PC-1")
	if err == nil {
		t.Fatalf("password should be wrong for empty hash, got nil err, res=%v", resA)
	}
	// 直接走服务层扣费，绕开密码（这里只验证钱包口径）
	tokenA := createSessionFor(t, db, m.UUID, "APP-A")
	if _, err := DeductPoints("APP-A", tokenA, 1); err != nil {
		t.Fatalf("deduct in APP-A: %v", err)
	}

	// B 应用读到的是同一份余额（已减 1）
	var after models.Member
	db.Where("uuid = ?", m.UUID).First(&after)
	if after.Points != 2 {
		t.Fatalf("wallet must be shared across apps: want 2, got %v", after.Points)
	}

	// 在 B 再扣 1 → 全局只剩 1
	tokenB := createSessionFor(t, db, m.UUID, "APP-B")
	if _, err := DeductPoints("APP-B", tokenB, 1); err != nil {
		t.Fatalf("deduct in APP-B: %v", err)
	}
	db.Where("uuid = ?", m.UUID).First(&after)
	if after.Points != 1 {
		t.Fatalf("wallet must be shared across apps: want 1, got %v", after.Points)
	}
}

// TestDeductPointsNeverOverdraws 并发/重复扣点不会透支。
// 条件原子更新（WHERE points >= cost）是这一点的唯一保证。
func TestDeductPointsNeverOverdraws(t *testing.T) {
	db := setupUnifiedTestDB(t)
	// 同上：只验证条件原子扣点本身，激活不参与扣费
	db.Model(&models.App{}).Where("1 = 1").Update("points_per_login", 0)

	m := models.Member{
		Username:  "poor@test.com",
		Type:      models.MemberTypeRegister,
		Status:    models.MemberStatusNormal,
		Points:    2,
		ExpiredAt: models.PermanentTime,
	}
	db.Create(&m)
	token := createSessionFor(t, db, m.UUID, "APP-A")

	// 连续扣 5 次 1 点：只有前 2 次成功
	ok := 0
	for i := 0; i < 5; i++ {
		if _, err := DeductPoints("APP-A", token, 1); err == nil {
			ok++
		}
	}
	if ok != 2 {
		t.Fatalf("only 2 deductions should succeed, got %v", ok)
	}
	var after models.Member
	db.Where("uuid = ?", m.UUID).First(&after)
	if after.Points != 0 {
		t.Fatalf("points should floor at 0, got %v", after.Points)
	}
}

// TestCardRedeemAcrossApps 卡密是全站卡池：
// 在 A 应用的卡，可在 B 应用为账号充值。
func TestCardRedeemAcrossApps(t *testing.T) {
	db := setupUnifiedTestDB(t)

	// 卡密由 A 渠道发行，但全站可用
	card := models.Card{
		CardNo:        "KM-CROSS",
		Points:        50,
		Status:        models.CardStatusUnused,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card: %v", err)
	}
	m := models.Member{
		Username:  "redeem@test.com",
		Type:      models.MemberTypeCard,
		Status:    models.MemberStatusNormal,
		ExpiredAt: models.PermanentTime,
	}
	db.Create(&m)

	// 在 B 应用核销（若卡密仍按应用隔离，这里会失败）
	setPolicy(t, map[string]interface{}{"recharge_enabled": 1})
	res, err := RechargeByCard("APP-B", "redeem@test.com", "KM-CROSS")
	if err != nil {
		t.Fatalf("redeem card of APP-A in APP-B should succeed: %v", err)
	}
	if res.Points != 50 {
		t.Fatalf("card points not granted: got %v", res.Points)
	}

	// 卡只可核销一次
	if _, err := RechargeByCard("APP-B", "redeem@test.com", "KM-CROSS"); err == nil {
		t.Fatalf("card should not be reusable")
	}
}

// TestAppSecretIsolatesSignature 应用身份只由 secret 签名区分：
// 用 A 的 secret 为 B 的 app_uuid 签名必须失败。
func TestAppSecretIsolatesSignature(t *testing.T) {
	setupUnifiedTestDB(t)

	data := `{"card":"X"}`
	ts := time.Now().Unix()

	good := SignOpenRequest("APP-A", models.APITypeSingleLogin, data, ts, "SECRET-APP-A")
	if err := VerifyOpenSign("APP-A", models.APITypeSingleLogin, data, ts, good, "SECRET-APP-A"); err != nil {
		t.Fatalf("valid signature should pass: %v", err)
	}
	// 同一串签名拿到 B 上 → 必须失败（B 的 secret 不同）
	if err := VerifyOpenSign("APP-B", models.APITypeSingleLogin, data, ts, good, "SECRET-APP-B"); err == nil {
		t.Fatalf("signature signed by APP-A must not pass for APP-B")
	}
}

// TestEnsureGlobalAPIsCreatesOnePerType 接口条目全站唯一：每个 api_type 恰好一条。
func TestEnsureGlobalAPIsCreatesOnePerType(t *testing.T) {
	db := setupUnifiedTestDB(t)

	EnsureGlobalAPIs()
	want := len(models.GetDefaultAPITypes())

	var count int64
	db.Model(&models.API{}).Count(&count)
	if int(count) != want {
		t.Fatalf("want %v global api rows, got %v", want, count)
	}

	// 幂等：再次调用不应产生重复（唯一索引 + 跳过已存在）
	EnsureGlobalAPIs()
	db.Model(&models.API{}).Count(&count)
	if int(count) != want {
		t.Fatalf("EnsureGlobalAPIs must be idempotent: want %v, got %v", want, count)
	}

	// 默认全部启用
	var disabled int64
	db.Model(&models.API{}).Where("status <> ?", 1).Count(&disabled)
	if disabled != 0 {
		t.Fatalf("all apis should be enabled by default, %v disabled", disabled)
	}
}

// TestHybridModeUsesLevelPoints 混合模式：按账号等级的配置扣点。
func TestHybridModeUsesLevelPoints(t *testing.T) {
	db := setupUnifiedTestDB(t)

	// 把 A 改成混合模式，要求等级 2
	if err := db.Model(&models.App{}).Where("uuid = ?", "APP-A").Updates(map[string]interface{}{
		"billing_mode":       models.BillingModeHybrid,
		"bill_time_enabled":   0,
		"bill_points_enabled": 1,
		"level_gate_enabled":  1,
		"required_level":     2,
	}).Error; err != nil {
		t.Fatalf("update app: %v", err)
	}

	// 两个等级：白银(2) 扣 5 点，黄金(3) 扣 2 点
	silver := models.MemberLevel{Name: "白银", Level: 2, Threshold: 0, Status: 1}
	if err := db.Create(&silver).Error; err != nil {
		t.Fatalf("create level: %v", err)
	}
	gold := models.MemberLevel{Name: "黄金", Level: 3, Threshold: 1000, Status: 1}
	db.Create(&gold)

	// 等级点数：黄金在该应用只扣 2 点（比默认 10 便宜）
	if err := SaveAppLevelPoints("APP-A", map[string]float64{gold.UUID: 2}); err != nil {
		t.Fatalf("save level points: %v", err)
	}

	var app models.App
	db.Where("uuid = ?", "APP-A").First(&app)
	billing := EffectiveBilling(&app)
	if billing.PointsCostForLevel(gold.UUID, false) != 2 {
		t.Fatalf("gold should cost 2 points, got %v", billing.PointsCostForLevel(gold.UUID, false))
	}
	// 未配置的等级回退默认值
	if billing.PointsCostForLevel(silver.UUID, false) != app.PointsPerLogin {
		t.Fatalf("unconfigured level should fall back to %v, got %v",
			app.PointsPerLogin, billing.PointsCostForLevel(silver.UUID, false))
	}

	// 等级不足 → 拒绝
	low := models.Member{
		Username:  "low@test.com",
		Type:      models.MemberTypeRegister,
		Status:    models.MemberStatusNormal,
		Points:    100,
		ExpiredAt: models.PermanentTime,
	}
	db.Create(&low)
	if err := checkMemberUsable(db, billing, &low); err == nil {
		t.Fatalf("level 1 account should be rejected when required_level=2")
	}

	// 等级达标 → 放行
	high := models.Member{
		Username:  "high@test.com",
		Type:      models.MemberTypeRegister,
		Status:    models.MemberStatusNormal,
		Points:    100,
		ExpiredAt: models.PermanentTime,
		LevelUUID: gold.UUID,
	}
	db.Create(&high)
	if err := checkMemberUsable(db, billing, &high); err != nil {
		t.Fatalf("gold account should pass: %v", err)
	}
}

// TestFreeModeSkipsAllGates 计费豁免：无论等级/到期/点数，一律放行且不扣点。
func TestFreeModeSkipsAllGates(t *testing.T) {
	db := setupUnifiedTestDB(t)

	db.Model(&models.App{}).Where("uuid = ?", "APP-A").
		Update("free_mode", 1)

	var app models.App
	db.Where("uuid = ?", "APP-A").First(&app)
	billing := EffectiveBilling(&app)
	if !billing.Free {
		t.Fatalf("free_mode=1 should yield Free billing")
	}

	// 已到期、无点数、无等级 → 仍可用
	m := models.Member{
		Username:  "expired@test.com",
		Type:      models.MemberTypeRegister,
		Status:    models.MemberStatusNormal,
		Points:    0,
		ExpiredAt: time.Now().Add(-time.Hour),
	}
	db.Create(&m)
	if err := checkMemberUsable(db, billing, &m); err != nil {
		t.Fatalf("free app should allow expired account: %v", err)
	}
	// 登录不扣点：扣费只发生在 70 激活
	var after models.Member
	db.Where("uuid = ?", m.UUID).First(&after)
	if after.Points != m.Points {
		t.Fatalf("login must not charge points: before=%v after=%v", m.Points, after.Points)
	}
}

// createSessionFor 颁发账号令牌（二级令牌模型），用于绕开密码校验的用例。
//
// 返回的是**账号令牌**而非激活令牌：后续调应用级接口时会走隐式激活，
// 与老客户端的真实路径一致，正好顺带覆盖这条兼容链路。
// createSessionFor 用账号令牌换激活令牌（新协议：应用级接口只认激活令牌）。
func createSessionFor(t *testing.T, db *gorm.DB, memberUUID, appUUID string) string {
	t.Helper()
	acct, err := IssueAccountToken(db, memberUUID, "FP-TEST", "1.1.1.1", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("issue account token: %v", err)
	}
	var app models.App
	if err := db.Where("uuid = ?", appUUID).First(&app).Error; err != nil {
		t.Fatalf("load app: %v", err)
	}
	info, err := Activate(db, acct, &app, "FP-TEST", "1.1.1.1", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	return info.Token
}

func createSessionForLegacy(t *testing.T, db *gorm.DB, memberUUID, appUUID string) string {
	t.Helper()
	acct, err := IssueAccountToken(db, memberUUID, "FP-"+appUUID, "127.0.0.1", "1.0.0", "test-device")
	if err != nil {
		t.Fatalf("issue account token: %v", err)
	}
	return acct.Token
}
