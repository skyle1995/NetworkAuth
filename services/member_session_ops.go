package services

import (
	"errors"
	"strings"
	"time"

	"NetworkAuth/database"
	"NetworkAuth/models"

	"gorm.io/gorm"
)

// ============================================================================
// 二级令牌下的「会话」相关操作
//
// 老的 member_sessions 表已被 app_activations + member_account_tokens 取代：
//   账号令牌 = 一次登录（一台设备）
//   激活令牌 = 一个应用实例
// 因此登出 / 踢号 / 到期查询都按令牌与激活处理，不再碰 member_sessions。
// ============================================================================

// DeactivateForApp 释放指定应用的激活，拒绝账号令牌与其他应用的令牌。
// 已释放但仍存在的激活可重复释放，保持接口幂等。
func DeactivateForApp(appUUID, token string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	var activation models.AppActivation
	if err := db.Where("token = ?", strings.TrimSpace(token)).First(&activation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("激活令牌无效")
		}
		return err
	}
	if activation.AppUUID != strings.TrimSpace(appUUID) {
		return errors.New("激活令牌不属于当前应用")
	}
	return Deactivate(db, activation.Token)
}

// MemberLogout 登出：释放该令牌名下的激活；账号令牌额外吊销自身。
//
//   - 激活令牌 → 只释放这一个应用实例
//   - 账号令牌 → 仅释放当前账号令牌名下的激活，并吊销该令牌
func MemberLogout(appUUID, token string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("令牌不能为空")
	}

	// 激活令牌：直接释放
	var activation models.AppActivation
	if err := db.Where("token = ?", token).First(&activation).Error; err == nil {
		if activation.AppUUID != strings.TrimSpace(appUUID) {
			return errors.New("激活令牌不属于当前应用")
		}
		return Deactivate(db, token)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 账号令牌：只吊销这次登录，其他设备不受影响。
	_, err = AuthAccountToken(db, token)
	if err != nil {
		return errors.New("会话无效")
	}
	return RevokeAccountToken(db, token)
}

// DeactivateByMember 释放某账号的全部活跃激活（登出/强制下线用）。
func DeactivateByMember(db *gorm.DB, memberUUID string) error {
	now := time.Now()
	return db.Model(&models.AppActivation{}).
		Where("member_uuid = ? AND deactivated_at IS NULL", memberUUID).
		Updates(map[string]any{
			"deactivated_at": now,
			"updated_at":     now,
		}).Error
}

// KickSession 手动顶号：凭据验证后踢掉该账号的指定在线实例。
//
// 满员时客户端拿不到激活令牌，因此用凭据（注册账号=用户名+密码，卡密=卡号）验证，
// 避免「满员→拿不到令牌→无法踢下线」的死循环。
// 仅允许踢出当前账号自己的实例，防止越权。
func KickSession(appUUID, username, password string, sessionID uint, ip string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	member, err := authMemberByCredential(db, username, password, ip)
	if err != nil {
		return err
	}
	if sessionID == 0 {
		return errors.New("实例ID不能为空")
	}

	var act models.AppActivation
	if err := db.Where("id = ? AND member_uuid = ?", sessionID, member.UUID).First(&act).Error; err != nil {
		return errors.New("实例不存在或不属于该账号")
	}
	if err := Deactivate(db, act.Token); err != nil {
		return err
	}
	AddMemberLog(appUUID, member.UUID, member.Username, "手动顶号",
		"下线实例 #"+strings.TrimSpace(itoaUint(sessionID)), ip)
	return nil
}

// itoaUint 轻量无符号整型转字符串（避免为一行引入 strconv 依赖冲突）。
func itoaUint(v uint) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
