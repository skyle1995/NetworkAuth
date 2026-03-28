package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/models"
	"NetworkAuth/services"
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
// API处理器
// ============================================================================

// LogsListHandler 日志列表API处理器
func LogsListHandler(c *gin.Context) {
	// 获取分页参数
	page, limit := logBaseController.GetPaginationParams(c)

	// 获取搜索参数
	operationType := strings.TrimSpace(c.Query("operation_type"))
	operator := strings.TrimSpace(c.Query("operator"))

	// 获取数据库连接
	db, ok := logBaseController.GetDB(c)
	if !ok {
		return
	}

	query := db.Model(&models.OperationLog{})

	// 筛选条件
	if operationType != "" {
		query = query.Where("operation_type = ?", operationType)
	}
	if operator != "" {
		// 支持按 UUID 或 用户名 筛选
		query = query.Where("operator_uuid = ? OR operator = ?", operator, operator)
	}

	// 筛选条件：时间范围
	query = logBaseController.ApplyTimeRangeQuery(c, query, "created_at")

	// 泛型分页查询
	logs, total, err := services.Paginate[models.OperationLog](query, page, limit, "created_at DESC")
	if err != nil {
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
	var operator, operatorUUID string
	if claims, _, err := GetCurrentAdminUserWithRefresh(c); err == nil && claims != nil {
		operator = claims.Username
		operatorUUID = claims.UUID
	} else {
		operator = "admin"
		operatorUUID = "00000000-0000-0000-0000-000000000000"
	}

	log := models.OperationLog{
		OperationType: "清空日志",
		Operator:      operator,
		OperatorUUID:  operatorUUID,
		Details:       "管理员清空了所有操作日志",
		CreatedAt:     time.Now(),
	}
	db.Create(&log)

	logBaseController.HandleSuccess(c, "日志已清空", nil)
}
