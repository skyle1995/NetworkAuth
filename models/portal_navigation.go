package models

import "time"

// PortalNavigation 门户导航表模型
// 用于维护门户页面展示的导航入口以及唯一首页标记
type PortalNavigation struct {
	ID         uint      `json:"id" gorm:"primaryKey;comment:导航ID，自增主键"`
	Name       string    `json:"name" gorm:"size:64;not null;comment:导航名称"`
	Type       string    `json:"type" gorm:"size:16;not null;default:link;comment:导航类型，link=链接，group=分组"`
	ParentID   uint      `json:"parent_id" gorm:"default:0;not null;comment:所属分组ID，0表示顶级导航"`
	Path       string    `json:"path" gorm:"size:255;not null;comment:导航地址或路由路径"`
	Sort       int       `json:"sort" gorm:"default:0;not null;comment:排序值，越小越靠前，0最优先"`
	IsHome     bool      `json:"is_home" gorm:"default:false;comment:是否为门户首页"`
	IsHidden   bool      `json:"is_hidden" gorm:"default:false;comment:是否隐藏"`
	IsExternal bool      `json:"is_external" gorm:"default:false;comment:是否外部打开"`
	CreatedAt  time.Time `json:"created_at" gorm:"comment:创建时间"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"comment:更新时间"`
}
