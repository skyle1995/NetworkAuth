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

// User 用户表模型
// 存储所有账号，包括超级管理员（Role=0）和子账号（Role=1等）
// CreatedAt/UpdatedAt 由 GORM 自动维护
type User struct {
	ID           uint      `gorm:"primaryKey;comment:账号ID，自增主键"`
	UUID         string    `gorm:"uniqueIndex;size:36;not null;comment:唯一标识符" json:"uuid"`
	Username     string    `gorm:"uniqueIndex;size:64;not null;comment:账号名，唯一索引" json:"username"`
	Password     string    `gorm:"size:255;not null;comment:密码哈希值"`
	PasswordSalt string    `gorm:"size:64;not null;comment:密码加密盐值"`
	Status       int       `gorm:"not null;default:1;comment:状态：0禁用，1启用" json:"status"`
	Role         int       `gorm:"not null;default:2;comment:角色类型：0超级管理员，1代理成员，2普通成员" json:"role"`
	Permissions  string    `gorm:"size:255;comment:权限列表，逗号分隔" json:"permissions"`
	Nickname     string    `gorm:"size:64;comment:用户昵称" json:"nickname"`
	Remark       string    `gorm:"size:255;comment:备注信息" json:"remark"`
	Avatar       string    `gorm:"size:255;comment:用户头像URL" json:"avatar"`
	CreatedAt    time.Time `gorm:"autoCreateTime;comment:创建时间" json:"created_at"`
	UpdatedAt    time.Time `gorm:"comment:更新时间"`
}

// ============================================================================
// 结构体方法
// ============================================================================

// BeforeCreate 在创建记录前自动生成UUID
func (user *User) BeforeCreate(tx *gorm.DB) error {
	// 生成UUID
	if user.UUID == "" {
		user.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}
