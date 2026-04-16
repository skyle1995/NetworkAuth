package database

import (
	"NetworkAuth/models"
	"errors"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// NeedSeedDefaultPortalNavigation 判断是否需要修复默认门户导航。
// 仅在门户导航表缺失、关键字段缺失、没有任何数据或存在旧版脏数据时返回 true。
func NeedSeedDefaultPortalNavigation() (bool, error) {
	db, err := GetDB()
	if err != nil {
		return false, err
	}

	if !db.Migrator().HasTable(&models.PortalNavigation{}) {
		return true, nil
	}

	requiredColumns := []string{"type", "parent_id", "is_home", "is_hidden", "is_external"}
	for _, column := range requiredColumns {
		if !db.Migrator().HasColumn(&models.PortalNavigation{}, column) {
			return true, nil
		}
	}

	var count int64
	if err := db.Model(&models.PortalNavigation{}).Count(&count).Error; err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil
	}

	if err := db.Model(&models.PortalNavigation{}).Where("type = '' OR type IS NULL").Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	return false, nil
}

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
			Type:       "link",
			ParentID:   0,
			Path:       "/home/index",
			Sort:       0,
			IsHome:     true,
			IsHidden:   false,
			IsExternal: false,
		},
		{
			Name:       "管理员登录",
			Type:       "link",
			ParentID:   0,
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
				"type":        "link",
				"parent_id":   0,
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

	if err := db.Model(&models.PortalNavigation{}).Where("type = '' OR type IS NULL").Updates(map[string]interface{}{
		"type":      "link",
		"parent_id": 0,
	}).Error; err != nil {
		return err
	}

	logrus.Info("默认门户导航初始化完成")
	return nil
}
