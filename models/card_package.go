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

// 套餐类型：决定面值语义，须与应用运营模式一致
const (
	PackageTypeTime   = 0 // 时长套餐（面值取 Duration）
	PackageTypePoints = 1 // 点数套餐（面值取 Points）
)

// ============================================================================
// 结构体定义
// ============================================================================

// CardPackage 卡密套餐：制卡的售卖单元，决定卡密面值与售价。
// 制卡时把 Duration/Points/Price 快照进 Card，套餐后续改动不影响已售出的卡。
type CardPackage struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:套餐ID，自增主键" json:"id"`

	// UUID：套餐唯一标识符，自动生成
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:套餐UUID" json:"uuid"`

	// AppUUID：归属应用UUID
	AppUUID string `gorm:"size:36;not null;index;comment:归属应用UUID" json:"app_uuid"`

	// Name：套餐名称（如「月卡」「1000点」）
	Name string `gorm:"size:100;not null;comment:套餐名称" json:"name"`

	// Type：套餐类型（0=时长，1=点数）
	Type int `gorm:"default:0;not null;comment:套餐类型，0=时长，1=点数" json:"type"`

	// Duration：面值时长（分钟），-1 为永久。Type=0 时使用
	Duration int `gorm:"default:0;not null;comment:面值时长，单位分钟，-1为永久" json:"duration"`

	// Points：面值点数。Type=1 时使用
	Points int `gorm:"default:0;not null;comment:面值点数" json:"points"`

	// Price：售价（单位：分）。累充升级按此计量，用整数分避免浮点误差
	Price int `gorm:"default:0;not null;comment:售价，单位分" json:"price"`

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
func (p *CardPackage) BeforeCreate(tx *gorm.DB) error {
	if p.UUID == "" {
		p.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}

// TableName 指定表名
func (CardPackage) TableName() string {
	return "card_packages"
}
