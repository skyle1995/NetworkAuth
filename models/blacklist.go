package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// 常量定义
// ============================================================================

// 黑名单类型
const (
	BlacklistTypeMachine = 0 // 机器码（设备）
	BlacklistTypeIP      = 1 // IP 地址
	BlacklistTypeRegion  = 2 // 地区（省+市，地级）
)

// ============================================================================
// 结构体定义
// ============================================================================

// Blacklist 黑名单表模型
// 按应用维度封禁「设备(机器码)/IP/地区」。登录收尾时命中任一即拒绝，
// 与账号级黑名单（Member.Status=2）互补：账号黑名单封的是人，本表封的是设备/网络/地域。
// 地区类型以「省+市」为粒度（地级），Value 存 "省/市" 供唯一约束与展示，Province/City 供匹配。
type Blacklist struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:黑名单ID，自增主键" json:"id"`

	// UUID：唯一标识符，自动生成
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:黑名单UUID" json:"uuid"`

	// AppUUID：归属应用UUID，与 Type+Value 联合唯一
	AppUUID string `gorm:"size:36;not null;index;uniqueIndex:idx_blacklist_app_type_value;comment:归属应用UUID" json:"app_uuid"`

	// Type：黑名单类型（0=机器码，1=IP，2=地区）
	Type int `gorm:"not null;uniqueIndex:idx_blacklist_app_type_value;comment:类型，0=机器码，1=IP，2=地区" json:"type"`

	// Value：命中值（机器码/IP原值；地区为 "省/市"）
	Value string `gorm:"size:255;not null;uniqueIndex:idx_blacklist_app_type_value;comment:命中值" json:"value"`

	// Province：省份（IP归属地展示 / 地区类型匹配）
	Province string `gorm:"size:64;comment:省份" json:"province"`

	// City：城市（IP归属地展示 / 地区类型匹配）
	City string `gorm:"size:64;comment:城市" json:"city"`

	// MemberUUID：来源账号UUID（从某账号拉黑而来时记录，可空）
	MemberUUID string `gorm:"size:36;index;comment:来源账号UUID" json:"member_uuid"`

	// Username：来源账号名（冗余，便于列表展示来源）
	Username string `gorm:"size:255;comment:来源账号名" json:"username"`

	// Remark：备注
	Remark string `gorm:"size:255;comment:备注" json:"remark"`

	// CreatedAt：创建时间，由 GORM 自动维护
	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
}

// ============================================================================
// 结构体方法
// ============================================================================

// BeforeCreate 在创建记录前自动生成UUID
func (b *Blacklist) BeforeCreate(tx *gorm.DB) error {
	if b.UUID == "" {
		b.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}

// TableName 指定表名
func (Blacklist) TableName() string {
	return "blacklists"
}
