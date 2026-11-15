package services

import (
	"errors"
	"testing"
	"time"

	"NetworkAuth/database"
	"NetworkAuth/models"

	"gorm.io/gorm"
)

// ============================================================================
// 二级令牌（账号令牌 + 应用激活）不变量测试
// ============================================================================
//
// 这些用例锁定「登录不计费 / 激活才计费」这条改造的核心语义，
// 任何一处被改回旧行为都会在这里失败。

// setupActivationDB 建库并准备两个应用 + 一个账号
func setupActivationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupUnifiedTestDB(t)
	return db
}

// seedApp 建应用
func seedApp(t *testing.T, db *gorm.DB, uuid string, mutate func(*models.App)) *models.App {
	t.Helper()
	app := models.App{
		UUID:           uuid,
		Name:           "应用 " + uuid,
		Secret:         "SECRET-" + uuid,
		Status:         1,
	}
	if mutate != nil {
		mutate(&app)
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("seed app %s: %v", uuid, err)
	}
	return &app
}

// seedAccountMember 建账号
func seedAccountMember(t *testing.T, db *gorm.DB, username string, points float64, expireIn time.Duration) *models.Member {
	t.Helper()
	m := models.Member{
		Username:  username,
		Type:      models.MemberTypeRegister,
		Status:    models.MemberStatusNormal,
		Points:    points,
		ExpiredAt: time.Now().Add(expireIn),
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	return &m
}

// ---------------------------------------------------------------------------
// 1. 登录不做任何应用级动作
// ---------------------------------------------------------------------------

func TestLoginDoesNotActivateOrCharge(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-L1", func(a *models.App) {
		a.BillingMode = models.BillingModePoints
		a.PointsChargeMode = models.PointsChargePerCount
		a.PointsPerLogin = 5
	})
	m := seedAccountMember(t, db, "login@purely.identity", 10, 24*time.Hour)

	acct, err := IssueAccountToken(db, m.UUID, "FP-L1", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	// 账号令牌存在
	if acct.Token == "" || acct.RefreshToken == "" {
		t.Fatalf("account token should have both token and refresh token")
	}
	// 没扣点
	var after models.Member
	db.Where("uuid = ?", m.UUID).First(&after)
	if after.Points != 10 {
		t.Fatalf("login must not charge: want 10 points, got %v", after.Points)
	}
	// 没建激活
	var n int64
	db.Model(&models.AppActivation{}).Where("member_uuid = ?", m.UUID).Count(&n)
	if n != 0 {
		t.Fatalf("login must not create activation, got %v", n)
	}
	// 没建设备绑定
	var binds int64
	db.Model(&models.Binding{}).Where("member_uuid = ?", m.UUID).Count(&binds)
	if binds != 0 {
		t.Fatalf("login must not bind device, got %v bindings", binds)
	}
	_ = app
}

// ---------------------------------------------------------------------------
// 2. 激活才扣费
// ---------------------------------------------------------------------------

func TestActivationChargesPoints(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-C1", func(a *models.App) {
		a.BillingMode = models.BillingModePoints
		a.PointsChargeMode = models.PointsChargePerCount
		a.PointsPerLogin = 3
	})
	m := seedAccountMember(t, db, "charge@test", 10, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP-C1", "1.2.3.4", "1.0.0", "dev")

	info, err := Activate(db, acct, app, "FP-C1", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if info.Points != 7 {
		t.Fatalf("activation should charge 3 (10->7), got %v", info.Points)
	}
	if info.Reused {
		t.Fatalf("first activation should not be reused")
	}
}

// ---------------------------------------------------------------------------
// 3. 幂等激活：同指纹重复激活不重复扣费
// ---------------------------------------------------------------------------

func TestActivationIdempotent(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-I1", func(a *models.App) {
		a.BillingMode = models.BillingModePoints
		a.PointsChargeMode = models.PointsChargePerCount
		a.PointsPerLogin = 4
	})
	m := seedAccountMember(t, db, "idem@test", 20, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP-I1", "1.2.3.4", "1.0.0", "dev")

	first, err := Activate(db, acct, app, "FP-I1", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("first activate: %v", err)
	}
	if first.Points != 16 || first.Reused {
		t.Fatalf("first activation should charge 4 (20->16), got %v reused=%v", first.Points, first.Reused)
	}

	// 同指纹第二次（模拟崩溃后重启）
	second, err := Activate(db, acct, app, "FP-I1", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("second activate: %v", err)
	}
	if !second.Reused {
		t.Fatalf("same fingerprint should reuse activation")
	}
	if second.Points != 16 {
		t.Fatalf("reuse must not charge again: want 16, got %v", second.Points)
	}
	if second.Token != first.Token {
		t.Fatalf("reuse should keep the same activation token")
	}
	// 只有一条激活记录
	var n int64
	db.Model(&models.AppActivation{}).Where("member_uuid = ? AND app_uuid = ?", m.UUID, app.UUID).Count(&n)
	if n != 1 {
		t.Fatalf("idempotent activation should keep 1 record, got %v", n)
	}
}

// ---------------------------------------------------------------------------
// 4. 不同指纹 = 不同占用，各自扣费
// ---------------------------------------------------------------------------

func TestActivationPerFingerprint(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-F1", func(a *models.App) {
		a.BillingMode = models.BillingModePoints
		a.PointsChargeMode = models.PointsChargePerCount
		a.PointsPerLogin = 2
	})
	setTestQuota(db, 3, 3, 20)
	setMachineVerify(db, 1)
	m := seedAccountMember(t, db, "fp@test", 10, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP-F1", "1.2.3.4", "1.0.0", "dev")

	a, err := Activate(db, acct, app, "FP-F1", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate dev1: %v", err)
	}
	b, err := Activate(db, acct, app, "FP-F2", "1.2.3.4", "1.0.0", "dev2")
	if err != nil {
		t.Fatalf("activate dev2: %v", err)
	}
	if b.Points != 6 || a.Points != 8 {
		t.Fatalf("each activation should charge: got %v then %v", a.Points, b.Points)
	}
	if ActiveActivationCount(db, m.UUID, app.UUID) != 2 {
		t.Fatalf("two fingerprints should take two slots")
	}
}

// ---------------------------------------------------------------------------
// 5. 跨应用各自计费，钱包共享
// ---------------------------------------------------------------------------

func TestActivationAcrossAppsSharesWallet(t *testing.T) {
	db := setupActivationDB(t)
	appA := seedApp(t, db, "APP-X1", func(a *models.App) {
		a.BillingMode = models.BillingModePoints
		a.PointsChargeMode = models.PointsChargePerCount
		a.PointsPerLogin = 2
	})
	appB := seedApp(t, db, "APP-X2", func(a *models.App) {
		a.BillingMode = models.BillingModePoints
		a.PointsChargeMode = models.PointsChargePerCount
		a.PointsPerLogin = 3
	})
	m := seedAccountMember(t, db, "cross@test", 20, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP-X", "1.2.3.4", "1.0.0", "dev")

	ra, err := Activate(db, acct, appA, "FP-X", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate A: %v", err)
	}
	rb, err := Activate(db, acct, appB, "FP-X", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate B: %v", err)
	}
	if ra.Points != 18 {
		t.Fatalf("A should charge 2 (20->18), got %v", ra.Points)
	}
	if rb.Points != 15 {
		t.Fatalf("B should charge 3 (18->15), got %v", rb.Points)
	}
	// 同一指纹在两个应用各一条激活，互不挤占
	if ActiveActivationCount(db, m.UUID, appA.UUID) != 1 || ActiveActivationCount(db, m.UUID, appB.UUID) != 1 {
		t.Fatalf("activations should be per-app")
	}
}

// ---------------------------------------------------------------------------
// 6. 释放立即归还名额
// ---------------------------------------------------------------------------

func TestDeactivateReleasesSlot(t *testing.T) {
	db := setupActivationDB(t)
	// 并发数是账号级配置，不再读 app.MultiOpenCount
	setTestQuota(db, 20, 1, 20)
	setMachineVerify(db, 1) // 并发额度只在开启设备校验时生效
	app := seedApp(t, db, "APP-D1", func(a *models.App) {
		a.LoginType = models.LoginTypeReject
		// 并发按「设备」计，需要可靠的设备标识
			})
	m := seedAccountMember(t, db, "deact@test", 0, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP", "1.2.3.4", "1.0.0", "dev")

	first, err := Activate(db, acct, app, "FP-D1", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate dev1: %v", err)
	}
	// 名额已满
	if _, err := Activate(db, acct, app, "FP-D2", "1.2.3.4", "1.0.0", "dev2"); err == nil {
		t.Fatalf("second device should be rejected when limit=1")
	}
	// 释放后可以再进
	if err := Deactivate(db, first.Token); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if ActiveActivationCount(db, m.UUID, app.UUID) != 0 {
		t.Fatalf("deactivation should free the slot immediately")
	}
	if _, err := Activate(db, acct, app, "FP-D2", "1.2.3.4", "1.0.0", "dev2"); err != nil {
		t.Fatalf("activation after release should pass: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 7. 租约过期自动失效
// ---------------------------------------------------------------------------

func TestLeaseExpiryFreesSlot(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-E1", func(a *models.App) {
		a.LeaseTTLMinutes = 30
	})
	setTestQuota(db, 20, 1, 20) // 设备数放宽，本用例测的是「并发名额」随租约释放
	setMachineVerify(db, 1)
	m := seedAccountMember(t, db, "lease@test", 0, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP", "1.2.3.4", "1.0.0", "dev")

	first, err := Activate(db, acct, app, "FP-E1", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if ActiveActivationCount(db, m.UUID, app.UUID) != 1 {
		t.Fatalf("should occupy 1 slot")
	}

	// 把租约推到过去（模拟客户端崩溃、没再心跳）
	past := time.Now().Add(-time.Minute)
	db.Model(&models.AppActivation{}).Where("token = ?", first.Token).
		Update("lease_expires_at", past)

	if ActiveActivationCount(db, m.UUID, app.UUID) != 0 {
		t.Fatalf("expired lease should not occupy a slot")
	}
	// 过期后令牌不可用
	if _, _, err := AuthActivation(db, first.Token); err == nil {
		t.Fatalf("expired lease token should be rejected")
	}
	// 另一台设备可以进来了
	if _, err := Activate(db, acct, app, "FP-E2", "1.2.3.4", "1.0.0", "dev2"); err != nil {
		t.Fatalf("new device should take the freed slot: %v", err)
	}
	// pruner 把过期记录标成已释放
	if n, err := PruneDeadActivations(db); err != nil || n == 0 {
		t.Fatalf("pruner should mark dead activations, n=%v err=%v", n, err)
	}
}

// ---------------------------------------------------------------------------
// 8. 心跳续租约
// ---------------------------------------------------------------------------

func TestRenewLeaseExtends(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-R1", func(a *models.App) {
		a.LeaseTTLMinutes = 30
	})
	m := seedAccountMember(t, db, "renew@test", 0, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP-R1", "1.2.3.4", "1.0.0", "dev")

	info, err := Activate(db, acct, app, "FP-R1", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	before := info.ExpiresAt

	// 把租约挪近到期，再心跳
	near := time.Now().Add(10 * time.Second)
	db.Model(&models.AppActivation{}).Where("token = ?", info.Token).
		Update("lease_expires_at", near)

	if _, _, _, err := RenewLease(db, info.Token); err != nil {
		t.Fatalf("renew: %v", err)
	}
	var act models.AppActivation
	db.Where("token = ?", info.Token).First(&act)
	if !act.LeaseExpiresAt.After(near) {
		t.Fatalf("renew should extend lease: before=%v after=%v", near, act.LeaseExpiresAt)
	}
	if act.LeaseExpiresAt.Before(before) {
		t.Fatalf("renew should not shorten the lease")
	}
}

// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 10. 账号登出级联失效全部激活
// ---------------------------------------------------------------------------

func TestLogoutRevokesActivations(t *testing.T) {
	db := setupActivationDB(t)
	appA := seedApp(t, db, "APP-O1", nil)
	appB := seedApp(t, db, "APP-O2", nil)
	m := seedAccountMember(t, db, "logout@test", 0, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP-O", "1.2.3.4", "1.0.0", "dev")

	if _, err := Activate(db, acct, appA, "FP-O", "1.2.3.4", "1.0.0", "dev"); err != nil {
		t.Fatalf("activate A: %v", err)
	}
	if _, err := Activate(db, acct, appB, "FP-O", "1.2.3.4", "1.0.0", "dev"); err != nil {
		t.Fatalf("activate B: %v", err)
	}
	if ActiveActivationCount(db, m.UUID, appA.UUID)+ActiveActivationCount(db, m.UUID, appB.UUID) != 2 {
		t.Fatalf("should have 2 activations")
	}

	if err := RevokeAccountToken(db, acct.Token); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if ActiveActivationCount(db, m.UUID, appA.UUID)+ActiveActivationCount(db, m.UUID, appB.UUID) != 0 {
		t.Fatalf("logout should revoke all activations")
	}
	// 账号令牌本身也没了
	if _, err := AuthAccountToken(db, acct.Token); err == nil {
		t.Fatalf("account token should be gone after logout")
	}
}

// ---------------------------------------------------------------------------
// 11. 按时模式：周期记账在激活级，不污染钱包到期时间
// ---------------------------------------------------------------------------

func TestPerTimeBillingUsesActivationPeriod(t *testing.T) {
	db := setupActivationDB(t)
	appA := seedApp(t, db, "APP-P1", func(a *models.App) {
		a.BillingMode = models.BillingModePoints
		a.PointsChargeMode = models.PointsChargePerTime
		a.PointsPeriodMinutes = 60
		a.PointsPerPeriod = 1
	})
	appB := seedApp(t, db, "APP-P2", func(a *models.App) {
		a.BillingMode = models.BillingModePoints
		a.PointsChargeMode = models.PointsChargePerTime
		a.PointsPeriodMinutes = 60
		a.PointsPerPeriod = 1
	})
	m := seedAccountMember(t, db, "pertime@test", 10, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP-P", "1.2.3.4", "1.0.0", "dev")

	ra, err := Activate(db, acct, appA, "FP-P", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate A: %v", err)
	}
	rb, err := Activate(db, acct, appB, "FP-P", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate B: %v", err)
	}
	if ra.Points != 9 || rb.Points != 8 {
		t.Fatalf("each app should charge one period: %v then %v", ra.Points, rb.Points)
	}

	var actA, actB models.AppActivation
	db.Where("token = ?", ra.Token).First(&actA)
	db.Where("token = ?", rb.Token).First(&actB)
	if actA.PeriodEnd == nil || actB.PeriodEnd == nil {
		t.Fatalf("per-time billing should record period_end on activation")
	}

	// 关键：钱包到期时间不应被按时计费推进（它只属于会员模式）
	var after models.Member
	db.Where("uuid = ?", m.UUID).First(&after)
	if after.ExpiredAt.After(time.Now().Add(48 * time.Hour)) {
		t.Fatalf("per-time billing must not extend members.expired_at, got %v", after.ExpiredAt)
	}

	// A 的周期过期不影响 B
	past := time.Now().Add(-time.Minute)
	db.Model(&models.AppActivation{}).Where("token = ?", ra.Token).Update("period_end", past)
	if _, _, _, err := RenewLease(db, ra.Token); err != nil {
		t.Fatalf("renew A: %v", err)
	}
	var actB2 models.AppActivation
	db.Where("token = ?", rb.Token).First(&actB2)
	if !actB2.PeriodEnd.Equal(*actB.PeriodEnd) {
		t.Fatalf("A's period renewal must not touch B")
	}
}
// ---------------------------------------------------------------------------
// 12. 账号令牌不能直接调应用级接口（必须先在 70 换激活令牌）
// ---------------------------------------------------------------------------

func TestAccountTokenCannotCallAppAPI(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-IM", func(a *models.App) {
		a.BillingMode = models.BillingModePoints
		a.PointsChargeMode = models.PointsChargePerCount
		a.PointsPerLogin = 2
	})
	m := seedAccountMember(t, db, "implicit@test", 10, 24*time.Hour)
	tok, err := IssueAccountToken(db, m.UUID, "FP-IM", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("issue account token: %v", err)
	}
	acct := tok

	// 账号令牌直接调应用级接口 → 明确要求先激活，且不产生任何扣费
	_, _, _, err = AuthForApp(db, app.UUID, acct.Token)
	if !errors.Is(err, ErrActivationRequiredMarker) {
		t.Fatalf("account token must be rejected with activation-required, got %v", err)
	}
	var after models.Member
	db.Where("uuid = ?", m.UUID).First(&after)
	if after.Points != 10 {
		t.Fatalf("rejected call must not charge, points=%v", after.Points)
	}

	// 正常路径：70 激活后拿激活令牌访问 → 扣一次
	res, err := Activate(db, acct, app, "FP-IM", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if _, _, _, err := AuthForApp(db, app.UUID, res.Token); err != nil {
		t.Fatalf("activation token should pass: %v", err)
	}
	db.Where("uuid = ?", m.UUID).First(&after)
	if after.Points != 8 {
		t.Fatalf("activation should charge once (10->8), got %v", after.Points)
	}
}
// ---------------------------------------------------------------------------
// 13. 令牌与应用不匹配要拒绝
// ---------------------------------------------------------------------------

func TestActivationTokenAppMismatchRejected(t *testing.T) {
	db := setupActivationDB(t)
	appA := seedApp(t, db, "APP-M1", nil)
	appB := seedApp(t, db, "APP-M2", nil)
	m := seedAccountMember(t, db, "mismatch@test", 0, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP-M", "1.2.3.4", "1.0.0", "dev")

	info, err := Activate(db, acct, appA, "FP-M", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate A: %v", err)
	}
	// 拿 A 的激活令牌去调 B 的接口
	if _, _, _, err := AuthForApp(db, appB.UUID, info.Token); err == nil {
		t.Fatalf("activation token of A must not work on B")
	}
}

// ---------------------------------------------------------------------------
// 14. 权益清单：只对可用应用下发 secret
// ---------------------------------------------------------------------------

func TestEntitlementsHidesSecretWhenUnusable(t *testing.T) {
	db := setupActivationDB(t)
	usable := seedApp(t, db, "APP-EN1", func(a *models.App) {
		a.BillingMode = models.BillingModeMember
		a.RequiredLevel = 1
	})
	locked := seedApp(t, db, "APP-EN2", func(a *models.App) {
		a.BillingMode = models.BillingModeMember
		a.RequiredLevel = 99 // 任何账号都达不到
	})
	m := seedAccountMember(t, db, "ent@test", 0, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP-EN", "1.2.3.4", "1.0.0", "dev")

	res, err := ListEntitlements(db, acct, "FP-EN")
	if err != nil {
		t.Fatalf("entitlements: %v", err)
	}
	byUUID := map[string]EntitlementItem{}
	for _, p := range res.Plugins {
		byUUID[p.UUID] = p
	}

	u, ok := byUUID[usable.UUID]
	if !ok || u.Usable != 1 || u.Secret == "" {
		t.Fatalf("usable app should expose secret: %+v", u)
	}
	l, ok := byUUID[locked.UUID]
	if !ok {
		t.Fatalf("locked app should still be listed (so UI can grey it out)")
	}
	if l.Usable != 0 || l.Secret != "" {
		t.Fatalf("locked app must not expose secret: %+v", l)
	}
	if l.Reason == "" {
		t.Fatalf("locked app should carry a reason")
	}
}

var _ = database.GetDB
