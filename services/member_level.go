package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// ============================================================================
// 会员等级：充值返利 + 累充自动升级
// ============================================================================
//
// 权益为「充值返利」：按当前等级的 RebateRate 多发放面值（时长/点数）。
// 升级按累计充值金额（Member.TotalRecharge，单位分，取自卡密售价快照 Card.Price）。
//
// 结算顺序（先返利再升级）：
//   1. rebatedCardValue —— 按「充值前」等级算返利，得到本次实发面值
//   2. 发放（各消费点自行赋值/累加）
//   3. settleMemberLevel —— 累加累充并结算升级，新等级下次充值生效
//
// 只升不降：累充只增不减，命中的最高等级只会更高或不变。

// memberRebateRate 取账号当前等级的返利比例（%）。无等级/等级禁用/查询失败均返回 0。
func memberRebateRate(db *gorm.DB, m *models.Member) int {
	if strings.TrimSpace(m.LevelUUID) == "" {
		return 0
	}
	var lv models.MemberLevel
	if err := db.Where("uuid = ? AND status = 1", m.LevelUUID).First(&lv).Error; err != nil {
		return 0
	}
	if lv.RebateRate <= 0 {
		return 0
	}
	return lv.RebateRate
}

// defaultLevelColor 默认档 / 未指定颜色的等级的展示色（灰）。
const defaultLevelColor = "#909399"

// levelColorOr 等级颜色，空则回退默认灰。
func levelColorOr(c string) string {
	if strings.TrimSpace(c) == "" {
		return defaultLevelColor
	}
	return c
}

// defaultLevelDisplay 默认档（无等级账号）的展示名与颜色：
// 该应用若配置了 level=1 的启用等级则用其名称/颜色，否则「默认会员」+灰。
func defaultLevelDisplay(db *gorm.DB, appUUID string) (name, color string) {
	var lv models.MemberLevel
	if err := db.Where("app_uuid = ? AND level = 1 AND status = 1", appUUID).
		Order("sort ASC, id ASC").First(&lv).Error; err == nil {
		return lv.Name, levelColorOr(lv.Color)
	}
	return "默认会员", defaultLevelColor
}

// memberLevelInfo 取账号当前等级名、权限等级值、返利比例、显示颜色。
// 无等级即**默认档**：权限等级为 1、返利 0、名称取该应用 level=1 等级或「默认会员」、颜色默认灰。
func memberLevelInfo(db *gorm.DB, m *models.Member) (name string, level, rebateRate int, color string) {
	if strings.TrimSpace(m.LevelUUID) != "" {
		var lv models.MemberLevel
		if err := db.Where("uuid = ?", m.LevelUUID).First(&lv).Error; err == nil {
			return lv.Name, lv.Level, lv.RebateRate, levelColorOr(lv.Color)
		}
	}
	name, color = defaultLevelDisplay(db, m.AppUUID)
	return name, 1, 0, color
}

// ResolveMemberLevelName 供后台列表展示：返回账号等级名（含默认档逻辑）、权限等级值、颜色。
func ResolveMemberLevelName(db *gorm.DB, appUUID, levelUUID string) (name string, level int, color string) {
	if strings.TrimSpace(levelUUID) != "" {
		var lv models.MemberLevel
		if err := db.Where("uuid = ?", levelUUID).First(&lv).Error; err == nil {
			return lv.Name, lv.Level, levelColorOr(lv.Color)
		}
	}
	name, color = defaultLevelDisplay(db, appUUID)
	return name, 1, color
}

// memberLevelExtras 取账号当前等级的额外福利（额外多开数、赠送免费换绑次数）；无等级返回 (0,0)。
func memberLevelExtras(db *gorm.DB, m *models.Member) (extraMultiOpen, extraRebind int) {
	if strings.TrimSpace(m.LevelUUID) == "" {
		return 0, 0
	}
	var lv models.MemberLevel
	if err := db.Where("uuid = ?", m.LevelUUID).First(&lv).Error; err != nil {
		return 0, 0
	}
	return lv.ExtraMultiOpen, lv.ExtraRebindCount
}

// rebateValue 按比例返利后的面值（向下取整）。非正数原样返回。
func rebateValue(value, rate int) int {
	if value <= 0 || rate <= 0 {
		return value
	}
	return value + value*rate/100
}

// rebatedCardValue 按账号「当前等级」返利后的卡密面值，返回 (时长分钟, 点数)。
// 永久卡（-1）不返利：已永久，返利无意义。
func rebatedCardValue(db *gorm.DB, m *models.Member, card *models.Card) (int, int) {
	rate := memberRebateRate(db, m)
	if rate <= 0 {
		return card.Duration, card.Points
	}
	duration := card.Duration
	if duration != models.CardDurationPermanent {
		duration = rebateValue(duration, rate)
	}
	return duration, rebateValue(card.Points, rate)
}

// resolveLevelUUID 按累充金额定位应处等级的 UUID；无匹配返回空（即默认的「免费账号」）。
func resolveLevelUUID(db *gorm.DB, appUUID string, total int) (string, error) {
	var lv models.MemberLevel
	err := db.Where("app_uuid = ? AND status = 1 AND threshold <= ?", appUUID, total).
		Order("threshold DESC").First(&lv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return lv.UUID, nil
}

// settleMemberLevel 累加累充金额并结算等级升级（事务内调用）。
// price 为本次卡密售价快照（分）；<=0（旧卡/无价套餐）不累充、不升级。
func settleMemberLevel(tx *gorm.DB, appUUID string, m *models.Member, price int) error {
	if price <= 0 {
		return nil
	}
	total := m.TotalRecharge + price
	updates := map[string]interface{}{"total_recharge": total}

	levelUUID, err := resolveLevelUUID(tx, appUUID, total)
	if err != nil {
		return err
	}
	// 累充只增，命中的等级只会更高或不变；无匹配则维持原等级，不降级
	if levelUUID != "" {
		updates["level_uuid"] = levelUUID
		m.LevelUUID = levelUUID
	}

	if err := tx.Model(m).Updates(updates).Error; err != nil {
		return err
	}
	m.TotalRecharge = total
	return nil
}

// 后台手动改写累充见 UpdateMemberProfile（services/member.go）：
// 手动改写是权威操作，会按新累充重新校准等级——改低也会相应降级（无匹配则回到「免费账号」），
// 避免出现「累充为 0 却仍是高等级」的不一致。

// ============================================================================
// 等级管理（后台）
// ============================================================================

// ListMemberLevels 列出应用的会员等级（按门槛升序）。
func ListMemberLevels(appUUID string) ([]models.MemberLevel, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	var levels []models.MemberLevel
	q := db.Model(&models.MemberLevel{})
	if appUUID = strings.TrimSpace(appUUID); appUUID != "" {
		q = q.Where("app_uuid = ?", appUUID)
	}
	if err := q.Order("sort ASC, threshold ASC").Find(&levels).Error; err != nil {
		return nil, err
	}
	return levels, nil
}

// SaveMemberLevel 新增或更新会员等级。UUID 为空则新增。
func SaveMemberLevel(level *models.MemberLevel) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	level.Name = strings.TrimSpace(level.Name)
	level.AppUUID = strings.TrimSpace(level.AppUUID)
	if level.AppUUID == "" || level.Name == "" {
		return errors.New("应用与等级名称不能为空")
	}
	if level.Threshold < 0 {
		return errors.New("累充门槛不能为负")
	}
	if level.RebateRate < 0 {
		return errors.New("返利比例不能为负")
	}
	if level.ExtraMultiOpen < 0 || level.ExtraRebindCount < 0 {
		return errors.New("额外多开与赠送换绑次数不能为负")
	}
	if level.Level < 1 {
		return errors.New("权限等级至少为 1")
	}

	if strings.TrimSpace(level.UUID) == "" {
		return db.Create(level).Error
	}
	var exists models.MemberLevel
	if err := db.Where("uuid = ?", level.UUID).First(&exists).Error; err != nil {
		return errors.New("等级不存在")
	}
	return db.Model(&exists).Updates(map[string]interface{}{
		"name":               level.Name,
		"level":              level.Level,
		"threshold":          level.Threshold,
		"rebate_rate":        level.RebateRate,
		"color":              level.Color,
		"extra_multi_open":   level.ExtraMultiOpen,
		"extra_rebind_count": level.ExtraRebindCount,
		"sort":               level.Sort,
		"status":             level.Status,
		"remark":             level.Remark,
	}).Error
}

// DeleteMemberLevel 删除会员等级，并清除账号上对该等级的引用。
func DeleteMemberLevel(uuid string) error {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return errors.New("等级UUID不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Member{}).Where("level_uuid = ?", uuid).
			Update("level_uuid", "").Error; err != nil {
			return err
		}
		return tx.Where("uuid = ?", uuid).Delete(&models.MemberLevel{}).Error
	})
}
