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
	if err := db.AutoMigrate(
		&models.Settings{},
		&models.OperationLog{},
		&models.LoginLog{},
		&models.App{},
		&models.API{},
		&models.Function{},
		&models.Variable{},
		&models.User{},
	); err != nil {
		logrus.WithError(err).Error("AutoMigrate 执行失败")
		return err
	}
	logrus.Info("AutoMigrate 执行完成")
	return nil
}
