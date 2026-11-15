package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"errors"
	"math"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// 卡密服务
// ============================================================================
//
// 卡密业务逻辑集中于此，controller 只做参数校验与响应封装。
// 卡密是独立的时长凭证，制卡时不锁定用途（卡密登录 / 充值均可消费）。

// 时长单位到分钟的换算表。卡密面值统一以分钟存储，与 App 表其它时间字段一致。
var durationUnitMinutes = map[string]int{
	"minute": 1,
	"hour":   60,
	"day":    24 * 60,
	"month":  30 * 24 * 60,
	"year":   365 * 24 * 60,
}

// CardDurationToMinutes 将“数值 + 单位”换算为分钟；unit 为 permanent 时返回永久标记。
// CardDurationToMinutes 把「数值 + 单位」换算成分钟。
//
// value 支持小数（0.5 天 = 720 分钟），换算后四舍五入到整分钟；
// 不足 1 分钟按 1 分钟计，避免用户填 0.001 天却拿到 0 分钟。
func CardDurationToMinutes(value float64, unit string) (int, error) {
	if unit == "permanent" {
		return models.CardDurationPermanent, nil
	}
	factor, ok := durationUnitMinutes[unit]
	if !ok {
		return 0, errors.New("不支持的时长单位")
	}
	if value <= 0 {
		return 0, errors.New("时长必须大于0")
	}
	mins := int(math.Round(value * float64(factor)))
	if mins < 1 {
		mins = 1
	}
	return mins, nil
}

// BatchCreateCards 批量制卡（全站卡池）。
// 面值与售价取自套餐快照。
func BatchCreateCards(prefix string, randomLen, count int, packageUUID, remark string) ([]models.Card, string, error) {
	if count <= 0 {
		return nil, "", errors.New("生成数量必须大于0")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, "", err
	}


	// 面值与售价取自套餐
	pkg, err := LoadEnabledPackage(db, packageUUID)
	if err != nil {
		return nil, "", err
	}
	if err := checkPackageValue(pkg); err != nil {
		return nil, "", err
	}

	codes, err := models.GenerateCardNos(prefix, randomLen, count)
	if err != nil {
		return nil, "", err
	}

	// 批次号使用毫秒时间戳，便于按批次筛选/删除
	batchNo := strconv.FormatInt(time.Now().UnixMilli(), 10)
	cards := make([]models.Card, 0, count)
	for _, code := range codes {
		// 面值与售价快照进卡：套餐后续改动不影响已售出的卡
		cards = append(cards, models.Card{
			CardNo:        code,
			BatchNo:       batchNo,
			PackageUUID:   pkg.UUID,
			Duration:      pkg.Duration,
			Points:        pkg.Points,
			Price:         pkg.Price,
			Status:        models.CardStatusUnused,
			Remark:        remark,
		})
	}

	if err := db.CreateInBatches(&cards, 200).Error; err != nil {
		return nil, "", err
	}
	return cards, batchNo, nil
}

// FreezeCards 批量冻结卡密（置为已冻结）。
func FreezeCards(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Model(&models.Card{}).Where("id IN ?", ids).Update("status", models.CardStatusFrozen).Error
}

// UnfreezeCards 批量解冻卡密。仅对已冻结的卡生效：
// 已核销过的卡（UsedByMember 非空）恢复为已使用，否则恢复为未使用。
func UnfreezeCards(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		// 未核销过的冻结卡 → 未使用
		if err := tx.Model(&models.Card{}).
			Where("id IN ? AND status = ? AND (used_by_member IS NULL OR used_by_member = '')", ids, models.CardStatusFrozen).
			Update("status", models.CardStatusUnused).Error; err != nil {
			return err
		}
		// 已核销过的冻结卡 → 已使用
		return tx.Model(&models.Card{}).
			Where("id IN ? AND status = ? AND used_by_member <> ''", ids, models.CardStatusFrozen).
			Update("status", models.CardStatusUsed).Error
	})
}

// DeleteCards 批量删除卡密。
func DeleteCards(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Delete(&models.Card{}, ids).Error
}

// DeleteCardsByBatch 按批次号删除整批卡密（全站），返回删除数量。
func DeleteCardsByBatch(batchNo string) (int64, error) {
	db, err := database.GetDB()
	if err != nil {
		return 0, err
	}
	res := db.Where("batch_no = ?", batchNo).Delete(&models.Card{})
	return res.RowsAffected, res.Error
}

// MarkCardUsed 在事务内核销卡密：置为已使用并记录去向。供第三步卡密登录/充值调用。
// 仅当卡密仍为「未使用」时才更新，并校验受影响行数，避免并发下同一张卡被重复核销（双花）。
func MarkCardUsed(tx *gorm.DB, id uint, memberUUID string) error {
	now := time.Now()
	res := tx.Model(&models.Card{}).
		Where("id = ? AND status = ?", id, models.CardStatusUnused).
		Updates(map[string]interface{}{
			"status":         models.CardStatusUsed,
			"used_by_member": memberUUID,
			"used_at":        &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("卡密已被使用")
	}
	return nil
}
