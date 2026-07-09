package server

import (
	adminctl "NetworkAuth/controllers/admin"
	"NetworkAuth/middleware"
	"time"

	"github.com/gin-gonic/gin"
)

// RegisterAdminRoutes 注册管理员后台相关路由
func RegisterAdminRoutes(rg *gin.RouterGroup) {
	admin := rg.Group("/admin")

	// Admin 认证相关路由
	admin.GET("/captcha", middleware.RateLimit(30, time.Minute), adminctl.CaptchaHandler)
	// 验证码类型 + 滑动拼图/点击文字验证码（未鉴权入口，同样加 IP 级限流）
	admin.GET("/captcha/type", adminctl.CaptchaTypeHandler)
	admin.GET("/captcha/slide", middleware.RateLimit(30, time.Minute), adminctl.SlideCaptchaHandler)
	admin.POST("/captcha/slide/verify", middleware.RateLimit(60, time.Minute), adminctl.SlideCaptchaVerifyHandler)
	admin.GET("/captcha/click", middleware.RateLimit(30, time.Minute), adminctl.ClickCaptchaHandler)
	admin.POST("/captcha/click/verify", middleware.RateLimit(60, time.Minute), adminctl.ClickCaptchaVerifyHandler)
	admin.GET("/csrf", adminctl.CSRFTokenHandler)
	admin.POST("/login", middleware.RateLimit(10, time.Minute), adminctl.LoginHandler)
	admin.POST("/refresh-token", middleware.RateLimit(30, time.Minute), adminctl.RefreshTokenHandler)

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

		// 设置API（系统级，仅超级管理员）
		authorized.GET("/settings", adminctl.SuperAdminRequired(), adminctl.SettingsQueryHandler)
		authorized.POST("/settings/update", adminctl.SuperAdminRequired(), adminctl.SettingsUpdateHandler)
		authorized.POST("/settings/generate-key", adminctl.SuperAdminRequired(), adminctl.SettingsGenerateKeyHandler)
		authorized.POST("/settings/test-mail", adminctl.SuperAdminRequired(), adminctl.SettingsTestMailHandler)

		// 系统在线更新（自更新）
		authorized.POST("/system/self-update/check", adminctl.SuperAdminRequired(), adminctl.SelfUpdateCheckHandler)
		authorized.POST("/system/self-update/check-force", adminctl.SuperAdminRequired(), adminctl.SelfUpdateCheckForceHandler)
		authorized.POST("/system/self-update/restart", adminctl.SuperAdminRequired(), adminctl.SelfUpdateRestartHandler)
		authorized.GET("/system/self-update/status", adminctl.SelfUpdateStatusHandler)
		authorized.GET("/system/self-update/versions", adminctl.SelfUpdateVersionsHandler)
		authorized.POST("/system/self-update/prepare", adminctl.SuperAdminRequired(), adminctl.SelfUpdatePrepareHandler)
		authorized.GET("/system/self-update/config", adminctl.SuperAdminRequired(), adminctl.SelfUpdateGetConfigHandler)
		authorized.PUT("/system/self-update/config", adminctl.SuperAdminRequired(), adminctl.SelfUpdateUpdateConfigHandler)
		authorized.POST("/system/self-update/test", adminctl.SuperAdminRequired(), adminctl.SelfUpdateTestConfigHandler)
		authorized.GET("/navigation", adminctl.PortalNavigationListHandler)
		authorized.POST("/navigation/create", adminctl.PortalNavigationCreateHandler)
		authorized.POST("/navigation/update", adminctl.PortalNavigationUpdateHandler)
		authorized.POST("/navigation/delete", adminctl.PortalNavigationDeleteHandler)

		// 操作日志API
		authorized.GET("/logs", adminctl.LogsListHandler)                      // 获取操作日志列表
		authorized.POST("/logs/clear", adminctl.LogsClearHandler)              // 清空操作日志
		authorized.POST("/logs/batch-delete", adminctl.LogsBatchDeleteHandler) // 批量删除操作日志
		apikeyGroup := authorized.Group("/apikey")
		{
			apikeyGroup.GET("/scopes", adminctl.GetApiKeyScopes)
			apikeyGroup.GET("/list", adminctl.GetApiKeyList)
			apikeyGroup.POST("/create", adminctl.CreateApiKey)
			apikeyGroup.PUT("/update", adminctl.UpdateApiKey)
			apikeyGroup.POST("/regenerate", adminctl.RegenerateApiKey)
			apikeyGroup.DELETE("/delete/:id", adminctl.DeleteApiKey)
		}

		// 登录日志API
		authorized.GET("/login_logs", adminctl.LoginLogsListHandler)                      // 获取登录日志列表
		authorized.POST("/login_logs/clear", adminctl.LoginLogsClearHandler)              // 清空登录日志
		authorized.POST("/login_logs/batch-delete", adminctl.LoginLogsBatchDeleteHandler) // 批量删除登录日志

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
			apisGroup.GET("/export", adminctl.SuperAdminRequired(), adminctl.APIExportKeysHandler)
			apisGroup.POST("/generate_keys", adminctl.APIGenerateKeysHandler)
			apisGroup.POST("/batch_set", adminctl.APIBatchSetAlgorithmHandler)
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

		// 卡密管理API
		cardGroup := authorized.Group("/card")
		{
			cardGroup.GET("/list", adminctl.CardListHandler)
			cardGroup.POST("/create", adminctl.CardCreateHandler)
			cardGroup.POST("/export", adminctl.CardExportHandler)
			cardGroup.POST("/freeze", adminctl.CardFreezeHandler)
			cardGroup.POST("/unfreeze", adminctl.CardUnfreezeHandler)
			cardGroup.POST("/batch_delete", adminctl.CardsBatchDeleteHandler)
			cardGroup.POST("/delete_batch", adminctl.CardDeleteByBatchHandler)
		}

		// 账号管理API
		memberGroup := authorized.Group("/member")
		{
			memberGroup.GET("/list", adminctl.MemberListHandler)
			memberGroup.POST("/create", adminctl.MemberCreateHandler)
			memberGroup.POST("/set_status", adminctl.MemberSetStatusHandler)
			memberGroup.POST("/recharge", adminctl.MemberRechargeHandler)
			memberGroup.POST("/deduct", adminctl.MemberDeductHandler)
			memberGroup.POST("/reset_password", adminctl.MemberResetPasswordHandler)
			memberGroup.POST("/update_remark", adminctl.MemberUpdateRemarkHandler)
			memberGroup.GET("/bindings", adminctl.MemberBindingsHandler)
			memberGroup.POST("/clear_bindings", adminctl.MemberClearBindingsHandler)
			memberGroup.GET("/online", adminctl.OnlineSessionsHandler)
			memberGroup.POST("/kick", adminctl.MemberKickSessionHandler)
			memberGroup.POST("/blacklist", adminctl.MemberBlacklistHandler)
			memberGroup.POST("/online/blacklist", adminctl.SessionBlacklistHandler)
			memberGroup.GET("/get_data", adminctl.MemberGetDataHandler)
			memberGroup.POST("/update_data", adminctl.MemberUpdateDataHandler)
			memberGroup.GET("/logs", adminctl.MemberLogListHandler)
			memberGroup.POST("/logs/clear", adminctl.MemberLogClearHandler)
			memberGroup.POST("/batch_delete", adminctl.MembersBatchDeleteHandler)
		}

		// 黑名单管理API（设备/IP/地区）
		blacklistGroup := authorized.Group("/blacklist")
		{
			blacklistGroup.GET("/list", adminctl.BlacklistListHandler)
			blacklistGroup.POST("/add", adminctl.BlacklistAddHandler)
			blacklistGroup.POST("/delete", adminctl.BlacklistDeleteHandler)
		}
	}
}
