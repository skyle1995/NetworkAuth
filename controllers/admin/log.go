package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/models"
	"NetworkAuth/services"
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
// 登录日志 API处理器
// ============================================================================

// DashboardLoginLogsHandler 获取管理员最近登录日志
func DashboardLoginLogsHandler(c *gin.Context) {
	db, ok := logBaseController.GetDB(c)
	if !ok {
		return
	}

	// 获取分页参数
	page, limit := logBaseController.GetPaginationParams(c)

	// 获取当前管理员信息
	uuid := c.GetString("admin_uuid")
	username := c.GetString("admin_username")

	var total int64
	query := db.Model(&models.LoginLog{})

	// 如果有用户名，则仅过滤该用户的日志
	if uuid != "" {
		query = query.Where("uuid = ? OR (uuid = '' AND username = ?)", uuid, username)
	}

	logs, total, err := services.Paginate[models.LoginLog](query, page, limit, "created_at desc")
	if err != nil {
		logBaseController.HandleInternalError(c, "获取登录日志失败", err)
		return
	}

	data := gin.H{
		"total": total,
		"list":  logs,
	}
	logBaseController.HandleSuccess(c, "获取登录日志成功", data)
}

// LoginLogsListHandler 登录日志列表API处理器
func LoginLogsListHandler(c *gin.Context) {
	// 获取分页参数
	page, limit := logBaseController.GetPaginationParams(c)

	// 获取数据库连接
	db, ok := logBaseController.GetDB(c)
	if !ok {
		return
	}

	query := db.Model(&models.LoginLog{})

	// 筛选条件：账号或UUID合并搜索
	if username := strings.TrimSpace(c.Query("username")); username != "" {
		query = query.Where("username = ? OR uuid = ?", username, username)
	}

	// 筛选条件：IP
	if ip := strings.TrimSpace(c.Query("ip")); ip != "" {
		query = query.Where("ip = ?", ip)
	}

	// 筛选条件：状态
	if statusStr := strings.TrimSpace(c.Query("status")); statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil {
			query = query.Where("status = ?", status)
		}
	}

	// 筛选条件：时间范围
	query = logBaseController.ApplyTimeRangeQuery(c, query, "created_at")

	// 泛型分页查询
	logs, total, err := services.Paginate[models.LoginLog](query, page, limit, "created_at DESC")
	if err != nil {
		logBaseController.HandleInternalError(c, "获取日志列表失败", err)
		return
	}

	// 转换数据格式
	var list []map[string]interface{}
	for _, log := range logs {
		list = append(list, map[string]interface{}{
			"id":   log.ID,
			"uuid": log.UUID,
			"username":   log.Username,
			"ip":         log.IP,
			"status":     log.Status,
			"message":    log.Message,
			"user_agent": log.UserAgent,
			"created_at": log.CreatedAt,
		})
	}

	logBaseController.HandleSuccess(c, "ok", gin.H{
		"list":  list,
		"total": total,
	})
}

// LoginLogsClearHandler 清空登录日志API处理器
func LoginLogsClearHandler(c *gin.Context) {
	// 鉴权拦截：仅超级管理员 (role=0) 允许清空日志
	if role, exists := c.Get("admin_role"); !exists || role.(int) != 0 {
		logBaseController.HandleValidationError(c, "权限不足，仅超级管理员可清空日志")
		return
	}

	db, ok := logBaseController.GetDB(c)
	if !ok {
		return
	}

	// 检查数据库类型
	dbType := db.Dialector.Name()

	if dbType == "sqlite" {
		// SQLite 不支持 TRUNCATE，直接使用 DELETE 和重置自增序列
		if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Where("1 = 1").Delete(&models.LoginLog{}).Error; err != nil {
			logrus.WithError(err).Error("Failed to clear login logs")
			logBaseController.HandleInternalError(c, "清空登录日志失败", err)
			return
		}
		// 重置 sqlite 的自增序列
		db.Exec("UPDATE sqlite_sequence SET seq = 0 WHERE name = 'login_logs'")
		// 释放空间
		db.Exec("VACUUM")
	} else {
		// 其他数据库（如 MySQL/PostgreSQL）尝试使用 TRUNCATE
		if err := db.Exec("TRUNCATE TABLE login_logs").Error; err != nil {
			// 如果 TRUNCATE 失败，回退到 DELETE
			if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Where("1 = 1").Delete(&models.LoginLog{}).Error; err != nil {
				logrus.WithError(err).Error("Failed to clear login logs")
				logBaseController.HandleInternalError(c, "清空登录日志失败", err)
				return
			}
		}
	}

	// 记录操作日志
	var operator, operatorUUID string
	operator = c.GetString("admin_username")
	operatorUUID = c.GetString("admin_uuid")
	if operator == "" {
		operator = "system"
		operatorUUID = "system"
	}

	log := models.OperationLog{
		OperationType: "清空登录日志",
		Operator:      operator,
		OperatorUUID:  operatorUUID,
		Details:       "管理员清空了所有登录日志",
		CreatedAt:     time.Now(),
	}
	db.Create(&log)

	logBaseController.HandleSuccess(c, "登录日志已清空", nil)
}

// ============================================================================
// 操作日志 API处理器
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
	// 鉴权拦截：仅超级管理员 (role=0) 允许清空日志
	if role, exists := c.Get("admin_role"); !exists || role.(int) != 0 {
		logBaseController.HandleValidationError(c, "权限不足，仅超级管理员可清空日志")
		return
	}

	db, ok := logBaseController.GetDB(c)
	if !ok {
		return
	}

	// 检查数据库类型
	dbType := db.Dialector.Name()

	if dbType == "sqlite" {
		// SQLite 不支持 TRUNCATE，直接使用 DELETE 和重置自增序列
		if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Where("1 = 1").Delete(&models.OperationLog{}).Error; err != nil {
			logrus.WithError(err).Error("清空操作日志失败")
			logBaseController.HandleInternalError(c, "清空操作日志失败", err)
			return
		}
		// 重置 sqlite 的自增序列
		db.Exec("UPDATE sqlite_sequence SET seq = 0 WHERE name = 'operation_logs'")
		// 释放空间
		db.Exec("VACUUM")
	} else {
		// 其他数据库（如 MySQL/PostgreSQL）尝试使用 TRUNCATE
		if err := db.Exec("TRUNCATE TABLE operation_logs").Error; err != nil {
			// 如果 TRUNCATE 失败，回退到 DELETE
			if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Where("1 = 1").Delete(&models.OperationLog{}).Error; err != nil {
				logrus.WithError(err).Error("清空操作日志失败")
				logBaseController.HandleInternalError(c, "清空操作日志失败", err)
				return
			}
		}
	}

	// 记录操作日志 (因为刚刚清空了，这条将是第一条)
	var operator, operatorUUID string
	operator = c.GetString("admin_username")
	operatorUUID = c.GetString("admin_uuid")
	if operator == "" {
		operator = "system"
		operatorUUID = "system"
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
