package models

import "time"

// ============================================================================
// 结构体定义
// ============================================================================

// MemberLog 账号调用审计日志
// 记录客户端公开 API 的关键动作（登录/充值/扣点/转绑/风控/登出等），
// 便于运营对账、追溯某卡某账号的历史。
//
// 一号通语义：账号全站唯一，日志按「事件来源应用」记录——
// AppUUID 表示该动作发生在哪个应用，用于按应用追溯，不代表账号归属。
type MemberLog struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:日志ID，自增主键" json:"id"`

	// AppUUID：事件来源应用UUID（该动作发生在哪个应用）
	AppUUID string `gorm:"size:36;not null;index;comment:事件来源应用UUID" json:"app_uuid"`

	// MemberUUID：账号UUID（部分动作可能为空，如注册前）
	MemberUUID string `gorm:"size:36;index;comment:账号UUID" json:"member_uuid"`

	// Username：用户名/卡号（冗余，便于检索）
	Username string `gorm:"size:64;index;comment:用户名或卡号" json:"username"`

	// Action：动作类型（卡密登录/账号登录/注册/充值/扣点/转绑/封停等）
	Action string `gorm:"size:32;index;comment:动作类型" json:"action"`

	// Detail：详情描述
	Detail string `gorm:"size:255;comment:详情" json:"detail"`

	// IP：客户端IP
	IP string `gorm:"size:50;comment:客户端IP" json:"ip"`

	// CreatedAt：发生时间
	CreatedAt time.Time `gorm:"index;comment:发生时间" json:"created_at"`
}

// TableName 指定表名
func (MemberLog) TableName() string {
	return "member_logs"
}
