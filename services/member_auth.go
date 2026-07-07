package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/utils"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// 终端用户认证服务（公开 API 业务核心）
// ============================================================================
//
// 面向客户端的登录/心跳/登出逻辑。会话采用单令牌模型：
// 登录颁发随机令牌写入 member.LoginToken，新登录覆盖旧令牌（即顶号），
// 登出清空令牌。多开（MultiOpenCount>1）的多会话留待后续会话表实现。

// LoginResult 登录成功返回的信息
type LoginResult struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Type      int       `json:"type"`
	Permanent bool      `json:"permanent"`
	ExpiredAt time.Time `json:"expired_at"`
}

// StatusResult 账号状态查询返回的信息
type StatusResult struct {
	Username  string    `json:"username"`
	Status    int       `json:"status"`
	Permanent bool      `json:"permanent"`
	ExpiredAt time.Time `json:"expired_at"`
}

// generateSessionToken 生成 32 字节随机会话令牌（64 位十六进制）
func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// isPermanent 判断到期时间是否为永久
func isPermanent(expiredAt time.Time) bool {
	return expiredAt.Equal(models.PermanentTime)
}

// CardLogin 卡密登录：卡号即身份。
// 未使用的卡首次登录激活并自动创建绑定该卡的终端用户；已使用的卡走登录校验。
func CardLogin(appUUID, cardNo, machineCode, ip string) (*LoginResult, error) {
	appUUID = strings.TrimSpace(appUUID)
	cardNo = strings.TrimSpace(cardNo)
	if appUUID == "" || cardNo == "" {
		return nil, errors.New("应用与卡号不能为空")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	// 校验应用存在且启用，并读取多开/机器验证配置
	var app models.App
	if err := db.Where("uuid = ?", appUUID).First(&app).Error; err != nil {
		return nil, errors.New("应用不存在")
	}
	if app.Status != 1 {
		return nil, errors.New("应用已停用")
	}
	if app.CardLoginEnabled != 1 {
		return nil, errors.New("该应用未开启卡密登录")
	}

	var member models.Member
	err = db.Transaction(func(tx *gorm.DB) error {
		var card models.Card
		if err := tx.Where("app_uuid = ? AND card_no = ?", appUUID, cardNo).First(&card).Error; err != nil {
			return errors.New("卡号不存在")
		}
		if card.Status == models.CardStatusFrozen {
			return errors.New("卡密已被冻结")
		}

		if card.Status == models.CardStatusUnused {
			// 首次使用：激活并创建绑定该卡的终端用户
			member = models.Member{
				AppUUID:   appUUID,
				Username:  cardNo,
				Type:      models.MemberTypeCard,
				CardUUID:  card.UUID,
				Status:    models.MemberStatusNormal,
				ExpiredAt: expiryFromDuration(card.Duration),
			}
			if err := tx.Create(&member).Error; err != nil {
				return errors.New("激活卡密失败")
			}
			if err := MarkCardUsed(tx, card.ID, member.UUID); err != nil {
				return errors.New("核销卡密失败")
			}
			return nil
		}

		// 已使用：定位其绑定的终端用户
		if err := tx.Where("app_uuid = ? AND username = ?", appUUID, cardNo).First(&member).Error; err != nil {
			return errors.New("卡密账号不存在")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return finishMemberLogin(db, &app, &member, machineCode, ip)
}

// finishMemberLogin 完成登录的公共收尾：状态/到期校验、机器码绑定、颁发令牌。
func finishMemberLogin(db *gorm.DB, app *models.App, member *models.Member, machineCode, ip string) (*LoginResult, error) {
	if member.Status == models.MemberStatusBlack {
		return nil, errors.New("账号已被拉黑")
	}
	if member.Status == models.MemberStatusDisabled {
		return nil, errors.New("账号已被封停")
	}
	if !isPermanent(member.ExpiredAt) && member.ExpiredAt.Before(time.Now()) {
		return nil, errors.New("账号已到期")
	}

	// 机器码绑定（开启机器验证时）：已绑定则放行，未绑定且未超多开则新增，超出则拒绝
	if app.MachineVerify == 1 && strings.TrimSpace(machineCode) != "" {
		if err := ensureMachineBinding(db, member.UUID, machineCode, app.MultiOpenCount); err != nil {
			return nil, err
		}
	}

	token, err := generateSessionToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := db.Model(member).Updates(map[string]interface{}{
		"login_token":   token,
		"last_login_at": &now,
		"last_login_ip": ip,
	}).Error; err != nil {
		return nil, err
	}

	return &LoginResult{
		Token:     token,
		Username:  member.Username,
		Type:      member.Type,
		Permanent: isPermanent(member.ExpiredAt),
		ExpiredAt: member.ExpiredAt,
	}, nil
}

// ensureMachineBinding 确保机器码已绑定；未绑定时在多开数量内新增，超出则拒绝。
func ensureMachineBinding(db *gorm.DB, memberUUID, machineCode string, multiOpenCount int) error {
	var existing models.Binding
	err := db.Where("member_uuid = ? AND type = ? AND value = ?",
		memberUUID, models.BindingTypeMachine, machineCode).First(&existing).Error
	if err == nil {
		return nil // 已绑定，放行
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	var count int64
	if err := db.Model(&models.Binding{}).
		Where("member_uuid = ? AND type = ?", memberUUID, models.BindingTypeMachine).
		Count(&count).Error; err != nil {
		return err
	}
	if multiOpenCount <= 0 {
		multiOpenCount = 1
	}
	if int(count) >= multiOpenCount {
		return errors.New("超出多开数量限制")
	}
	return db.Create(&models.Binding{
		MemberUUID: memberUUID,
		Type:       models.BindingTypeMachine,
		Value:      machineCode,
	}).Error
}

// authMemberByToken 按应用与令牌定位有效终端用户
func authMemberByToken(db *gorm.DB, appUUID, token string) (*models.Member, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("令牌不能为空")
	}
	var member models.Member
	if err := db.Where("app_uuid = ? AND login_token = ?", strings.TrimSpace(appUUID), token).First(&member).Error; err != nil {
		return nil, errors.New("会话无效或已被顶号")
	}
	return &member, nil
}

// authActiveMember 校验令牌并要求账号正常且未到期，返回有效终端用户。
// 供需要“已登录且可用”前提的接口（数据获取、改密、转绑等）复用。
func authActiveMember(db *gorm.DB, appUUID, token string) (*models.Member, error) {
	member, err := authMemberByToken(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	if member.Status != models.MemberStatusNormal {
		return nil, errors.New("账号状态异常")
	}
	if !isPermanent(member.ExpiredAt) && member.ExpiredAt.Before(time.Now()) {
		return nil, errors.New("账号已到期")
	}
	return member, nil
}

// CheckMemberStatus 心跳/状态查询：校验令牌有效、账号正常且未到期。
func CheckMemberStatus(appUUID, token string) (*StatusResult, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	member, err := authActiveMember(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	return &StatusResult{
		Username:  member.Username,
		Status:    member.Status,
		Permanent: isPermanent(member.ExpiredAt),
		ExpiredAt: member.ExpiredAt,
	}, nil
}

// MemberLogout 登出：清空当前会话令牌。
func MemberLogout(appUUID, token string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	member, err := authMemberByToken(db, appUUID, token)
	if err != nil {
		return err
	}
	return db.Model(member).Update("login_token", "").Error
}

// ============================================================================
// 账号模式（注册/登录/充值/到期查询）
// ============================================================================

// loadEnabledApp 读取应用并校验其存在且启用
func loadEnabledApp(db *gorm.DB, appUUID string) (*models.App, error) {
	var app models.App
	if err := db.Where("uuid = ?", strings.TrimSpace(appUUID)).First(&app).Error; err != nil {
		return nil, errors.New("应用不存在")
	}
	if app.Status != 1 {
		return nil, errors.New("应用已停用")
	}
	return &app, nil
}

// registerInitialExpiry 注册账号的初始到期时间：开启试用则给试用时长，否则注册即过期需充值。
func registerInitialExpiry(app *models.App) time.Time {
	if app.TrialEnabled == 1 && app.TrialDuration > 0 {
		return time.Now().Add(time.Duration(app.TrialDuration) * time.Minute)
	}
	return time.Now()
}

// AccountRegister 账号注册：创建注册型终端用户并返回账号信息。
// 不颁发会话令牌——注册账号在无试用时初始即过期，需登录（或先充值）后方可使用。
func AccountRegister(appUUID, username, password string) (*StatusResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, errors.New("用户名与密码不能为空")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	if app.RegisterEnabled != 1 {
		return nil, errors.New("该应用未开启账号注册")
	}

	var dup int64
	if err := db.Model(&models.Member{}).Where("app_uuid = ? AND username = ?", app.UUID, username).Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
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
		AppUUID:      app.UUID,
		Username:     username,
		Type:         models.MemberTypeRegister,
		Password:     hashed,
		PasswordSalt: salt,
		Status:       models.MemberStatusNormal,
		ExpiredAt:    registerInitialExpiry(app),
	}
	if err := db.Create(&member).Error; err != nil {
		return nil, errors.New("注册失败")
	}

	return &StatusResult{
		Username:  member.Username,
		Status:    member.Status,
		Permanent: isPermanent(member.ExpiredAt),
		ExpiredAt: member.ExpiredAt,
	}, nil
}

// AccountLogin 账号登录：校验用户名密码后颁发令牌。
func AccountLogin(appUUID, username, password, machineCode, ip string) (*LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, errors.New("用户名与密码不能为空")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}

	var member models.Member
	if err := db.Where("app_uuid = ? AND username = ?", app.UUID, username).First(&member).Error; err != nil {
		return nil, errors.New("账号或密码错误")
	}
	if !utils.VerifyPasswordWithSalt(password, member.PasswordSalt, member.Password) {
		return nil, errors.New("账号或密码错误")
	}

	return finishMemberLogin(db, app, &member, machineCode, ip)
}

// RechargeByCard 用一张卡为账号充值：把卡面值加到该账号到期时间，并核销卡密。
// 卡与账号须属同一应用；卡须未使用；永久卡直接将账号设为永久。
func RechargeByCard(appUUID, username, cardNo string) (*StatusResult, error) {
	username = strings.TrimSpace(username)
	cardNo = strings.TrimSpace(cardNo)
	if username == "" || cardNo == "" {
		return nil, errors.New("用户名与卡号不能为空")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	if app.RechargeEnabled != 1 {
		return nil, errors.New("该应用未开启卡密充值")
	}

	var member models.Member
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_uuid = ? AND username = ?", app.UUID, username).First(&member).Error; err != nil {
			return errors.New("账号不存在")
		}
		var card models.Card
		if err := tx.Where("app_uuid = ? AND card_no = ?", app.UUID, cardNo).First(&card).Error; err != nil {
			return errors.New("卡号不存在")
		}
		if card.Status != models.CardStatusUnused {
			return errors.New("该卡已被使用或冻结")
		}

		// 计算充值后的到期时间
		var newExpiry time.Time
		if isPermanent(member.ExpiredAt) {
			return errors.New("账号已是永久，无需充值")
		}
		if card.Duration == models.CardDurationPermanent {
			newExpiry = models.PermanentTime
		} else {
			base := member.ExpiredAt
			if base.Before(time.Now()) {
				base = time.Now()
			}
			newExpiry = base.Add(time.Duration(card.Duration) * time.Minute)
		}

		if err := tx.Model(&member).Update("expired_at", newExpiry).Error; err != nil {
			return err
		}
		member.ExpiredAt = newExpiry
		return MarkCardUsed(tx, card.ID, member.UUID)
	})
	if err != nil {
		return nil, err
	}

	return &StatusResult{
		Username:  member.Username,
		Status:    member.Status,
		Permanent: isPermanent(member.ExpiredAt),
		ExpiredAt: member.ExpiredAt,
	}, nil
}

// GetMemberExpiry 获取到期时间（type 40）：校验令牌有效，返回到期信息（不因已过期而报错）。
func GetMemberExpiry(appUUID, token string) (*StatusResult, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	member, err := authMemberByToken(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	return &StatusResult{
		Username:  member.Username,
		Status:    member.Status,
		Permanent: isPermanent(member.ExpiredAt),
		ExpiredAt: member.ExpiredAt,
	}, nil
}
