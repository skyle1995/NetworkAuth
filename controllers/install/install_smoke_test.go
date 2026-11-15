package install

import (
	"strconv"
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ============================================================================
// 全新安装回归测试
// ============================================================================
//
// 一号通改造「不做数据迁移、按全新安装设计」，安装流程成了唯一的数据入口。
// 一旦 install 里漏调某个 seed，新库就会处于「有表无数据」状态：
// 后台接口页空白、/api/open 全部返回「接口未配置」，且没有历史数据可参照。
//
// 本测试按 AutoMigrate → SeedDefaultSettings → SeedDefaultPortalNavigation
// → EnsureGlobalAPIs 的顺序，在空内存库上复现安装流程，逐项校验结果。

// runFreshInstall 在空库上执行与 InstallSubmitHandler 一致的数据初始化步骤。
func runFreshInstall(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	database.SetDB(db)
	services.ResetSettingsService()

	if err := database.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := database.SeedDefaultSettings(); err != nil {
		t.Fatalf("SeedDefaultSettings: %v", err)
	}
	if err := database.SeedDefaultPortalNavigation(); err != nil {
		t.Fatalf("SeedDefaultPortalNavigation: %v", err)
	}
	services.EnsureGlobalAPIs()

	return db
}

// TestFreshInstallCreatesGlobalAPIs 安装后 27 条接口就绪且默认启用。
func TestFreshInstallCreatesGlobalAPIs(t *testing.T) {
	db := runFreshInstall(t)

	want := len(models.GetDefaultAPITypes())
	if want == 0 {
		t.Fatal("GetDefaultAPITypes() 为空，无法校验")
	}

	var count int64
	db.Model(&models.API{}).Count(&count)
	if int(count) != want {
		t.Fatalf("want %v apis after fresh install, got %v", want, count)
	}

	var disabled int64
	db.Model(&models.API{}).Where("status <> ?", 1).Count(&disabled)
	if disabled != 0 {
		t.Fatalf("all %v apis should be enabled after install, %v disabled", want, disabled)
	}

	// 接口定义全站唯一：不应存在 app_uuid 列
	if db.Migrator().HasColumn(&models.API{}, "app_uuid") {
		t.Fatal("apis.app_uuid should not exist after unified-account refactor")
	}
}

// TestFreshInstallSeedsAccountPolicy 安装后 14 项全局账号策略齐备且关键默认值正确。
func TestFreshInstallSeedsAccountPolicy(t *testing.T) {
	db := runFreshInstall(t)

	names := models.GlobalSettingNames()
	if len(names) == 0 {
		t.Fatal("GlobalSettingNames() 为空")
	}
	for _, name := range names {
		var n int64
		db.Model(&models.Settings{}).Where("name = ?", name).Count(&n)
		if n != 1 {
			t.Fatalf("setting %q should exist exactly once after install, got %v", name, n)
		}
	}

	p := services.GetAccountPolicy()
	// 注册次数必须 > 1：全站计数下填 1 会让同一 IP 注册一个号后再也注册不了
	if p.RegisterCount != 3 {
		t.Fatalf("default register_count should be 3 (full-site counting), got %v", p.RegisterCount)
	}
	// 卡密登录/充值默认开启，注册默认开启，试用默认关闭
	if !p.RegisterEnabled || !p.CardLoginEnabled || !p.RechargeEnabled {
		t.Fatalf("register/card-login/recharge should default to enabled, got %+v", p)
	}
	if p.TrialEnabled {
		t.Fatalf("trial should default to disabled")
	}
	if p.HasTrialQuota() {
		t.Fatalf("trial quota should be empty by default, got points=%s minutes=%v",
			strconv.FormatFloat(p.TrialPoints, 'f', -1, 64), p.TrialDurationMinutes)
	}
}

// TestFreshInstallAppSchema apps 表结构符合一号通：有计费三字段，无 13 项账号策略字段。
func TestFreshInstallAppSchema(t *testing.T) {
	db := runFreshInstall(t)

	present := []string{"free_mode", "billing_mode", "required_level",
		"points_charge_mode", "points_per_login", "points_period_minutes",
		"points_per_period", "points_heartbeat_charge"}
	for _, col := range present {
		if !db.Migrator().HasColumn(&models.App{}, col) {
			t.Fatalf("apps.%s should exist", col)
		}
	}

	// 这 13 项已迁到全局 settings，不应再出现在 apps 表
	removed := []string{
		"operation_mode",
		"register_enabled", "email_verify_enabled", "card_register_enabled",
		"register_limit_enabled", "register_device_limit_enabled",
		"register_limit_time", "register_count",
		"card_login_enabled", "recharge_enabled",
		"trial_enabled", "trial_limit_time", "trial_duration", "trial_claim_mode",
	}
	for _, col := range removed {
		if db.Migrator().HasColumn(&models.App{}, col) {
			t.Fatalf("apps.%s should be removed (moved to global settings or replaced)", col)
		}
	}

	// 新建应用要有可用的计费默认值
	app := models.App{UUID: "APP-DEFAULT", Name: "默认", Secret: "S", Status: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	var got models.App
	db.Where("uuid = ?", "APP-DEFAULT").First(&got)
	if got.BillingMode != models.BillingModeMember {
		t.Fatalf("default billing_mode should be %v (member), got %v", models.BillingModeMember, got.BillingMode)
	}
	if got.RequiredLevel != models.DefaultMemberLevel {
		t.Fatalf("default required_level should be %v, got %v", models.DefaultMemberLevel, got.RequiredLevel)
	}
	if got.FreeMode != 0 {
		t.Fatalf("default free_mode should be 0, got %v", got.FreeMode)
	}
}

// TestFreshInstallGlobalTablesHaveNoAppUUID 全局资产表不含 app_uuid。
func TestFreshInstallGlobalTablesHaveNoAppUUID(t *testing.T) {
	db := runFreshInstall(t)

	for _, m := range []any{
		&models.Member{}, &models.Card{}, &models.CardPackage{},
		&models.MemberLevel{}, &models.Blacklist{},
	} {
		if db.Migrator().HasColumn(m, "app_uuid") {
			t.Fatalf("%T should not carry app_uuid after unified-account refactor", m)
		}
	}
	// 卡密是全站卡池：既无 app_uuid，也无发卡渠道之类的归属字段
	if db.Migrator().HasColumn(&models.Card{}, "source_app_uuid") {
		t.Fatal("cards.source_app_uuid should be gone: cards are site-wide")
	}

	// 会话与日志保留 app_uuid，作为「来源应用」
	if !db.Migrator().HasColumn(&models.AppActivation{}, "app_uuid") {
		t.Fatal("member_sessions.app_uuid should be kept as source marker")
	}
	if !db.Migrator().HasColumn(&models.MemberLog{}, "app_uuid") {
		t.Fatal("member_logs.app_uuid should be kept as source marker")
	}
}

// TestFreshInstallUniqueIndexes 唯一约束在建表时一次定义正确。
func TestFreshInstallUniqueIndexes(t *testing.T) {
	db := runFreshInstall(t)

	// members.username 全站唯一
	if err := db.Create(&models.Member{Username: "dup@test.com"}).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	if err := db.Create(&models.Member{Username: "dup@test.com"}).Error; err == nil {
		t.Fatal("duplicate member.username should violate unique index")
	}

	// cards.card_no 全站唯一
	if err := db.Create(&models.Card{CardNo: "KM-DUP", Duration: 60}).Error; err != nil {
		t.Fatalf("create card: %v", err)
	}
	if err := db.Create(&models.Card{CardNo: "KM-DUP", Duration: 60}).Error; err == nil {
		t.Fatal("duplicate card.card_no should violate unique index")
	}

	// blacklists (type, value) 复合唯一
	b := models.Blacklist{Type: models.BlacklistTypeMachine, Value: "MC-DUP"}
	if err := db.Create(&b).Error; err != nil {
		t.Fatalf("create blacklist: %v", err)
	}
	b2 := models.Blacklist{Type: models.BlacklistTypeMachine, Value: "MC-DUP"}
	if err := db.Create(&b2).Error; err == nil {
		t.Fatal("duplicate blacklist (type, value) should violate unique index")
	}

	// app_level_points (app_uuid, member_level_uuid) 复合唯一
	lp := models.AppLevelPoint{AppUUID: "APP-DEFAULT", MemberLevelUUID: "LV-1", Points: 5}
	if err := db.Create(&lp).Error; err != nil {
		t.Fatalf("create app_level_point: %v", err)
	}
	lp2 := models.AppLevelPoint{AppUUID: "APP-DEFAULT", MemberLevelUUID: "LV-1", Points: 9}
	if err := db.Create(&lp2).Error; err == nil {
		t.Fatal("duplicate (app_uuid, member_level_uuid) should violate unique index")
	}
}

// TestFreshInstallAPILookupWithoutApp 新库上按 api_type 即可定位接口配置，
// 说明「接口查询不再依赖 app_uuid」这条链路在安装后立即可用。
func TestFreshInstallAPILookupWithoutApp(t *testing.T) {
	db := runFreshInstall(t)

	var api models.API
	if err := db.Where("api_type = ?", models.APITypeSingleLogin).First(&api).Error; err != nil {
		t.Fatalf("api_type %v should exist right after install: %v", models.APITypeSingleLogin, err)
	}
	if api.Status != 1 {
		t.Fatalf("card-login api should be enabled by default, got status %v", api.Status)
	}
}
