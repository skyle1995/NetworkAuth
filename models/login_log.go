package models

import (
	"time"
)

// LoginLog 登录日志模型
type LoginLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Type      string    `gorm:"type:varchar(20);index;comment:日志类型(admin/user)" json:"type"`
	UUID      string    `gorm:"type:char(36);index;comment:用户UUID" json:"uuid"`
	Username  string    `gorm:"type:varchar(100);index;comment:登录用户名" json:"username"`
	IP        string    `gorm:"type:varchar(50);comment:登录IP" json:"ip"`
	Status    int       `gorm:"type:tinyint;comment:登录状态 1:成功 0:失败" json:"status"`
	Message   string    `gorm:"type:varchar(255);comment:日志详情" json:"message"`
	UserAgent string    `gorm:"type:varchar(255);comment:用户代理" json:"user_agent"`
	CreatedAt time.Time `gorm:"index;comment:创建时间" json:"created_at"`
}
