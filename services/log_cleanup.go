package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"fmt"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// StartLogCleanupTask 启动日志清理定时任务
// 每天执行一次，且服务启动后也会尝试执行一次
func StartLogCleanupTask() {
	go func() {
		// 启动后延迟1分钟执行首次清理，避免影响启动速度
		time.Sleep(1 * time.Minute)
		cleanupLogs()

		// 每天执行一次
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			cleanupLogs()
		}
	}()
}

func cleanupLogs() {
	logrus.Debug("开始执行日志清理任务...")

	// 获取清理配置 (使用实时查询)
	loginLogDays := getSettingInt("login_log_cleanup_days", 30)
	loginLogLimit := getSettingInt("login_log_cleanup_limit", 10000)
	operationLogDays := getSettingInt("operation_log_cleanup_days", 30)
	operationLogLimit := getSettingInt("operation_log_cleanup_limit", 10000)

	// 清理登录日志
	if err := cleanupTable(&models.LoginLog{}, loginLogDays, loginLogLimit); err != nil {
		logrus.WithError(err).Error("清理登录日志失败")
	}

	// 清理操作日志
	if err := cleanupTable(&models.OperationLog{}, operationLogDays, operationLogLimit); err != nil {
		logrus.WithError(err).Error("清理操作日志失败")
	}

	logrus.Debug("日志清理任务执行完成")
}

// getSettingInt 获取配置整数值
func getSettingInt(key string, defaultValue int) int {
	setting, err := GetSettingsService().GetSettingRealtime(key)
	if err != nil {
		return defaultValue
	}

	val, err := strconv.Atoi(setting.Value)
	if err != nil {
		return defaultValue
	}
	return val
}

// cleanupTable 通用清理函数
func cleanupTable(model interface{}, retentionDays int, maxCount int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	// 1. 按天清理
	if retentionDays > 0 {
		cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
		result := db.Unscoped().Where("created_at < ?", cutoffDate).Delete(model)
		if result.Error != nil {
			return fmt.Errorf("按天清理失败: %w", result.Error)
		}
		if result.RowsAffected > 0 {
			logrus.Debugf("清理日志 (按天): 删除 %d 条记录", result.RowsAffected)
		}
	}

	// 2. 按数量清理
	if maxCount > 0 {
		count, err := CountEntitiesByCondition(model, "", db)
		if err != nil {
			return fmt.Errorf("查询总数失败: %w", err)
		}

		if count > int64(maxCount) {
			// 找出保留范围内的最后一条记录（第 maxCount 条，按时间倒序）
			var keepRecord struct {
				ID uint
			}

			// 假设 ID 是自增的主键，且 ID 越大代表记录越新
			if err := db.Model(model).Select("id").Order("id DESC").Offset(maxCount - 1).Limit(1).Scan(&keepRecord).Error; err != nil {
				return fmt.Errorf("查询分界记录失败: %w", err)
			}

			if keepRecord.ID > 0 {
				result := db.Unscoped().Where("id < ?", keepRecord.ID).Delete(model)
				if result.Error != nil {
					return fmt.Errorf("按数量清理失败: %w", result.Error)
				}
				if result.RowsAffected > 0 {
					logrus.Debugf("清理日志 (按数量): 删除 %d 条记录", result.RowsAffected)
				}
			}
		}
	}

	return nil
}
