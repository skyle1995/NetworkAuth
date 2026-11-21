package server

import (
	openapictl "NetworkAuth/controllers/openapi"
	"NetworkAuth/middleware"
	"NetworkAuth/services"
	"time"

	"github.com/gin-gonic/gin"
)

// RegisterOpenAPIRoutes 注册对外 API（API 密钥鉴权，非后台 JWT）。
//
// 分组：/api/openapi
// 鉴权：请求头 X-API-Key（或 Authorization: Bearer），能力由密钥的 scope 决定。
// 通用档 api:read / api:write 可兜底所有 *:read / *:write。
func RegisterOpenAPIRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/openapi")
	g.Use(middleware.RateLimit(300, time.Minute))
	{
		// 密钥自检：确认密钥有效并查看具备的能力（不要求具体 scope）
		g.GET("/me", middleware.RequireAPIKey(""), openapictl.WhoAmI)

		// 应用
		g.GET("/apps", middleware.RequireAPIKey(services.ScopeAppRead), openapictl.ListApps)
		g.GET("/apps/:uuid", middleware.RequireAPIKey(services.ScopeAppRead), openapictl.GetApp)
		g.POST("/apps", middleware.RequireAPIKey(services.ScopeAppWrite), openapictl.CreateApp)
		g.POST("/apps/:uuid/reset-secret", middleware.RequireAPIKey(services.ScopeAppWrite), openapictl.ResetAppSecret)
		g.DELETE("/apps/:uuid", middleware.RequireAPIKey(services.ScopeAppWrite), openapictl.DeleteApp)
		g.PUT("/apps/:uuid/billing", middleware.RequireAPIKey(services.ScopeBillingWrite), openapictl.UpdateAppBilling)

		// 变量
		g.GET("/variables", middleware.RequireAPIKey(services.ScopeVariableRead), openapictl.ListVariables)
		g.GET("/variables/:uuid", middleware.RequireAPIKey(services.ScopeVariableRead), openapictl.GetVariable)
		g.POST("/variables", middleware.RequireAPIKey(services.ScopeVariableWrite), openapictl.CreateVariable)
		g.PUT("/variables/:uuid", middleware.RequireAPIKey(services.ScopeVariableWrite), openapictl.UpdateVariable)
		g.DELETE("/variables/:uuid", middleware.RequireAPIKey(services.ScopeVariableWrite), openapictl.DeleteVariable)

		// 函数
		g.GET("/functions", middleware.RequireAPIKey(services.ScopeFunctionRead), openapictl.ListFunctions)
		g.GET("/functions/:uuid", middleware.RequireAPIKey(services.ScopeFunctionRead), openapictl.GetFunction)
		g.POST("/functions", middleware.RequireAPIKey(services.ScopeFunctionWrite), openapictl.CreateFunction)
		g.PUT("/functions/:uuid", middleware.RequireAPIKey(services.ScopeFunctionWrite), openapictl.UpdateFunction)
		g.DELETE("/functions/:uuid", middleware.RequireAPIKey(services.ScopeFunctionWrite), openapictl.DeleteFunction)

		// 终端账号
		g.GET("/members", middleware.RequireAPIKey(services.ScopeMemberRead), openapictl.ListMembers)
		g.GET("/members/:uuid", middleware.RequireAPIKey(services.ScopeMemberRead), openapictl.GetMember)
		g.POST("/members", middleware.RequireAPIKey(services.ScopeMemberWrite), openapictl.CreateMember)
		g.PUT("/members/:uuid", middleware.RequireAPIKey(services.ScopeMemberWrite), openapictl.UpdateMember)
		g.POST("/members/:uuid/status", middleware.RequireAPIKey(services.ScopeMemberWrite), openapictl.SetMemberStatus)
		g.POST("/members/:uuid/points", middleware.RequireAPIKey(services.ScopeMemberWrite), openapictl.AdjustMemberPoints)
		g.POST("/members/:uuid/time", middleware.RequireAPIKey(services.ScopeMemberWrite), openapictl.AdjustMemberTime)
		g.POST("/members/:uuid/password", middleware.RequireAPIKey(services.ScopeMemberWrite), openapictl.ResetMemberPassword)
		g.DELETE("/members/:uuid", middleware.RequireAPIKey(services.ScopeMemberWrite), openapictl.DeleteMember)
	}
}
