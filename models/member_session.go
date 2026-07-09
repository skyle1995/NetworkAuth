package models

import (
	"time"
)

// ============================================================================
// 结构体定义
// ============================================================================

// MemberSession 账号会话
// 支持多开：一个用户可同时存在多个会话（受 App.MultiOpenCount 限制）。
// 令牌即会话标识；心跳刷新 LastActiveAt，超过 App.CheckInterval 未活跃视为失效。
type MemberSession struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:会话ID，自增主键" json:"id"`

	// Token：会话令牌，唯一
	Token string `gorm:"uniqueIndex;size:64;not null;comment:会话令牌" json:"token"`

	// MemberUUID：所属账号UUID
	MemberUUID string `gorm:"size:36;not null;index;comment:所属账号UUID" json:"member_uuid"`

	// AppUUID：归属应用UUID（冗余，便于按应用清理与鉴权）
	AppUUID string `gorm:"size:36;not null;index;comment:归属应用UUID" json:"app_uuid"`

	// MachineCode：登录机器码
	MachineCode string `gorm:"size:255;comment:登录机器码" json:"machine_code"`

	// IP：登录IP
	IP string `gorm:"size:50;comment:登录IP" json:"ip"`

	// LastActiveAt：最近活跃时间（心跳刷新）
	LastActiveAt time.Time `gorm:"comment:最近活跃时间" json:"last_active_at"`

	// CreatedAt：会话创建时间
	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
}

// ============================================================================
// 结构体方法
// ============================================================================

// TableName 指定表名
func (MemberSession) TableName() string {
	return "member_sessions"
}
