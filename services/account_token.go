package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"NetworkAuth/models"

	"gorm.io/gorm"
)

// ============================================================================
// 账号令牌（Account Token）
// ============================================================================
//
// 对标 OAuth 2.0：只证明「你是谁」，不计费、不绑设备、不算并发。
// 用它换取「应用激活」后才进入计费与设备校验。
//
// 与旧 member_sessions 的差异：
//   - 不带 app_uuid（账号级，全站生效）
//   - 带 RefreshToken 与 ExpiresAt（可刷新，有 TTL）
//   - 不做任何绑定/多开校验（这些移到激活）

// AccountTokenTTL 账号令牌有效期（默认 30 天）
const AccountTokenTTL = 30 * 24 * time.Hour

// ErrAccountTokenInvalid 账号令牌无效/过期（带标准错误码）
var ErrAccountTokenInvalid = NewCodedError(models.ErrAccountTokenInvalid, "")

// generateSecureToken 生成 32 字节随机令牌（64 位十六进制）
func generateSecureToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// IssueAccountToken 为账号颁发账号令牌（登录成功后调用）。
//
// 同一账号 + 同一指纹的旧令牌会被保留（多设备各自持有），
// 但可通过 RevokeAccountTokens 一键踢下所有设备。
func IssueAccountToken(db *gorm.DB, memberUUID, fingerprint, ip, version, deviceName string) (*models.MemberAccountToken, error) {
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}
	if strings.TrimSpace(memberUUID) == "" {
		return nil, errors.New("缺少账号标识")
	}

	token, err := generateSecureToken()
	if err != nil {
		return nil, err
	}
	refresh, err := generateSecureToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	record := &models.MemberAccountToken{
		Token:        token,
		RefreshToken: refresh,
		MemberUUID:   memberUUID,
		Fingerprint:  strings.TrimSpace(fingerprint),
		IP:           ip,
		Version:      version,
		DeviceName:   deviceName,
		ExpiresAt:    now.Add(AccountTokenTTL),
		LastActiveAt: now,
	}
	if err := db.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// AuthAccountToken 校验账号令牌并返回所属账号。
//
// 只做三件事：令牌存在、未过期、账号状态可用（封停/拉黑由调用方按需再查）。
// 不做计费、不做绑定校验 —— 那是 Activate 的职责。
func AuthAccountToken(db *gorm.DB, token string) (*models.MemberAccountToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrAccountTokenInvalid
	}

	var record models.MemberAccountToken
	if err := db.Where("token = ?", token).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountTokenInvalid
		}
		return nil, err
	}
	if record.IsExpired(time.Now()) {
		return nil, ErrAccountTokenInvalid
	}
	return &record, nil
}

// TouchAccountToken 刷新账号令牌的活跃时间（账号级接口调用时）。
// 失败不阻断主流程。
func TouchAccountToken(db *gorm.DB, tokenUUID string) {
	if db == nil || tokenUUID == "" {
		return
	}
	db.Model(&models.MemberAccountToken{}).
		Where("uuid = ?", tokenUUID).
		Update("last_active_at", time.Now())
}

// RefreshAccountToken 在有效期与正常账号状态内，用刷新令牌原子换一对新令牌。
//
// 旧令牌立即失效 —— 这是标准做法，可限制令牌泄漏的影响面。
func RefreshAccountToken(db *gorm.DB, refreshToken string) (*models.MemberAccountToken, error) {
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, ErrAccountTokenInvalid
	}

	var record models.MemberAccountToken
	if err := db.Where("refresh_token = ?", refreshToken).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountTokenInvalid
		}
		return nil, err
	}
	if record.IsExpired(time.Now()) {
		return nil, ErrAccountTokenInvalid
	}
	newToken, err := generateSecureToken()
	if err != nil {
		return nil, err
	}
	newRefresh, err := generateSecureToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	// 将旧刷新令牌、期限和账号状态纳入更新条件，并发消费只允许一个成功。
	result := db.Model(&models.MemberAccountToken{}).
		Where("uuid = ? AND refresh_token = ? AND expires_at > ?", record.UUID, refreshToken, now).
		Where("member_uuid IN (?)", db.Model(&models.Member{}).Select("uuid").
			Where("status = ?", models.MemberStatusNormal)).
		Updates(map[string]any{
			"token":          newToken,
			"refresh_token":  newRefresh,
			"expires_at":     now.Add(AccountTokenTTL),
			"last_active_at": now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrAccountTokenInvalid
	}

	record.Token = newToken
	record.RefreshToken = newRefresh
	record.ExpiresAt = now.Add(AccountTokenTTL)
	record.LastActiveAt = now
	return &record, nil
}

// RevokeAccountToken 吊销单个账号令牌，并级联失效其全部应用激活。
// 用于「退出登录」。
func RevokeAccountToken(db *gorm.DB, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}

	var record models.MemberAccountToken
	if err := db.Where("token = ?", token).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // 幂等：已不存在视为成功
		}
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		// 级联：该账号令牌下所有活跃激活立即释放
		if err := tx.Model(&models.AppActivation{}).
			Where("account_uuid = ? AND deactivated_at IS NULL", record.UUID).
			Updates(map[string]any{
				"deactivated_at": now,
				"updated_at":     now,
			}).Error; err != nil {
			return err
		}
		return tx.Where("uuid = ?", record.UUID).Delete(&models.MemberAccountToken{}).Error
	})
}

// RevokeMemberAccountTokens 吊销某账号的全部令牌（封号、改密后强制下线）。
// 同时级联失效其全部应用激活。
func RevokeMemberAccountTokens(db *gorm.DB, memberUUID string) error {
	memberUUID = strings.TrimSpace(memberUUID)
	if memberUUID == "" {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&models.AppActivation{}).
			Where("member_uuid = ? AND deactivated_at IS NULL", memberUUID).
			Updates(map[string]any{
				"deactivated_at": now,
				"updated_at":     now,
			}).Error; err != nil {
			return err
		}
		return tx.Where("member_uuid = ?", memberUUID).
			Delete(&models.MemberAccountToken{}).Error
	})
}

// ============================================================================
// 隐式激活支持：从「账号令牌 + 应用」找到可复用的激活
// ============================================================================

// FindAliveActivation 查找某账号在某应用上仍活跃的激活（同指纹优先）。
//
// 供隐式激活使用：老客户端用账号令牌直接调 `41` 时，
// 若该设备已有活跃激活则直接复用，不重复扣费。
func FindAliveActivation(db *gorm.DB, memberUUID, appUUID, fingerprint string) *models.AppActivation {
	if db == nil {
		return nil
	}
	now := time.Now()

	var list []models.AppActivation
	q := db.Where("member_uuid = ? AND app_uuid = ? AND deactivated_at IS NULL AND lease_expires_at > ?",
		memberUUID, appUUID, now)
	if strings.TrimSpace(fingerprint) != "" {
		// 有指纹时优先精确匹配（幂等的基础）
		q = q.Where("fingerprint = ?", strings.TrimSpace(fingerprint))
	}
	if err := q.Order("last_active_at DESC").Limit(1).Find(&list).Error; err != nil || len(list) == 0 {
		return nil
	}
	return &list[0]
}
