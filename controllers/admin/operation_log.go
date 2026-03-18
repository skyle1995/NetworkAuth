package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/models"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ============================================================================
// 全局变量
// ============================================================================

var logBaseController = controllers.NewBaseController()

// ============================================================================
// 页面处理器
// ============================================================================

// LogsFragmentHandler 日志操作页面片段处理器
func LogsFragmentHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "operation_logs.html", gin.H{
		"Title": "操作日志",
	})
}

// ============================================================================
// API处理器
// ============================================================================

// LogsListHandler 日志列表API处理器
func LogsListHandler(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = 10
	}

	// 获取搜索参数
	startTimeStr := strings.TrimSpace(c.Query("start_time"))
	endTimeStr := strings.TrimSpace(c.Query("end_time"))
	operationType := strings.TrimSpace(c.Query("operation_type"))
	operator := strings.TrimSpace(c.Query("operator"))

	// 构建查询
	db, ok := logBaseController.GetDB(c)
	if !ok {
		return
	}

	var logs []models.OperationLog
	var total int64

	query := db.Model(&models.OperationLog{})

	// 筛选条件
	if operationType != "" {
		query = query.Where("operation_type = ?", operationType)
	}
	if operator != "" {
		// 支持按 UUID 或 用户名 筛选
		query = query.Where("operator_uuid = ? OR operator = ?", operator, operator)
	}
	if startTimeStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", startTimeStr, time.Local); err == nil {
			query = query.Where("created_at >= ?", t)
		} else if t, err := time.ParseInLocation("2006-01-02 15:04:05", startTimeStr, time.Local); err == nil {
			query = query.Where("created_at >= ?", t)
		} else {
			query = query.Where("created_at >= ?", startTimeStr)
		}
	}
	if endTimeStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", endTimeStr, time.Local); err == nil {
			t = t.Add(24*time.Hour - time.Nanosecond)
			query = query.Where("created_at <= ?", t)
		} else if t, err := time.ParseInLocation("2006-01-02 15:04:05", endTimeStr, time.Local); err == nil {
			query = query.Where("created_at <= ?", t)
		} else {
			if len(endTimeStr) == 10 { // yyyy-MM-dd
				endTimeStr += " 23:59:59"
			}
			query = query.Where("created_at <= ?", endTimeStr)
		}
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		logrus.WithError(err).Error("获取日志总数失败")
		logBaseController.HandleInternalError(c, "获取日志总数失败", err)
		return
	}

	// 分页查询（时间倒序，从新到旧）
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		logrus.WithError(err).Error("查询日志列表失败")
		logBaseController.HandleInternalError(c, "查询日志列表失败", err)
		return
	}

	logBaseController.HandleSuccess(c, "获取日志列表成功", gin.H{
		"list":  logs,
		"total": total,
	})
}

// LogsClearHandler 清空日志API处理器
func LogsClearHandler(c *gin.Context) {
	db, ok := logBaseController.GetDB(c)
	if !ok {
		return
	}

	// 开启事务进行清空
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.OperationLog{}).Error; err != nil {
		logrus.WithError(err).Error("清空操作日志失败")
		logBaseController.HandleInternalError(c, "清空操作日志失败", err)
		return
	}

	// 记录操作日志 (因为刚刚清空了，这条将是第一条)
	operator := "admin"
	log := models.OperationLog{
		OperationType: "清空日志",
		Operator:      operator,
		OperatorUUID:  "",
		Details:       "管理员清空了所有操作日志",
		CreatedAt:     time.Now(),
	}
	db.Create(&log)

	logBaseController.HandleSuccess(c, "日志已清空", nil)
}
