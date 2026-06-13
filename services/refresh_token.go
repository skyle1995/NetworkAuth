package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RefreshTokenService 提供 refreshToken 的持久化、轮换、撤销等业务能力
type RefreshTokenService struct{}

var refreshTokenService = &RefreshTokenService{}

// GetRefreshTokenService 单例获取
func GetRefreshTokenService() *RefreshTokenService {
	return refreshTokenService
}

// NewJTI 生成新的 jti
func (s *RefreshTokenService) NewJTI() string {
	return uuid.New().String()
}

// NewFamilyID 生成新的 family_id（每次登录都新建）
func (s *RefreshTokenService) NewFamilyID() string {
	return uuid.New().String()
}

// Create 持久化一条 refreshToken 记录
//   - 登录场景：传入新的 familyID + absolute（now + 绝对过期天数）
//   - 刷新场景：复用旧 familyID 与旧 absolute，保证滑动续期不能突破上限
func (s *RefreshTokenService) Create(jti, familyID, userUUID, userType string,
	expiresAt, absoluteExpiresAt time.Time, ua, ip string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	rec := models.RefreshToken{
		JTI:               jti,
		FamilyID:          familyID,
		UserUUID:          userUUID,
		UserType:          userType,
		IssuedAt:          time.Now(),
		ExpiresAt:         expiresAt,
		AbsoluteExpiresAt: absoluteExpiresAt,
		UserAgent:         ua,
		IP:                ip,
	}
	return db.Create(&rec).Error
}

// CreateAndRotate 在单个事务内完成令牌轮换：插入新 refreshToken 记录 + 撤销旧记录
// - 保证"新令牌已写入"与"旧令牌已撤销"两步的原子性，避免中途失败留下不一致的中间态
// - 入参与 Create 一致，额外的 oldJTI 为被替换的旧令牌
func (s *RefreshTokenService) CreateAndRotate(jti, familyID, userUUID, userType string,
	expiresAt, absoluteExpiresAt time.Time, ua, ip, oldJTI string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		rec := models.RefreshToken{
			JTI:               jti,
			FamilyID:          familyID,
			UserUUID:          userUUID,
			UserType:          userType,
			IssuedAt:          time.Now(),
			ExpiresAt:         expiresAt,
			AbsoluteExpiresAt: absoluteExpiresAt,
			UserAgent:         ua,
			IP:                ip,
		}
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}
		// 撤销旧令牌并记录被替换为新 jti
		return tx.Model(&models.RefreshToken{}).
			Where("jti = ?", oldJTI).
			Updates(map[string]interface{}{
				"revoked":     true,
				"replaced_by": jti,
			}).Error
	})
}

// FindByJTI 根据 jti 查询
func (s *RefreshTokenService) FindByJTI(jti string) (*models.RefreshToken, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	var rec models.RefreshToken
	if err := db.Where("jti = ?", jti).First(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

// RevokeFamily 撤销整个 family 下所有未撤销的 refreshToken（用于重用检测/登出）
func (s *RefreshTokenService) RevokeFamily(familyID string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Model(&models.RefreshToken{}).
		Where("family_id = ? AND revoked = ?", familyID, false).
		Update("revoked", true).Error
}

// RevokeByJTI 撤销单条 refreshToken（一般在轮换时使用 Rotate）
func (s *RefreshTokenService) RevokeByJTI(jti string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Model(&models.RefreshToken{}).
		Where("jti = ?", jti).
		Update("revoked", true).Error
}

// Rotate 标记旧 jti 为已撤销，并记录被替换为新 jti
func (s *RefreshTokenService) Rotate(oldJTI, newJTI string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Model(&models.RefreshToken{}).
		Where("jti = ?", oldJTI).
		Updates(map[string]interface{}{
			"revoked":     true,
			"replaced_by": newJTI,
		}).Error
}

// CleanupExpired 清理过期且过期时间早于 retentionDays 天前的记录
func (s *RefreshTokenService) CleanupExpired(retentionDays int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	return db.Where("expires_at < ?", cutoff).Delete(&models.RefreshToken{}).Error
}

// ErrRefreshNotFound refreshToken 不存在
var ErrRefreshNotFound = errors.New("refresh token not found")

// IsNotFound 判断是否为记录未找到错误
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
