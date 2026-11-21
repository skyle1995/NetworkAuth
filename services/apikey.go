package services

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"NetworkAuth/database"
	"NetworkAuth/models"
)

// API 密钥能力(scope)常量。
//
// 分两层：
//   - 通用档 api:read / api:write：授予后可用所有只读/写入接口（老密钥继续有效）
//   - 业务档 xxx:read / xxx:write：最小权限，只放行对应资源
//
// 鉴权时精确匹配优先；没有精确能力时，通用档按后缀兜底（见 apiKeyAllows）。
const (
	ScopeAPIRead  = "api:read"  // 通用只读
	ScopeAPIWrite = "api:write" // 通用写入

	ScopeAppRead      = "app:read"      // 应用：读取
	ScopeAppWrite     = "app:write"     // 应用：创建/修改
	ScopeBillingWrite = "billing:write" // 应用：调整计费策略

	ScopeVariableRead  = "var:read"  // 变量：读取
	ScopeVariableWrite = "var:write" // 变量：增删改

	ScopeFunctionRead  = "fn:read"  // 函数：读取
	ScopeFunctionWrite = "fn:write" // 函数：增删改

	ScopeMemberRead  = "member:read"  // 终端账号：读取
	ScopeMemberWrite = "member:write" // 终端账号：增删改、充值扣减
)

// ApiScopeMeta scope 元信息（供后台展示选择）。
type ApiScopeMeta struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// AvailableAPIScopes 返回所有可分配的能力（后台密钥管理用）。
func AvailableAPIScopes() []ApiScopeMeta {
	return []ApiScopeMeta{
		{Value: ScopeAPIRead, Label: "通用只读（放行所有读取接口）"},
		{Value: ScopeAPIWrite, Label: "通用写入（放行所有写入接口）"},
		{Value: ScopeAppRead, Label: "应用 · 读取"},
		{Value: ScopeAppWrite, Label: "应用 · 创建/修改"},
		{Value: ScopeBillingWrite, Label: "应用 · 调整计费"},
		{Value: ScopeVariableRead, Label: "变量 · 读取"},
		{Value: ScopeVariableWrite, Label: "变量 · 增删改"},
		{Value: ScopeFunctionRead, Label: "函数 · 读取"},
		{Value: ScopeFunctionWrite, Label: "函数 · 增删改"},
		{Value: ScopeMemberRead, Label: "终端账号 · 读取"},
		{Value: ScopeMemberWrite, Label: "终端账号 · 增删改/充值"},
	}
}

// apiKeyAllows 判断密钥是否满足 required 能力。
//
// 规则：精确匹配优先；缺精确能力时按后缀由通用档兜底——
// 有 api:read 即可用所有 *:read，有 api:write 即可用所有 *:write。
// 这样老密钥（只发了 api:read/api:write）不会因为细分能力而失效。
func apiKeyAllows(key *models.ApiKey, required string) bool {
	if key == nil {
		return false
	}
	if key.HasScope(required) || key.HasScope("*") {
		return true
	}
	switch {
	case strings.HasSuffix(required, ":read"):
		return key.HasScope(ScopeAPIRead)
	case strings.HasSuffix(required, ":write"):
		return key.HasScope(ScopeAPIWrite)
	}
	return false
}

// 鉴权可能返回的错误，便于上层区分提示。
var (
	ErrAPIKeyMissing  = errors.New("缺少 API 密钥")
	ErrAPIKeyInvalid  = errors.New("无效的 API 密钥")
	ErrAPIKeyDisabled = errors.New("API 密钥已禁用")
	ErrAPIKeyExpired  = errors.New("API 密钥已过期")
	ErrAPIKeyScope    = errors.New("API 密钥无此权限")
)

// GenerateAPIKeyString 生成高强度随机密钥（sk_ 前缀 + 40 位 base62）。
func GenerateAPIKeyString() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 40)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = charset[i%len(charset)]
		}
	} else {
		for i := range b {
			b[i] = charset[int(b[i])%len(charset)]
		}
	}
	return "sk_" + string(b)
}

// AuthAPIKey 校验密钥并要求具备指定能力，成功返回密钥记录。
// 供中间件与需要内联鉴权的接口复用，保证口径一致。
func AuthAPIKey(raw, scope, ip string) (*models.ApiKey, error) {
	if raw == "" {
		return nil, ErrAPIKeyMissing
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	// 取出候选后用常量时间比较，避免时序侧信道
	var key models.ApiKey
	if e := db.Where("`key` = ?", raw).First(&key).Error; e != nil {
		return nil, ErrAPIKeyInvalid
	}
	if subtle.ConstantTimeCompare([]byte(key.Key), []byte(raw)) != 1 {
		return nil, ErrAPIKeyInvalid
	}
	if key.Status != 1 {
		return nil, ErrAPIKeyDisabled
	}
	if key.Expired() {
		return nil, ErrAPIKeyExpired
	}
	if scope != "" && !apiKeyAllows(&key, scope) {
		return nil, ErrAPIKeyScope
	}
	touchAPIKey(&key, ip)
	return &key, nil
}

// touchAPIKey 记录最近使用（节流：超过 60s 才落库，降低写压力，best-effort）。
func touchAPIKey(key *models.ApiKey, ip string) {
	now := time.Now()
	if key.LastUsedAt != nil && now.Sub(*key.LastUsedAt) < time.Minute {
		return
	}
	db, err := database.GetDB()
	if err != nil {
		return
	}
	db.Model(&models.ApiKey{}).Where("id = ?", key.ID).Updates(map[string]interface{}{
		"last_used_at": now,
		"last_used_ip": ip,
	})
}
