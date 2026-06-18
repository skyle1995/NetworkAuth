package services

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"time"

	"NetworkAuth/database"
	"NetworkAuth/models"
)

// API 密钥能力(scope)常量。脚手架仅提供通用读/写两档，
// 业务项目可在此登记自有能力（如 inventory:generate、mail:read）。
const (
	ScopeAPIRead  = "api:read"  // 只读类接口
	ScopeAPIWrite = "api:write" // 写入类接口
)

// ApiScopeMeta scope 元信息（供后台展示选择）。
type ApiScopeMeta struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// AvailableAPIScopes 返回所有可分配的能力（后台密钥管理用）。
func AvailableAPIScopes() []ApiScopeMeta {
	return []ApiScopeMeta{
		{Value: ScopeAPIRead, Label: "只读接口"},
		{Value: ScopeAPIWrite, Label: "写入接口"},
	}
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
	if scope != "" && !key.HasScope(scope) {
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
