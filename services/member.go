package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/utils"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// 终端用户服务
// ============================================================================
//
// 终端用户（Member）是应用的最终用户，区别于后台管理员（User）。
// 注册账号与卡密账号统一存储于 members 表，本服务负责后台对其的管理操作。

// CreateMember 后台手动创建一个注册型终端用户。
// durationMinutes 为初始时长（分钟），models.CardDurationPermanent(-1) 表示永久。
func CreateMember(appUUID, username, password string, durationMinutes int, remark string) (*models.Member, error) {
	appUUID = strings.TrimSpace(appUUID)
	username = strings.TrimSpace(username)
	if appUUID == "" || username == "" {
		return nil, errors.New("应用与用户名不能为空")
	}
	if password == "" {
		return nil, errors.New("密码不能为空")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	// 校验应用存在
	var appCount int64
	if err := db.Model(&models.App{}).Where("uuid = ?", appUUID).Count(&appCount).Error; err != nil {
		return nil, err
	}
	if appCount == 0 {
		return nil, errors.New("应用不存在")
	}

	// 同应用下用户名唯一
	var dupCount int64
	if err := db.Model(&models.Member{}).Where("app_uuid = ? AND username = ?", appUUID, username).Count(&dupCount).Error; err != nil {
		return nil, err
	}
	if dupCount > 0 {
		return nil, errors.New("该应用下用户名已存在")
	}

	salt, err := utils.GenerateRandomSalt()
	if err != nil {
		return nil, err
	}
	hashed, err := utils.HashPasswordWithSalt(password, salt)
	if err != nil {
		return nil, err
	}

	member := models.Member{
		AppUUID:      appUUID,
		Username:     username,
		Type:         models.MemberTypeRegister,
		Password:     hashed,
		PasswordSalt: salt,
		Status:       models.MemberStatusNormal,
		ExpiredAt:    expiryFromDuration(durationMinutes),
		Remark:       remark,
	}
	if err := db.Create(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

// expiryFromDuration 由初始时长计算到期时间：永久返回 PermanentTime，否则 now + 时长。
func expiryFromDuration(durationMinutes int) time.Time {
	if durationMinutes == models.CardDurationPermanent {
		return models.PermanentTime
	}
	return time.Now().Add(time.Duration(durationMinutes) * time.Minute)
}

// SetMembersStatus 批量设置终端用户状态（正常/封停/黑名单）。
func SetMembersStatus(ids []uint, status int) error {
	if len(ids) == 0 {
		return nil
	}
	if status != models.MemberStatusNormal &&
		status != models.MemberStatusDisabled &&
		status != models.MemberStatusBlack {
		return errors.New("无效的状态值")
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Model(&models.Member{}).Where("id IN ?", ids).Update("status", status).Error
}

// RechargeMemberTime 为终端用户充值时长（分钟）。
// 已过期的账号从当前时间起算；永久账号保持永久不变。
func RechargeMemberTime(id uint, minutes int) error {
	if minutes <= 0 {
		return errors.New("充值时长必须大于0")
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	var member models.Member
	if err := db.First(&member, id).Error; err != nil {
		return errors.New("终端用户不存在")
	}
	if member.ExpiredAt.Equal(models.PermanentTime) {
		// 永久账号无需充值
		return nil
	}
	base := member.ExpiredAt
	now := time.Now()
	if base.Before(now) {
		base = now
	}
	newExpiry := base.Add(time.Duration(minutes) * time.Minute)
	return db.Model(&member).Update("expired_at", newExpiry).Error
}

// DeductMemberTime 扣除终端用户时长（分钟），到期时间不早于当前时间。
// 永久账号不允许直接扣时，需先通过 SetMemberExpiry 重设到期时间。
func DeductMemberTime(id uint, minutes int) error {
	if minutes <= 0 {
		return errors.New("扣除时长必须大于0")
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	var member models.Member
	if err := db.First(&member, id).Error; err != nil {
		return errors.New("终端用户不存在")
	}
	if member.ExpiredAt.Equal(models.PermanentTime) {
		return errors.New("永久账号无法扣时，请先重设到期时间")
	}
	now := time.Now()
	newExpiry := member.ExpiredAt.Add(-time.Duration(minutes) * time.Minute)
	if newExpiry.Before(now) {
		newExpiry = now
	}
	return db.Model(&member).Update("expired_at", newExpiry).Error
}

// SetMemberExpiry 直接设置终端用户到期时间（用于修正/设为永久）。
func SetMemberExpiry(id uint, expiredAt time.Time) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Model(&models.Member{}).Where("id = ?", id).Update("expired_at", expiredAt).Error
}

// ResetMemberPassword 重置终端用户密码（重新生成盐值）。
func ResetMemberPassword(id uint, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return errors.New("新密码不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	salt, err := utils.GenerateRandomSalt()
	if err != nil {
		return err
	}
	hashed, err := utils.HashPasswordWithSalt(newPassword, salt)
	if err != nil {
		return err
	}
	return db.Model(&models.Member{}).Where("id = ?", id).Updates(map[string]interface{}{
		"password":      hashed,
		"password_salt": salt,
	}).Error
}

// UpdateMemberRemark 更新终端用户备注。
func UpdateMemberRemark(id uint, remark string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Model(&models.Member{}).Where("id = ?", id).Update("remark", remark).Error
}

// ClearMemberBindings 清空某终端用户的机器码/IP 绑定（后台解绑）。
func ClearMemberBindings(memberUUID string) error {
	memberUUID = strings.TrimSpace(memberUUID)
	if memberUUID == "" {
		return errors.New("终端用户UUID不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Where("member_uuid = ?", memberUUID).Delete(&models.Binding{}).Error
}

// DeleteMembers 批量删除终端用户，并级联删除其绑定记录。
func DeleteMembers(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		// 先取出待删用户的 UUID，用于级联清理绑定
		var uuids []string
		if err := tx.Model(&models.Member{}).Where("id IN ?", ids).Pluck("uuid", &uuids).Error; err != nil {
			return err
		}
		if len(uuids) > 0 {
			if err := tx.Where("member_uuid IN ?", uuids).Delete(&models.Binding{}).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&models.Member{}, ids).Error
	})
}
