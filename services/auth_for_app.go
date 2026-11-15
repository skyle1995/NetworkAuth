package services

import (
	"errors"
	"strings"

	"NetworkAuth/models"

	"gorm.io/gorm"
)

// ============================================================================
// 应用级接口的统一鉴权（激活令牌优先，账号令牌走隐式激活）
// ============================================================================
//
// 面向需要「已进入某个应用」的接口：41 心跳、42/43/44/45/54 数据与功能、53 扣点。
//
// 两种令牌都接受：
//   激活令牌 —— 正常路径。直接命中 app_activations，无额外开销
//   账号令牌 —— 兼容路径。说明客户端（多为老版本）没调 70 就直接用了应用级接口，
//               此时**隐式执行一次激活**（幂等，不重复扣费），然后继续处理本次请求。
//
// 隐式激活的必要性：协议向后兼容。老客户端 `20 登录` 后直接调 `41`，
// 不能因为没调 70 就整个坏掉。

// ErrTokenAppMismatch 令牌与应用不匹配（拿 A 的令牌调 B 的接口）
var ErrTokenAppMismatch = NewCodedError(models.ErrInvalidRequest, "令牌与当前应用不匹配")

// AuthForApp 应用级接口的鉴权入口。
//
// 返回：账号、应用、激活记录。
// 若传入的是账号令牌，会隐式创建/复用激活（可能产生扣费）。
func AuthForApp(db *gorm.DB, appUUID, token string) (*models.Member, *models.App, *models.AppActivation, error) {
	token = strings.TrimSpace(token)
	appUUID = strings.TrimSpace(appUUID)
	if token == "" {
		return nil, nil, nil, ErrActivationRequiredMarker
	}

	// ① 激活令牌：正常路径
	if member, act, err := AuthActivation(db, token); err == nil {
		var app models.App
		if err := db.Where("uuid = ?", act.AppUUID).First(&app).Error; err != nil {
			return nil, nil, nil, ErrAppNotFound
		}
		// 信封里的 app_uuid 必须与激活所属应用一致，否则等于跨应用盗用令牌
		if appUUID != "" && !strings.EqualFold(act.AppUUID, appUUID) {
			return nil, nil, nil, ErrTokenAppMismatch
		}
		if app.Status != 1 {
			return nil, nil, nil, ErrAppDisabled
		}
		if member.Status != models.MemberStatusNormal {
			return nil, nil, nil, errors.New("账号状态异常")
		}
		return member, &app, act, nil
	}
	// 账号令牌不能直接调应用级接口：必须先用 70 激活换激活令牌。
	// （不做隐式激活——激活会扣费，静默替客户端扣钱是危险的副作用）
	if _, err := AuthAccountToken(db, token); err == nil {
		return nil, nil, nil, ErrActivationRequiredMarker
	}
	return nil, nil, nil, ErrActivationInvalid
}

// AuthForAccount 账号级接口的鉴权入口（接受账号令牌）。
// 若客户端拿激活令牌调账号级接口，也放行（惰性兼容，避免客户端换令牌逻辑出错）。
func AuthForAccount(db *gorm.DB, token string) (*models.Member, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrAccountTokenInvalid
	}

	if acct, err := AuthAccountToken(db, token); err == nil {
		var member models.Member
		if err := db.Where("uuid = ?", acct.MemberUUID).First(&member).Error; err != nil {
			return nil, ErrAccountTokenInvalid
		}
		TouchAccountToken(db, acct.UUID)
		return &member, nil
	}

	// 兼容：激活令牌也可用于账号级接口
	if member, _, err := AuthActivation(db, token); err == nil {
		return member, nil
	}
	return nil, ErrAccountTokenInvalid
}

// AuthForAccountAny 账号级接口的宽松鉴权：账号令牌或激活令牌都接受。
//
// 用于「查询类」账号接口（如 40 查到期），客户端手里可能只有其中一种令牌。
// 写操作的账号接口仍应走 AuthForAccount，要求账号令牌。
func AuthForAccountAny(db *gorm.DB, token string) (*models.Member, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrAccountTokenInvalid
	}
	if acct, err := AuthAccountToken(db, token); err == nil {
		var member models.Member
		if err := db.Where("uuid = ?", acct.MemberUUID).First(&member).Error; err != nil {
			return nil, ErrAccountTokenInvalid
		}
		return &member, nil
	}
	_, act, err := AuthActivation(db, token)
	if err != nil {
		return nil, ErrAccountTokenInvalid
	}
	var member models.Member
	if err := db.Where("uuid = ?", act.MemberUUID).First(&member).Error; err != nil {
		return nil, ErrAccountTokenInvalid
	}
	return &member, nil
}