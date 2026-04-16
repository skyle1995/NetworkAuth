package database

import (
	"NetworkAuth/models"
	"errors"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// SeedDefaultPortalNavigation 初始化默认门户导航
// 当系统首次安装或升级后缺少默认入口时，自动补充首页和管理员登录入口
func SeedDefaultPortalNavigation() error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	defaultItems := []models.PortalNavigation{
		{
			Name:       "首页",
			Path:       "/home/index",
			Sort:       0,
			IsHome:     true,
			IsHidden:   false,
			IsExternal: false,
		},
		{
			Name:       "管理员登录",
			Path:       "admin",
			Sort:       999,
			IsHome:     false,
			IsHidden:   false,
			IsExternal: false,
		},
	}

	for _, item := range defaultItems {
		var exists models.PortalNavigation
		if err := db.Where("path = ?", item.Path).First(&exists).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := db.Create(&item).Error; err != nil {
				logrus.WithError(err).WithField("path", item.Path).Error("创建默认门户导航失败")
				return err
			}
			continue
		}

		switch exists.Path == "admin" {
		case true:
			if err := db.Model(&models.PortalNavigation{}).Where("id = ?", exists.ID).Updates(map[string]interface{}{
				"name":        "管理员登录",
				"path":        "admin",
				"sort":        999,
				"is_home":     false,
				"is_external": false,
			}).Error; err != nil {
				logrus.WithError(err).WithField("path", item.Path).Error("更新默认门户导航失败")
				return err
			}
		default:
			continue
		}
	}

	logrus.Info("默认门户导航初始化完成")
	return nil
}
