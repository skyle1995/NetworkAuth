package services

import (
	"NetworkAuth/models"
	"strings"

	"gorm.io/gorm"
)

const portalNavigationAdminPath = "admin"
const portalNavigationAdminSort = 999
const portalNavigationTypeLink = "link"
const portalNavigationTypeGroup = "group"

// NormalizePortalNavigation 规范化门户导航数据
// 统一清理首尾空白，避免保存脏数据
func NormalizePortalNavigation(item *models.PortalNavigation) {
	item.Name = strings.TrimSpace(item.Name)
	item.Type = strings.ToLower(strings.TrimSpace(item.Type))
	if item.Type == "" {
		item.Type = portalNavigationTypeLink
	}
	item.Path = strings.TrimSpace(item.Path)
	if item.Sort < 0 {
		item.Sort = 0
	}
	if item.Type == portalNavigationTypeGroup {
		item.ParentID = 0
		item.Path = ""
		item.IsExternal = false
		item.IsHome = false
	}
	if item.IsHome {
		item.IsHidden = false
		item.ParentID = 0
	}
}

// IsPortalNavigationGroup 判断是否为分组导航
func IsPortalNavigationGroup(item models.PortalNavigation) bool {
	return strings.EqualFold(strings.TrimSpace(item.Type), portalNavigationTypeGroup)
}

// IsPortalNavigationLink 判断是否为链接导航
func IsPortalNavigationLink(item models.PortalNavigation) bool {
	return !IsPortalNavigationGroup(item)
}

// IsPortalNavigationAdminEntry 判断是否为管理员入口
// 管理员入口属于系统保留导航项，不允许修改基础信息
func IsPortalNavigationAdminEntry(item models.PortalNavigation) bool {
	return strings.EqualFold(strings.TrimSpace(item.Path), portalNavigationAdminPath)
}

// LockPortalNavigationProtectedFields 锁定系统保留导航字段
// 管理员入口仅允许调整隐藏状态，其余字段保持数据库原值
func LockPortalNavigationProtectedFields(item *models.PortalNavigation, exists models.PortalNavigation) {
	switch IsPortalNavigationAdminEntry(exists) {
	case true:
		item.Name = "管理员登录"
		item.Type = portalNavigationTypeLink
		item.ParentID = 0
		item.Path = portalNavigationAdminPath
		item.Sort = portalNavigationAdminSort
		item.IsHome = false
		item.IsExternal = false
	default:
		return
	}
}

// SavePortalNavigation 保存门户导航
// 当当前记录被设置为门户首页时，会自动取消其他记录的首页状态
func SavePortalNavigation(db *gorm.DB, item *models.PortalNavigation, exists ...models.PortalNavigation) error {
	if len(exists) > 0 {
		LockPortalNavigationProtectedFields(item, exists[0])
	}
	NormalizePortalNavigation(item)

	return db.Transaction(func(tx *gorm.DB) error {
		if item.IsHome {
			query := tx.Model(&models.PortalNavigation{}).Where("is_home = ?", true)
			if item.ID > 0 {
				query = query.Where("id <> ?", item.ID)
			}
			if err := query.Update("is_home", false).Error; err != nil {
				return err
			}
		}

		switch {
		case item.ID == 0:
			return tx.Create(item).Error
		default:
			return tx.Model(&models.PortalNavigation{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
				"name":        item.Name,
				"type":        item.Type,
				"parent_id":   item.ParentID,
				"path":        item.Path,
				"sort":        item.Sort,
				"is_home":     item.IsHome,
				"is_hidden":   item.IsHidden,
				"is_external": item.IsExternal,
			}).Error
		}
	})
}
