package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"time"

	"github.com/sirupsen/logrus"
)

// RecordOperationLog 记录操作日志
func RecordOperationLog(operationType, operator, operatorUUID, transactionID, appName, productName, details string) {
	db, err := database.GetDB()
	if err != nil {
		logrus.WithError(err).Error("获取数据库连接失败，无法记录操作日志")
		return
	}

	log := models.OperationLog{
		OperationType: operationType,
		Operator:      operator,
		OperatorUUID:  operatorUUID,
		TransactionID: transactionID,
		AppName:       appName,
		ProductName:   productName,
		Details:       details,
		CreatedAt:     time.Now(),
	}

	if err := db.Create(&log).Error; err != nil {
		logrus.WithError(err).Error("创建操作日志失败")
	}
}
