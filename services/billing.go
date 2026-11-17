package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"

	"gorm.io/gorm"
)

// ============================================================================
// 应用计费策略（一号通：钱包全局，规则按应用）
// ============================================================================
//
// 账号的到期时间、点数、会员等级全站唯一（见 models.Member）。
// 每个应用自己决定用什么方式收费，见 models.BillingMode*：
//
//	计时模式：账号有效（未到期/永久）+ 会员权限等级 ≥ RequiredLevel，不扣点
//	点数模式：消耗账号点数（每次登录扣 PointsPerLogin，或按周期扣）
//	高级模式：等级 ≥ RequiredLevel，再按「该等级在此应用配置的点数」扣点
//
// 免费豁免：App.FreeMode=1 跳过全部门禁与扣费。
//
// 读取路径：App 行（计费字段）+ app_level_points 表（高级模式的等级点数）
// 热点路径（登录/心跳）会查等级点数，故高级模式下一律走带缓存的服务单例。

// Billing 某应用当前生效的计费参数（已解析完毕，调用方无需再查库）。
type Billing struct {
	// AppUUID 用于按等级取点数额时定位配置
	AppUUID string

	// Free 为 true 时跳过全部门禁与扣费
	Free bool

	// 三个正交门禁开关：真实判定依据，可任意组合
	TimeEnabled   bool // 要求账号有效期（卖时长）
	PointsEnabled bool // 按点数扣费
	LevelGate     bool // 要求会员等级 ≥ RequiredLevel

	// Mode 仅为兼容展示：由上面三个开关推导（models.DeriveBillingMode），不做判定
	Mode          int
	RequiredLevel int // 要求的最低权限等级（1=不限）
	ChargeMode    int // PointsChargePerCount / PointsChargePerTime
	// Scope：计费范围（0=按设备单独计费，1=账号合并计费）
	Scope         int
	PerLogin      float64 // 点数模式：按次每次扣点（支持小数）
	PeriodMinutes int // 按时周期（分钟）
	PerPeriod     float64 // 点数模式：按时每周期扣点（支持小数）
	HeartbeatCharge int // HeartbeatChargeOnLogin / HeartbeatChargeOnBeat

	// db 计算单价时使用的连接句柄。
	//
	// 事务内必须传 tx：SQLite 连接池只有 1 条连接（database.go: SetMaxOpenConns(1)），
	// 在事务里再调 database.GetDB() 会去抢那条已被事务占用的连接，互等死锁。
	db *gorm.DB

	// levelPoints：(MemberLevel.UUID → Points) 快照，点数开关打开时按等级取价
	levelPoints map[string]float64
	// loaded 表示 levelPoints 已加载（避免每次调用重复查库）
	loaded bool
}

// EffectiveBilling 解析某应用生效的计费参数。
// app 为 nil 时返回「免费」——调用方通常都会有 app，nil 只用于少数无上下文的场景。
func EffectiveBilling(app *models.App) *Billing {
	if app == nil {
		return &Billing{Free: true, Mode: models.BillingModeMember, RequiredLevel: models.DefaultMemberLevel, loaded: true}
	}
	b := &Billing{
		AppUUID:         app.UUID,
		Free:            app.FreeMode == 1,
		TimeEnabled:     app.BillTimeEnabled == 1,
		PointsEnabled:   app.BillPointsEnabled == 1,
		LevelGate:       app.LevelGateEnabled == 1,
		RequiredLevel:   app.RequiredLevel,
		ChargeMode:      app.PointsChargeMode,
		Scope:           app.BillingScope,
		PerLogin:        app.PointsPerLogin,
		PeriodMinutes:   app.PointsPeriodMinutes,
		PerPeriod:       app.PointsPerPeriod,
		HeartbeatCharge: app.PointsHeartbeatCharge,
	}
	// 兼容展示值：由三个开关推导，不再读 app.BillingMode
	b.Mode = models.DeriveBillingMode(b.TimeEnabled, b.PointsEnabled, b.LevelGate)
	// 安全兜底：非法值收敛到最保守的默认，避免「扣 0 点」或除零
	if b.RequiredLevel < models.DefaultMemberLevel {
		b.RequiredLevel = models.DefaultMemberLevel
	}
	if b.ChargeMode != models.PointsChargePerCount && b.ChargeMode != models.PointsChargePerTime {
		b.ChargeMode = models.PointsChargePerCount
	}
	if b.PerLogin < 0 {
		b.PerLogin = 0
	}
	if b.PerPeriod <= 0 {
		b.PerPeriod = 1
	}
	if b.PeriodMinutes <= 0 {
		b.PeriodMinutes = 60
	}
	return b
}

// ============================================================================
// 门禁判定
// ============================================================================
//
// 三个开关彼此独立，可任意组合：
//   计时 + 点数 → 既不能过期，也要扣点
//   仅计时      → 卖时长
//   仅点数      → 纯点数，账号到期也能用
//   两个都关    → 不按时长/点数收费（仍可能受会员限制）
// 会员限制可以叠加在任意组合之上。

// RequiresLevel 是否要求会员等级（只看 LevelGate）
func (b *Billing) RequiresLevel() bool {
	return !b.Free && b.LevelGate
}

// RequiresValidPeriod 是否要求账号有效期（只看计时开关）
// 只开点数时，账号即便时长到期，仍可通过点数继续使用。
func (b *Billing) RequiresValidPeriod() bool {
	return !b.Free && b.TimeEnabled
}

// UsesPoints 是否消耗点数（只看点数开关）
func (b *Billing) UsesPoints() bool {
	return !b.Free && b.PointsEnabled
}

// IsPointsChargeMode 是否按次扣点（否则按时）
func (b *Billing) IsPointsChargeMode() bool {
	return b.ChargeMode == models.PointsChargePerCount
}

// ============================================================================
// 等级点数（点数开关打开时按等级取价）
// ============================================================================

// WithDB 返回绑定了指定连接句柄的副本。
//
// 事务内取价时必须先调用它（如 chargeActivationOnce / chargePeriod），
// 否则惰性加载会走全局连接池，在单连接配置下与事务互等。
func (b *Billing) WithDB(db *gorm.DB) *Billing {
	if b == nil || db == nil {
		return b
	}
	c := *b
	c.db = db
	return &c
}

// loadLevelPoints 惰性加载该应用的等级点数配置。
// 只要点数开关打开就加载：等级点数不再是「高级模式专属」，
// 而是点数计费的一种定价覆盖（按等级给不同价格）。
// 直接查库；如需热点路径优化可再加进程内缓存（配置变更时失效）。
func (b *Billing) loadLevelPoints() {
	if b.loaded {
		return
	}
	b.loaded = true
	b.levelPoints = map[string]float64{}
	if !b.UsesPoints() || b.AppUUID == "" {
		return
	}
	// 优先用调用方给的句柄（事务内即 tx）；只有真正没有句柄时才回落全局池
	db := b.db
	if db == nil {
		global, err := database.GetDB()
		if err != nil || global == nil {
			return
		}
		db = global
	}
	if !db.Migrator().HasTable(&models.AppLevelPoint{}) {
		return
	}
	var rows []models.AppLevelPoint
	if err := db.Where("app_uuid = ?", b.AppUUID).Find(&rows).Error; err != nil {
		return
	}
	for _, r := range rows {
		b.levelPoints[r.MemberLevelUUID] = r.Points
	}
}

// IsMerged 是否为「账号合并计费」。
//
// 合并后按「账号 + 应用」只算一份钱：多设备不再重复扣点、也不再各自维护周期。
func (b *Billing) IsMerged() bool {
	return b.Scope == models.BillingScopeAccount
}

// PointsCostForLevel 返回本次扣费的点数：
//   - 该等级在「等级点数」里配了价 → 用配的价
//   - 没配 → 回退应用默认的 PerLogin / PerPeriod
//
// 等级点数对所有按点数计费的应用生效，不再限定旧「高级模式」。
// levelUUID 为账号当前会员等级（空表示无等级，即默认档）。
func (b *Billing) PointsCostForLevel(levelUUID string, perPeriod bool) float64 {
	if !b.UsesPoints() {
		return 0
	}
	fallback := b.PerLogin
	if perPeriod {
		fallback = b.PerPeriod
	}
	b.loadLevelPoints()
	if pts, ok := b.levelPoints[levelUUID]; ok {
		return pts
	}
	return fallback
}

// PointsPerPeriodForLevel 按时模式每周期点数（至少 1，避免免费续期漏洞）
func (b *Billing) PointsPerPeriodForLevel(levelUUID string) float64 {
	v := b.PointsCostForLevel(levelUUID, true)
	if v <= 0 {
		return 1
	}
	return v
}

// PointsPeriodMinutes 按时模式周期分钟数（至少 1）
func (b *Billing) PointsPeriodMinutes() int {
	if b.PeriodMinutes <= 0 {
		return 60
	}
	return b.PeriodMinutes
}

// ============================================================================
// 全局账号策略（一号通）
// ============================================================================
//
// 注册、卡密登录、充值、试用、邮箱验证、注册限制全部为平台级策略：
// 这些动作产生或消耗的是全局资源（账号、钱包、卡密、试用额度），
// 不属于任何单一应用，故不走 App 行。

// AccountPolicy 当前生效的账号策略。
type AccountPolicy struct {
	// 注册
	RegisterEnabled     bool
	EmailVerifyEnabled  bool
	CardRegisterEnabled bool

	// 注册限制（IP / 设备，全站计数）
	RegisterLimitEnabled       bool
	RegisterDeviceLimitEnabled bool
	RegisterLimitTime          int
	RegisterCount              int

	// 卡密
	CardLoginEnabled bool
	RechargeEnabled  bool


	// 绑定与转绑（账号级，跨应用统一）
	MachineVerify  int // 是否校验设备码：0 不校验 / 1 校验
	IPVerify       int // IP 校验强度：0 关闭 / 1 精确IP / 2 同市 / 3 同省
	MachineRebindEnabled int // 允许设备转绑：0 关闭 / 1 开启
	IPRebindEnabled      int // 允许 IP 转绑：0 关闭 / 1 开启
	// 设备转绑额度（与 IP 完全独立：换电脑和换网络是两回事）
	MachineRebindFreeCount int
	MachineRebindLimitTime int // 0 每天 / 1 永久
	MachineRebindMaxCount  int // 0=不限
	MachineRebindDeductMinutes int // 计时模式：超免费后每次扣分钟
	MachineRebindDeductPoints  int // 点数模式：超免费后每次扣点数
	// IP 转绑额度
	IPRebindFreeCount int
	IPRebindLimitTime int
	IPRebindMaxCount  int
	IPRebindDeductMinutes int
	IPRebindDeductPoints  int

	// 账号级资源配额（跨应用共用）
	MaxDevices       int // 可绑定设备数
	MaxConcurrent    int // 同时活跃实例数
	RebindFreeCount  int // 每窗口免费转绑次数
	RebindLimitTime  int // 0 每天 / 1 永久

	// 试用（可同时发放点数与时长，各自由配置决定）
	TrialEnabled         bool
	TrialLimitTime       int
	TrialPoints          float64 // 发放点数（支持小数）
	TrialDurationMinutes int // 发放在期时长（分钟）
	TrialClaimMode       int
}

// GetAccountPolicy 读取全局账号策略（带安全默认值）。
func GetAccountPolicy() *AccountPolicy {
	ss := GetSettingsService()
	p := &AccountPolicy{
		RegisterEnabled:            ss.GetBool(models.SettingAccountRegisterEnabled, true),
		EmailVerifyEnabled:         ss.GetBool(models.SettingAccountEmailVerifyEnabled, false),
		CardRegisterEnabled:        ss.GetBool(models.SettingAccountCardRegisterEnabled, false),
		RegisterLimitEnabled:       ss.GetBool(models.SettingAccountRegisterLimitEnabled, false),
		RegisterDeviceLimitEnabled: ss.GetBool(models.SettingAccountRegisterDeviceLimitEnabled, false),
		RegisterLimitTime:          ss.GetInt(models.SettingAccountRegisterLimitTime, models.LimitWindowPermanent),
		// 全站计数后默认值需大于 1：否则同一 IP 注册过第一个账号后就再也注册不了
		RegisterCount:        ss.GetInt(models.SettingAccountRegisterCount, 3),
		CardLoginEnabled:     ss.GetBool(models.SettingAccountCardLoginEnabled, true),
		RechargeEnabled:      ss.GetBool(models.SettingAccountRechargeEnabled, true),
		TrialEnabled:         ss.GetBool(models.SettingAccountTrialEnabled, false),
		TrialLimitTime:       ss.GetInt(models.SettingAccountTrialLimitTime, models.LimitWindowPermanent),
		TrialPoints:          ss.GetFloat(models.SettingAccountTrialPoints, 0),
		TrialDurationMinutes: ss.GetInt(models.SettingAccountTrialDurationMinutes, 0),
		TrialClaimMode:       ss.GetInt(models.SettingAccountTrialClaimMode, models.TrialClaimExhaustedOnly),


		// 绑定与转绑：默认校验设备、不校验 IP、允许转绑、不限次数、不扣费
		MachineVerify:  ss.GetInt(models.SettingAccountMachineVerify, 0),
		IPVerify:       ss.GetInt(models.SettingAccountIPVerify, 0),
		MachineRebindEnabled: ss.GetInt(models.SettingAccountMachineRebindEnabled, 1),
		IPRebindEnabled:      ss.GetInt(models.SettingAccountIPRebindEnabled, 1),

		MachineRebindFreeCount: ss.GetInt(models.SettingAccountMachineRebindFreeCount, 1),
		MachineRebindLimitTime: ss.GetInt(models.SettingAccountMachineRebindLimitTime, models.LimitWindowPermanent),
		MachineRebindMaxCount:  ss.GetInt(models.SettingAccountMachineRebindMaxCount, 0),
		MachineRebindDeductMinutes: ss.GetInt(models.SettingAccountMachineRebindDeductMinutes, 0),
		MachineRebindDeductPoints:  ss.GetInt(models.SettingAccountMachineRebindDeductPoints, 0),

		IPRebindFreeCount: ss.GetInt(models.SettingAccountIPRebindFreeCount, 2),
		IPRebindLimitTime: ss.GetInt(models.SettingAccountIPRebindLimitTime, models.LimitWindowPermanent),
		IPRebindMaxCount:  ss.GetInt(models.SettingAccountIPRebindMaxCount, 0),
		IPRebindDeductMinutes: ss.GetInt(models.SettingAccountIPRebindDeductMinutes, 0),
		IPRebindDeductPoints:  ss.GetInt(models.SettingAccountIPRebindDeductPoints, 0),

		// 账号级配额：默认 3 台设备、1 个并发实例、1 次免费转绑
		MaxDevices:      ss.GetInt(models.SettingAccountMaxDevices, 3),
		MaxConcurrent:   ss.GetInt(models.SettingAccountMaxConcurrent, 1),

	}
	if p.RegisterCount <= 0 {
		p.RegisterCount = 1
	}
	if p.TrialPoints < 0 {
		p.TrialPoints = 0
	}
	if p.TrialDurationMinutes < 0 {
		p.TrialDurationMinutes = 0
	}

	// 设备数与并发数至少为 1，否则账号永远进不去
	if p.MaxDevices <= 0 {
		p.MaxDevices = 1
	}
	if p.MaxConcurrent <= 0 {
		p.MaxConcurrent = 1
	}
	if p.RebindFreeCount < 0 {
		p.RebindFreeCount = 0
	}
	return p
}

// HasTrialQuota 是否配置了试用额度（点数和时长至少一项 > 0）
func (p *AccountPolicy) HasTrialQuota() bool {
	return p.TrialPoints > 0 || p.TrialDurationMinutes > 0
}

// ============================================================================
// 等级点数维护（后台）
// ============================================================================

// SaveAccountPolicy 批量写入全局账号策略并刷新缓存。
// items 为 (settings.name → 值字符串)；已存在的键更新，不存在的创建。
func SaveAccountPolicy(items map[string]string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		for name, value := range items {
			var count int64
			if err := tx.Model(&models.Settings{}).Where("name = ?", name).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := tx.Create(&models.Settings{Name: name, Value: value}).Error; err != nil {
					return err
				}
				continue
			}
			if err := tx.Model(&models.Settings{}).Where("name = ?", name).
				Update("value", value).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	// 写库成功后刷新进程内缓存，使新策略立即生效
	GetSettingsService().RefreshCache()
	return nil
}

// ListAppLevelPoints 列出某应用的等级点数配置（按等级排序）。
func ListAppLevelPoints(appUUID string) ([]models.AppLevelPoint, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	var rows []models.AppLevelPoint
	if err := db.Where("app_uuid = ?", appUUID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// SaveAppLevelPoints 批量保存某应用的等级点数配置（覆盖式）。
// items 为 (会员等级UUID → 点数)；点数为 0 表示显式配置为「该等级不扣点」。
func SaveAppLevelPoints(appUUID string, items map[string]float64) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_uuid = ?", appUUID).Delete(&models.AppLevelPoint{}).Error; err != nil {
			return err
		}
		for levelUUID, points := range items {
			if levelUUID == "" {
				continue
			}
			if points < 0 {
				points = 0
			}
			if err := tx.Create(&models.AppLevelPoint{
				AppUUID:         appUUID,
				MemberLevelUUID: levelUUID,
				Points:          points,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
