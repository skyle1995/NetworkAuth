package models

import (
	"time"
)

// OperationLog 操作日志模型
type OperationLog struct {
	ID uint `gorm:"primarykey" json:"id"`

	// 操作信息
	OperationType string `gorm:"type:varchar(50);index;comment:操作方式" json:"operation_type"` // 如：入库成功、凭证分配等

	// 操作人信息
	Operator     string `gorm:"type:varchar(100);index;comment:操作账号" json:"operator"`
	OperatorUUID string `gorm:"type:varchar(36);index;comment:操作账号UUID" json:"operator_uuid"`

	Details string `gorm:"type:text;comment:操作详情" json:"details"`

	CreatedAt time.Time `gorm:"index;comment:创建时间" json:"created_at"`
}
