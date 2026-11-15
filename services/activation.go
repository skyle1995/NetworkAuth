package services

import (
	"errors"
	"strings"
	"time"

	"NetworkAuth/models"

	"gorm.io/gorm"
)

// ============================================================================
// 应用激活（App Activation）—— 计费与授权唯一发生地
// ============================================================================
//
// 对标 Keygen machine activation：
//   - 幂等：同一 (app, member, fingerprint) 重复激活复用原记录、不重复扣费
//   - 租约：lease_expires_at = now + App.LeaseTTLMinutes，心跳续期
//   - 显式释放：Deactivate 立即归还名额
//   - 失效（释放或租约过期）的激活可被复活，复活按新一次占用计费
//
// 计费口径（三模式）：
//   计时模式        激活时只校验等级与有效期，不扣点
//   点数模式-按次    激活时扣 PointsCostForLevel(level, false)
//   点数模式-按时    激活时（FROM_CREATION）扣一个周期并写 activation.PeriodEnd
//                    或首次心跳（FROM_FIRST_PING）扣
//   高级模式        等级门禁 + 上述点数口径
//
// ⚠️ 按时模式的周期记账**只写 activation.PeriodEnd**，绝不写 members.expired_at
//    —— 后者是全站钱包，多应用按时计费若都写它必然互相延长可用期。

// ActivationInfo 激活成功后的返回体
type ActivationInfo struct {
	// 令牌与租约
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expires_at"`
	LeaseTTL   int       `json:"lease_ttl"`   // 秒
	RenewAfter int       `json:"renew_after"` // 建议续约间隔（秒）
	Reused     bool      `json:"reused"`      // true=复用已有激活，本次未扣费

	// 应用
	AppUUID  string `json:"app_uuid"`
	AppName  string `json:"app_name"`

	// 账号
	Username string `json:"username"`
	Type     int    `json:"type"`

	// 计费与钱包态
	Mode              int       `json:"mode"`
	Free              bool      `json:"free"`
	RequiredLevel     int       `json:"required_level"`
	Permanent         bool      `json:"permanent"`
	ExpiredAt         time.Time `json:"expired_at"`
	Points            float64       `json:"points"`
	HeartbeatInterval int       `json:"heartbeat_interval"`

	// 会员
	TotalRecharge int    `json:"total_recharge"`
	Level         int    `json:"level"`
	LevelName     string `json:"level_name"`
	LevelColor    string `json:"level_color"`
	RebateRate    int    `json:"rebate_rate"`
}

// 激活相关错误（均带标准错误码，供客户端分支）
var (
	// ErrAppNotFound 应用不存在
	ErrAppNotFound = NewCodedError(models.ErrAppNotFound, "")
	// ErrAppDisabled 应用已停用
	ErrAppDisabled = NewCodedError(models.ErrAppDisabled, "")
	// ErrActivationInvalid 激活令牌无效/已释放
	ErrActivationInvalid = NewCodedError(models.ErrActivationTokenInvalid, "")
	// ErrLeaseExpired 租约已过期（需重新激活，会重新计费）
	ErrLeaseExpired = NewCodedError(models.ErrLeaseExpired, "")
)

// leaseTTL 激活租约时长（分钟 → 时长），兜底 30 分钟
func leaseTTL(app *models.App) time.Duration {
	m := app.LeaseTTLMinutes
	if m <= 0 {
		m = 30
	}
	return time.Duration(m) * time.Minute
}

// offlineTimeoutMinutes 与旧逻辑保持一致的离线超时（分钟）
func offlineTimeoutMinutesAct(app *models.App) int {
	if app.OfflineTimeout > 0 {
		return app.OfflineTimeout
	}
	return 30
}

// checkActivationGate 激活/续约前的门禁（不含绑定与并发，那些在事务内单独做）。
//
// 与旧 checkMemberUsable 的差别：按时模式的"是否在周期内"不在这里判断，
// 因为周期记在 activation.PeriodEnd 上，而不是 members.expired_at。
func checkActivationGate(db *gorm.DB, billing *Billing, m *models.Member) error {
	if m.Status == models.MemberStatusBlack {
		return errors.New("账号已被拉黑")
	}
	if m.Status == models.MemberStatusDisabled {
		return errors.New("账号已被封停")
	}
	if billing.Free {
		return nil
	}

	// 等级门禁（计时模式 / 高级模式）
	if billing.RequiresLevel() {
		if lv := memberPermissionLevel(db, m); lv < billing.RequiredLevel {
			return &EntitlementError{
				NeedLevel: billing.RequiredLevel,
				NeedName:  levelNameByValue(db, billing.RequiredLevel),
			}
		}
	}

	// 有效期门禁（仅计时模式）
	if billing.RequiresValidPeriod() {
		if !isPermanent(m.ExpiredAt) && m.ExpiredAt.Before(time.Now()) {
			return &LicenseExpiredError{}
		}
	}

	// 注意：**点数门禁不在这里**。
	// 幂等复用同一激活时不应扣费，也就不该被余额拦住 —— 余额检查放在实际扣费处
	// （chargeActivationOnce / chargePeriod），只对「新建」「复活」「续周期」生效。
	return nil
}

// chargeActivationOnce 激活时的扣费（按次一次 / 按时一周期）。
// 返回是否实际扣费。
func chargeActivationOnce(tx *gorm.DB, billing *Billing, m *models.Member, act *models.AppActivation) (bool, error) {
	if !billing.UsesPoints() {
		return false, nil
	}

	if billing.IsPointsChargeMode() {
		// 按次 + 合并计费：本应用只要还有别的设备在线，就说明这一轮已经付过钱了，
		// 第二台、第三台设备不该再扣一次。
		if billing.IsMerged() {
			var others int64
			if err := tx.Model(&models.AppActivation{}).
				Where("member_uuid = ? AND app_uuid = ? AND deactivated_at IS NULL AND lease_expires_at > ?",
					m.UUID, act.AppUUID, time.Now()).
				Count(&others).Error; err != nil {
				return false, err
			}
			if others > 0 {
				return false, nil
			}
		}
		// 按次
		cost := billing.PointsCostForLevel(m.LevelUUID, false)
		if cost <= 0 {
			return false, nil
		}
		res := tx.Model(&models.Member{}).
			Where("uuid = ? AND points + 0.000001 >= ?", m.UUID, cost).
			Update("points", gorm.Expr("ROUND(points - ?, 6)", cost))
		if res.Error != nil {
			return false, res.Error
		}
		if res.RowsAffected == 0 {
			return false, &InsufficientCreditsError{Need: cost, Have: m.Points}
		}
		m.Points -= cost
		return true, nil
	}

	// 按时：FROM_FIRST_PING 激活时不扣，等首次心跳
	if act.ChargeBasis == models.ChargeBasisFromFirstPing {
		return false, nil
	}
	return chargePeriod(tx, billing, m, act)
}

// chargePeriod 扣一个按时周期并推进 activation.PeriodEnd。
// 只动激活的周期，不动 members.expired_at。
func chargePeriod(tx *gorm.DB, billing *Billing, m *models.Member, act *models.AppActivation) (bool, error) {
	if billing.IsMerged() {
		return chargePeriodMerged(tx, billing, m, act)
	}
	cost := billing.PointsPerPeriodForLevel(m.LevelUUID)
	if cost <= 0 {
		// 不扣点也要推进周期，避免每次心跳都判定"需扣费"
		now := time.Now()
		end := now.Add(time.Duration(billing.PointsPeriodMinutes()) * time.Minute)
		act.PeriodEnd = &end
		return false, tx.Model(&models.AppActivation{}).
			Where("uuid = ?", act.UUID).
			Update("period_end", end).Error
	}

	now := time.Now()
	// 从当前周期末尾顺延（不吞掉剩余时间）；首次则从现在起算
	base := now
	if act.PeriodEnd != nil && act.PeriodEnd.After(now) {
		base = *act.PeriodEnd
	}
	end := base.Add(time.Duration(billing.PointsPeriodMinutes()) * time.Minute)

	res := tx.Model(&models.Member{}).
		Where("uuid = ? AND points + 0.000001 >= ?", m.UUID, cost).
		Update("points", gorm.Expr("ROUND(points - ?, 6)", cost))
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		return false, &InsufficientCreditsError{Need: cost, Have: m.Points}
	}
	m.Points -= cost

	if err := tx.Model(&models.AppActivation{}).
		Where("uuid = ?", act.UUID).
		Update("period_end", end).Error; err != nil {
		return false, err
	}
	act.PeriodEnd = &end
	return true, nil
}

// Activate 激活应用（幂等）。这是唯一计费入口。
//
// 流程：
//  1. 校验应用（存在/启用/非宿主）
//  2. 账号状态与门禁
//  3. 黑名单
//  4. 事务内：账号行锁 → 绑定校验 → 幂等查找 → 并发计数 → 扣费 → 建/复活激活
func Activate(db *gorm.DB, acct *models.MemberAccountToken, app *models.App, fingerprint, ip, version, deviceName string) (*ActivationInfo, error) {
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}
	if app == nil {
		return nil, ErrAppNotFound
	}
	if app.Status != 1 {
		return nil, ErrAppDisabled
	}
	if acct == nil {
		return nil, ErrAccountTokenInvalid
	}

	fingerprint = strings.TrimSpace(fingerprint)
	deviceName = strings.TrimSpace(deviceName)
	version = strings.TrimSpace(version)

	// 设备指纹回退：客户端可能只在登录（20/10）时提交了 machine_code，
	// 激活（70）时省略了 fingerprint。两者是同一个东西，账号令牌里已记录，
	// 这里直接复用，避免「登录传了设备码、激活却提示未提供」的割裂。
	if fingerprint == "" {
		fingerprint = strings.TrimSpace(acct.Fingerprint)
	}
	if deviceName == "" {
		deviceName = strings.TrimSpace(acct.DeviceName)
	}
	if version == "" {
		version = strings.TrimSpace(acct.Version)
	}

	// 需要设备指纹却仍未提供 → 拒绝（应用开了机器验证就必须能定位设备）
	policy := GetAccountPolicy()
	if policy.MachineVerify == 1 && fingerprint == "" {
		return nil, ErrFingerprintRequired
	}

	var member models.Member
	if err := db.Where("uuid = ?", acct.MemberUUID).First(&member).Error; err != nil {
		return nil, ErrAccountTokenInvalid
	}

	billing := EffectiveBilling(app)
	if err := checkActivationGate(db, billing, &member); err != nil {
		return nil, err
	}

	// 黑名单（全站生效）
	blProvince, blCity := ResolveIPRegion(ip)
	if blocked, reason := CheckBlacklist(db, fingerprint, ip, blProvince, blCity); blocked {
		return nil, errors.New(reason)
	}

	// 账号级配额：设备数、并发数都是账号资源，跨所有应用共用
	quota := ResolveAccountQuota(db, &member)
	ttl := leaseTTL(app)
	now := time.Now()

	// 无指纹（应用未开机器验证）时用随机串占位，使每次激活都算独立占用
	if fingerprint == "" {
		anon, err := generateSecureToken()
		if err != nil {
			return nil, err
		}
		fingerprint = "anon:" + anon[:16]
	}

	var (
		act    models.AppActivation
		reused bool
	)
	err := db.Transaction(func(tx *gorm.DB) error {
		// 账号级互斥：串行化同账号并发激活，保护绑定计数、并发计数、幂等查找
		if err := lockMemberRow(tx, member.UUID); err != nil {
			return err
		}

		// 绑定校验（事务内、受锁保护）
		if policy.MachineVerify == 1 && fingerprint != "" {
			if err := ensureMachineBinding(tx, member.UUID, fingerprint, deviceName, quota.MaxDevices); err != nil {
				return &FingerprintMismatchError{Cause: err}
			}
		}
		if err := ensureIPBinding(tx, member.UUID, ip, policy.IPVerify, quota.MaxDevices); err != nil {
			return err
		}

		// 幂等查找：同 (app, member, fingerprint)
		var existing models.AppActivation
		findErr := tx.Where("app_uuid = ? AND member_uuid = ? AND fingerprint = ?",
			app.UUID, member.UUID, fingerprint).First(&existing).Error

		switch {
		case findErr == nil && existing.IsAlive(now):
			// ① 活跃 → 复用：续租约，不扣费
			act = existing
			reused = true
			if err := tx.Model(&models.AppActivation{}).
				Where("uuid = ?", act.UUID).
				Updates(map[string]any{
					"lease_expires_at": now.Add(ttl),
					"last_active_at":   now,
					"ip":               ip,
					"version":          version,
					"device_name":      deviceName,
				}).Error; err != nil {
				return err
			}
			act.LeaseExpiresAt = now.Add(ttl)
			act.LastActiveAt = now

		case findErr == nil && existing.IsDead(now):
			// ② 已失效 → 复活：按新一次占用扣费
			act = existing
			act.DeactivatedAt = nil
			act.LeaseExpiresAt = now.Add(ttl)
			act.LastActiveAt = now
			act.ChargeBasis = app.PointsHeartbeatCharge
			if app.PointsHeartbeatCharge == models.ChargeBasisFromCreation {
				// FROM_CREATION 重新计时：清掉旧周期
				act.PeriodEnd = nil
			} else {
				act.FirstPingAt = nil
			}
			if _, err := chargeActivationOnce(tx, billing, &member, &act); err != nil {
				return err
			}
			if err := tx.Model(&models.AppActivation{}).
				Where("uuid = ?", act.UUID).
				Updates(map[string]any{
					"deactivated_at":   nil,
					"lease_expires_at": act.LeaseExpiresAt,
					"last_active_at":   act.LastActiveAt,
					"charge_basis":     act.ChargeBasis,
					"period_end":       act.PeriodEnd,
					"first_ping_at":    nil,
					"ip":               ip,
					"version":          version,
					"device_name":      deviceName,
				}).Error; err != nil {
				return err
			}

		case errors.Is(findErr, gorm.ErrRecordNotFound):
			// ③ 不存在 → 并发检查后新建
			// 只有开了机器码验证的应用才按设备计在线数；无可靠设备标识时不限制
			// （否则未开验证的应用每次激活都会被当成新设备，几个插件就把名额占满）
			if policy.MachineVerify == 1 && !strings.HasPrefix(fingerprint, "anon:") {
				if err := enforceMaxConcurrent(tx, app, &member, quota.MaxConcurrent, now, fingerprint); err != nil {
					return err
				}
			}
			token, err := generateSessionToken()
			if err != nil {
				return err
			}
			act = models.AppActivation{
				Token:          token,
				MemberUUID:     member.UUID,
				AppUUID:        app.UUID,
				AccountUUID:    acct.UUID,
				Fingerprint:    fingerprint,
				IP:             ip,
				Version:        version,
				DeviceName:     deviceName,
				LeaseExpiresAt: now.Add(ttl),
				ChargeBasis:    app.PointsHeartbeatCharge,
				LastActiveAt:   now,
			}
			if _, err := chargeActivationOnce(tx, billing, &member, &act); err != nil {
				return err
			}
			if err := tx.Create(&act).Error; err != nil {
				return err
			}

		default:
			return findErr
		}

		// 回写账号的登录痕迹
		updates := map[string]any{
			"last_login_at": now,
			"last_login_ip": ip,
		}
		if strings.TrimSpace(member.RegisterMachine) == "" && fingerprint != "" && !strings.HasPrefix(fingerprint, "anon:") {
			updates["register_machine"] = fingerprint
			member.RegisterMachine = fingerprint
		}
		return tx.Model(&models.Member{}).Where("uuid = ?", member.UUID).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}

	// 重新读一次账号，拿到扣费后的最新钱包
	if err := db.Where("uuid = ?", member.UUID).First(&member).Error; err != nil {
		return nil, err
	}

	AddMemberLog(app.UUID, member.UUID, member.Username, "应用激活", fingerprint, ip)

	return buildActivationInfo(db, app, billing, &member, &act, reused, ttl), nil
}

// enforceMaxConcurrent 账号级「同时在线」控制。
//
// 计数口径是**设备**，不是插件实例：
// 同一台电脑上开透视、自瞄、加速三个插件，只占 1 个在线名额。
// 这样「多开」（同设备开多个插件）不会被当成多次上线，插件启动只负责计费。
//
// 满员时按应用配置的 login_type 处理（顶号 / 拒绝 / 让用户手选）。
func enforceMaxConcurrent(tx *gorm.DB, app *models.App, m *models.Member, maxOpen int, now time.Time, curFingerprint string) error {
	var acts []models.AppActivation
	if err := tx.Where("member_uuid = ? AND deactivated_at IS NULL AND lease_expires_at > ?",
		m.UUID, now).
		Order("last_active_at ASC").Find(&acts).Error; err != nil {
		return err
	}

	// 按指纹去重：同一设备上的多个激活共享一个在线名额。
	// current 为本次要进入的设备；若它已在集合里，说明只是多开一个插件，直接放行。
	devices := make(map[string][]models.AppActivation)
	order := make([]string, 0, len(acts))
	for _, a := range acts {
		fp := a.Fingerprint
		if fp == "" {
			// 无指纹的历史记录各自算独立设备（无法归并）
			fp = "legacy:" + a.UUID
		}
		if _, ok := devices[fp]; !ok {
			order = append(order, fp)
		}
		devices[fp] = append(devices[fp], a)
	}
	// 本次设备已在在线名单里 → 不新增名额，直接通过
	if _, exists := devices[curFingerprint]; exists {
		return nil
	}
	if len(order) < maxOpen {
		return nil
	}

	// 满员：把最早的**整台设备**（该设备上的全部激活）作为一组处理
	acts = devices[order[0]]

	if app.LoginType == models.LoginTypeManual {
		infos := make([]SessionInfo, 0, len(acts))
		for _, a := range acts {
			province, city := ResolveIPRegion(a.IP)
			infos = append(infos, SessionInfo{
				ID:           a.ID,
				DeviceName:   a.DeviceName,
				MachineCode:  a.Fingerprint,
				IP:           a.IP,
				Province:     province,
				City:         city,
				Version:      a.Version,
				LastActiveAt: a.LastActiveAt,
				CreatedAt:    a.CreatedAt,
			})
		}
		return &SessionsFullError{Sessions: infos}
	}
	if app.LoginType == models.LoginTypeReject {
		return &MachinesLimitError{Limit: maxOpen}
	}

	// 顶号：整台最早设备的**全部激活**一起下线，腾出一个设备名额
	ids := make([]uint, 0, len(acts))
	for _, a := range acts {
		ids = append(ids, a.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	return tx.Model(&models.AppActivation{}).
		Where("id IN ?", ids).
		Update("deactivated_at", now).Error
}

// Deactivate 释放激活（幂等）。立即归还一个并发名额，已扣点数不退。
func Deactivate(db *gorm.DB, activationToken string) error {
	activationToken = strings.TrimSpace(activationToken)
	if activationToken == "" {
		return nil
	}
	now := time.Now()
	return db.Model(&models.AppActivation{}).
		Where("token = ? AND deactivated_at IS NULL", activationToken).
		Updates(map[string]any{
			"deactivated_at": now,
			"updated_at":     now,
		}).Error
}

// DeactivateByMemberApp 释放某账号在某应用的全部活跃激活（后台强制下线用）
func DeactivateByMemberApp(db *gorm.DB, memberUUID, appUUID string) error {
	now := time.Now()
	q := db.Model(&models.AppActivation{}).Where("deactivated_at IS NULL")
	if strings.TrimSpace(memberUUID) != "" {
		q = q.Where("member_uuid = ?", memberUUID)
	}
	if strings.TrimSpace(appUUID) != "" {
		q = q.Where("app_uuid = ?", appUUID)
	}
	return q.Updates(map[string]any{
		"deactivated_at": now,
		"updated_at":     now,
	}).Error
}

// AuthActivation 校验激活令牌，返回账号与激活记录。
// 不续约、不扣费（那是 RenewLease 的职责）。
func AuthActivation(db *gorm.DB, token string) (*models.Member, *models.AppActivation, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil, ErrActivationRequiredMarker
	}

	var act models.AppActivation
	if err := db.Where("token = ?", token).First(&act).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrActivationInvalid
		}
		return nil, nil, err
	}
	if act.IsReleased() {
		return nil, nil, ErrActivationInvalid
	}
	if act.IsLeaseExpired(time.Now()) {
		return nil, nil, ErrLeaseExpired
	}

	var member models.Member
	if err := db.Where("uuid = ?", act.MemberUUID).First(&member).Error; err != nil {
		return nil, nil, ErrAccountTokenInvalid
	}
	return &member, &act, nil
}

// RenewLease 心跳续约：复查门禁、按时扣周期、推进租约。
//
// 这是 Keygen 的 heartbeat ping：租约到期未续即视为失效，名额自动回收。
func RenewLease(db *gorm.DB, activationToken string) (*models.Member, *models.AppActivation, *Billing, error) {
	token := strings.TrimSpace(activationToken)
	if token == "" {
		return nil, nil, nil, ErrActivationRequiredMarker
	}

	var act models.AppActivation
	if err := db.Where("token = ?", token).First(&act).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, ErrActivationInvalid
		}
		return nil, nil, nil, err
	}
	if act.IsReleased() {
		return nil, nil, nil, ErrActivationInvalid
	}
	now := time.Now()
	if act.IsLeaseExpired(now) {
		return nil, nil, nil, ErrLeaseExpired
	}

	var app models.App
	if err := db.Where("uuid = ?", act.AppUUID).First(&app).Error; err != nil {
		return nil, nil, nil, ErrAppNotFound
	}
	billing := EffectiveBilling(&app)

	var member models.Member
	if err := db.Where("uuid = ?", act.MemberUUID).First(&member).Error; err != nil {
		return nil, nil, nil, ErrAccountTokenInvalid
	}

	ttl := leaseTTL(&app)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := lockMemberRow(tx, member.UUID); err != nil {
			return err
		}
		if err := checkActivationGate(tx, billing, &member); err != nil {
			return err
		}

		// 按时模式：需要新周期时扣费
		if billing.UsesPoints() && !billing.IsPointsChargeMode() {
			// FROM_FIRST_PING 的首次心跳：开始计时
			if act.ChargeBasis == models.ChargeBasisFromFirstPing && act.FirstPingAt == nil {
				if _, err := chargePeriod(tx, billing, &member, &act); err != nil {
					return err
				}
				if err := tx.Model(&models.AppActivation{}).
					Where("uuid = ?", act.UUID).
					Update("first_ping_at", now).Error; err != nil {
					return err
				}
				act.FirstPingAt = &now
			} else if need, err := needsPeriodCharge(tx, billing, &member, &act, now); err != nil {
				return err
			} else if need {
				if _, err := chargePeriod(tx, billing, &member, &act); err != nil {
					return err
				}
			}
		}

		// 推进租约
		newExpiry := now.Add(ttl)
		if err := tx.Model(&models.AppActivation{}).
			Where("uuid = ?", act.UUID).
			Updates(map[string]any{
				"lease_expires_at": newExpiry,
				"last_active_at":   now,
			}).Error; err != nil {
			return err
		}
		act.LeaseExpiresAt = newExpiry
		act.LastActiveAt = now
		return nil
	})
	if err != nil {
		// 门禁/扣费失败：把激活置死，让客户端重新激活
		if isFatalActivationError(err) {
			db.Model(&models.AppActivation{}).
				Where("uuid = ?", act.UUID).
				Update("deactivated_at", now)
		}
		return nil, nil, nil, err
	}

	// 取扣费后最新钱包
	if err := db.Where("uuid = ?", member.UUID).First(&member).Error; err != nil {
		return nil, nil, nil, err
	}
	return &member, &act, billing, nil
}

// PruneDeadActivations 兜底清理：把租约早已过期且未标记释放的激活打上释放标记。
// 幂等；供后台定时任务调用。
func PruneDeadActivations(db *gorm.DB) (int64, error) {
	now := time.Now()
	res := db.Model(&models.AppActivation{}).
		Where("deactivated_at IS NULL AND lease_expires_at < ?", now).
		Updates(map[string]any{
			"deactivated_at": now,
			"updated_at":     now,
		})
	return res.RowsAffected, res.Error
}

// ActiveActivationCount 某账号在某应用上的活跃激活数
func ActiveActivationCount(db *gorm.DB, memberUUID, appUUID string) int64 {
	var n int64
	db.Model(&models.AppActivation{}).
		Where("member_uuid = ? AND app_uuid = ? AND deactivated_at IS NULL AND lease_expires_at > ?",
			memberUUID, appUUID, time.Now()).
		Count(&n)
	return n
}

// ActiveAppCount 某账号同时激活的不同应用数（max_concurrent_apps 用）
func ActiveAppCount(db *gorm.DB, memberUUID string) int64 {
	var n int64
	db.Model(&models.AppActivation{}).
		Where("member_uuid = ? AND deactivated_at IS NULL AND lease_expires_at > ?",
			memberUUID, time.Now()).
		Distinct("app_uuid").Count(&n)
	return n
}

// buildActivationInfo 组装返回体
func buildActivationInfo(db *gorm.DB, app *models.App, billing *Billing, m *models.Member, act *models.AppActivation, reused bool, ttl time.Duration) *ActivationInfo {
	levelName, level, rebate, levelColor := memberLevelInfo(db, m)
	hb := app.CheckInterval
	if hb <= 0 {
		hb = 10
	}
	renewAfter := int(ttl.Seconds()) / 3
	if renewAfter < 60 {
		renewAfter = 60
	}
	return &ActivationInfo{
		Token:             act.Token,
		ExpiresAt:         act.LeaseExpiresAt,
		LeaseTTL:          int(ttl.Seconds()),
		RenewAfter:        renewAfter,
		Reused:            reused,
		AppUUID:           app.UUID,
		AppName:           app.Name,
		Username:          m.Username,
		Type:              m.Type,
		Mode:              billing.Mode,
		Free:              billing.Free,
		RequiredLevel:     billing.RequiredLevel,
		Permanent:         isPermanent(m.ExpiredAt),
		ExpiredAt:         m.ExpiredAt,
		Points:            m.Points,
		HeartbeatInterval: hb,
		TotalRecharge:     m.TotalRecharge,
		Level:             level,
		LevelName:         levelName,
		LevelColor:        levelColor,
		RebateRate:        rebate,
	}
}

// ============================================================================
// 心跳续约（兼容隐式激活）
// ============================================================================

// renewLeaseOrImplicit 心跳入口：先尝试续约激活令牌；
// 若传入的是账号令牌（老客户端没调 70），先隐式激活再续约。
func renewLeaseOrImplicit(db *gorm.DB, appUUID, token string) (*models.Member, *models.AppActivation, *Billing, error) {
	// 激活令牌：标准路径
	if _, _, err := AuthActivation(db, token); err == nil {
		return RenewLease(db, token)
	}

	// 账号令牌：隐式激活（Activate 内部幂等，同指纹不会重复扣费）
	member, app, act, err := AuthForApp(db, appUUID, token)
	if err != nil {
		return nil, nil, nil, err
	}
	billing := EffectiveBilling(app)
	return member, act, billing, nil
}
// chargePeriodMerged 合并计费下扣一个周期。
//
// 与单独计费的区别：周期账本记在 **账号 + 应用** 维度（app_account_billings），
// 而不是某个 activation 上——同一账号 3 台设备共享同一个 period_end，只付一份。
func chargePeriodMerged(tx *gorm.DB, billing *Billing, m *models.Member, act *models.AppActivation) (bool, error) {
	now := time.Now()
	period := time.Duration(billing.PointsPeriodMinutes()) * time.Minute

	// 取（或建）该账号在本应用的周期账本
	var row models.AppAccountBilling
	err := tx.Where("member_uuid = ? AND app_uuid = ?", m.UUID, act.AppUUID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.AppAccountBilling{MemberUUID: m.UUID, AppUUID: act.AppUUID}
		if err := tx.Create(&row).Error; err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}

	// 周期还在有效期内 → 同一份周期，第二台设备直接共享，不再扣费
	if row.PeriodEnd != nil && row.PeriodEnd.After(now) {
		act.PeriodEnd = row.PeriodEnd
		return false, nil
	}

	// 从当前周期末尾顺延（不吞掉剩余时间）；首次则从现在起算
	base := now
	if row.PeriodEnd != nil && row.PeriodEnd.After(now) {
		base = *row.PeriodEnd
	}
	end := base.Add(period)

	cost := billing.PointsPerPeriodForLevel(m.LevelUUID)
	if cost <= 0 {
		// 不扣点也要推进周期，避免每次心跳都判定「需扣费」
		act.PeriodEnd = &end
		return false, tx.Model(&models.AppAccountBilling{}).
			Where("id = ?", row.ID).
			Update("period_end", end).Error
	}

	res := tx.Model(&models.Member{}).
		Where("uuid = ? AND points + 0.000001 >= ?", m.UUID, cost).
		Update("points", gorm.Expr("ROUND(points - ?, 6)", cost))
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		return false, &InsufficientCreditsError{Need: cost, Have: m.Points}
	}
	m.Points -= cost

	if err := tx.Model(&models.AppAccountBilling{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"period_end":     end,
			"last_charge_at": now,
		}).Error; err != nil {
		return false, err
	}
	act.PeriodEnd = &end
	return true, nil
}

// needsPeriodCharge 判断某个激活此刻是否该扣下一个周期。
//
// 单独计费看激活自己的 period_end；合并计费看账号在该应用的共享周期。
func needsPeriodCharge(tx *gorm.DB, billing *Billing, m *models.Member, act *models.AppActivation, now time.Time) (bool, error) {
	if !billing.IsMerged() {
		return act.NeedsPeriodCharge(now), nil
	}
	var row models.AppAccountBilling
	err := tx.Where("member_uuid = ? AND app_uuid = ?", m.UUID, act.AppUUID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return row.PeriodEnd == nil || !row.PeriodEnd.After(now), nil
}