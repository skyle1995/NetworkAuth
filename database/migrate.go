package database

import (
	"NetworkAuth/models"

	"github.com/sirupsen/logrus"
)

// AutoMigrate 自动迁移数据库模型
// - 会确保必要的数据表结构存在
// - 不会破坏已有数据
func AutoMigrate() error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	if err := db.AutoMigrate(AllModels()...); err != nil {
		logrus.WithError(err).Error("AutoMigrate 执行失败")
		return err
	}
	logrus.Info("AutoMigrate 执行完成")
	return nil
}

// NormalizeUpdateStrategy 一次性把旧版「强制更新(force_update) + 更新方式(download_type: 自动/手动)」
// 规整为合并后的新三态 download_type（0不启用/1强制/2自由），随后删除已废弃的 force_update 列：
//   - 启用更新(download_type<>0)且原为强制 → download_type=1（强制）
//   - 启用更新且原非强制 → download_type=2（自由）
//   - download_type=0 保持不启用
// 幂等：force_update 列不存在（新库或已迁移过）时直接跳过。
func NormalizeUpdateStrategy() error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	m := db.Migrator()
	// 列已不存在（全新库或已迁移过）→ 无需处理
	if !m.HasColumn(&models.App{}, "force_update") {
		return nil
	}
	// 按旧 force_update 把已启用更新的应用映射到新方式（WHERE/SET 直接用列名，不依赖模型字段）
	if err := db.Model(&models.App{}).
		Where("download_type <> ? AND force_update = 1", models.DownloadTypeDisabled).
		Update("download_type", models.DownloadTypeForce).Error; err != nil {
		return err
	}
	if err := db.Model(&models.App{}).
		Where("download_type <> ? AND force_update = 0", models.DownloadTypeDisabled).
		Update("download_type", models.DownloadTypeFree).Error; err != nil {
		return err
	}
	// 删除废弃列（modernc/sqlite 支持 ALTER TABLE DROP COLUMN）
	return db.Exec("ALTER TABLE apps DROP COLUMN force_update").Error
}

// AllModels 返回参与迁移与结构整理的全部模型清单。
// AutoMigrate 与 db tidy 维护命令共用此清单，避免两处各维护一份导致遗漏。
func AllModels() []any {
	return []any{
		&models.Settings{},
		&models.PortalNavigation{},
		&models.OperationLog{},
		&models.LoginLog{},
		&models.User{},
		&models.App{},
		&models.API{},
		&models.Variable{},
		&models.Function{},
		&models.RefreshToken{},
		&models.ApiKey{},
		&models.Card{},
		&models.Member{},
		&models.Binding{},
		&models.MemberSession{},
		&models.MemberLog{},
		&models.Blacklist{},
	}
}
