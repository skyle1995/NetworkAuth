package admin

import (
	"NetworkAuth/constants"
	"NetworkAuth/controllers"
	"NetworkAuth/middleware"
	"NetworkAuth/models"
	"NetworkAuth/utils/timeutil"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// ============================================================================
// 全局变量
// ============================================================================

// 创建基础控制器实例
var handlersBaseController = controllers.NewBaseController()

// ============================================================================
// 辅助函数
// ============================================================================

// formatDBType 格式化数据库类型显示
// 将配置文件中的小写类型转换为友好的显示格式
func formatDBType(dbType string) string {
	switch dbType {
	case "mysql":
		return "MySQL"
	case "sqlite":
		return "SQLite"
	case "postgresql", "postgres":
		return "PostgreSQL"
	case "sqlserver":
		return "SQL Server"
	default:
		return "SQLite" // 默认显示
	}
}

// ============================================================================
// API处理器
// ============================================================================
// 返回系统运行状态的JSON数据，用于前端定时刷新
func SystemInfoHandler(c *gin.Context) {
	version := constants.AppVersion
	mode := middleware.IsDevModeFromContext(c)
	dbType := viper.GetString("database.type")
	if dbType == "" {
		dbType = "sqlite"
	}
	uptime := timeutil.GetServerUptimeString()
	uptimeSeconds := int64(timeutil.GetServerUptime().Seconds())

	data := gin.H{
		"version":        version,
		"mode":           mode,
		"db_type":        formatDBType(dbType),
		"uptime":         uptime,
		"uptime_seconds": uptimeSeconds,
	}

	handlersBaseController.HandleSuccess(c, "ok", data)
}

// DashboardStatsHandler 仪表盘统计数据API接口
// - 返回应用统计数据的JSON数据，包括全部/启用/变量数量
func DashboardStatsHandler(c *gin.Context) {
	// 获取数据库连接
	db, ok := handlersBaseController.GetDB(c)
	if !ok {
		return
	}

	// 统计应用数据
	var totalApps int64
	var totalFunctions int64
	var totalVariables int64

	// 统计全部应用数量
	if err := db.Model(&models.App{}).Count(&totalApps).Error; err != nil {
		handlersBaseController.HandleInternalError(c, "统计应用数量失败", err)
		return
	}

	// 统计函数数量
	if err := db.Model(&models.Function{}).Count(&totalFunctions).Error; err != nil {
		handlersBaseController.HandleInternalError(c, "统计函数数量失败", err)
		return
	}

	// 统计变量数量
	if err := db.Model(&models.Variable{}).Count(&totalVariables).Error; err != nil {
		handlersBaseController.HandleInternalError(c, "统计变量数量失败", err)
		return
	}

	data := gin.H{
		"total_apps":      totalApps,
		"total_functions": totalFunctions,
		"total_variables": totalVariables,
	}

	handlersBaseController.HandleSuccess(c, "ok", data)
}
