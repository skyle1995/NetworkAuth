package services

import (
	"NetworkAuth/models"
	"testing"
	"time"

	"gorm.io/gorm"
)

// TestMergedBillingPerCountChargesOncePerAccount 验证「合并计费 + 按次」：
// 同一账号在同应用的 N 台设备只扣 1 次；第一台释放后重新进入才再扣。
func TestMergedBillingPerCountChargesOncePerAccount(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-MG-CNT", func(a *models.App) {
		a.ApplyBillingFlags(models.BillingModePoints)
		a.BillingScope = models.BillingScopeAccount
		a.PointsChargeMode = models.PointsChargePerCount
		a.PointsPerLogin = 2
			})
	m := seedAccountMember(t, db, "merge-cnt@test", 10, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP", "1.2.3.4", "1.0.0", "dev")

	// 设备 1：扣 2
	if _, err := Activate(db, acct, app, "MC-A", "1.2.3.4", "1.0.0", "A"); err != nil {
		t.Fatalf("activate A: %v", err)
	}
	if got := loadPoints(t, db, m.UUID); got != 8 {
		t.Fatalf("first device should charge 2, points=%v", got)
	}

	// 设备 2：合并计费 → 不再扣
	if _, err := Activate(db, acct, app, "MC-B", "1.2.3.4", "1.0.0", "B"); err != nil {
		t.Fatalf("activate B: %v", err)
	}
	if got := loadPoints(t, db, m.UUID); got != 8 {
		t.Fatalf("second device must not charge again, points=%v", got)
	}

	// 设备 3：同样不扣
	if _, err := Activate(db, acct, app, "MC-C", "1.2.3.4", "1.0.0", "C"); err != nil {
		t.Fatalf("activate C: %v", err)
	}
	if got := loadPoints(t, db, m.UUID); got != 8 {
		t.Fatalf("third device must not charge again, points=%v", got)
	}

	// 全部释放后重新进入 → 新的一轮，扣 2
	for _, fp := range []string{"MC-A", "MC-B", "MC-C"} {
		var act models.AppActivation
		if err := db.Where("member_uuid = ? AND fingerprint = ?", m.UUID, fp).First(&act).Error; err != nil {
			t.Fatalf("load activation %s: %v", fp, err)
		}
		if err := Deactivate(db, act.Token); err != nil {
			t.Fatalf("deactivate %s: %v", fp, err)
		}
	}
	if _, err := Activate(db, acct, app, "MC-D", "1.2.3.4", "1.0.0", "D"); err != nil {
		t.Fatalf("activate D after all released: %v", err)
	}
	if got := loadPoints(t, db, m.UUID); got != 6 {
		t.Fatalf("new round should charge 2, points=%v", got)
	}
}

// TestDeviceBillingPerCountChargesPerDevice 验证「按设备单独计费」仍然每台扣一次（默认行为不回退）。
func TestDeviceBillingPerCountChargesPerDevice(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-SP-CNT", func(a *models.App) {
		a.ApplyBillingFlags(models.BillingModePoints)
		a.BillingScope = models.BillingScopeDevice
		a.PointsChargeMode = models.PointsChargePerCount
		a.PointsPerLogin = 2
			})
	m := seedAccountMember(t, db, "single-cnt@test", 10, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP", "1.2.3.4", "1.0.0", "dev")

	for i, fp := range []string{"MC-1", "MC-2"} {
		if _, err := Activate(db, acct, app, fp, "1.2.3.4", "1.0.0", fp); err != nil {
			t.Fatalf("activate %s: %v", fp, err)
		}
		want := 10.0 - 2*float64(i+1)
		if got := loadPoints(t, db, m.UUID); got != want {
			t.Fatalf("device %d: want points=%v, got %v", i+1, want, got)
		}
	}
}

// TestMergedBillingPerTimeSharesOnePeriod 验证「合并计费 + 按时」：
// 同一账号两台设备共享一个 period_end，只扣一份周期费。
func TestMergedBillingPerTimeSharesOnePeriod(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-MG-TIME", func(a *models.App) {
		a.ApplyBillingFlags(models.BillingModePoints)
		a.BillingScope = models.BillingScopeAccount
		a.PointsChargeMode = models.PointsChargePerTime
		a.PointsPerPeriod = 3
		a.PointsPeriodMinutes = 60
		a.PointsHeartbeatCharge = models.ChargeBasisFromCreation
			})
	m := seedAccountMember(t, db, "merge-time@test", 20, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP", "1.2.3.4", "1.0.0", "dev")

	// 设备 1：扣 1 个周期
	act1, err := Activate(db, acct, app, "MC-T1", "1.2.3.4", "1.0.0", "T1")
	if err != nil {
		t.Fatalf("activate T1: %v", err)
	}
	if got := loadPoints(t, db, m.UUID); got != 17 {
		t.Fatalf("first period should charge 3, points=%v", got)
	}

	// 设备 2：周期还在有效期内 → 不扣
	act2, err := Activate(db, acct, app, "MC-T2", "1.2.3.4", "1.0.0", "T2")
	if err != nil {
		t.Fatalf("activate T2: %v", err)
	}
	if got := loadPoints(t, db, m.UUID); got != 17 {
		t.Fatalf("second device shares period, must not charge, points=%v", got)
	}

	// 两台共享同一份周期账本
	var row models.AppAccountBilling
	if err := db.Where("member_uuid = ? AND app_uuid = ?", m.UUID, app.UUID).First(&row).Error; err != nil {
		t.Fatalf("merged period row missing: %v", err)
	}
	if row.PeriodEnd == nil {
		t.Fatalf("merged period_end should be set")
	}
	// 两个激活记录上的 period_end 也应镜像同一时刻（供客户端展示）
	var acts []models.AppActivation
	db.Where("member_uuid = ? AND app_uuid = ?", m.UUID, app.UUID).Find(&acts)
	if len(acts) != 2 {
		t.Fatalf("expected 2 activations, got %d", len(acts))
	}
	for _, a := range acts {
		if a.PeriodEnd == nil {
			t.Fatalf("activation %s period_end should mirror merged period", a.Fingerprint)
		}
		if !a.PeriodEnd.Equal(*row.PeriodEnd) {
			t.Fatalf("activation %s should share merged period_end", a.Fingerprint)
		}
	}
	_ = act1
	_ = act2
}

// TestDeviceBillingPerTimeHasOwnPeriods 验证「按设备单独计费 + 按时」：
// 两台设备各自有一份周期，各扣各的。
func TestDeviceBillingPerTimeHasOwnPeriods(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-SP-TIME", func(a *models.App) {
		a.ApplyBillingFlags(models.BillingModePoints)
		a.BillingScope = models.BillingScopeDevice
		a.PointsChargeMode = models.PointsChargePerTime
		a.PointsPerPeriod = 3
		a.PointsPeriodMinutes = 60
		a.PointsHeartbeatCharge = models.ChargeBasisFromCreation
			})
	m := seedAccountMember(t, db, "single-time@test", 20, 24*time.Hour)
	acct, _ := IssueAccountToken(db, m.UUID, "FP", "1.2.3.4", "1.0.0", "dev")

	if _, err := Activate(db, acct, app, "MC-D1", "1.2.3.4", "1.0.0", "D1"); err != nil {
		t.Fatalf("activate D1: %v", err)
	}
	if _, err := Activate(db, acct, app, "MC-D2", "1.2.3.4", "1.0.0", "D2"); err != nil {
		t.Fatalf("activate D2: %v", err)
	}
	if got := loadPoints(t, db, m.UUID); got != 14 {
		t.Fatalf("each device should charge its own period (20-3-3=14), got %v", got)
	}
	// 单独计费不该写合并账本
	var count int64
	db.Model(&models.AppAccountBilling{}).Where("member_uuid = ?", m.UUID).Count(&count)
	if count != 0 {
		t.Fatalf("device-scope billing must not create merged period rows, got %d", count)
	}
}

// loadPoints 读取账号当前点数
func loadPoints(t *testing.T, db *gorm.DB, memberUUID string) float64 {
	t.Helper()
	var m models.Member
	if err := db.Where("uuid = ?", memberUUID).First(&m).Error; err != nil {
		t.Fatalf("load member: %v", err)
	}
	return m.Points
}
