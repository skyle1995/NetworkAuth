package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"NetworkAuth/database"
	"NetworkAuth/models"

	"gorm.io/gorm"
)

// ============================================================================
// 权益清单（Entitlements）
// ============================================================================
//
// 对应客户端 `72 权益清单`：
//   - 下发「本账号可用的应用」列表（uuid + secret），客户端直接用 uuid 对应到自己的插件；
//   - **secret 只对可用的应用下发**：未购买/等级不足的应用返回 usable=0 且 secret 留空，
//     避免逆向客户端后拿到全部签发密钥；
//   - 附带账号当前的钱包与会员态，供管理端渲染。
//
// 设计上刻意与「激活」解耦：清单查询**不扣费、不建激活**，
// 只是把门禁预判结果提前告诉客户端，减少无意义的 70 调用与报错弹窗。

// EntitlementBilling 单个应用的收费信息（供客户端展示「收费模式/消耗方式/会员限制」）
type EntitlementBilling struct {
	Mode            int     `json:"mode"`             // 0 计时 / 1 点数 / 2 高级
	ModeName        string  `json:"mode_name"`        // 计时模式 / 点数模式 / 高级模式
	Free            bool    `json:"free"`             // 免费应用：跳过门禁与扣费
	RequiredLevel   int     `json:"required_level"`   // 最低会员等级（1 = 默认，无额外限制）
	Scope           int     `json:"scope"`            // 0 按设备单独 / 1 账号合并
	ScopeName       string  `json:"scope_name"`
	ChargeMode      int     `json:"charge_mode"`      // 0 按次（登录扣）/ 1 按时（周期扣）
	ChargeModeName  string  `json:"charge_mode_name"` // 按次扣点 / 按时扣点
	PerLogin        float64 `json:"per_login"`        // 每次登录扣点数
	PeriodMinutes   int     `json:"period_minutes"`   // 扣费周期（分钟）
	PerPeriod       float64 `json:"per_period"`       // 每周期扣点数
	HeartbeatCharge int     `json:"heartbeat_charge"` // 0 登录预扣 / 1 心跳触发
}

// EntitlementItem 单个应用的权益项
//
// 除「能不能用」外还给出「为什么不能用、还差什么」，客户端据此直接渲染，
// 不必自己推导规则；DenyCode 与 70 激活返回的错误码同源，便于统一处理。
type EntitlementItem struct {
	UUID        string `json:"uuid"`
	Secret      string `json:"secret,omitempty"` // 仅 usable=1 时下发
	Name        string `json:"name"`
	Version     string `json:"version"`
	MinVersion  string `json:"min_version,omitempty"`
	Mode        int    `json:"mode"`
	Free        bool   `json:"free"`

	// —— 等级门禁 ——
	RequiredLevel     int    `json:"required_level"`      // 本应用要求的最低权限等级（1=不限）
	RequiredLevelName string `json:"required_level_name"` // 要求等级的名称（如「黄金」），便于直接展示
	MyLevel           int    `json:"my_level"`            // 账号当前权限等级
	MyLevelName       string `json:"my_level_name"`
	LevelOk           bool   `json:"level_ok"` // 等级是否达标（不要求等级时恒为 true）

	// —— 有效期门禁（仅计时模式）——
	Permanent bool      `json:"permanent"`
	ExpiredAt time.Time `json:"expired_at"`
	PeriodOk  bool      `json:"period_ok"` // 有效期是否达标

	// —— 点数预判（点数/高级模式）——
	ChargeMode    int     `json:"charge_mode"`    // 0 按次 / 1 按时
	PointsNeeded  float64 `json:"points_needed"`  // 启动一次（按时=一个周期）需要多少点
	PointsBalance float64 `json:"points_balance"` // 账号当前点数余额
	PointsEnough  bool    `json:"points_enough"`  // 余额是否够启动
	BlockedByPoints bool  `json:"blocked_by_points"` // 点数不足（独立于 usable，供客户端提示「充值即可用」）

	Usable    int                 `json:"usable"` // 1=可用 / 0=不可用
	Reason    string              `json:"reason,omitempty"`
	DenyCode  string              `json:"deny_code,omitempty"` // 机器可读的拒绝码，与 70 激活错误码同源
	Billing   *EntitlementBilling `json:"billing,omitempty"`
}

// EntitlementsResult 权益清单返回体
type EntitlementsResult struct {
	Plugins []EntitlementItem `json:"plugins"`
	Account struct {
		Username      string    `json:"username"`
		Type          int       `json:"type"`
		Permanent     bool      `json:"permanent"`
		ExpiredAt     time.Time `json:"expired_at"`
		Points        float64       `json:"points"`
		TotalRecharge int       `json:"total_recharge"`
		Level         int       `json:"level"`
		LevelName     string    `json:"level_name"`
		LevelColor    string    `json:"level_color"`
		RebateRate    int       `json:"rebate_rate"`
	} `json:"account"`
}

// ErrEntitlementAccount 账号不可用
var ErrEntitlementAccount = NewCodedError(models.ErrLicenseSuspended, "")

// entitlementDenyCode 把门禁错误映射成与 70 激活一致的错误码。
func entitlementDenyCode(err error) string {
	var entErr *EntitlementError
	if errors.As(err, &entErr) {
		return models.ErrEntitlementRequired
	}
	var expErr *LicenseExpiredError
	if errors.As(err, &expErr) {
		return models.ErrLicenseExpired
	}
	// CodedError 是接口：实现了 ErrCode() 的可直接取码
	var coded interface{ ErrCode() string }
	if errors.As(err, &coded) {
		if c := coded.ErrCode(); c != "" {
			return c
		}
	}
	return models.ErrAccountPolicyDenied
}

// ListEntitlements 生成某账号的权益清单。
//
// includeHost 控制是否把宿主应用也列出来（默认不列：外壳不是"插件"）。
func ListEntitlements(db *gorm.DB, acct *models.MemberAccountToken, fingerprint string) (*EntitlementsResult, error) {
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}
	if acct == nil {
		return nil, ErrAccountTokenInvalid
	}

	var member models.Member
	if err := db.Where("uuid = ?", acct.MemberUUID).First(&member).Error; err != nil {
		return nil, ErrAccountTokenInvalid
	}
	if member.Status == models.MemberStatusBlack {
		return nil, NewCodedError(models.ErrLicenseSuspended, "账号已被拉黑")
	}
	if member.Status == models.MemberStatusDisabled {
		return nil, NewCodedError(models.ErrLicenseSuspended, "账号已被封停")
	}

	// 全站黑名单（设备/IP 维度）命中时，所有应用都不可用
	blProvince, blCity := ResolveIPRegion(acct.IP)
	blocked, blockReason := CheckBlacklist(db, fingerprint, acct.IP, blProvince, blCity)

	var apps []models.App
	if err := db.Where("status = 1").Order("name ASC").Find(&apps).Error; err != nil {
		return nil, err
	}

	res := &EntitlementsResult{Plugins: make([]EntitlementItem, 0, len(apps))}
	for i := range apps {
		app := &apps[i]
		billing := EffectiveBilling(app)
		chargeName := "按次扣点"
		if billing.ChargeMode == models.PointsChargePerTime {
			chargeName = "按时扣点"
		}
		item := EntitlementItem{
			UUID:          app.UUID,
			Name:          app.Name,
			Version:       app.Version,
			Mode:          billing.Mode,
			Free:          billing.Free,
			RequiredLevel: billing.RequiredLevel,
			Usable:        1,
			Billing: &EntitlementBilling{
				Mode:            billing.Mode,
				ModeName:        models.BillingModeName(billing.Mode),
				Free:            billing.Free,
				RequiredLevel:   billing.RequiredLevel,
				Scope:           billing.Scope,
				ScopeName:       models.BillingScopeName(billing.Scope),
				ChargeMode:      billing.ChargeMode,
				ChargeModeName:  chargeName,
				PerLogin:        billing.PerLogin,
				PeriodMinutes:   billing.PeriodMinutes,
				PerPeriod:       billing.PerPeriod,
				HeartbeatCharge: billing.HeartbeatCharge,
			},
		}

		// 等级与有效期：与 70 激活用同一个门禁函数，保证「清单说能用」= 「激活真能过」
		item.MyLevel = memberPermissionLevel(db, &member)
		item.MyLevelName = levelNameByValue(db, item.MyLevel)
		// 等级表可能没有 1 级记录：给客户端一个可显示的名字，避免出现空标签
		if item.MyLevelName == "" {
			item.MyLevelName = "默认档"
		}
		item.RequiredLevelName = levelNameByValue(db, billing.RequiredLevel)
		if billing.RequiredLevel > 1 && item.RequiredLevelName == "" {
			item.RequiredLevelName = fmt.Sprintf("Lv.%d", billing.RequiredLevel)
		}
		item.Permanent = isPermanent(member.ExpiredAt)
		item.ExpiredAt = member.ExpiredAt
		item.LevelOk = !billing.RequiresLevel() || item.MyLevel >= billing.RequiredLevel
		item.PeriodOk = !billing.RequiresValidPeriod() || item.Permanent || member.ExpiredAt.After(time.Now())

		// 点数预判：单价与实扣同源（PointsCostForLevel / PointsPerPeriodForLevel）
		item.ChargeMode = billing.ChargeMode
		item.PointsBalance = member.Points
		item.PointsEnough = true
		if billing.UsesPoints() {
			if billing.ChargeMode == models.PointsChargePerTime {
				item.PointsNeeded = billing.PointsPerPeriodForLevel(member.LevelUUID)
			} else {
				item.PointsNeeded = billing.PointsCostForLevel(member.LevelUUID, false)
			}
			item.PointsEnough = member.Points+1e-9 >= item.PointsNeeded
		}

		switch {
		case blocked:
			item.Usable = 0
			item.Reason = blockReason
			item.DenyCode = models.ErrLicenseSuspended
		default:
			if err := checkActivationGate(db, billing, &member); err != nil {
				item.Usable = 0
				item.Reason = err.Error()
				item.DenyCode = entitlementDenyCode(err)
			} else if !item.PointsEnough {
				// 点数门禁不在 checkActivationGate 内（幂等复用不该被余额拦住），
				// 但「能否启动」的预判必须算上：余额不足直接标不可用，客户端才不会白试一次 70
				item.Usable = 0
				item.BlockedByPoints = true
				item.DenyCode = models.ErrInsufficientCredits
				item.Reason = fmt.Sprintf("点数不足：本次需 %.3f 点，当前 %.3f 点", item.PointsNeeded, item.PointsBalance)
			}
		}

		// 可用才下发 secret：逆向客户端拿不到无权限应用的密钥
		if item.Usable == 1 {
			item.Secret = app.Secret
		}
		res.Plugins = append(res.Plugins, item)
	}

	levelName, level, rebate, levelColor := memberLevelInfo(db, &member)
	res.Account.Username = member.Username
	res.Account.Type = member.Type
	res.Account.Permanent = isPermanent(member.ExpiredAt)
	res.Account.ExpiredAt = member.ExpiredAt
	res.Account.Points = member.Points
	res.Account.TotalRecharge = member.TotalRecharge
	res.Account.Level = level
	res.Account.LevelName = levelName
	res.Account.LevelColor = levelColor
	res.Account.RebateRate = rebate

	return res, nil
}

// ============================================================================
// 高层入口（供 controllers 调用，内部取连接）
// ============================================================================

// getDBOrErr 取全局数据库连接
func getDBOrErr() (*gorm.DB, error) {
	return database.GetDB()
}

// ActivateByToken 用账号令牌激活应用（type 70）。
func ActivateByToken(appUUID, accountToken, fingerprint, ip, version, deviceName string) (*ActivationInfo, error) {
	db, err := getDBOrErr()
	if err != nil {
		return nil, err
	}
	acct, err := AuthAccountToken(db, accountToken)
	if err != nil {
		return nil, err
	}
	app, err := loadAppForActivation(db, appUUID)
	if err != nil {
		return nil, err
	}
	return Activate(db, acct, app, fingerprint, ip, version, deviceName)
}

// DeactivateByToken 释放激活（type 71）。幂等。
func DeactivateByToken(activationToken string) error {
	db, err := getDBOrErr()
	if err != nil {
		return err
	}
	return Deactivate(db, activationToken)
}

// EntitlementsByToken 权益清单（type 72）。
func EntitlementsByToken(accountToken, fingerprint string) (*EntitlementsResult, error) {
	db, err := getDBOrErr()
	if err != nil {
		return nil, err
	}
	acct, err := AuthAccountToken(db, accountToken)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fingerprint) == "" {
		fingerprint = acct.Fingerprint
	}
	return ListEntitlements(db, acct, fingerprint)
}

// loadAppForActivation 加载待激活的应用并做存在性/状态校验
func loadAppForActivation(db *gorm.DB, appUUID string) (*models.App, error) {
	appUUID = strings.TrimSpace(appUUID)
	if appUUID == "" {
		return nil, ErrAppNotFound
	}
	var app models.App
	if err := db.Where("uuid = ?", appUUID).First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAppNotFound
		}
		return nil, err
	}
	return &app, nil
}
