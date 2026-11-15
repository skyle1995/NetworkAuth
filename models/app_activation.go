package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// 应用激活（App Activation）—— 本方案的核心表
// ============================================================================
//
// 对标 Keygen 的 machine activation（floating license）：
//   - 一次「进入某个应用」= 一条激活记录，占用一个 machine 名额；
//   - 记录带**租约**（LeaseExpiresAt），心跳续期，到期未续自动失效并归还名额；
//   - 显式释放（DeactivatedAt）立即归还名额，不必等租约到期；
//   - 唯一键 (AppUUID, MemberUUID, Fingerprint) 保证**幂等**：
//     同设备重复激活时复用原记录、不重复扣费。
//
// 计费只发生在「创建/复活激活」时，不发生在登录时。
type AppActivation struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:激活ID，自增主键" json:"id"`

	// UUID：唯一标识
	UUID string `gorm:"size:36;uniqueIndex;comment:激活UUID" json:"uuid"`

	// Token：激活令牌（租约凭证）。心跳/数据/扣点接口都用它
	Token string `gorm:"uniqueIndex;size:64;not null;comment:激活令牌" json:"token"`

	// MemberUUID：所属账号
	MemberUUID string `gorm:"size:36;not null;index:idx_activation_member;comment:所属账号UUID" json:"member_uuid"`

	// AppUUID：被激活的应用
	AppUUID string `gorm:"size:64;not null;index:idx_activation_identity,priority:2;comment:应用UUID" json:"app_uuid"`

	// AccountUUID：来源账号令牌的 UUID。账号登出时据此级联失效
	AccountUUID string `gorm:"size:36;index;comment:来源账号令牌UUID" json:"account_uuid"`

	// Fingerprint：设备指纹（= KTRW_DEVICE_CODE）。
	// machine_verify=0 的应用不绑设备，此处填 "anon:<随机>"，每次激活都算新占用。
	// 参与复合唯一索引，是幂等激活的基石。
	Fingerprint string `gorm:"size:255;not null;default:'';index:idx_activation_identity,priority:3;comment:设备指纹" json:"fingerprint"`

	// IP：激活时的客户端 IP
	IP string `gorm:"size:50;comment:激活时客户端IP" json:"ip"`

	// Version：激活时的客户端版本
	Version string `gorm:"size:50;comment:客户端版本号" json:"version"`

	// DeviceName：设备名称，供后台在线列表展示
	DeviceName string `gorm:"size:100;comment:设备名称" json:"device_name"`

	// LeaseExpiresAt：租约到期时间（now + App.LeaseTTLMinutes）。
	// 心跳续期；到期未续 → 视为失效，名额自动回收。
	LeaseExpiresAt time.Time `gorm:"index:idx_activation_lease;comment:租约到期时间" json:"lease_expires_at"`

	// ChargeBasis：计费计时基准（0=FROM_CREATION 激活即计时 / 1=FROM_FIRST_PING 首次心跳计时）。
	// 取值来自应用当时的 PointsHeartbeatCharge，快照进激活记录，避免后续改配置影响已存在的占用。
	ChargeBasis int `gorm:"default:0;not null;comment:计费基准，0=激活即计时，1=首次心跳计时" json:"charge_basis"`

	// PeriodEnd：按时模式下**当前已付费周期**的结束时间。
	// 必须落在激活级而非 members.expired_at：后者是全站钱包，
	// 多应用按时计费若都写它，会互相延长可用期。
	PeriodEnd *time.Time `gorm:"comment:按时模式已付费周期结束时间" json:"period_end"`

	// FirstPingAt：首次心跳时间（ChargeBasis=1 时用于确定计时起点）
	FirstPingAt *time.Time `gorm:"comment:首次心跳时间" json:"first_ping_at"`

	// DeactivatedAt：显式释放时间。非空 = 已释放，不计入并发数
	DeactivatedAt *time.Time `gorm:"index:idx_activation_dead;comment:显式释放时间，非空表示已释放" json:"deactivated_at"`

	// LastActiveAt：最近心跳时间
	LastActiveAt time.Time `gorm:"comment:最近心跳时间" json:"last_active_at"`

	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// TableName 指定表名
func (AppActivation) TableName() string { return "app_activations" }

// BeforeCreate 生成 UUID
func (a *AppActivation) BeforeCreate(tx *gorm.DB) error {
	if a.UUID == "" {
		a.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}

// ============================================================================
// 状态判定
// ============================================================================

// IsReleased 是否已显式释放
func (a *AppActivation) IsReleased() bool {
	return a.DeactivatedAt != nil && !a.DeactivatedAt.IsZero()
}

// IsLeaseExpired 租约是否已过期
func (a *AppActivation) IsLeaseExpired(now time.Time) bool {
	return !a.LeaseExpiresAt.IsZero() && a.LeaseExpiresAt.Before(now)
}

// IsDead 是否已失效（显式释放 或 租约过期）。
// 失效的激活不占用并发名额，可被同 fingerprint 的激活「复活」。
func (a *AppActivation) IsDead(now time.Time) bool {
	return a.IsReleased() || a.IsLeaseExpired(now)
}

// IsAlive 是否仍活跃（占用名额中）
func (a *AppActivation) IsAlive(now time.Time) bool {
	return !a.IsDead(now)
}

// NeedsPeriodCharge 按时模式下是否需要扣下个周期
func (a *AppActivation) NeedsPeriodCharge(now time.Time) bool {
	if a.PeriodEnd == nil || a.PeriodEnd.IsZero() {
		return true
	}
	return a.PeriodEnd.Before(now)
}

// ============================================================================
// 计费基准（对齐 Keygen heartbeatBasis）
// ============================================================================

const (
	// ChargeBasisFromCreation：激活即开始计时（等价旧语义「登录预扣」）
	// 激活时立即扣一个周期，PeriodEnd = now + 周期
	ChargeBasisFromCreation = 0
	// ChargeBasisFromFirstPing：首次心跳才开始计时（等价旧语义「心跳触发」）
	// 激活时不扣，首次心跳时扣并设置 PeriodEnd
	ChargeBasisFromFirstPing = 1
)

// ChargeBasisName 返回计费基准名称
func ChargeBasisName(basis int) string {
	if basis == ChargeBasisFromFirstPing {
		return "首次心跳计时"
	}
	return "激活即计时"
}
