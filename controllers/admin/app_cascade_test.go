package admin

import (
	"NetworkAuth/models"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCascadeDeleteAppData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.App{}, &models.API{}, &models.Card{},
		&models.Member{}, &models.Binding{}, &models.MemberSession{},
		&models.MemberLog{}, &models.Variable{}, &models.Function{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	mustCreate := func(v any) {
		if err := db.Create(v).Error; err != nil {
			t.Fatalf("create %T: %v", v, err)
		}
	}

	// —— APP-1 及其全部衍生数据 ——
	mustCreate(&models.App{UUID: "APP-1", Name: "待删", Secret: "S"})
	member := models.Member{AppUUID: "APP-1", Username: "u1"}
	mustCreate(&member)
	mustCreate(&models.Binding{MemberUUID: member.UUID, Type: models.BindingTypeMachine, Value: "MC-1"})
	mustCreate(&models.MemberSession{Token: "t1", MemberUUID: member.UUID, AppUUID: "APP-1"})
	mustCreate(&models.MemberLog{AppUUID: "APP-1", Username: "u1", Action: "卡密登录"})
	mustCreate(&models.Card{CardNo: "K1", AppUUID: "APP-1"})
	mustCreate(&models.API{AppUUID: "APP-1", APIType: 10})
	mustCreate(&models.Variable{Alias: "v1", AppUUID: "APP-1"})
	time.Sleep(2 * time.Millisecond) // Variable/Function.Number 为毫秒唯一索引
	mustCreate(&models.Function{Alias: "f1", AppUUID: "APP-1"})

	// —— 应保留：全局变量/函数(app_uuid="0") 与其它应用 ——
	time.Sleep(2 * time.Millisecond)
	mustCreate(&models.Variable{Alias: "gv", AppUUID: "0"})
	time.Sleep(2 * time.Millisecond)
	mustCreate(&models.Function{Alias: "gf", AppUUID: "0"})
	mustCreate(&models.App{UUID: "APP-2", Name: "保留", Secret: "S2"})
	mustCreate(&models.Member{AppUUID: "APP-2", Username: "o1"})

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

	// APP-1 衍生数据应清空
	checks := []struct {
		name string
		got  int64
	}{
		{"card", count(&models.Card{}, "app_uuid = ?", "APP-1")},
		{"member", count(&models.Member{}, "app_uuid = ?", "APP-1")},
		{"binding", count(&models.Binding{}, "member_uuid = ?", member.UUID)},
		{"session", count(&models.MemberSession{}, "app_uuid = ?", "APP-1")},
		{"memberlog", count(&models.MemberLog{}, "app_uuid = ?", "APP-1")},
		{"api", count(&models.API{}, "app_uuid = ?", "APP-1")},
		{"variable", count(&models.Variable{}, "app_uuid = ?", "APP-1")},
		{"function", count(&models.Function{}, "app_uuid = ?", "APP-1")},
	}
	for _, c := range checks {
		if c.got != 0 {
			t.Fatalf("%s should be cascade-deleted, got %d", c.name, c.got)
		}
	}

	// 全局与其它应用应保留
	if count(&models.Variable{}, "app_uuid = ?", "0") != 1 {
		t.Fatalf("global variable should survive")
	}
	if count(&models.Function{}, "app_uuid = ?", "0") != 1 {
		t.Fatalf("global function should survive")
	}
	if count(&models.Member{}, "app_uuid = ?", "APP-2") != 1 {
		t.Fatalf("other app's member should survive")
	}
}
