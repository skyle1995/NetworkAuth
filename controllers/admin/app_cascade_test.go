package admin

import (
	"NetworkAuth/models"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestCascadeDeleteAppData 验证删除应用时的清理边界（一号通）：
// 只删该应用专属的会话/日志/变量/函数；
// **账号、卡密、接口是全站共享资产，绝不随应用删除**。
func TestCascadeDeleteAppData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.App{}, &models.API{}, &models.Card{},
		&models.Member{}, &models.Binding{}, &models.AppActivation{},
		&models.MemberLog{}, &models.Variable{}, &models.Function{}, &models.MemberAccountToken{}, &models.AppActivation{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	mustCreate := func(v any) {
		if err := db.Create(v).Error; err != nil {
			t.Fatalf("create %T: %v", v, err)
		}
	}

	// —— APP-1 及其专属数据 ——
	mustCreate(&models.App{UUID: "APP-1", Name: "待删", Secret: "S"})
	member := models.Member{Username: "u1"}
	mustCreate(&member)
	mustCreate(&models.Binding{MemberUUID: member.UUID, Type: models.BindingTypeMachine, Value: "MC-1"})
	mustCreate(&models.AppActivation{Token: "t1", MemberUUID: member.UUID, AppUUID: "APP-1"})
	mustCreate(&models.MemberLog{AppUUID: "APP-1", Username: "u1", Action: "卡密登录"})

	// —— 全站共享资产：账号、卡密、接口 ——
	mustCreate(&models.Card{CardNo: "K1"})
	mustCreate(&models.API{APIType: 10})

	// —— 变量/函数是全站资源，不属于任何应用，删应用不应动它们 ——
	time.Sleep(2 * time.Millisecond)
	mustCreate(&models.Variable{Alias: "gv"})
	time.Sleep(2 * time.Millisecond)
	mustCreate(&models.Function{Alias: "gf"})
	mustCreate(&models.App{UUID: "APP-2", Name: "保留", Secret: "S2"})
	mustCreate(&models.Member{Username: "o1"})
	mustCreate(&models.AppActivation{Token: "t2", MemberUUID: member.UUID, AppUUID: "APP-2"})

	if err := cascadeDeleteAppData(db, []string{"APP-1"}); err != nil {
		t.Fatalf("cascadeDeleteAppData: %v", err)
	}

	count := func(model any, cond string, args ...any) int64 {
		var n int64
		q := db.Model(model)
		if cond != "" {
			q = q.Where(cond, args...)
		}
		q.Count(&n)
		return n
	}

	// 应用专属数据应清空
	checks := []struct {
		name string
		got  int64
	}{
		{"session(APP-1)", count(&models.AppActivation{}, "app_uuid = ?", "APP-1")},
		{"memberlog(APP-1)", count(&models.MemberLog{}, "app_uuid = ?", "APP-1")},

	}
	for _, c := range checks {
		if c.got != 0 {
			t.Fatalf("%s should be cascade-deleted, got %v", c.name, c.got)
		}
	}

	// 全站共享资产必须存活（这是本次改造最关键的回归点）
	if count(&models.Member{}, "") != 2 {
		t.Fatalf("members are global assets and must survive, got %v", count(&models.Member{}, ""))
	}
	if count(&models.Binding{}, "") != 1 {
		t.Fatalf("bindings belong to accounts and must survive, got %v", count(&models.Binding{}, ""))
	}
	if count(&models.Variable{}, "") != 1 {
		t.Fatalf("variables are site-wide resources and must survive app deletion, got %v", count(&models.Variable{}, ""))
	}
	if count(&models.Function{}, "") != 1 {
		t.Fatalf("functions are site-wide resources and must survive app deletion, got %v", count(&models.Function{}, ""))
	}
	if count(&models.Card{}, "") != 1 {
		t.Fatalf("cards are global pool and must survive, got %v", count(&models.Card{}, ""))
	}
	if count(&models.API{}, "") != 1 {
		t.Fatalf("apis are global definitions and must survive, got %v", count(&models.API{}, ""))
	}
	if count(&models.AppActivation{}, "app_uuid = ?", "APP-2") != 1 {
		t.Fatalf("other app's session should survive")
	}
}
