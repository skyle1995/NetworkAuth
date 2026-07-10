package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// EnsureAppAPIs 为所有应用补齐缺失的默认接口记录。
// 新增 api_type 后向后兼容：已存在的 (app_uuid, api_type) 跳过，新建的默认禁用且不加密。幂等。
func EnsureAppAPIs() {
	db, err := database.GetDB()
	if err != nil {
		return
	}

	var appUUIDs []string
	if err := db.Model(&models.App{}).Pluck("uuid", &appUUIDs).Error; err != nil {
		return
	}
	if len(appUUIDs) == 0 {
		return
	}

	types := models.GetDefaultAPITypes()
	created := 0
	for _, appUUID := range appUUIDs {
		var existing []int
		db.Model(&models.API{}).Where("app_uuid = ?", appUUID).Pluck("api_type", &existing)
		have := make(map[int]struct{}, len(existing))
		for _, t := range existing {
			have[t] = struct{}{}
		}
		for _, t := range types {
			if _, ok := have[t]; ok {
				continue
			}
			api := models.API{
				APIType:         t,
				AppUUID:         appUUID,
				Status:          0,
				SubmitAlgorithm: models.AlgorithmNone,
				ReturnAlgorithm: models.AlgorithmNone,
			}
			if err := db.Create(&api).Error; err == nil {
				created++
			}
		}
	}
	if created > 0 {
		logrus.Infof("接口补齐：为现有应用新增 %d 条缺失接口记录", created)
	}

	purgeLegacyRebindAPI(db)
}

// purgeLegacyRebindAPI 彻底移除历史「IP转绑(52)」接口记录（转绑已统一为 51）。幂等。
func purgeLegacyRebindAPI(db *gorm.DB) {
	res := db.Where("api_type = ?", models.LegacyAPITypeIPChangeBind).Delete(&models.API{})
	if res.Error == nil && res.RowsAffected > 0 {
		logrus.Infof("接口清理：移除 %d 条历史 IP转绑(52) 记录（转绑已统一为 51）", res.RowsAffected)
	}
}
