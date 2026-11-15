package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// 账号令牌（Account Token）
// ============================================================================
//
// 对标 OAuth 2.0 的 Access + Refresh Token：
//   - 只证明「你是谁」，不做 audience 绑定、不做计费、不做设备绑定校验；
//   - 全站有效，可刷新，有独立 TTL（默认 30 天）；
//   - 用它去换取「应用激活」后，才进入计费与设备校验。
//
// 与 AppActivation 的分工：
//   账号令牌 —— 身份（Identity）     20/10 登录颁发，22/24/40/50/72 使用
//   激活令牌 —— 使用权（Entitlement） 70 激活颁发，    41/42/44/53 使用
type MemberAccountToken struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:令牌ID，自增主键" json:"id"`

	// UUID：唯一标识，供后台管理与日志关联
	UUID string `gorm:"size:36;uniqueIndex;comment:令牌UUID" json:"uuid"`

	// Token：账号令牌，全局唯一。需要登录态的账号级接口都带它
	Token string `gorm:"uniqueIndex;size:64;not null;comment:账号令牌" json:"token"`

	// RefreshToken：刷新令牌。账号令牌过期后用它换新的，避免用户重新输密码
	RefreshToken string `gorm:"uniqueIndex;size:64;not null;comment:刷新令牌" json:"refresh_token"`

	// MemberUUID：所属账号
	MemberUUID string `gorm:"size:36;not null;index;comment:所属账号UUID" json:"member_uuid"`

	// Fingerprint：颁发时的设备指纹（仅记录，不做校验；校验发生在激活时）
	Fingerprint string `gorm:"size:255;comment:设备指纹（仅记录）" json:"fingerprint"`

	// IP：颁发时的客户端 IP
	IP string `gorm:"size:50;comment:颁发时客户端IP" json:"ip"`

	// Version：颁发时的客户端版本
	Version string `gorm:"size:50;comment:客户端版本号" json:"version"`

	// DeviceName：设备名称，供后台在线列表展示
	DeviceName string `gorm:"size:100;comment:设备名称" json:"device_name"`

	// ExpiresAt：账号令牌过期时间（默认 30 天）
	ExpiresAt time.Time `gorm:"index;comment:账号令牌过期时间" json:"expires_at"`

	// LastActiveAt：最近活跃时间（账号级接口刷新），供后台展示与清理
	LastActiveAt time.Time `gorm:"comment:最近活跃时间" json:"last_active_at"`

	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// TableName 指定表名
func (MemberAccountToken) TableName() string { return "member_account_tokens" }

// BeforeCreate 生成 UUID
func (t *MemberAccountToken) BeforeCreate(tx *gorm.DB) error {
	if t.UUID == "" {
		t.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}

// IsExpired 是否已过期
func (t *MemberAccountToken) IsExpired(now time.Time) bool {
	return !t.ExpiresAt.IsZero() && t.ExpiresAt.Before(now)
}
