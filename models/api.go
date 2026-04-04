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
	APITypeUserLogin    = 20 // 字系统登录
	APITypeUserRegin    = 21 // 字系统注册
	APITypeUserRecharge = 22 // 用户充值

	// 登出操作
	APITypeLogOut = 30 // 退出登录

	// 状态查询
	APITypeGetExpired      = 40 // 获取到期时间
	APITypeCheckUserStatus = 41 // 检测账号状态
	APITypeGetAppData      = 42 // 获取程序数据
	APITypeGetVariable     = 43 // 获取变量数据
	APITypeExecuteFunction = 44 // 执行远程函数

	// 用户操作
	APITypeUpdatePwd     = 50 // 修改账号密码
	APITypeMacChangeBind = 51 // 机器码转绑
	APITypeIPChangeBind  = 52 // IP转绑

	// 风控操作
	APITypeDisableUser      = 60 // 封停用户
	APITypeBlackUser        = 61 // 添加黑名单
	APITypeUserDeductedTime = 62 // 扣除时间
)

// 算法类型常量
const (
	AlgorithmNone       = 0 // 不加密
	AlgorithmRC4        = 1 // RC4
	AlgorithmRSA        = 2 // RSA
	AlgorithmRSADynamic = 3 // RSA（动态）
	AlgorithmEasy       = 4 // 易加密
)

// ============================================================================
// 结构体定义
// ============================================================================

// API 接口表模型
// 用于管理API接口的配置信息
// CreatedAt/UpdatedAt 由 GORM 自动维护
type API struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:API接口ID，自增主键" json:"id"`

	// UUID：API接口唯一标识符，自动生成
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:API接口UUID，唯一标识符" json:"uuid"`

	// API类型（int型）
	APIType int `gorm:"not null;comment:API类型" json:"api_type"`

	// 应用UUID，关联到App表
	AppUUID string `gorm:"size:36;not null;index;comment:关联的应用UUID" json:"app_uuid"`

	// 接口状态（1=启用，0=禁用）
	Status int `gorm:"default:0;not null;comment:接口状态，1=启用，0=禁用" json:"status"`

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
			Name: "账号管理",
			Types: []APITypeInfo{
				{Type: APITypeUserLogin, Name: "子系统登录"},
				{Type: APITypeUserRegin, Name: "子系统注册"},
				{Type: APITypeUserRecharge, Name: "用户充值"},
			},
		},
		{
			Name: "登出操作",
			Types: []APITypeInfo{
				{Type: APITypeLogOut, Name: "退出登录"},
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
			},
		},
		{
			Name: "用户操作",
			Types: []APITypeInfo{
				{Type: APITypeUpdatePwd, Name: "修改账号密码"},
				{Type: APITypeMacChangeBind, Name: "机器码转绑"},
				{Type: APITypeIPChangeBind, Name: "IP转绑"},
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
	return algorithm >= AlgorithmNone && algorithm <= AlgorithmEasy
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
