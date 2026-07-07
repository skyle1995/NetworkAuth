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

// 终端用户来源类型
const (
	MemberTypeRegister = 0 // 注册账号（用户名+密码）
	MemberTypeCard     = 1 // 卡密账号（卡密登录时自动创建，绑定卡密）
)

// 终端用户状态常量
const (
	MemberStatusDisabled = 0 // 封停
	MemberStatusNormal   = 1 // 正常
	MemberStatusBlack    = 2 // 黑名单
)

// PermanentTime 永久有效的到期时间标记。
// 永久卡激活/充值时将 ExpiredAt 置为该时间，避免使用 nil 造成“未设置/永久”语义歧义。
var PermanentTime = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)

// ============================================================================
// 结构体定义
// ============================================================================

// Member 终端用户表模型
// 应用的终端用户（区别于后台管理员 [User]），两种来源统一存储于此表：
//   - 注册账号（Type=0）：用户名+密码登录，可用卡密充值
//   - 卡密账号（Type=1）：卡密登录时自动创建，Username=卡号，通过 CardUUID 绑定来源卡
//
// 到期时间、绑定、封停等一切运行时状态都落在本表，两种来源在运行时逻辑上无差别。
// CreatedAt/UpdatedAt 由 GORM 自动维护
type Member struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:终端用户ID，自增主键" json:"id"`

	// UUID：终端用户唯一标识符，自动生成
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:终端用户UUID，唯一标识符" json:"uuid"`

	// AppUUID：归属应用UUID，与 Username 联合唯一（不同应用的用户互相隔离）
	AppUUID string `gorm:"size:36;not null;index;uniqueIndex:idx_member_app_username;comment:归属应用UUID" json:"app_uuid"`

	// Username：用户名。注册账号为用户填写，卡密账号为卡号
	Username string `gorm:"size:64;not null;uniqueIndex:idx_member_app_username;comment:用户名，卡密账号为卡号" json:"username"`

	// Type：来源类型（0=注册账号，1=卡密账号）
	Type int `gorm:"default:0;not null;comment:来源类型，0=注册账号，1=卡密账号" json:"type"`

	// CardUUID：绑定的来源卡密UUID，卡密账号回指其来源卡，注册账号为空
	CardUUID string `gorm:"size:36;index;comment:绑定的来源卡密UUID，注册账号为空" json:"card_uuid"`

	// Password：密码哈希值，卡密账号可为空（仅凭卡号登录）
	Password string `gorm:"size:255;comment:密码哈希值，卡密账号可空" json:"-"`

	// PasswordSalt：密码加密盐值
	PasswordSalt string `gorm:"size:64;comment:密码加密盐值" json:"-"`

	// Status：状态（0=封停，1=正常，2=黑名单）
	Status int `gorm:"default:1;not null;comment:状态，0=封停，1=正常，2=黑名单" json:"status"`

	// ExpiredAt：到期时间。永久有效使用 PermanentTime 标记
	ExpiredAt time.Time `gorm:"comment:到期时间，永久有效为2099年" json:"expired_at"`

	// 机器码转绑计数（配合 App 的重绑限制“每天/永久”做重置）
	// MachineRebindUsed：机器码已用转绑次数
	MachineRebindUsed int `gorm:"default:0;not null;comment:机器码已用转绑次数" json:"machine_rebind_used"`
	// MachineRebindDate：机器码转绑计数日期（用于“每天”限制的按天重置）
	MachineRebindDate string `gorm:"size:10;comment:机器码转绑计数日期，格式YYYY-MM-DD" json:"machine_rebind_date"`

	// IP转绑计数
	// IPRebindUsed：IP已用转绑次数
	IPRebindUsed int `gorm:"default:0;not null;comment:IP已用转绑次数" json:"ip_rebind_used"`
	// IPRebindDate：IP转绑计数日期
	IPRebindDate string `gorm:"size:10;comment:IP转绑计数日期，格式YYYY-MM-DD" json:"ip_rebind_date"`

	// LoginToken：当前会话令牌（单会话/顶号模型；登出或新登录时更新）
	LoginToken string `gorm:"size:64;index;comment:当前会话令牌" json:"-"`
	// LastLoginAt：最近登录时间
	LastLoginAt *time.Time `gorm:"comment:最近登录时间" json:"last_login_at"`
	// LastLoginIP：最近登录IP
	LastLoginIP string `gorm:"size:50;comment:最近登录IP" json:"last_login_ip"`

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
func (member *Member) BeforeCreate(tx *gorm.DB) error {
	if member.UUID == "" {
		member.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}

// TableName 指定表名
func (Member) TableName() string {
	return "members"
}
