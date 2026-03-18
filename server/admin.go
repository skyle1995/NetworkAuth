package server

import (
	adminctl "NetworkAuth/controllers/admin"
	"NetworkAuth/utils"

	"github.com/gin-gonic/gin"
)

// RegisterAdminRoutes 注册管理员后台相关路由
// - /admin/login: 支持GET渲染登录页、POST提交登录
// - /admin/logout: 管理员退出登录
// - /admin/dashboard: 管理员仪表盘
// - /admin/fragment/*: 布局内动态片段加载
// - /admin/api/*: 各种业务API
func RegisterAdminRoutes(r *gin.Engine) {
	// /admin 根与前缀统一入口：根据是否登录跳转
	r.GET("/admin", adminctl.AdminIndexHandler)
	r.GET("/admin/", adminctl.AdminIndexHandler)

	// Admin 认证相关路由
	r.GET("/admin/login", adminctl.LoginPageHandler)
	r.POST("/admin/login", adminctl.LoginHandler)

	// 退出登录
	r.GET("/admin/logout", adminctl.LogoutHandler)
	r.POST("/admin/logout", adminctl.LogoutHandler)

	// 验证码生成路由（无需认证）
	r.GET("/admin/captcha", adminctl.CaptchaHandler)

	// CSRF令牌获取API（无需认证，但需要在登录页面等地方获取）
	r.GET("/admin/api/csrf-token", func(c *gin.Context) {
		// 生成新的CSRF令牌
		token, err := utils.GenerateCSRFToken()
		if err != nil {
			c.JSON(500, gin.H{"success": false, "message": "生成CSRF令牌失败"})
			return
		}

		// 设置令牌到Cookie和响应头
		utils.SetCSRFToken(c, token)

		// 返回令牌给前端
		c.JSON(200, gin.H{
			"code":    0, // 统一使用 code 0 表示成功
			"success": true,
			"message": "CSRF令牌生成成功",
			"data": gin.H{
				"csrf_token": token,
			},
		})
	})

	// 需要认证的路由组
	authorized := r.Group("/admin")
	authorized.Use(adminctl.AdminAuthRequired())
	{
		// 后台布局页
		authorized.GET("/layout", adminctl.AdminLayoutHandler)

		// 片段路由
		authorized.GET("/dashboard", adminctl.DashboardFragmentHandler)
		authorized.GET("/profile", adminctl.ProfileFragmentHandler)
		authorized.GET("/settings", adminctl.SettingsFragmentHandler)
		authorized.GET("/operation_logs", adminctl.LogsFragmentHandler)
		authorized.GET("/login_logs", adminctl.LoginLogsFragmentHandler)
		authorized.GET("/apps", adminctl.AppsFragmentHandler)
		authorized.GET("/apis", adminctl.APIFragmentHandler)
		authorized.GET("/variables", adminctl.VariableFragmentHandler)
		authorized.GET("/functions", adminctl.FunctionFragmentHandler)

		// 系统信息API
		authorized.GET("/api/system/info", adminctl.SystemInfoHandler)
		// 仪表盘数据
		authorized.GET("/api/dashboard/stats", adminctl.DashboardStatsHandler)
		authorized.GET("/api/dashboard/login-logs", adminctl.DashboardLoginLogsHandler)

		// API 路由组
		api := authorized.Group("/api")
		{
			// 个人资料API
			profileGroup := api.Group("/profile")
			{
				profileGroup.GET("/info", adminctl.ProfileInfoHandler)
				profileGroup.POST("/update", adminctl.ProfileUpdateHandler)
				profileGroup.POST("/password", adminctl.ProfilePasswordUpdateHandler)
			}

			// 系统设置API
			settingsGroup := api.Group("/settings")
			{
				settingsGroup.GET("", adminctl.SettingsQueryHandler)
				settingsGroup.POST("/update", adminctl.SettingsUpdateHandler)
				settingsGroup.POST("/generate-key", adminctl.SettingsGenerateKeyHandler)
			}

			// 操作日志API
			logsGroup := api.Group("/logs")
			{
				logsGroup.GET("", adminctl.LogsListHandler)
				logsGroup.POST("/clear", adminctl.LogsClearHandler)
			}

			// 登录日志API
			loginLogsGroup := api.Group("/login_logs")
			{
				loginLogsGroup.GET("", adminctl.LoginLogsListHandler)
				loginLogsGroup.POST("/clear", adminctl.LoginLogsClearHandler)
			}

			// 应用管理API
			appsGroup := api.Group("/apps")
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
			apisGroup := api.Group("/apis")
			{
				apisGroup.GET("/list", adminctl.APIListHandler)
				apisGroup.POST("/update", adminctl.APIUpdateHandler)
				apisGroup.POST("/update_status", adminctl.APIUpdateStatusHandler)
				apisGroup.GET("/types", adminctl.APIGetTypesHandler)
				apisGroup.POST("/generate_keys", adminctl.APIGenerateKeysHandler)
			}
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
