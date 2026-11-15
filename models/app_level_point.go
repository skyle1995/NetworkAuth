package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AppLevelPoint 高级模式：某应用下「某会员等级」每次计费消耗的点数。
//
// 背景：高级模式下不同等级应付不同点数（等级越高越便宜），
// 因此点数额不能挂在应用上，必须按 (应用, 等级) 配置一条。
//
// 查找规则（见 services.Billing.PointsCostForLevel）：
//   - 精确命中 (app_uuid, level_uuid) 用该条的 Points
//   - 未配置则回退到 App.PointsPerLogin / App.PointsPerPeriod
//
// 唯一键 (app_uuid, level_uuid)：一个应用对同一等级只能有一条配置。
type AppLevelPoint struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:配置ID，自增主键" json:"id"`

	// UUID：唯一标识符，自动生成
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:配置UUID" json:"uuid"`

	// AppUUID：所属应用
	AppUUID string `gorm:"size:36;not null;index;uniqueIndex:idx_app_level_point;comment:所属应用UUID" json:"app_uuid"`

	// MemberLevelUUID：对应的全站会员等级（MemberLevel.UUID）
	MemberLevelUUID string `gorm:"size:36;not null;index;uniqueIndex:idx_app_level_point;comment:会员等级UUID" json:"member_level_uuid"`

	// Points：该等级在本应用每次计费消耗的点数（按次=每次登录；按时=每周期）
	Points float64 `gorm:"default:0;not null;comment:该等级在本应用的点数消耗（支持小数）" json:"points"`

	// Remark：备注
	Remark string `gorm:"size:255;comment:备注" json:"remark"`

	// 时间字段
	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// BeforeCreate 在创建记录前自动生成UUID
func (p *AppLevelPoint) BeforeCreate(tx *gorm.DB) error {
	if p.UUID == "" {
		p.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}

// TableName 指定表名
func (AppLevelPoint) TableName() string {
	return "app_level_points"
}
