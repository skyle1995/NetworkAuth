package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupMemberTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.App{}, &models.Card{}, &models.Member{}, &models.Binding{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	// 建一个应用供外键校验通过
	if err := db.Create(&models.App{UUID: "APP-1", Name: "测试应用", Secret: "SECRET"}).Error; err != nil {
		t.Fatalf("seed app: %v", err)
	}
	database.SetDB(db)
}

func TestCreateMemberAndTimeMath(t *testing.T) {
	setupMemberTestDB(t)

	// 创建 1 天时长的注册账号
	m, err := CreateMember("APP-1", "alice", "pass123", 24*60, 0, "vip")
	if err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	if m.Type != models.MemberTypeRegister {
		t.Fatalf("want register type, got %d", m.Type)
	}
	if m.Password == "" || m.PasswordSalt == "" {
		t.Fatalf("password not hashed")
	}
	// 到期时间约为 now + 1 天
	wantExpiry := time.Now().Add(24 * time.Hour)
	if diff := m.ExpiredAt.Sub(wantExpiry); diff > time.Minute || diff < -time.Minute {
		t.Fatalf("unexpected expiry, off by %v", diff)
	}

	// 同应用下用户名重复应报错
	if _, err := CreateMember("APP-1", "alice", "x", 60, 0, ""); err == nil {
		t.Fatalf("expected duplicate username error")
	}

	// 充值 1 天 → 到期时间再加 1 天
	before := loadMember(t, m.ID).ExpiredAt
	if err := RechargeMemberTime(m.ID, 24*60); err != nil {
		t.Fatalf("RechargeMemberTime: %v", err)
	}
	after := loadMember(t, m.ID).ExpiredAt
	if diff := after.Sub(before) - 24*time.Hour; diff > time.Second || diff < -time.Second {
		t.Fatalf("recharge did not add 1 day, diff %v", diff)
	}

	// 扣时 2 天（超过剩余）→ 到期时间落到不早于 now
	if err := DeductMemberTime(m.ID, 2*24*60); err != nil {
		t.Fatalf("DeductMemberTime: %v", err)
	}
	got := loadMember(t, m.ID).ExpiredAt
	if got.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("deduction floor breached: %v", got)
	}
}

func TestRechargePermanentIsNoop(t *testing.T) {
	setupMemberTestDB(t)
	m, err := CreateMember("APP-1", "bob", "pw", models.CardDurationPermanent, 0, "")
	if err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	if !m.ExpiredAt.Equal(models.PermanentTime) {
		t.Fatalf("permanent member expiry wrong: %v", m.ExpiredAt)
	}
	// 永久账号充值应保持永久
	if err := RechargeMemberTime(m.ID, 60); err != nil {
		t.Fatalf("RechargeMemberTime: %v", err)
	}
	if !loadMember(t, m.ID).ExpiredAt.Equal(models.PermanentTime) {
		t.Fatalf("permanent expiry changed after recharge")
	}
	// 永久账号扣时应报错
	if err := DeductMemberTime(m.ID, 60); err == nil {
		t.Fatalf("expected error deducting from permanent account")
	}
}

func TestDeleteMembersCascadesBindings(t *testing.T) {
	setupMemberTestDB(t)
	m, err := CreateMember("APP-1", "carol", "pw", 60, 0, "")
	if err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	db, _ := database.GetDB()
	if err := db.Create(&models.Binding{MemberUUID: m.UUID, Type: models.BindingTypeMachine, Value: "MC-1"}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}

	if err := DeleteMembers([]uint{m.ID}); err != nil {
		t.Fatalf("DeleteMembers: %v", err)
	}
	var bindingCount int64
	db.Model(&models.Binding{}).Where("member_uuid = ?", m.UUID).Count(&bindingCount)
	if bindingCount != 0 {
		t.Fatalf("expected bindings cascade-deleted, got %d", bindingCount)
	}
}

func TestAdminMemberPointsMode(t *testing.T) {
	setupMemberTestDB(t)
	db, _ := database.GetDB()
	// APP-1 切到点数模式
	db.Model(&models.App{}).Where("uuid = ?", "APP-1").
		Update("operation_mode", models.OperationModePoints)

	// 创建时用初始点数（时长参数被忽略）
	m, err := CreateMember("APP-1", "pointsuser", "pw", 999, 50, "")
	if err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	if m.Points != 50 {
		t.Fatalf("want 50 initial points, got %d", m.Points)
	}
	if mode, _ := GetMemberAppMode(m.ID); mode != models.OperationModePoints {
		t.Fatalf("GetMemberAppMode should report points mode")
	}

	// 充值 +30 → 80
	if err := RechargeMemberPoints(m.ID, 30); err != nil {
		t.Fatalf("RechargeMemberPoints: %v", err)
	}
	if loadMember(t, m.ID).Points != 80 {
		t.Fatalf("want 80 after recharge")
	}
	// 扣 100 → 下限 0
	if err := DeductMemberPoints(m.ID, 100); err != nil {
		t.Fatalf("DeductMemberPoints: %v", err)
	}
	if loadMember(t, m.ID).Points != 0 {
		t.Fatalf("deduct should floor at 0, got %d", loadMember(t, m.ID).Points)
	}
}

func loadMember(t *testing.T, id uint) models.Member {
	t.Helper()
	db, _ := database.GetDB()
	var m models.Member
	if err := db.First(&m, id).Error; err != nil {
		t.Fatalf("load member: %v", err)
	}
	return m
}
