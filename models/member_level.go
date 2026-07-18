package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// 结构体定义
// ============================================================================

// MemberLevel 会员等级：按累计充值金额自动升级，权益为充值返利。
// 账号累充（Member.TotalRecharge）达到 Threshold 即升到该等级，只升不降。
type MemberLevel struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:等级ID，自增主键" json:"id"`

	// UUID：等级唯一标识符，自动生成
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:等级UUID" json:"uuid"`

	// AppUUID：归属应用UUID
	AppUUID string `gorm:"size:36;not null;index;comment:归属应用UUID" json:"app_uuid"`

	// Name：等级名称（如「白银」「黄金」）
	Name string `gorm:"size:100;not null;comment:等级名称" json:"name"`

	// Threshold：累充金额门槛（单位：分），累充达到即升级
	Threshold int `gorm:"default:0;not null;index;comment:累充金额门槛，单位分" json:"threshold"`

	// RebateRate：充值返利比例（百分比），如 10 表示多返 10%
	RebateRate int `gorm:"default:0;not null;comment:充值返利比例，百分比" json:"rebate_rate"`

	// ExtraMultiOpen：额外多开数。有效多开 = App.MultiOpenCount + 本值
	ExtraMultiOpen int `gorm:"default:0;not null;comment:额外多开数" json:"extra_multi_open"`

	// ExtraRebindCount：赠送免费换绑次数。有效免费次数 = App 免费次数 + 本值（机器/IP 共用）
	ExtraRebindCount int `gorm:"default:0;not null;comment:赠送免费换绑次数" json:"extra_rebind_count"`

	// Sort：排序值，越小越靠前
	Sort int `gorm:"default:0;not null;comment:排序值" json:"sort"`

	// Status：状态（0=禁用，1=启用）
	Status int `gorm:"default:1;not null;comment:状态，0=禁用，1=启用" json:"status"`

	// Remark：备注信息
	Remark string `gorm:"size:255;comment:备注信息" json:"remark"`

	// 时间字段
	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// ============================================================================
// 结构体方法
// ============================================================================

// BeforeCreate 在创建记录前自动生成UUID
func (l *MemberLevel) BeforeCreate(tx *gorm.DB) error {
	if l.UUID == "" {
		l.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}

// TableName 指定表名
func (MemberLevel) TableName() string {
	return "member_levels"
}
