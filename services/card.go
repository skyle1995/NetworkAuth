package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"errors"
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
func CardDurationToMinutes(value int, unit string) (int, error) {
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
	return value * factor, nil
}

// BatchCreateCards 为指定应用批量制卡。
// durationMinutes 为面值时长（-1 表示永久，时长模式用）；points 为面值点数（点数模式用）。
// 返回生成的卡密记录及本次批次号。
func BatchCreateCards(appUUID, prefix string, randomLen, count, durationMinutes, points int, remark string) ([]models.Card, string, error) {
	if count <= 0 {
		return nil, "", errors.New("生成数量必须大于0")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, "", err
	}

	// 校验应用存在
	var appCount int64
	if err := db.Model(&models.App{}).Where("uuid = ?", appUUID).Count(&appCount).Error; err != nil {
		return nil, "", err
	}
	if appCount == 0 {
		return nil, "", errors.New("应用不存在")
	}

	codes, err := models.GenerateCardNos(prefix, randomLen, count)
	if err != nil {
		return nil, "", err
	}

	// 批次号使用毫秒时间戳，便于按批次筛选/删除
	batchNo := strconv.FormatInt(time.Now().UnixMilli(), 10)
	cards := make([]models.Card, 0, count)
	for _, code := range codes {
		cards = append(cards, models.Card{
			CardNo:   code,
			AppUUID:  appUUID,
			BatchNo:  batchNo,
			Duration: durationMinutes,
			Points:   points,
			Status:   models.CardStatusUnused,
			Remark:   remark,
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

// DeleteCardsByBatch 按批次号删除某应用下整批卡密，返回删除数量。
func DeleteCardsByBatch(appUUID, batchNo string) (int64, error) {
	db, err := database.GetDB()
	if err != nil {
		return 0, err
	}
	res := db.Where("app_uuid = ? AND batch_no = ?", appUUID, batchNo).Delete(&models.Card{})
	return res.RowsAffected, res.Error
}

// MarkCardUsed 在事务内核销卡密：置为已使用并记录去向。供第三步卡密登录/充值调用。
func MarkCardUsed(tx *gorm.DB, id uint, memberUUID string) error {
	now := time.Now()
	return tx.Model(&models.Card{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":         models.CardStatusUsed,
		"used_by_member": memberUUID,
		"used_at":        &now,
	}).Error
}
