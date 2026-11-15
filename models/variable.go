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

// Variable 变量表模型
// 用于管理应用程序的变量数据
// CreatedAt/UpdatedAt 由 GORM 自动维护
type Variable struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:变量ID，自增主键" json:"id"`

	// UUID：变量的唯一标识符，36位字符串
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:变量的唯一标识符" json:"uuid"`

	// Number：变量编号，13位Unix时间戳（毫秒级）
	Number string `gorm:"uniqueIndex;size:16;not null;comment:变量编号，13位Unix毫秒 + 3位随机" json:"number"`


	// AppUUID：变量所属应用；空串=全局变量（所有应用都能读到）
	AppUUID string `gorm:"index;size:36;comment:所属应用UUID；空=全局" json:"app_uuid"`

	// Alias：变量别名；同一应用内唯一（配合 AppUUID 组成唯一键）
	Alias string `gorm:"index;size:100;not null;comment:变量别名" json:"alias"`

	// Data：变量数据内容
	Data string `gorm:"type:text;comment:变量数据" json:"data"`

	// Remark：备注信息，用于描述变量用途
	Remark string `gorm:"type:text;comment:备注信息" json:"remark"`

	// 时间字段
	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// ============================================================================
// 结构体方法
// ============================================================================

// BeforeCreate 在创建记录前自动生成UUID和Number
func (variable *Variable) BeforeCreate(tx *gorm.DB) error {
	// 生成UUID
	if variable.UUID == "" {
		variable.UUID = strings.ToUpper(uuid.New().String())
	}

	// 生成Number：13 位 Unix 毫秒 + 3 位随机（共 16 位）
	//
	// 纯毫秒戳在同一毫秒内并发创建会撞唯一索引，补 3 位随机把概率压到 1/1000。
	variable.Number = fmt.Sprintf("%d%03d", time.Now().UnixMilli(), rand.Intn(1000))
	return nil
}

// TableName 指定表名
func (Variable) TableName() string {
	return "variables"
}
