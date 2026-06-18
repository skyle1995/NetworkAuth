package models

import (
	"strings"
	"time"
)

// ApiKey 对外 API 授权密钥（密钥库）。
// 每个 key 独立、可限能力(scopes)、可启停/过期/审计最近使用。
// 脚手架通用实现：不耦合任何业务对象，新增业务时按需扩展字段。
type ApiKey struct {
	ID         uint64     `gorm:"primarykey" json:"id"`
	Name       string     `gorm:"type:varchar(100);not null;comment:用途名称" json:"name"`
	Key        string     `gorm:"type:varchar(80);not null;uniqueIndex;comment:密钥(明文)" json:"key"`
	Scopes     string     `gorm:"type:varchar(255);not null;comment:能力范围(逗号分隔)" json:"scopes"`
	Status     int        `gorm:"type:tinyint;not null;default:1;index;comment:状态(1启用,0禁用)" json:"status"`
	ExpireAt   *time.Time `gorm:"comment:过期时间(可选)" json:"expire_at"`
	LastUsedAt *time.Time `gorm:"comment:最近使用时间" json:"last_used_at"`
	LastUsedIP string     `gorm:"type:varchar(64);comment:最近使用IP" json:"last_used_ip"`
	CreatedAt  time.Time  `gorm:"comment:创建时间" json:"createdAt"`
	UpdatedAt  time.Time  `gorm:"comment:更新时间" json:"updatedAt"`
}

// TableName 指定表名
func (ApiKey) TableName() string {
	return "api_keys"
}

// HasScope 判断该密钥是否具备某能力。
func (k *ApiKey) HasScope(scope string) bool {
	for _, s := range strings.Split(k.Scopes, ",") {
		if strings.TrimSpace(s) == scope {
			return true
		}
	}
	return false
}

// Expired 判断密钥是否已过期。
func (k *ApiKey) Expired() bool {
	return k.ExpireAt != nil && time.Now().After(*k.ExpireAt)
}
