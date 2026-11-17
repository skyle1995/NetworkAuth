package models

import "time"

// AppVersion 应用/插件的版本列表：一个应用可以有多条历史版本，
// 更新检查取「已启用」里的最高版本；apps 表不再存版本号与下载地址。
type AppVersion struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AppUUID     string    `gorm:"size:64;not null;index:idx_app_version,priority:1" json:"app_uuid"`
	Version     string    `gorm:"size:32;not null;uniqueIndex:idx_app_version,priority:2" json:"version"`
	DownloadType int      `gorm:"default:2;comment:0=不启用 1=强制更新 2=自由更新" json:"download_type"`
	DownloadURL string    `gorm:"size:2048" json:"download_url"`
	Changelog   string    `gorm:"type:text" json:"changelog"`
	Enabled     bool      `gorm:"default:true;index" json:"enabled"`
	PublishedAt int64     `gorm:"not null" json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
