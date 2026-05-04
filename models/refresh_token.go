package models

import "time"

// RefreshToken 刷新令牌持久化记录
// - 一次一换：每次刷新生成新 jti，并把旧记录标记 Revoked
// - 同一登录会话共享 FamilyID，便于整体撤销
// - 重用检测：一旦某条已 Revoked 的 token 被再次提交，整个 family 全部失效
type RefreshToken struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	JTI               string    `gorm:"uniqueIndex;size:64;not null;comment:JWT ID" json:"jti"`
	FamilyID          string    `gorm:"index;size:64;not null;comment:同一登录会话的 token 族" json:"family_id"`
	UserUUID          string    `gorm:"index;size:36;not null;comment:用户UUID" json:"user_uuid"`
	UserType          string    `gorm:"size:16;not null;comment:用户类型 admin/user" json:"user_type"`
	IssuedAt          time.Time `gorm:"not null;comment:签发时间" json:"issued_at"`
	ExpiresAt         time.Time `gorm:"not null;comment:过期时间" json:"expires_at"`
	AbsoluteExpiresAt time.Time `gorm:"not null;comment:会话绝对过期时间" json:"absolute_expires_at"`
	Revoked           bool      `gorm:"not null;default:false;comment:是否已撤销" json:"revoked"`
	ReplacedBy        string    `gorm:"size:64;comment:被哪个新 jti 替换" json:"replaced_by"`
	UserAgent         string    `gorm:"size:255;comment:登录设备 UA" json:"user_agent"`
	IP                string    `gorm:"size:64;comment:登录 IP" json:"ip"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
