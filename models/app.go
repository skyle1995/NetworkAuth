package models

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// 常量定义
// ============================================================================

// 更新方式（download_type）：合并原「强制更新 + 更新方式」为单一三态。
const (
	DownloadTypeDisabled = 0 // 不启用更新
	DownloadTypeForce    = 1 // 强制更新（有新版本必须更新才能继续）
	DownloadTypeFree     = 2 // 自由更新（提示有新版本，可自行选择是否更新）
)

// 多开范围：一个"多开"以什么为单位计数
const (

)

// 登录方式：会话满员时的处理策略
const (
	LoginTypeKickOldest = 0 // 顶号登录：自动踢掉最早的「开」
	LoginTypeReject     = 1 // 非顶号登录：直接拒绝新登录
	LoginTypeManual     = 2 // 手动顶号：返回在线会话列表，由客户端选择踢出
)

// SystemAppUUID 表示「非应用来源」的事件（后台操作、系统任务）。
// 用于 MemberLog.AppUUID 等需要来源应用标识的场合。
const SystemAppUUID = "0"

// ============================================================================
// 结构体定义
// ============================================================================

// App 应用表模型（一号通：仅承载「应用载体 + 运行时策略」）
//
// 一号通改造后，本表**不再承载任何平台级策略**：
//   - 计费策略（运营模式、点数单价/周期）→ 全局，见 [models.Setting] 与 services/global_settings.go
//   - 账号策略（注册/卡密登录/充值/试用/邮箱验证/注册限制）→ 全局，同上
//
// 保留的都是「只影响本应用如何使用」的配置：内容分发、会话节奏、多开数量、验证强度、登录方式。
// 归属判断原则见 docs/plan-unified-account.md 第二章。
//
// CreatedAt/UpdatedAt 由 GORM 自动维护
type App struct {
	// ID：主键，自增，同时通过 json 标签保证前端接收为 id
	ID uint `gorm:"primaryKey;comment:应用ID，自增主键" json:"id"`
	// UUID：应用唯一标识符，自动生成
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:应用UUID，唯一标识符" json:"uuid"`
	// Status：状态（1=启用，0=禁用）；json 名称与前端一致
	Status int `gorm:"default:0;not null;comment:应用状态，1=启用，0=禁用" json:"status"`
	// Name：应用名称；json 名称与前端一致
	Name string `gorm:"size:100;not null;comment:应用名称" json:"name"`
	// Secret：应用密钥，一号通下是**应用身份的唯一凭据**（用于请求签名）
	Secret string `gorm:"size:255;not null;comment:应用密钥，应用身份唯一凭据，用于请求签名" json:"secret"`
	// Version：应用版本号
	Version string `gorm:"size:50;default:'1.0.0';comment:应用版本号" json:"version"`
	// DownloadType：更新方式（0=不启用，1=强制更新，2=自由更新）。启用（非0）时须配 DownloadURL。
	// 强制与否由本字段表达（=1 为强制），客户端据此判断，无需单独的 force_update。
	DownloadType int `gorm:"default:0;not null;comment:更新方式，0=不启用，1=强制更新，2=自由更新" json:"download_type"`
	// DownloadURL：下载地址
	DownloadURL string `gorm:"size:500;comment:下载地址" json:"download_url"`

	// AppData：应用数据（base64编码存储）
	AppData string `gorm:"type:text;comment:应用数据，base64编码存储" json:"app_data"`

	// Announcement：程序公告内容（base64编码存储）
	Announcement string `gorm:"type:text;comment:程序公告内容，base64编码存储" json:"announcement"`

	// LoginType：登陆方式（0=顶号登录（默认），1=非顶号登录，2=手动顶号）
	LoginType int `gorm:"default:0;not null;comment:登陆方式，0=顶号登录，1=非顶号登录，2=手动顶号" json:"login_type"`
	
	// CleanInterval：清理间隔（单位：小时，默认1小时）
	CleanInterval int `gorm:"default:1;not null;comment:清理间隔，单位小时" json:"clean_interval"`
	// CheckInterval：心跳间隔（单位：分钟，默认10分钟）——登录后返回客户端，告知其心跳周期
	CheckInterval int `gorm:"default:10;not null;comment:心跳间隔，单位分钟" json:"check_interval"`
	// OfflineTimeout：自动离线时长（单位：分钟，默认30分钟）——超过该时长未心跳，后台扫描自动离线
	OfflineTimeout int `gorm:"default:30;not null;comment:自动离线时长，单位分钟" json:"offline_timeout"`
	

	// 机器验证相关字段
	// MachineVerify：机器验证（0=关闭，1=开启）
	// MachineRebindEnabled：机器重绑开关（0=关闭，1=开启）
	
	// MachineFreeCount：机器免费次数（默认0）
	// MachineRebindCount：机器重绑次数（默认0）
	// MachineRebindDeduct：机器重绑扣除（默认0，单位：分钟）

	// IP地址验证相关字段
	// IPVerify：IP地址验证（0=关闭，1=开启，2=开启(市)，3=开启(省)）
	// IPRebindEnabled：IP地址重绑开关（0=关闭，1=开启）
	
	// IPFreeCount：IP地址免费次数（默认0）
	// IPRebindCount：IP地址重绑次数（默认0）
	// IPRebindDeduct：IP地址重绑扣除（默认0，单位：分钟）

	// ===== 计费策略（按应用）=====
	//
	// 一号通：钱包（到期时间/点数/等级）全站唯一，但「怎么收费」由各应用自定。

	// FreeMode：计费豁免（1=本应用完全免费）。
	// 开启后跳过全部门禁与扣费：不看等级、不看到期、不扣点。
	FreeMode int `gorm:"default:0;not null;comment:计费豁免，1=本应用完全免费不计费" json:"free_mode"`

	// BillingMode：计费模式（见 models.BillingMode*）
	//   0=计时模式：账号有效 + 会员权限等级 ≥ RequiredLevel，不扣点
	//   1=点数模式：消耗账号点数
	//   2=高级模式：等级 ≥ RequiredLevel，并按该等级在本应用配置的点数扣点
	BillingMode int `gorm:"default:0;not null;comment:计费模式，0=会员，1=点数，2=混合" json:"billing_mode"`

	// RequiredLevel：要求的最低会员权限等级（1=不限，即默认档）。
	// 仅计时模式与高级模式生效；按 MemberLevel.Level 比较。
	RequiredLevel int `gorm:"default:1;not null;comment:要求的最低会员权限等级，1=不限" json:"required_level"`

	// BillingScope：计费范围（0=按设备单独计费，1=账号合并计费）。仅点数模式与高级模式生效
	//
	//   0 单独计费：每个激活各自扣费。同一账号在 3 台设备上 = 3 份钱。
	//   1 合并计费：按「账号 + 应用」只算一份。3 台设备同时在线也只扣一次/一份周期。
	//
	// 按次模式下，「合并」的含义是：账号在本应用从「没有活跃设备」变为「有活跃设备」时扣一次，
	// 期间其他设备再进来不重复扣。
	// 按时模式下，「合并」的含义是：周期账本提升到账号维度（见 AppAccountBilling），
	// 所有设备共享同一个 period_end。
	BillingScope int `gorm:"default:0;not null;comment:计费范围，0=按设备单独计费，1=账号合并计费" json:"billing_scope"`
	// PointsChargeMode：点数扣费方式（0=按次，1=按时/预扣费）。点数模式与高级模式生效
	PointsChargeMode int `gorm:"default:0;not null;comment:点数扣费方式，0=按次，1=按时" json:"points_charge_mode"`
	// PointsPerLogin：按次模式每次登录扣除的点数（点数模式用；高级模式按等级表取值）
	PointsPerLogin float64 `gorm:"default:1;not null;comment:按次模式每次登录扣点（支持小数）" json:"points_per_login"`
	// PointsPeriodMinutes：按时模式的计费周期（分钟）
	PointsPeriodMinutes int `gorm:"default:60;not null;comment:按时模式计费周期，分钟" json:"points_period_minutes"`
	// PointsPerPeriod：按时模式每周期扣除的点数（点数模式用；高级模式按等级表取值）
	PointsPerPeriod float64 `gorm:"default:1;not null;comment:按时模式每周期扣点（支持小数）" json:"points_per_period"`
	// PointsHeartbeatCharge：按时扣费触发方式（0=登录预扣，1=心跳触发）。仅按时模式生效
	//
	// 行业标准对应：Keygen 的 heartbeatBasis
	//   0 = FROM_CREATION   激活即开始计时（等价"登录预扣"）
	//   1 = FROM_FIRST_PING 首次心跳才开始计时（等价"心跳触发"）
	PointsHeartbeatCharge int `gorm:"default:0;not null;comment:按时扣费触发方式，0=登录预扣，1=心跳触发" json:"points_heartbeat_charge"`

	// ========================================================================
	// LeaseTTLMinutes：应用激活的租约时长（分钟，默认 30）。
	// 激活时 expires_at = now + 本值；客户端按心跳续约，到期未续则名额自动回收。
	// 与 OfflineTimeout 的差别：OfflineTimeout 是"多久没心跳算离线"（观测口径），
	// LeaseTTL 是"凭证多久失效"（授权口径）。默认取同一数值。
	LeaseTTLMinutes int `gorm:"default:30;not null;comment:激活租约时长，分钟" json:"lease_ttl_minutes"`

	// CreatedAt/UpdatedAt：时间字段，返回为 created_at/updated_at，便于前端展示
	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// ============================================================================
// 结构体方法
// ============================================================================

// BeforeCreate 在创建记录前自动生成UUID和密钥
func (app *App) BeforeCreate(tx *gorm.DB) error {
	if app.UUID == "" {
		app.UUID = strings.ToUpper(uuid.New().String())
	}
	if app.Secret == "" {
		// 生成32位大写16进制随机字符
		bytes := make([]byte, 16) // 16字节 = 32位16进制字符
		rand.Read(bytes)
		app.Secret = strings.ToUpper(hex.EncodeToString(bytes))
	}
	return nil
}

// TableName 指定表名
func (App) TableName() string {
	return "apps"
}
