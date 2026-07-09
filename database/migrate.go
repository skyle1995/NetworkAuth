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
