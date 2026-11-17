package services

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"NetworkAuth/database"
	"NetworkAuth/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupSingleConnDB 复刻生产环境的 SQLite 连接池：**只允许 1 个连接**。
//
// 生产配置见 database/database.go：
//
//	sqlDB.SetMaxOpenConns(1)
//	sqlDB.SetMaxIdleConns(1)
//
// 这个约束下，「事务内部再调用 database.GetDB()」会去抢那条已经被事务占用的连接，
// 永远等不到 → 请求挂死，事务也不提交，SQLite 写锁一直不放。
func setupSingleConnDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + filepath.Join(t.TempDir(), "single.db") +
		"?cache=shared&_busy_timeout=5000&_fk=1"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
		t.Fatalf("enable wal: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.AutoMigrate(
		&models.App{}, &models.Card{}, &models.Member{}, &models.Binding{},
		&models.API{}, &models.AppActivation{}, &models.MemberLog{},
		&models.CardPackage{}, &models.MemberLevel{}, &models.AppLevelPoint{},
		&models.Settings{}, &models.Variable{}, &models.Function{},
		&models.MemberAccountToken{}, &models.AppAccountBilling{}, &models.Blacklist{}, &models.AppVersion{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	database.SetDB(db)
	ResetSettingsService()
	seedTestQuota(db)

	// 关闭连接池，否则 TempDir 清理时文件仍被占用
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// TestActivateCompletesWithSingleConn 单连接下激活必须能走完。
//
// 回归点：点数计费的单价是**惰性**加载的（Billing.loadLevelPoints），
// 若它在事务内部才第一次触发，就会调用 database.GetDB() 抢连接，
// 与持有唯一连接的事务互等，表现为「请求卡死 + 事务锁不释放」。
func TestActivateCompletesWithSingleConn(t *testing.T) {
	db := setupSingleConnDB(t)

	app := models.App{
		UUID:              "APP-SC",
		Name:              "单连接应用",
		Secret:            "SECRET-SC",
		Status:            1,
		BillPointsEnabled: 1,
		PointsChargeMode:  models.PointsChargePerCount,
		PointsPerLogin:    3,
		PointsPerPeriod:   1,
		PointsPeriodMinutes: 60,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("seed app: %v", err)
	}

	// 等级点数也配一条：确保惰性加载路径确实会被走到
	if err := SaveAppLevelPoints(app.UUID, map[string]float64{"SOME-LEVEL": 2}); err != nil {
		t.Fatalf("seed level points: %v", err)
	}

	m := models.Member{
		Username:  "sc@test.com",
		Type:      models.MemberTypeRegister,
		Status:    models.MemberStatusNormal,
		Points:    10,
		ExpiredAt: time.Now().Add(24 * time.Hour),
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}

	acct, err := IssueAccountToken(db, m.UUID, "FP-SC", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := Activate(db, acct, &app, "FP-SC", "1.2.3.4", "1.0.0", "dev")
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("activate: %v", err)
		}
	case <-time.After(10 * time.Second):
		dumpGoroutines(t)
		t.Fatalf("激活在单连接池下卡死 10s：事务内部又去抢 database.GetDB() 的连接，导致事务永不提交、写锁不释放")
	}

	// 扣费应该真的发生：10 - 3 = 7
	var after models.Member
	if err := db.Where("uuid = ?", m.UUID).First(&after).Error; err != nil {
		t.Fatalf("reload member: %v", err)
	}
	if after.Points != 7 {
		t.Fatalf("points should be 7 after charge, got %v", after.Points)
	}
}

// TestRenewLeaseCompletesWithSingleConn 单连接下续约同样必须能走完。
func TestRenewLeaseCompletesWithSingleConn(t *testing.T) {
	db := setupSingleConnDB(t)

	app := models.App{
		UUID:                "APP-RN",
		Name:                "单连接续约",
		Secret:              "SECRET-RN",
		Status:              1,
		BillPointsEnabled:   1,
		PointsChargeMode:    models.PointsChargePerTime,
		PointsPerLogin:      1,
		PointsPerPeriod:     2,
		PointsPeriodMinutes: 60,
		LeaseTTLMinutes:     30,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("seed app: %v", err)
	}
	if err := SaveAppLevelPoints(app.UUID, map[string]float64{"SOME-LEVEL": 2}); err != nil {
		t.Fatalf("seed level points: %v", err)
	}

	m := models.Member{
		Username:  "rn@test.com",
		Type:      models.MemberTypeRegister,
		Status:    models.MemberStatusNormal,
		Points:    10,
		ExpiredAt: time.Now().Add(24 * time.Hour),
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	acct, err := IssueAccountToken(db, m.UUID, "FP-RN", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	info, err := Activate(db, acct, &app, "FP-RN", "1.2.3.4", "1.0.0", "dev")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, _, _, err := RenewLease(db, info.Token)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("renew: %v", err)
		}
	case <-time.After(10 * time.Second):
		dumpGoroutines(t)
		t.Fatalf("续约在单连接池下卡死 10s：事务内部再次惰性加载等级点数，抢不到连接")
	}
}

// dumpGoroutines 卡死时打印全部协程栈，直接看是谁在等连接。
func dumpGoroutines(t *testing.T) {
	t.Helper()
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	t.Logf("==== goroutine dump ====\n%s", buf[:n])
}