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
	deduct      int    // 单次扣除分钟数
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

	// 免费次数用尽后按配置扣除（时长扣分钟 / 点数扣点数）
	deduct := 0
	if used >= p.freeCount {
		deduct = p.deduct
	}
	pointsMode := app.OperationMode == models.OperationModePoints

	err := db.Transaction(func(tx *gorm.DB) error {
		if deduct > 0 {
			if pointsMode {
				newPoints := member.Points - deduct
				if newPoints < 0 {
					newPoints = 0
				}
				if err := tx.Model(member).Update("points", newPoints).Error; err != nil {
					return err
				}
				member.Points = newPoints
			} else if !isPermanent(member.ExpiredAt) {
				newExpiry := member.ExpiredAt.Add(-time.Duration(deduct) * time.Minute)
				if newExpiry.Before(time.Now()) {
					newExpiry = time.Now()
				}
				if err := tx.Model(member).Update("expired_at", newExpiry).Error; err != nil {
					return err
				}
				member.ExpiredAt = newExpiry
			}
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

// Rebind 统一转绑（原 type 51 机器码 / 52 IP 合并为一个功能）。
// 凭账号密码鉴权（卡密账号以卡号为身份，无需密码），**不要求登录会话**——
// 从而打破“设备/IP 对不上→登不进→拿不到令牌→无法转绑”的死循环。
// 按应用配置转绑已开启的维度：机器码转绑须带设备码，IP 转绑用当前请求 IP。
// 两个维度都开时依次转绑，各自独立计次/扣费。
func Rebind(appUUID, username, password, machineCode, ip string) (*StatusResult, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	member, err := authMemberByCredential(db, appUUID, username, password)
	if err != nil {
		return nil, err
	}

	if app.MachineRebindEnabled != 1 && app.IPRebindEnabled != 1 {
		return nil, errors.New("该应用未开启转绑")
	}

	var result *StatusResult

	// 机器码转绑：开启则必须带设备码
	if app.MachineRebindEnabled == 1 {
		if strings.TrimSpace(machineCode) == "" {
			return nil, errors.New("机器码转绑需提供设备码")
		}
		result, err = rebindCore(db, app, member, machineCode, rebindParams{
			bindingType: models.BindingTypeMachine,
			typeName:    "机器码",
			enabled:     app.MachineRebindEnabled,
			limit:       app.MachineRebindLimit,
			freeCount:   app.MachineFreeCount,
			maxCount:    app.MachineRebindCount,
			deduct:      app.MachineRebindDeduct,
			used:        member.MachineRebindUsed,
			dateStr:     member.MachineRebindDate,
			usedCol:     "machine_rebind_used",
			dateCol:     "machine_rebind_date",
		})
		if err != nil {
			return nil, err
		}
	}

	// IP 转绑：用当前请求 IP（客户端须从新 IP 调用）
	if app.IPRebindEnabled == 1 {
		if strings.TrimSpace(ip) == "" {
			return nil, errors.New("IP转绑无法获取当前IP")
		}
		result, err = rebindCore(db, app, member, ip, rebindParams{
			bindingType: models.BindingTypeIP,
			typeName:    "IP",
			enabled:     app.IPRebindEnabled,
			limit:       app.IPRebindLimit,
			freeCount:   app.IPFreeCount,
			maxCount:    app.IPRebindCount,
			deduct:      app.IPRebindDeduct,
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
