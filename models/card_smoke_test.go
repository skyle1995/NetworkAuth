package models

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCardMemberBindingMigrateAndCreate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := db.AutoMigrate(&App{}, &Card{}, &API{}, &Member{}, &Binding{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	// 卡号生成：批量去重 + 格式
	codes, err := GenerateCardNos("KM", 16, 500)
	if err != nil {
		t.Fatalf("GenerateCardNos: %v", err)
	}
	if len(codes) != 500 {
		t.Fatalf("want 500 codes, got %d", len(codes))
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if !strings.HasPrefix(code, "KM") {
			t.Fatalf("code missing prefix: %s", code)
		}
		if len(code) != len("KM")+16 {
			t.Fatalf("unexpected code length: %s", code)
		}
		if strings.ContainsAny(code[2:], "01OI") {
			t.Fatalf("code contains confusable char: %s", code)
		}
		if seen[code] {
			t.Fatalf("duplicate code: %s", code)
		}
		seen[code] = true
	}

	// 建卡 + BeforeCreate 生成 UUID
	card := Card{CardNo: codes[0], Duration: 60, Status: CardStatusUnused}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card: %v", err)
	}
	if card.UUID == "" {
		t.Fatalf("card UUID not generated")
	}

	// card_no 全站唯一：同卡号重复插入应报错（一号通：卡密不再按应用隔离）
	dup := Card{CardNo: codes[0], Duration: 60}
	if err := db.Create(&dup).Error; err == nil {
		t.Fatalf("expected unique-index violation for duplicate card_no")
	}

	// username 全站唯一
	m := Member{Username: codes[0], Type: MemberTypeCard, CardUUID: card.UUID, ExpiredAt: PermanentTime}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	dupM := Member{Username: codes[0]}
	if err := db.Create(&dupM).Error; err == nil {
		t.Fatalf("expected unique-index violation for duplicate username")
	}

	// (member_uuid, type, value) 联合唯一
	b := Binding{MemberUUID: m.UUID, Type: BindingTypeMachine, Value: "MACHINE-1"}
	if err := db.Create(&b).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}
	dupB := Binding{MemberUUID: m.UUID, Type: BindingTypeMachine, Value: "MACHINE-1"}
	if err := db.Create(&dupB).Error; err == nil {
		t.Fatalf("expected unique-index violation for duplicate binding")
	}

	// api_type 全站唯一（一号通：接口条目不再按应用复制）
	api := API{APIType: 10, Status: 1}
	if err := db.Create(&api).Error; err != nil {
		t.Fatalf("create api: %v", err)
	}
	dupAPI := API{APIType: 10, Status: 1}
	if err := db.Create(&dupAPI).Error; err == nil {
		t.Fatalf("expected unique-index violation for duplicate api_type")
	}
}
