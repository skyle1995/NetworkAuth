package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ============================================================================
// 全局接口条目初始化（一号通）
// ============================================================================
//
// 接口定义全站唯一：每个 api_type 恰好一条记录，不再按应用复制。
// 应用身份由请求签名（app_uuid + app_secret）区分，与接口条目无关。
//
// 安装时由 install 流程调用；服务启动时再次调用作幂等兜底。

// EnsureGlobalAPIs 补齐全局缺失的接口条目（幂等）。
//   - 已存在的 api_type 跳过，不覆盖已有配置（算法/密钥/启用状态）
//   - 新建的条目默认**启用**且不加密：接口是协议能力，装完即可联调；
//     需要加固时在后台逐条配置算法与密钥
func EnsureGlobalAPIs() {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return
	}
	if !db.Migrator().HasTable(&models.API{}) {
		return
	}

	var existing []int
	if err := db.Model(&models.API{}).Pluck("api_type", &existing).Error; err != nil {
		return
	}
	have := make(map[int]struct{}, len(existing))
	for _, t := range existing {
		have[t] = struct{}{}
	}

	created := 0
	for _, t := range models.GetDefaultAPITypes() {
		if _, ok := have[t]; ok {
			continue
		}
		api := models.API{
			APIType:         t,
			Status:          1, // 默认启用（全站生效）
			SubmitAlgorithm: models.AlgorithmNone,
			ReturnAlgorithm: models.AlgorithmNone,
		}
		if err := db.Create(&api).Error; err == nil {
			created++
		} else {
			logrus.WithError(err).WithField("api_type", t).Warn("创建全局接口条目失败")
		}
	}
	if created > 0 {
		logrus.Infof("全局接口初始化：新增 %d 条接口记录", created)
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

// LoadEnabledGlobalAPI 按 api_type 读取已启用的全局接口配置。
// 接口配置全站共用，调用方无需再关心应用维度。
func LoadEnabledGlobalAPI(db *gorm.DB, apiType int) (*models.API, error) {
	var api models.API
	if err := db.Where("api_type = ?", apiType).First(&api).Error; err != nil {
		return nil, err
	}
	return &api, nil
}
