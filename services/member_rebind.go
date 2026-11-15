package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// 机器码 / IP 转绑（type 51 / 52）
// ============================================================================
//
// 机器码与 IP 转绑逻辑一致，仅字段不同，抽成共享核心 rebindCore：
//   - 校验转绑开关；按“每天/永久”限制重置并校验次数；
//   - 免费次数内不扣时，超出后每次扣除配置的分钟数（永久账号不扣）；
//   - 转绑即用新值替换该类型的旧绑定（单机转移语义）。

// rebindParams 单次转绑所需的配置与当前计数
type rebindParams struct {
	bindingType int    // 绑定类型：机器码/IP
	typeName    string // 展示名："机器码"/"IP"
	enabled     int    // 转绑开关（app）
	limit       int    // 限制周期：0=每天，1=永久
	freeCount   int    // 免费转绑次数
	maxCount    int    // 允许转绑次数（0=不限）
	deductMinutes int     // 计时模式下的单次扣除（分钟）
	deductPoints  float64 // 点数模式下的单次扣除（点，支持小数）
	used        int    // 该用户当前已用次数
	dateStr     string // 该用户计数日期
	usedCol     string // 已用次数列名
	dateCol     string // 计数日期列名
}

// rebindCore 执行一次转绑，返回转绑后的账号状态。
// 转绑扣除按运营模式：时长模式扣分钟，点数模式扣点数。
func rebindCore(db *gorm.DB, app *models.App, member *models.Member, newValue string, p rebindParams) (*StatusResult, error) {
	newValue = strings.TrimSpace(newValue)
	if newValue == "" {
		return nil, errors.New("新" + p.typeName + "不能为空")
	}
	if p.enabled != 1 {
		return nil, errors.New("该应用未开启" + p.typeName + "转绑")
	}

	// 防误重绑：目标值已是当前绑定则直接放行，不计次、不扣费（幂等，避免同设备白扣一次）
	var alreadyBound int64
	if err := db.Model(&models.Binding{}).
		Where("member_uuid = ? AND type = ? AND value = ?", member.UUID, p.bindingType, newValue).
		Count(&alreadyBound).Error; err != nil {
		return nil, err
	}
	if alreadyBound > 0 {
		return buildStatusResult(app, member), nil
	}

	today := time.Now().Format("2006-01-02")
	used := p.used
	// 每天限制：跨天则重置计数
	if p.limit == 0 && p.dateStr != today {
		used = 0
	}
	// 次数上限（maxCount>0 时生效）
	if p.maxCount > 0 && used >= p.maxCount {
		return nil, errors.New(p.typeName + "转绑次数已达上限")
	}

	// 免费次数用尽后按钱包类型扣：计时扣分钟、点数扣点数（两者各有一套配置）
	billing := EffectiveBilling(app)
	var deduct float64
	if used >= p.freeCount {
		if billing.UsesPoints() {
			deduct = p.deductPoints
		} else {
			deduct = float64(p.deductMinutes)
		}
	}
	// 计费豁免应用：转绑一律不扣费（仍照常计次、执行绑定替换）
	if billing.Free {
		deduct = 0
	}


	err := db.Transaction(func(tx *gorm.DB) error {
		if err := applyRebindDeduct(tx, billing, member, deduct, billing.UsesPoints()); err != nil {
			return err
		}
		// 用新值替换该类型的旧绑定
		if err := tx.Where("member_uuid = ? AND type = ?", member.UUID, p.bindingType).
			Delete(&models.Binding{}).Error; err != nil {
			return err
		}
		newBinding := models.Binding{
			MemberUUID: member.UUID,
			Type:       p.bindingType,
			Value:      newValue,
		}
		// IP 转绑时补充归属地，支撑后续省/市级验证
		if p.bindingType == models.BindingTypeIP {
			newBinding.Province, newBinding.City = ResolveIPRegion(newValue)
		}
		if err := tx.Create(&newBinding).Error; err != nil {
			return err
		}
		// 更新计数与日期
		return tx.Model(member).Updates(map[string]interface{}{
			p.usedCol: used + 1,
			p.dateCol: today,
		}).Error
	})
	if err != nil {
		return nil, err
	}

	AddMemberLog(app.UUID, member.UUID, member.Username, p.typeName+"转绑", "新值 "+newValue, "")
	return buildStatusResult(app, member), nil
}

// applyRebindDeduct 换绑扣费（事务内）：点数模式扣点，时长模式扣分钟（永久账号不扣）。deduct<=0 直接返回。
func applyRebindDeduct(tx *gorm.DB, billing *Billing, member *models.Member, deduct float64, pointsMode bool) error {
	if deduct <= 0 {
		return nil
	}
	if pointsMode {
		newPoints := member.Points - deduct
		if newPoints < 0 {
			newPoints = 0
		}
		if err := tx.Model(member).Update("points", newPoints).Error; err != nil {
			return err
		}
		member.Points = newPoints
		return nil
	}
	if isPermanent(member.ExpiredAt) {
		return nil
	}
	newExpiry := member.ExpiredAt.Add(-time.Duration(deduct) * time.Minute)
	if newExpiry.Before(time.Now()) {
		newExpiry = time.Now()
	}
	if err := tx.Model(member).Update("expired_at", newExpiry).Error; err != nil {
		return err
	}
	member.ExpiredAt = newExpiry
	return nil
}

// BoundDevice 换绑设备列表项：设备号 + 设备名 + 绑定时间，供用户识别要替换哪个设备。
type BoundDevice struct {
	MachineCode string `json:"machine_code"`
	DeviceName  string `json:"device_name"`
	BoundAt     string `json:"bound_at"`
}

// BindingInfo 账号的绑定规模与转绑额度快照，登录与心跳都随响应下发。
//
// 设备转绑与 IP 转绑是**两套独立额度**：换电脑和换网络是两件事，
// 共用配额会让「出差连了别的 WiFi」把设备转绑次数吃掉。
type BindingInfo struct {
	MaxDevices   int
	BoundDevices []BoundDevice

	// 设备转绑
	MachineFreeLeft  int
	MachineUsed      int
	MachineMaxCount  int
	MachineLimitTime int

	// IP 转绑
	IPFreeAllLeft    int
	IPUsed           int
	IPMaxCount       int
	IPLimitTime      int
	BoundIPs         []string
}

// BuildBindingInfo 汇总「已绑设备 / IP / 各自剩余免费转绑次数」。
//
// 计数窗口：limitTime=0 时跨天归零，=1 时永久累计。
// 免费次数 = 账号策略 + 会员等级加量（extra_rebind_count），设备与 IP 各自加一次。
func BuildBindingInfo(db *gorm.DB, m *models.Member) BindingInfo {
	quota := ResolveAccountQuota(db, m)
	policy := GetAccountPolicy()
	_, _, extraRebind := memberLevelExtras(db, m)
	today := time.Now().Format("2006-01-02")

	// —— 设备转绑 ——
	mUsed := m.MachineRebindUsed
	if policy.MachineRebindLimitTime == 0 && m.MachineRebindDate != today {
		mUsed = 0
	}
	mFreeLeft := policy.MachineRebindFreeCount + extraRebind - mUsed
	if mFreeLeft < 0 {
		mFreeLeft = 0
	}

	// —— IP 转绑 ——
	ipUsed := m.IPRebindUsed
	if policy.IPRebindLimitTime == 0 && m.IPRebindDate != today {
		ipUsed = 0
	}
	ipFreeLeft := policy.IPRebindFreeCount + extraRebind - ipUsed
	if ipFreeLeft < 0 {
		ipFreeLeft = 0
	}

	return BindingInfo{
		MaxDevices:       quota.MaxDevices,
		BoundDevices:     listMachineBindings(db, m.UUID),
		MachineFreeLeft:  mFreeLeft,
		MachineUsed:      mUsed,
		MachineMaxCount:  policy.MachineRebindMaxCount,
		MachineLimitTime: policy.MachineRebindLimitTime,
		IPFreeAllLeft:    ipFreeLeft,
		IPUsed:           ipUsed,
		IPMaxCount:       policy.IPRebindMaxCount,
		IPLimitTime:      policy.IPRebindLimitTime,
		BoundIPs:         listIPBindings(db, m.UUID),
	}
}

// listIPBindings 返回账号当前绑定的 IP 列表（按绑定时间升序）。
func listIPBindings(db *gorm.DB, memberUUID string) []string {
	var bindings []models.Binding
	if err := db.Where("member_uuid = ? AND type = ?", memberUUID, models.BindingTypeIP).
		Order("created_at ASC").Find(&bindings).Error; err != nil {
		return []string{}
	}
	list := make([]string, 0, len(bindings))
	for _, b := range bindings {
		list = append(list, b.Value)
	}
	return list
}
// listMachineBindings 返回账号当前的机器码绑定设备列表（按绑定时间升序）。
func listMachineBindings(db *gorm.DB, memberUUID string) []BoundDevice {
	var bindings []models.Binding
	db.Where("member_uuid = ? AND type = ?", memberUUID, models.BindingTypeMachine).
		Order("created_at ASC").Find(&bindings)
	list := make([]BoundDevice, 0, len(bindings))
	for _, b := range bindings {
		list = append(list, BoundDevice{
			MachineCode: b.Value,
			DeviceName:  b.DeviceName,
			BoundAt:     b.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return list
}

// rebindMachine 机器码换绑（多设备感知）：
//   - 目标设备已绑定 → 幂等放行，不计次不扣费
//   - 未达有效多开上限 → 直接新增该设备绑定
//   - 已满 → 须指定要替换的旧设备 replaceMachine，精确替换那一个
//     （有效多开=1 且恰好 1 个绑定时，自动替换唯一设备，兼容单设备旧客户端）
//
// 计次/免费次数/扣费规则与 IP 换绑一致；freeCount 已含会员赠送的免费次数。
func rebindMachine(db *gorm.DB, app *models.App, member *models.Member, newMachine, replaceMachine, deviceName string, effMultiOpen, freeCount, rebindWindow, maxCount, deductMinutes int, deductPoints float64) (*StatusResult, error) {
	newMachine = strings.TrimSpace(newMachine)
	if newMachine == "" {
		return nil, errors.New("新机器码不能为空")
	}

	// 目标已绑定 → 幂等，不计次不扣费
	var already int64
	if err := db.Model(&models.Binding{}).
		Where("member_uuid = ? AND type = ? AND value = ?", member.UUID, models.BindingTypeMachine, newMachine).
		Count(&already).Error; err != nil {
		return nil, err
	}
	if already > 0 {
		return buildStatusResult(app, member), nil
	}

	// 计次（每天限制则跨天重置）与上限
	today := time.Now().Format("2006-01-02")
	used := member.MachineRebindUsed
	if rebindWindow == 0 && member.MachineRebindDate != today {
		used = 0
	}
	if maxCount > 0 && used >= maxCount {
		return nil, errors.New("机器码转绑次数已达上限")
	}

	var bindings []models.Binding
	if err := db.Where("member_uuid = ? AND type = ?", member.UUID, models.BindingTypeMachine).
		Order("created_at ASC").Find(&bindings).Error; err != nil {
		return nil, err
	}

	// 满员时确定要替换的旧设备；未满则纯新增（toDelete 为空）
	replaceMachine = strings.TrimSpace(replaceMachine)
	toDelete := ""
	if len(bindings) >= effMultiOpen {
		if replaceMachine == "" {
			if effMultiOpen == 1 && len(bindings) == 1 {
				toDelete = bindings[0].Value
			} else {
				return nil, errors.New("设备数已达上限，请指定要替换的设备")
			}
		} else {
			for _, b := range bindings {
				if b.Value == replaceMachine {
					toDelete = replaceMachine
					break
				}
			}
			if toDelete == "" {
				return nil, errors.New("要替换的设备不存在")
			}
		}
	}

	billing := EffectiveBilling(app)
	var deduct float64
	if used >= freeCount {
		if billing.UsesPoints() {
			deduct = deductPoints
		} else {
			deduct = float64(deductMinutes)
		}
	}
	if billing.Free {
		deduct = 0
	}


	err := db.Transaction(func(tx *gorm.DB) error {
		if err := applyRebindDeduct(tx, billing, member, deduct, billing.UsesPoints()); err != nil {
			return err
		}
		if toDelete != "" {
			if err := tx.Where("member_uuid = ? AND type = ? AND value = ?",
				member.UUID, models.BindingTypeMachine, toDelete).Delete(&models.Binding{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&models.Binding{
			MemberUUID: member.UUID,
			Type:       models.BindingTypeMachine,
			Value:      newMachine,
			DeviceName: strings.TrimSpace(deviceName),
		}).Error; err != nil {
			return err
		}
		return tx.Model(member).Updates(map[string]interface{}{
			"machine_rebind_used": used + 1,
			"machine_rebind_date": today,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	AddMemberLog(app.UUID, member.UUID, member.Username, "机器码转绑", "新设备 "+newMachine, "")
	return buildStatusResult(app, member), nil
}

// Rebind 统一转绑（原 type 51 机器码 / 52 IP 合并为一个功能）。
// 凭账号密码鉴权（卡密账号以卡号为身份，无需密码），**不要求登录会话**——
// 从而打破“设备/IP 对不上→登不进→拿不到令牌→无法转绑”的死循环。
// 按应用配置转绑已开启的维度：机器码转绑须带设备码，IP 转绑用当前请求 IP。
// 两个维度都开时依次转绑，各自独立计次/扣费。
// machineCode 为空且开启机器码转绑时，返回当前绑定设备列表（供客户端选择要替换的设备），不做任何变更。
// 携带 machineCode 时：未满多开则新增该设备，已满则替换 replaceMachine 指定的旧设备。
// deviceName 为客户端采集的设备名，随新绑定记录。会员等级赠送的免费换绑次数并入免费次数。
func Rebind(appUUID, username, password, machineCode, replaceMachine, deviceName, ip string) (any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	member, err := authMemberByCredential(db, username, password)
	if err != nil {
		return nil, err
	}

	// 绑定与转绑全部是**账号级**配置：换一台电脑，全站所有应用一起生效，
	// 不再出现「在 A 应用改过、B 应用却看不到」的错位。
	policy := GetAccountPolicy()
	if policy.MachineRebindEnabled != 1 && policy.IPRebindEnabled != 1 {
		return nil, errors.New("未开启转绑")
	}

	quota := ResolveAccountQuota(db, member)
	maxDevices := quota.MaxDevices

	var result *StatusResult

	// 机器码转绑（多设备感知）
	if policy.MachineRebindEnabled == 1 {
		// 未提交设备码 → 本接口退化为「查询已绑定设备」，客户端据此选择要替换的那台
		if strings.TrimSpace(machineCode) == "" {
			return map[string]any{"devices": listMachineBindings(db, member.UUID)}, nil
		}
		result, err = rebindMachine(db, app, member, machineCode, replaceMachine, deviceName,
			maxDevices, policy.MachineRebindFreeCount, policy.MachineRebindLimitTime,
			policy.MachineRebindMaxCount, policy.MachineRebindDeductMinutes, float64(policy.MachineRebindDeductPoints))
		if err != nil {
			return nil, err
		}
	}

	// IP 转绑：用当前请求 IP（单值替换；免费次数含会员赠送）
	if policy.IPRebindEnabled == 1 {
		result, err = rebindCore(db, app, member, ip, rebindParams{
			bindingType: models.BindingTypeIP,
			typeName:    "IP",
			enabled:     policy.IPRebindEnabled,
			limit:       policy.IPRebindLimitTime,
			freeCount:   policy.IPRebindFreeCount,
			maxCount:    policy.IPRebindMaxCount,
			deductMinutes: policy.IPRebindDeductMinutes,
			deductPoints:  float64(policy.IPRebindDeductPoints),
			used:        member.IPRebindUsed,
			dateStr:     member.IPRebindDate,
			usedCol:     "ip_rebind_used",
			dateCol:     "ip_rebind_date",
		})
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}
