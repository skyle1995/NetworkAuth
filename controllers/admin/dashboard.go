package admin

import (
	"NetworkAuth/constants"
	"NetworkAuth/controllers"
	"NetworkAuth/middleware"
	"NetworkAuth/models"
	"NetworkAuth/utils/timeutil"
	"time"

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

	// count 统计辅助：出错按 0 计，避免单个查询失败拖垮整个仪表盘
	count := func(model interface{}, query interface{}, args ...interface{}) int64 {
		var n int64
		q := db.Model(model)
		if query != nil {
			q = q.Where(query, args...)
		}
		q.Count(&n)
		return n
	}

	// 今日 0 点，用于“今日新增”统计
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	data := gin.H{
		// 应用
		"total_apps":   count(&models.App{}, nil),
		"enabled_apps": count(&models.App{}, "status = ?", 1),
		// 账号
		"total_members":     count(&models.Member{}, nil),
		"normal_members":    count(&models.Member{}, "status = ?", models.MemberStatusNormal),
		"disabled_members":  count(&models.Member{}, "status = ?", models.MemberStatusDisabled),
		"black_members":     count(&models.Member{}, "status = ?", models.MemberStatusBlack),
		"today_new_members": count(&models.Member{}, "created_at >= ?", todayStart),
		// 卡密
		"total_cards":  count(&models.Card{}, nil),
		"unused_cards": count(&models.Card{}, "status = ?", models.CardStatusUnused),
		"used_cards":   count(&models.Card{}, "status = ?", models.CardStatusUsed),
		"frozen_cards": count(&models.Card{}, "status = ?", models.CardStatusFrozen),
		// 资源
		"total_apis":      count(&models.API{}, nil),
		"total_functions": count(&models.Function{}, nil),
		"total_variables": count(&models.Variable{}, nil),
		// 在线会话（当前有效登录数）
		"online_sessions": count(&models.MemberSession{}, nil),
	}

	handlersBaseController.HandleSuccess(c, "ok", data)
}
