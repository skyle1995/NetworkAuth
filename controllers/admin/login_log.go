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
)

// ============================================================================
// 全局变量
// ============================================================================

var loginLogBaseController = controllers.NewBaseController()

// ============================================================================
// 辅助函数
// ============================================================================

// RecordLoginLog 记录登录日志
func RecordLoginLog(c *gin.Context, username string, status int, message string) {
	db, ok := loginLogBaseController.GetDB(c)
	if !ok {
		return
	}

	log := models.LoginLog{
		Type:      "admin",
		Username:  username,
		IP:        c.ClientIP(),
		Status:    status,
		Message:   message,
		UserAgent: c.Request.UserAgent(),
		CreatedAt: time.Now(),
	}

	if err := db.Create(&log).Error; err != nil {
		logrus.WithError(err).Error("Failed to create login log")
	}
}

// LoginLogsFragmentHandler 登录日志页面片段处理器
func LoginLogsFragmentHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "login_logs.html", gin.H{
		"Title": "登录日志",
	})
}

// ============================================================================
// API处理器
// ============================================================================

// LoginLogsListHandler 登录日志列表API处理器
func LoginLogsListHandler(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = 10
	}

	// 构建查询
	db, ok := loginLogBaseController.GetDB(c)
	if !ok {
		return
	}

	var logs []models.LoginLog
	var total int64

	// 兼容旧数据（Type为空）和新数据（Type=admin）
	query := db.Model(&models.LoginLog{}).Where("type = ? OR type = ? OR type IS NULL", "admin", "")

	// 筛选条件：用户名
	if username := strings.TrimSpace(c.Query("username")); username != "" {
		query = query.Where("username = ?", username)
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
	startTime := strings.TrimSpace(c.Query("start_time"))
	endTime := strings.TrimSpace(c.Query("end_time"))
	if startTime != "" && endTime != "" {
		query = query.Where("created_at BETWEEN ? AND ?", startTime, endTime)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		loginLogBaseController.HandleInternalError(c, "获取日志总数失败", err)
		return
	}

	// 查询数据（时间倒序，从新到旧）
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		loginLogBaseController.HandleInternalError(c, "获取日志列表失败", err)
		return
	}

	// 转换数据格式
	var list []map[string]interface{}
	for _, log := range logs {
		list = append(list, map[string]interface{}{
			"id":         log.ID,
			"username":   log.Username,
			"ip":         log.IP,
			"status":     log.Status,
			"message":    log.Message,
			"user_agent": log.UserAgent,
			"created_at": log.CreatedAt,
		})
	}

	loginLogBaseController.HandleSuccess(c, "ok", gin.H{
		"list":  list,
		"total": total,
	})
}

// LoginLogsClearHandler 清空登录日志API处理器
func LoginLogsClearHandler(c *gin.Context) {
	db, ok := loginLogBaseController.GetDB(c)
	if !ok {
		return
	}

	// 物理删除所有登录日志
	if err := db.Where("type = ?", "admin").Delete(&models.LoginLog{}).Error; err != nil {
		logrus.WithError(err).Error("Failed to clear login logs")
		loginLogBaseController.HandleInternalError(c, "清空登录日志失败", err)
		return
	}

	// 记录操作日志
	// 由于 NetworkAuth 中没有 SystemAdminUser 全局变量，这里暂时使用 "admin"
	operator := "admin"
	// 尝试从上下文获取用户名（如果中间件设置了的话）
	// if user, exists := c.Get("username"); exists {
	// 	operator = user.(string)
	// }

	log := models.OperationLog{
		OperationType: "清空登录日志",
		Operator:      operator,
		OperatorUUID:  "", // NetworkAuth 中暂时无法获取 UUID
		AppName:       "-",
		ProductName:   "-",
		TransactionID: "-",
		Details:       "管理员清空了所有登录日志",
		CreatedAt:     time.Now(),
	}
	db.Create(&log)

	loginLogBaseController.HandleSuccess(c, "登录日志已清空", nil)
}
