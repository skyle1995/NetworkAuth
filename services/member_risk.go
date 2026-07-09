package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// 风控接口（封停 / 黑名单 / 扣时，type 60 / 61 / 62）
// ============================================================================
//
// 风控为作者侧操作，按用户名定位账号。调用授权依赖信封签名（持有应用密钥），
// 应从可信/服务端环境发起。封停与拉黑会同时清除该用户全部会话使其立即掉线。

// findMemberByUsername 按应用与用户名定位账号
func findMemberByUsername(db *gorm.DB, appUUID, username string) (*models.Member, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}
	var member models.Member
	if err := db.Where("app_uuid = ? AND username = ?", strings.TrimSpace(appUUID), username).First(&member).Error; err != nil {
		return nil, errors.New("账号不存在")
	}
	return &member, nil
}

// riskSetStatus 设置用户状态并清除其全部会话（封停/拉黑共用）
func riskSetStatus(appUUID, username string, status int) (any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	member, err := findMemberByUsername(db, appUUID, username)
	if err != nil {
		return nil, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(member).Update("status", status).Error; err != nil {
			return err
		}
		return tx.Where("member_uuid = ?", member.UUID).Delete(&models.MemberSession{}).Error
	})
	if err != nil {
		return nil, err
	}
	riskAction := "封停"
	switch status {
	case models.MemberStatusBlack:
		riskAction = "拉黑"
	case models.MemberStatusNormal:
		riskAction = "解封"
	}
	AddMemberLog(member.AppUUID, member.UUID, member.Username, riskAction, "", "")
	return map[string]any{"username": member.Username, "status": status}, nil
}

// RiskDisableMember 封停用户（type 60）
func RiskDisableMember(appUUID, username string) (any, error) {
	return riskSetStatus(appUUID, username, models.MemberStatusDisabled)
}

// RiskBlacklistMember 加入黑名单（type 61）
func RiskBlacklistMember(appUUID, username string) (any, error) {
	return riskSetStatus(appUUID, username, models.MemberStatusBlack)
}

// RiskDeductMember 扣除用户资源（type 62）：时长模式按分钟扣时，点数模式按点数扣点。
func RiskDeductMember(appUUID, username string, amount int) (any, error) {
	if amount <= 0 {
		return nil, errors.New("扣除数量必须大于0")
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	member, err := findMemberByUsername(db, appUUID, username)
	if err != nil {
		return nil, err
	}

	if app.OperationMode == models.OperationModePoints {
		newPoints := member.Points - amount
		if newPoints < 0 {
			newPoints = 0
		}
		if err := db.Model(member).Update("points", newPoints).Error; err != nil {
			return nil, err
		}
		AddMemberLog(member.AppUUID, member.UUID, member.Username, "扣点", "风控扣"+strconv.Itoa(amount)+"点", "")
		return map[string]any{"username": member.Username, "points": newPoints}, nil
	}

	if isPermanent(member.ExpiredAt) {
		return nil, errors.New("永久账号无法扣时")
	}
	newExpiry := member.ExpiredAt.Add(-time.Duration(amount) * time.Minute)
	if newExpiry.Before(time.Now()) {
		newExpiry = time.Now()
	}
	if err := db.Model(member).Update("expired_at", newExpiry).Error; err != nil {
		return nil, err
	}
	AddMemberLog(member.AppUUID, member.UUID, member.Username, "扣时", "风控扣"+strconv.Itoa(amount)+"分钟", "")
	return map[string]any{"username": member.Username, "expired_at": newExpiry}, nil
}
