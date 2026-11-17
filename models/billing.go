package models

// ============================================================================
// 应用计费策略常量（一号通：钱包全局，规则按应用）
// ============================================================================
//
// 一号通的分界线：
//   - **钱包全局**：账号的到期时间、点数、累充、会员等级全站唯一（见 Member）
//   - **规则按应用**：每个应用自己决定「用什么方式收钱」（本文件）
//
// 三种计费模式：
//
//	计时模式：账号必须有效（未到期/永久）且会员权限等级 ≥ RequiredLevel，不扣点（买时长）
//	点数模式：消耗账号点数，每次登录扣 PointsPerLogin（或按周期扣）
//	高级模式：等级必须达标，同时按「该等级在此应用配置的点数」扣点
//
// 免费豁免：App.FreeMode=1 时跳过全部门禁与扣费（不论 BillingMode）。

// 应用计费模式
const (
	BillingModeMember = 0 // 计时模式：账号未到期 + 会员等级达标（不扣点）
	BillingModePoints = 1 // 点数模式：消耗账号点数
	BillingModeHybrid = 2 // 高级模式：会员等级达标 + 按等级消耗点数
)

// 计费范围：同一账号在多台设备上，是各自扣费还是合并成一份
const (
	BillingScopeDevice  = 0 // 按设备单独计费：每个激活各自扣
	BillingScopeAccount = 1 // 账号合并计费：整个账号只算一份
)

// BillingScopeName 计费范围的中文名
func BillingScopeName(scope int) string {
	if scope == BillingScopeAccount {
		return "账号合并"
	}
	return "按设备单独"
}

// 点数扣费方式（点数模式与高级模式共用）
const (
	PointsChargePerCount = 0 // 按次（登录扣点）
	PointsChargePerTime  = 1 // 按时（预扣费，按周期结算）
)

// 按时扣费触发方式
const (
	HeartbeatChargeOnLogin = 0 // 登录预扣（默认）
	HeartbeatChargeOnBeat  = 1 // 心跳触发/登录不预扣
)

// 默认权限等级：无等级账号的权限等级值（见 memberLevelInfo）
const DefaultMemberLevel = 1

// ============================================================================
// 账号策略常量（全局，因为产生/消耗的是全局资源）
// ============================================================================

// 试用领取方案
const (
	TrialClaimUnlimited     = 0 // 无限制：满足次数限制即可领取
	TrialClaimExhaustedOnly = 1 // 到期可领：仅账号资源耗尽（时长到期且点数为0）时可领取
)

// 限制窗口（注册限制 / 试用限制共用）
const (
	LimitWindowDaily     = 0 // 每天重置
	LimitWindowPermanent = 1 // 永久累计
)

// ============================================================================
// 全局配置键名（settings.name）
// ============================================================================
//
// 只有「产生/消耗全局资源」的策略在这里：注册、卡密登录、充值、试用、注册限制。
// 计费策略不在此处 —— 它按应用配置（见 models.App）。

// 账号策略
const (
	SettingAccountRegisterEnabled            = "account_register_enabled"             // 账号注册开关
	SettingAccountEmailVerifyEnabled         = "account_email_verify_enabled"         // 注册邮箱验证开关
	SettingAccountCardRegisterEnabled        = "account_card_register_enabled"        // 卡密注册开关
	SettingAccountRegisterLimitEnabled       = "account_register_limit_enabled"       // IP 注册限制开关
	SettingAccountRegisterDeviceLimitEnabled = "account_register_device_limit_enabled" // 设备注册限制开关
	SettingAccountRegisterLimitTime          = "account_register_limit_time"          // 0 每天 / 1 永久
	SettingAccountRegisterCount              = "account_register_count"               // 注册次数上限
	SettingAccountCardLoginEnabled           = "account_card_login_enabled"           // 卡密登录开关
	SettingAccountRechargeEnabled            = "account_recharge_enabled"             // 卡密充值开关
	SettingAccountTrialEnabled               = "account_trial_enabled"                // 试用领取开关
	SettingAccountTrialLimitTime             = "account_trial_limit_time"             // 0 每天 / 1 永久
	SettingAccountTrialPoints                = "account_trial_points"                 // 试用发放点数
	SettingAccountTrialDurationMinutes       = "account_trial_duration_minutes"       // 试用发放在期时长（分钟）
	SettingAccountTrialClaimMode             = "account_trial_claim_mode"             // 0 无限制 / 1 资源耗尽可领

	// ---- 账号级资源配额（设备数 / 并发数 / 转绑次数）----
	//
	// 这三项是**账号维度**的资源，不属于任何单个应用：
	//   · 设备数：这个账号一共能绑几台电脑（跨所有应用共用一份设备白名单）
	//   · 并发数：这个账号最多同时开着几个实例（跨所有应用合计）
	//   · 转绑次数：每个窗口内能免费换几次设备
	//
	// 会员等级可在此基础上加量（member_levels.extra_*）。
	SettingAccountMaxDevices       = "account_max_devices"        // 账号可绑定设备数
	SettingAccountMaxConcurrent   = "account_max_concurrent"     // 账号同时活跃实例数

	// 绑定与转绑：账号级（跨应用统一），不再按应用配置
	SettingAccountMachineVerify  = "account_machine_verify"   // 是否校验设备码
	SettingAccountIPVerify       = "account_ip_verify"        // 0关闭 1精确IP 2同市 3同省
	SettingAccountMachineRebindEnabled = "account_machine_rebind_enabled" // 允许设备转绑
	SettingAccountIPRebindEnabled      = "account_ip_rebind_enabled"      // 允许 IP 转绑

	// 设备转绑额度（与 IP 分开计次、分开扣费）
	SettingAccountMachineRebindFreeCount = "account_machine_rebind_free_count"
	SettingAccountMachineRebindLimitTime = "account_machine_rebind_limit_time"
	SettingAccountMachineRebindMaxCount  = "account_machine_rebind_max_count"
	SettingAccountMachineRebindDeductMinutes = "account_machine_rebind_deduct_minutes"
	SettingAccountMachineRebindDeductPoints  = "account_machine_rebind_deduct_points"

	// IP 转绑额度（换网络很常见，通常比设备宽松甚至不扣）
	SettingAccountIPRebindFreeCount = "account_ip_rebind_free_count"
	SettingAccountIPRebindLimitTime = "account_ip_rebind_limit_time"
	SettingAccountIPRebindMaxCount  = "account_ip_rebind_max_count"
	SettingAccountIPRebindDeductMinutes = "account_ip_rebind_deduct_minutes"
	SettingAccountIPRebindDeductPoints  = "account_ip_rebind_deduct_points"

)

// GlobalSettingNames 返回全部全局配置键名，供安装时批量写入默认值。
func GlobalSettingNames() []string {
	return []string{
		SettingAccountRegisterEnabled,
		SettingAccountEmailVerifyEnabled,
		SettingAccountCardRegisterEnabled,
		SettingAccountRegisterLimitEnabled,
		SettingAccountRegisterDeviceLimitEnabled,
		SettingAccountRegisterLimitTime,
		SettingAccountRegisterCount,
		SettingAccountCardLoginEnabled,
		SettingAccountRechargeEnabled,
		SettingAccountTrialEnabled,
		SettingAccountTrialLimitTime,
		SettingAccountTrialPoints,
		SettingAccountTrialDurationMinutes,
		SettingAccountTrialClaimMode,
		SettingAccountMaxDevices,
		SettingAccountMaxConcurrent,
		SettingAccountMachineVerify,
		SettingAccountIPVerify,
		SettingAccountMachineRebindEnabled,
		SettingAccountIPRebindEnabled,
		SettingAccountMachineRebindFreeCount,
		SettingAccountMachineRebindLimitTime,
		SettingAccountMachineRebindMaxCount,
		SettingAccountMachineRebindDeductMinutes,
		SettingAccountMachineRebindDeductPoints,
		SettingAccountIPRebindFreeCount,
		SettingAccountIPRebindLimitTime,
		SettingAccountIPRebindMaxCount,
		SettingAccountIPRebindDeductMinutes,
		SettingAccountIPRebindDeductPoints,
	}
}

// BillingModeName 计费模式文案
func BillingModeName(mode int) string {
	switch mode {
	case BillingModeMember:
		return "计时模式"
	case BillingModePoints:
		return "点数模式"
	case BillingModeHybrid:
		return "高级模式"
	default:
		return "未知模式"
	}
}

// IsValidBillingMode 校验计费模式
func IsValidBillingMode(mode int) bool {
	return mode >= BillingModeMember && mode <= BillingModeHybrid
}

// DeriveBillingMode 由三个正交开关推导「兼容展示用」的计费模式。
//
// 真实门禁只看 BillTimeEnabled / BillPointsEnabled / LevelGateEnabled，
// 本函数只负责给旧字段 billing_mode 一个说得过去的值，
// 让还没升级的客户端与老代码读到的 mode 不至于失真。
func DeriveBillingMode(timeEnabled, pointsEnabled, levelGate bool) int {
	switch {
	case timeEnabled && pointsEnabled:
		// 时长与点数同时要：相当于旧的「混合」
		return BillingModeHybrid
	case timeEnabled:
		return BillingModeMember
	case pointsEnabled && levelGate:
		// 等级 + 点数：旧「高级模式」
		return BillingModeHybrid
	case pointsEnabled:
		return BillingModePoints
	default:
		// 两个收费门禁都不开：视为不按模式收费（可能只靠等级限制）
		return BillingModeMember
	}
}

// BillingFlagName 三个开关的组合文案，用于日志与接口说明。
func BillingFlagName(timeEnabled, pointsEnabled, levelGate bool) string {
	switch {
	case timeEnabled && pointsEnabled:
		return "时长 + 点数"
	case timeEnabled:
		return "时长"
	case pointsEnabled:
		return "点数"
	default:
		return "不收费"
	}
}

// ApplyBillingFlags 按旧的单一枚举设置三个开关，并回写兼容值 BillingMode。
//
// 仅用于「程序内直接构造 App」的场景（测试夹具、一次性脚本）；
// HTTP 接口一律直接传三个开关，不要再用这个函数。
func (a *App) ApplyBillingFlags(mode int) {
	switch mode {
	case BillingModePoints:
		// 旧点数模式不看等级：只有显式配了 >1 的门槛才打开等级限制
		a.BillTimeEnabled, a.BillPointsEnabled = 0, 1
		if a.RequiredLevel > DefaultMemberLevel {
			a.LevelGateEnabled = 1
		} else {
			a.LevelGateEnabled = 0
		}
	case BillingModeHybrid:
		a.BillTimeEnabled, a.BillPointsEnabled = 0, 1
		a.LevelGateEnabled = 1
	default: // BillingModeMember
		// 旧计时模式始终比较等级（1 = 不限），等价于「等级门禁开着但门槛为 1」
		a.BillTimeEnabled, a.BillPointsEnabled = 1, 0
		a.LevelGateEnabled = 1
	}
	a.BillingMode = DeriveBillingMode(
		a.BillTimeEnabled == 1, a.BillPointsEnabled == 1, a.LevelGateEnabled == 1)
}