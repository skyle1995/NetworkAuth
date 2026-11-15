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
// 账号服务
// ============================================================================
//
// 账号（Member）是应用的最终用户，区别于后台管理员（User）。
// 注册账号与卡密账号统一存储于 members 表，本服务负责后台对其的管理操作。

// MemberProfileUpdate 后台编辑账号的可选字段：nil 表示不修改该项。
type MemberProfileUpdate struct {
	ExpiredAt     *time.Time // 到期时间（时长模式）
	Permanent     *bool      // true=设为永久，优先于 ExpiredAt
	Points        *float64   // 点数余额（支持小数）
	TotalRecharge *int       // 累计充值（分），改动后按新值重新校准会员等级
	Remark        *string
}

// UpdateMemberProfile 后台编辑账号资料：按需修改到期/点数/累充/备注，只改提交了的字段。
// 改动累充会按新值重新校准会员等级（改低也会降级），与 SetMemberTotalRecharge 语义一致。
func UpdateMemberProfile(id uint, up MemberProfileUpdate) (*models.Member, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	var member models.Member
	if err := db.First(&member, id).Error; err != nil {
		return nil, errors.New("账号不存在")
	}

	updates := map[string]interface{}{}

	switch {
	case up.Permanent != nil && *up.Permanent:
		updates["expired_at"] = models.PermanentTime
		member.ExpiredAt = models.PermanentTime
	case up.ExpiredAt != nil:
		updates["expired_at"] = *up.ExpiredAt
		member.ExpiredAt = *up.ExpiredAt
	}

	if up.Points != nil {
		if *up.Points < 0 {
			return nil, errors.New("点数不能为负")
		}
		updates["points"] = *up.Points
		member.Points = *up.Points
	}

	if up.Remark != nil {
		updates["remark"] = *up.Remark
		member.Remark = *up.Remark
	}

	// 累充改动需同步校准等级，避免「累充与等级不一致」
	if up.TotalRecharge != nil {
		if *up.TotalRecharge < 0 {
			return nil, errors.New("累计充值不能为负")
		}
		levelUUID, err := resolveLevelUUID(db, *up.TotalRecharge)
		if err != nil {
			return nil, err
		}
		updates["total_recharge"] = *up.TotalRecharge
		updates["level_uuid"] = levelUUID
		member.TotalRecharge = *up.TotalRecharge
		member.LevelUUID = levelUUID
	}

	if len(updates) == 0 {
		return &member, nil
	}
	if err := db.Model(&member).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

// CreateMember 后台手动创建一个注册型账号（全站账号）。
// durationMinutes 为初始时长（分钟），models.CardDurationPermanent(-1) 表示永久。
// 按全局计费模式决定初始资源：时长模式用 durationMinutes，点数模式用 points。
func CreateMember(username, password string, durationMinutes int, points float64, remark string) (*models.Member, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}
	if password == "" {
		return nil, errors.New("密码不能为空")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	// 用户名全站唯一
	var dupCount int64
	if err := db.Model(&models.Member{}).Where("username = ?", username).Count(&dupCount).Error; err != nil {
		return nil, err
	}
	if dupCount > 0 {
		return nil, errors.New("用户名已存在")
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
		Username:     username,
		Type:         models.MemberTypeRegister,
		Password:     hashed,
		PasswordSalt: salt,
		Status:       models.MemberStatusNormal,
		Remark:       remark,
	}
	// 一号通：账号同时拥有「到期时间」与「点数」两个钱包，初始资源按调用方给的值发放。
	if durationMinutes != 0 {
		member.ExpiredAt = expiryFromDuration(durationMinutes)
	}
	if points > 0 {
		member.Points = points
	}
	if err := db.Create(&member).Error; err != nil {
		return nil, err
	}
	// 建号即落到默认档（门槛为 0 的等级），避免出现 level_uuid 为空的账号
	if err := settleMemberLevel(db, &member, 0); err != nil {
		return nil, err
	}
	return &member, nil
}

// RechargeMemberPoints 后台为账号增加点数余额。
func RechargeMemberPoints(id uint, points float64) error {
	if points <= 0 {
		return errors.New("充值点数必须大于0")
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	var member models.Member
	if err := db.First(&member, id).Error; err != nil {
		return errors.New("账号不存在")
	}
	return db.Model(&member).Update("points", member.Points+points).Error
}

// DeductMemberPoints 后台扣除账号点数余额（不低于0）。
func DeductMemberPoints(id uint, points float64) error {
	if points <= 0 {
		return errors.New("扣除点数必须大于0")
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	var member models.Member
	if err := db.First(&member, id).Error; err != nil {
		return errors.New("账号不存在")
	}
	newPoints := member.Points - points
	if newPoints < 0 {
		newPoints = 0
	}
	return db.Model(&member).Update("points", newPoints).Error
}

// GetAppBillingMode 返回某应用的计费模式文案（供后台展示）。
func GetAppBillingMode(app *models.App) string {
	return models.BillingModeName(EffectiveBilling(app).Mode)
}

// expiryFromDuration 由初始时长计算到期时间：永久返回 PermanentTime，否则 now + 时长。
func expiryFromDuration(durationMinutes int) time.Time {
	if durationMinutes == models.CardDurationPermanent {
		return models.PermanentTime
	}
	return time.Now().Add(time.Duration(durationMinutes) * time.Minute)
}

// SetMembersStatus 批量设置账号状态（正常/封停/黑名单）。
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

// RechargeMemberTime 为账号充值时长（分钟）。
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
		return errors.New("账号不存在")
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

// DeductMemberTime 扣除账号时长（分钟），到期时间不早于当前时间。
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
		return errors.New("账号不存在")
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

// SetMemberExpiry 直接设置账号到期时间（用于修正/设为永久）。
func SetMemberExpiry(id uint, expiredAt time.Time) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Model(&models.Member{}).Where("id = ?", id).Update("expired_at", expiredAt).Error
}

// BatchRechargeResult 批量加时/加点结果统计。
type BatchRechargeResult struct {
	Total         int `json:"total"`          // 命中目标账号数
	TimeAdded     int `json:"time_added"`     // 时长模式已加时账号数
	PointsAdded   int `json:"points_added"`   // 点数模式已加点账号数
	SkipPermanent int `json:"skip_permanent"` // 加时时跳过的永久账号数
}

// BatchRecharge 批量为账号加时长/点数（维护补偿等场景）。
// all=false 时按 ids 定向；all=true 时全体账号（全站，无应用维度）。
// minutes 与 points 各自独立累加到对应钱包（都可填，也可只填一个）。
func BatchRecharge(ids []uint, all bool, minutes int, points float64) (*BatchRechargeResult, error) {
	if minutes <= 0 && points <= 0 {
		return nil, errors.New("请填写要增加的时长或点数")
	}
	if !all && len(ids) == 0 {
		return nil, errors.New("请选择要操作的账号")
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	q := db.Model(&models.Member{})
	if !all {
		q = q.Where("id IN ?", ids)
	}
	var members []models.Member
	if err := q.Find(&members).Error; err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return &BatchRechargeResult{}, nil
	}

	// 一号通：账号同时有「到期时间」与「点数」两个钱包，批量补偿时各自独立累加。
	res := &BatchRechargeResult{Total: len(members)}
	now := time.Now()
	err = db.Transaction(func(tx *gorm.DB) error {
		for i := range members {
			m := &members[i]
			if points > 0 {
				// 原子加，避免与并发的登录扣点相互覆盖
				if err := tx.Model(&models.Member{}).Where("uuid = ?", m.UUID).
					Update("points", gorm.Expr("ROUND(points + ?, 6)", points)).Error; err != nil {
					return err
				}
				m.Points += points
				res.PointsAdded++
			}
			if minutes > 0 {
				if m.ExpiredAt.Equal(models.PermanentTime) {
					res.SkipPermanent++
					continue
				}
				base := m.ExpiredAt
				if base.Before(now) {
					base = now
				}
				newExpiry := base.Add(time.Duration(minutes) * time.Minute)
				if err := tx.Model(m).Update("expired_at", newExpiry).Error; err != nil {
					return err
				}
				res.TimeAdded++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ResetMemberPassword 重置账号密码（重新生成盐值）。
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

// UpdateMemberRemark 更新账号备注。
func UpdateMemberRemark(id uint, remark string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Model(&models.Member{}).Where("id = ?", id).Update("remark", remark).Error
}

// ClearMemberBindings 清空某账号的机器码/IP 绑定（后台解绑）。
func ClearMemberBindings(memberUUID string) error {
	memberUUID = strings.TrimSpace(memberUUID)
	if memberUUID == "" {
		return errors.New("账号UUID不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Where("member_uuid = ?", memberUUID).Delete(&models.Binding{}).Error
}

// RemoveMemberBinding 按绑定UUID移除单条机器码/IP绑定记录（后台单条解绑）。
// 记录不存在时返回明确错误；成功时返回被移除绑定的类型与值，供操作审计日志使用。
func RemoveMemberBinding(bindingUUID string) (bindingType int, value string, err error) {
	bindingUUID = strings.TrimSpace(bindingUUID)
	if bindingUUID == "" {
		return 0, "", errors.New("绑定UUID不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return 0, "", err
	}
	var binding models.Binding
	if err := db.Where("uuid = ?", bindingUUID).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, "", errors.New("绑定记录不存在")
		}
		return 0, "", err
	}
	if err := db.Delete(&binding).Error; err != nil {
		return 0, "", err
	}
	return binding.Type, binding.Value, nil
}

// DeleteMembers 批量删除账号，并级联删除其绑定记录。
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
