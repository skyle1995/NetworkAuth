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

// 绑定类型
const (
	BindingTypeMachine = 0 // 机器码绑定
	BindingTypeIP      = 1 // IP地址绑定
)

// ============================================================================
// 结构体定义
// ============================================================================

// Binding 终端用户绑定表模型
// 存储终端用户的机器码/IP 绑定关系，支撑多开与转绑逻辑。
// 一个用户可有多条绑定（受 App 的 MultiOpenCount 限制），因此单独成表。
// 卡密账号与注册账号的绑定统一归此表，owner 恒为终端用户，运行时逻辑无需区分来源。
// CreatedAt/UpdatedAt 由 GORM 自动维护
type Binding struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:绑定ID，自增主键" json:"id"`

	// UUID：绑定唯一标识符，自动生成
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:绑定UUID，唯一标识符" json:"uuid"`

	// MemberUUID：归属终端用户UUID，与 Type+Value 联合唯一
	MemberUUID string `gorm:"size:36;not null;index;uniqueIndex:idx_binding_member_type_value;comment:归属终端用户UUID" json:"member_uuid"`

	// Type：绑定类型（0=机器码，1=IP地址）
	Type int `gorm:"not null;uniqueIndex:idx_binding_member_type_value;comment:绑定类型，0=机器码，1=IP" json:"type"`

	// Value：绑定值（机器码或IP地址）
	Value string `gorm:"size:255;not null;uniqueIndex:idx_binding_member_type_value;comment:绑定值，机器码或IP" json:"value"`

	// Province：IP归属省份（IP绑定时记录，支撑省级验证）
	Province string `gorm:"size:64;comment:IP归属省份" json:"province"`

	// City：IP归属城市（IP绑定时记录，支撑市级验证）
	City string `gorm:"size:64;comment:IP归属城市" json:"city"`

	// 时间字段
	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// ============================================================================
// 结构体方法
// ============================================================================

// BeforeCreate 在创建记录前自动生成UUID
func (binding *Binding) BeforeCreate(tx *gorm.DB) error {
	if binding.UUID == "" {
		binding.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}

// TableName 指定表名
func (Binding) TableName() string {
	return "bindings"
}
