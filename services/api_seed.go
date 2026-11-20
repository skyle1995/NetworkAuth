package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/nacl/box"
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
//   - 新建条目默认启用双向 Box，每个接口独立生成密钥对；失败时回滚本次初始化。
func EnsureGlobalAPIs() error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	if db == nil {
		return errors.New("数据库未连接")
	}
	return db.Transaction(seedMissingAPIs)
}

func seedMissingAPIs(db *gorm.DB) error {
	var existing []int
	if err := db.Model(&models.API{}).Pluck("api_type", &existing).Error; err != nil {
		return err
	}
	have := make(map[int]struct{}, len(existing))
	for _, t := range existing {
		have[t] = struct{}{}
	}

	for _, t := range models.GetDefaultAPITypes() {
		if _, ok := have[t]; ok {
			continue
		}
		pub, priv, err := GenerateBoxKeys()
		if err != nil {
			return err
		}
		api := models.API{
			APIType:         t,
			Status:          1, // 默认启用（全站生效）
			SubmitAlgorithm: models.AlgorithmBox,
			SubmitPublicKey: pub,
			SubmitPrivateKey: priv,
			ReturnAlgorithm: models.AlgorithmBox,
		}
		if err := db.Create(&api).Error; err != nil {
			return err
		}
	}
	return nil
}

// GenerateBoxKeys 使用系统随机源生成提交加密所需的十六进制公私钥对。
func GenerateBoxKeys() (string, string, error) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(pub[:]), hex.EncodeToString(priv[:]), nil
}
