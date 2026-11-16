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

// API类型常量定义
const (
	// 基础信息
	APITypeGetBulletin     = 1 // 获取程序公告
	APITypeGetUpdateUrl    = 2 // 获取更新地址
	APITypeCheckAppVersion = 3 // 检测最新版本
	APITypeGetCardInfo     = 4 // 获取卡密信息

	// 卡密相关
	APITypeSingleLogin = 10 // 卡密登录

	// 账号管理
	APITypeUserLogin     = 20 // 账号登录
	APITypeUserRegin     = 21 // 账号注册
	APITypeUserRecharge  = 22 // 账号充值
	APITypeSendEmailCode = 23 // 发送注册验证码
	APITypeClaimTrial    = 24 // 领取试用
	APITypeSendResetCode = 25 // 发送找回密码验证码
	APITypeResetPassword = 26 // 找回密码

	// 登出操作
	APITypeLogOut      = 30 // 退出登录
	APITypeKickSession = 31 // 踢下线（手动顶号：凭据验证后踢掉本人账号的指定会话）

	// 状态查询
	APITypeGetExpired      = 40 // 获取到期时间
	APITypeCheckUserStatus = 41 // 检测账号状态
	APITypeGetAppData      = 42 // 获取程序数据
	APITypeGetVariable     = 43 // 获取变量数据
	APITypeExecuteFunction = 44 // 执行远程函数（服务端沙箱执行）
	APITypeGetUserData     = 45 // 获取账号数据（当前用户独有数据）

	// 用户操作
	APITypeUpdatePwd     = 50 // 修改账号密码
	APITypeMacChangeBind = 51 // 转绑（机器码+IP 统一转绑）
	APITypeDeductPoints  = 53 // 功能扣点（点数模式）
	APITypeSetUserData   = 54 // 设置账号数据（当前用户独有数据）

	// 风控操作
	APITypeDisableUser      = 60 // 封停用户
	APITypeBlackUser        = 61 // 添加黑名单
	APITypeUserDeductedTime = 62 // 扣除时间

	// ========================================================================
	// 二级令牌（外壳 + 插件）
	// ========================================================================
	//
	// 70/71/72 与原 1~62 的关系：
	//   登录（10/20）只发「账号令牌」，不计费、不绑定；
	//   70 用账号令牌换取「应用激活」，**计费与设备绑定都发生在这里**；
	//   71 显式释放激活，立即归还并发名额；
	//   72 下发权益清单（含插件的 uuid/secret），按账号权限过滤。
	//
	// 编号选 70+ 是为了与既有 1~62 完全隔开，老客户端忽略未知类型即可。

	// APITypeAppActivate 应用激活（幂等，计费入口）
	APITypeAppActivate = 70
	// APITypeAppDeactivate 应用释放（deactivation，立即归还名额）
	APITypeAppDeactivate = 71
	// APITypeEntitlements 权益清单（entitlements，含 secret 下发）
	APITypeEntitlements = 72
	// APITypePluginUpdates 批量查询插件签名更新清单（不计费）。
)

// 已移除的历史接口类型：仅用于启动时清理数据库中的遗留记录，不再作为有效接口。
const (
	LegacyAPITypeIPChangeBind = 52 // 原「IP转绑」，已并入统一转绑(51)
)

// 算法类型常量
const (
	AlgorithmNone       = 0 // 不加密
	AlgorithmRC4        = 1 // RC4
	AlgorithmRSA        = 2 // RSA
	AlgorithmRSADynamic = 3 // RSA（动态）
	AlgorithmEasy       = 4 // 易加密
	AlgorithmBox        = 5 // Curve25519 Box (端到端加密)
)

// ============================================================================
// 结构体定义
// ============================================================================

// API 接口表模型（一号通：全站接口定义）
//
// 接口条目全局唯一，每个 api_type 全站仅一条，应用不再各自持有一套。
// 应用身份由请求签名（app_uuid + app_secret）区分，见 services.VerifyOpenSign，
// 因此本表不区分应用：任何应用都用同一套接口定义与加密配置。
//
// 安全模型：
//   - 应用鉴权 = 签名（证明持有该应用的 secret）
//   - 载荷加密 = 全站共用一套密钥（防抓包/提高协议分析成本，不承担身份认证职责）
//
// CreatedAt/UpdatedAt 由 GORM 自动维护
type API struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:API接口ID，自增主键" json:"id"`

	// UUID：接口唯一标识符，自动生成
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:接口UUID，唯一标识符" json:"uuid"`

	// APIType：接口类型，全站唯一（每个类型只有一条配置）
	APIType int `gorm:"not null;uniqueIndex:idx_api_type;comment:接口类型，全站唯一" json:"api_type"`

	// Status：接口状态（1=启用，0=禁用）。全站生效：禁用后所有应用均不可调用
	Status int `gorm:"default:0;not null;comment:接口状态，1=启用，0=禁用（全站生效）" json:"status"`

	// 接口提交算法
	// 支持的算法：0=不加密，1=RC4，2=RSA，3=RSA（动态），4=易加密
	SubmitAlgorithm int `gorm:"default:0;not null;comment:提交算法，0=不加密，1=RC4，2=RSA，3=RSA动态，4=易加密" json:"submit_algorithm"`

	// 接口返回算法
	// 支持的算法：0=不加密，1=RC4，2=RSA，3=RSA（动态），4=易加密
	ReturnAlgorithm int `gorm:"default:0;not null;comment:返回算法，0=不加密，1=RC4，2=RSA，3=RSA动态，4=易加密" json:"return_algorithm"`

	// 提交算法公钥（明文PEM存储）
	SubmitPublicKey string `gorm:"type:text;comment:提交算法公钥，明文PEM" json:"submit_public_key"`

	// 提交算法私钥（明文PEM存储）
	SubmitPrivateKey string `gorm:"type:text;comment:提交算法私钥，明文PEM" json:"submit_private_key"`

	// 返回算法公钥（明文PEM存储）
	ReturnPublicKey string `gorm:"type:text;comment:返回算法公钥，明文PEM" json:"return_public_key"`

	// 返回算法私钥（明文PEM存储）
	ReturnPrivateKey string `gorm:"type:text;comment:返回算法私钥，明文PEM" json:"return_private_key"`

	// 时间字段
	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// ============================================================================
// 结构体方法
// ============================================================================

// BeforeCreate 在创建记录前自动生成UUID
func (api *API) BeforeCreate(tx *gorm.DB) error {
	if api.UUID == "" {
		api.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}

// TableName 指定表名
func (API) TableName() string {
	return "apis"
}

// ============================================================================
// 独立函数
// ============================================================================

// GetAlgorithmName 获取算法名称
func GetAlgorithmName(algorithm int) string {
	switch algorithm {
	case AlgorithmNone:
		return "不加密"
	case AlgorithmRC4:
		return "RC4"
	case AlgorithmRSA:
		return "RSA"
	case AlgorithmRSADynamic:
		return "RSA（动态）"
	case AlgorithmEasy:
		return "易加密"
	case AlgorithmBox:
		return "Curve25519"
	default:
		return "未知算法"
	}
}

// ============================================================================
// 基础结构体定义
// ============================================================================

// APITypeInfo 接口类型信息
type APITypeInfo struct {
	Type int    `json:"type"` // 接口类型
	Name string `json:"name"` // 接口名称
}

// APICategoryInfo 接口分类信息
type APICategoryInfo struct {
	Name  string        `json:"name"`  // 分类名称
	Types []APITypeInfo `json:"types"` // 该分类下的接口类型列表
}

// ============================================================================
// 核心功能函数
// ============================================================================

// GetAPITypes 获取API接口类型，支持按分类返回或返回完整列表
// categorized: true=按分类返回[]APICategoryInfo, false=返回完整列表[]int
func GetAPITypes(categorized bool) interface{} {
	// 层次化的接口类型组织结构
	apiCategories := []APICategoryInfo{
		{
			Name: "基础信息",
			Types: []APITypeInfo{
				{Type: APITypeGetBulletin, Name: "获取程序公告"},
				{Type: APITypeGetUpdateUrl, Name: "获取更新地址"},
				{Type: APITypeCheckAppVersion, Name: "检测最新版本"},
				{Type: APITypeGetCardInfo, Name: "获取卡密信息"},
			},
		},
		{
			Name: "卡密相关",
			Types: []APITypeInfo{
				{Type: APITypeSingleLogin, Name: "卡密登录"},
			},
		},
		{
			Name: "应用激活",
			Types: []APITypeInfo{
				{Type: APITypeAppActivate, Name: "应用激活（计费入口，幂等）"},
				{Type: APITypeAppDeactivate, Name: "应用释放"},
				{Type: APITypeEntitlements, Name: "权益清单"},
			},
		},
		{
			Name: "账号管理",
			Types: []APITypeInfo{
				{Type: APITypeUserLogin, Name: "账号登录"},
				{Type: APITypeUserRegin, Name: "账号注册"},
				{Type: APITypeUserRecharge, Name: "账号充值"},
				{Type: APITypeSendEmailCode, Name: "发送注册验证码"},
				{Type: APITypeClaimTrial, Name: "领取试用"},
				{Type: APITypeSendResetCode, Name: "发送找回密码验证码"},
				{Type: APITypeResetPassword, Name: "找回密码"},
			},
		},
		{
			Name: "登出操作",
			Types: []APITypeInfo{
				{Type: APITypeLogOut, Name: "退出登录"},
				{Type: APITypeKickSession, Name: "踢下线（手动顶号）"},
			},
		},
		{
			Name: "状态查询",
			Types: []APITypeInfo{
				{Type: APITypeGetExpired, Name: "获取到期时间"},
				{Type: APITypeCheckUserStatus, Name: "检测账号状态"},
				{Type: APITypeGetAppData, Name: "获取程序数据"},
				{Type: APITypeGetVariable, Name: "获取变量数据"},
				{Type: APITypeExecuteFunction, Name: "执行远程函数"},
				{Type: APITypeGetUserData, Name: "获取账号数据"},
			},
		},
		{
			Name: "用户操作",
			Types: []APITypeInfo{
				{Type: APITypeUpdatePwd, Name: "修改账号密码"},
				{Type: APITypeMacChangeBind, Name: "转绑"},
				{Type: APITypeDeductPoints, Name: "功能扣点"},
				{Type: APITypeSetUserData, Name: "设置账号数据"},
			},
		},
		{
			Name: "风控操作",
			Types: []APITypeInfo{
				{Type: APITypeDisableUser, Name: "封停用户"},
				{Type: APITypeBlackUser, Name: "添加黑名单"},
				{Type: APITypeUserDeductedTime, Name: "扣除时间"},
			},
		},
	}

	if categorized {
		// 返回层次化的分类结构
		return apiCategories
	}

	// 返回所有接口类型的扁平列表
	var allTypes []int
	for _, category := range apiCategories {
		for _, typeInfo := range category.Types {
			allTypes = append(allTypes, typeInfo.Type)
		}
	}
	return allTypes
}

// GetAPITypeName 获取API类型名称
// 通过调用GetAPITypes函数来获取名称，避免重复维护数据
func GetAPITypeName(apiType int) string {
	// 获取分类化的API类型数据
	categories := GetAPITypes(true).([]APICategoryInfo)

	// 遍历所有分类和类型，查找匹配的API类型
	for _, category := range categories {
		for _, typeInfo := range category.Types {
			if typeInfo.Type == apiType {
				return typeInfo.Name
			}
		}
	}

	// 如果没有找到匹配的类型，返回默认值
	return "未知API类型"
}

// ============================================================================
// 验证函数
// ============================================================================

// IsValidAlgorithm 验证算法类型是否有效
func IsValidAlgorithm(algorithm int) bool {
	return algorithm >= AlgorithmNone && algorithm <= AlgorithmBox
}

// IsValidAPIType 验证API类型是否有效
func IsValidAPIType(apiType int) bool {
	validTypes := GetDefaultAPITypes()

	for _, validType := range validTypes {
		if apiType == validType {
			return true
		}
	}
	return false
}

// ============================================================================
// 兼容性函数
// ============================================================================

// GetDefaultAPITypes 获取默认创建的API接口类型列表（兼容性函数）
func GetDefaultAPITypes() []int {
	return GetAPITypes(false).([]int)
}

// GetAPITypesByCategory 根据分类获取API类型列表（兼容性函数）
// 返回传统的 map[string][]int 格式以保持向后兼容
func GetAPITypesByCategory() map[string][]int {
	categories := GetAPITypes(true).([]APICategoryInfo)
	result := make(map[string][]int)

	for _, category := range categories {
		var types []int
		for _, typeInfo := range category.Types {
			types = append(types, typeInfo.Type)
		}
		result[category.Name] = types
	}

	return result
}

// GetAPICategoriesInfo 获取完整的层次化分类信息
// 返回新的 []APICategoryInfo 格式，包含完整的类型名称信息
func GetAPICategoriesInfo() []APICategoryInfo {
	return GetAPITypes(true).([]APICategoryInfo)
}
