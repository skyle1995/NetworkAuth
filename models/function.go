package models

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// 结构体定义
// ============================================================================

// Function 函数表模型
// 用于管理应用程序的函数代码
// CreatedAt/UpdatedAt 由 GORM 自动维护
type Function struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:函数ID，自增主键" json:"id"`

	// UUID：函数的唯一标识符，36位字符串
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:函数的唯一标识符" json:"uuid"`

	// Number：函数编号，13位Unix时间戳（毫秒级）
	Number string `gorm:"uniqueIndex;size:16;not null;comment:函数编号，13位Unix毫秒 + 3位随机" json:"number"`


	// Alias：函数别名，便于识别和管理
	Alias string `gorm:"uniqueIndex;size:100;not null;comment:函数别名" json:"alias"`

	// Code：函数代码内容
	Code string `gorm:"type:text;comment:函数代码" json:"code"`

	// Remark：备注信息，用于描述函数用途
	Remark string `gorm:"type:text;comment:备注信息" json:"remark"`

	// 时间字段
	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// ============================================================================
// 结构体方法
// ============================================================================

// BeforeCreate 在创建记录前自动生成UUID和Number
func (function *Function) BeforeCreate(tx *gorm.DB) error {
	// 生成UUID
	if function.UUID == "" {
		function.UUID = strings.ToUpper(uuid.New().String())
	}

	// 生成Number：13 位 Unix 毫秒 + 3 位随机（共 16 位）
	function.Number = fmt.Sprintf("%d%03d", time.Now().UnixMilli(), rand.Intn(1000))
	return nil
}

// TableName 指定表名
func (Function) TableName() string {
	return "functions"
}
