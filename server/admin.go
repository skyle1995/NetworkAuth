package server

import (
	adminctl "NetworkAuth/controllers/admin"

	"github.com/gin-gonic/gin"
)

// RegisterAdminRoutes 注册管理员后台相关路由
func RegisterAdminRoutes(rg *gin.RouterGroup) {
	admin := rg.Group("/admin")

	// Admin 认证相关路由
	admin.GET("/captcha", adminctl.CaptchaHandler)
	admin.GET("/csrf", adminctl.CSRFTokenHandler)
	admin.POST("/login", adminctl.LoginHandler)

	// 公开设置API
	admin.GET("/settings/public", adminctl.SettingsPublicHandler)

	// 退出登录
	admin.POST("/logout", adminctl.LogoutHandler)

	// 需要认证的路由组
	authorized := admin.Group("/")
	authorized.Use(adminctl.AdminAuthRequired())
	{
		// 系统信息API
		authorized.GET("/system/info", adminctl.SystemInfoHandler)
		authorized.GET("/dashboard/stats", adminctl.DashboardStatsHandler)
		authorized.GET("/dashboard/login-logs", adminctl.DashboardLoginLogsHandler)

		// 个人资料API
		authorized.GET("/profile", adminctl.ProfileQueryHandler)
		authorized.POST("/profile/update", adminctl.ProfileUpdateHandler)
		authorized.POST("/profile/password", adminctl.ProfilePasswordUpdateHandler)

		// 设置API
		authorized.GET("/settings", adminctl.SettingsQueryHandler)
		authorized.POST("/settings/update", adminctl.SettingsUpdateHandler)
		authorized.POST("/settings/generate-key", adminctl.SettingsGenerateKeyHandler)

		// 操作日志API
		authorized.GET("/logs", adminctl.LogsListHandler)
		authorized.POST("/logs/clear", adminctl.LogsClearHandler)

		// 登录日志API
		authorized.GET("/login_logs", adminctl.LoginLogsListHandler)
		authorized.POST("/login_logs/clear", adminctl.LoginLogsClearHandler)

		// 子账号相关API (Mock)
		authorized.GET("/subaccounts/simple", adminctl.SubAccountSimpleListHandler)

		// 应用管理API
		appsGroup := authorized.Group("/apps")
		{
			appsGroup.GET("/list", adminctl.AppsListHandler)
			appsGroup.GET("/simple", adminctl.AppsSimpleListHandler)
			appsGroup.POST("/create", adminctl.AppCreateHandler)
			appsGroup.POST("/update", adminctl.AppUpdateHandler)
			appsGroup.POST("/delete", adminctl.AppDeleteHandler)
			appsGroup.POST("/batch_delete", adminctl.AppsBatchDeleteHandler)
			appsGroup.POST("/batch_update_status", adminctl.AppsBatchUpdateStatusHandler)
			appsGroup.POST("/update_status", adminctl.AppUpdateStatusHandler)
			appsGroup.POST("/reset_secret", adminctl.AppResetSecretHandler)
			appsGroup.GET("/get_app_data", adminctl.AppGetAppDataHandler)
			appsGroup.POST("/update_app_data", adminctl.AppUpdateAppDataHandler)
			appsGroup.GET("/get_announcement", adminctl.AppGetAnnouncementHandler)
			appsGroup.POST("/update_announcement", adminctl.AppUpdateAnnouncementHandler)
			appsGroup.GET("/get_multi_config", adminctl.AppGetMultiConfigHandler)
			appsGroup.POST("/update_multi_config", adminctl.AppUpdateMultiConfigHandler)
			appsGroup.GET("/get_bind_config", adminctl.AppGetBindConfigHandler)
			appsGroup.POST("/update_bind_config", adminctl.AppUpdateBindConfigHandler)
			appsGroup.GET("/get_register_config", adminctl.AppGetRegisterConfigHandler)
			appsGroup.POST("/update_register_config", adminctl.AppUpdateRegisterConfigHandler)
		}

		// API接口管理API
		apisGroup := authorized.Group("/apis")
		{
			apisGroup.GET("/list", adminctl.APIListHandler)
			apisGroup.POST("/update", adminctl.APIUpdateHandler)
			apisGroup.POST("/update_status", adminctl.APIUpdateStatusHandler)
			apisGroup.GET("/types", adminctl.APIGetTypesHandler)
			apisGroup.POST("/generate_keys", adminctl.APIGenerateKeysHandler)
		}

		// 变量管理API
		variableGroup := authorized.Group("/variable")
		{
			variableGroup.GET("/list", adminctl.VariableListHandler)
			variableGroup.POST("/create", adminctl.VariableCreateHandler)
			variableGroup.POST("/update", adminctl.VariableUpdateHandler)
			variableGroup.POST("/delete", adminctl.VariableDeleteHandler)
			variableGroup.POST("/batch_delete", adminctl.VariablesBatchDeleteHandler)
		}

		// 函数管理API
		functionGroup := authorized.Group("/function")
		{
			functionGroup.GET("/list", adminctl.FunctionListHandler)
			functionGroup.POST("/create", adminctl.FunctionCreateHandler)
			functionGroup.POST("/update", adminctl.FunctionUpdateHandler)
			functionGroup.POST("/delete", adminctl.FunctionDeleteHandler)
			functionGroup.POST("/batch_delete", adminctl.FunctionsBatchDeleteHandler)
		}
	}
}
