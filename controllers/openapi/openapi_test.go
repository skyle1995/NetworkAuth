package openapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"NetworkAuth/database"
	"NetworkAuth/middleware"
	"NetworkAuth/models"
	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupOpenAPITest 建库 + 造两张密钥 + 装好路由
func setupOpenAPITest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "openapi.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.ApiKey{}, &models.App{}, &models.AppVersion{}, &models.Variable{},
		&models.Function{}, &models.Member{}, &models.MemberLevel{}, &models.Settings{},
		&models.Binding{}, &models.AppActivation{}, &models.MemberAccountToken{},
		&models.AppLevelPoint{}, &models.AppAccountBilling{}, &models.Card{}, &models.MemberLog{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	database.SetDB(db)
	services.ResetSettingsService()
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})

	// 只读密钥：只用通用档 api:read，验证兜底规则
	db.Create(&models.ApiKey{Name: "ro", Key: "sk_ro", Scopes: services.ScopeAPIRead, Status: 1})
	// 全权密钥
	db.Create(&models.ApiKey{Name: "rw", Key: "sk_rw", Status: 1, Scopes: joinScopes(
		services.ScopeAppRead, services.ScopeAppWrite, services.ScopeBillingWrite,
		services.ScopeVariableWrite, services.ScopeVariableRead,
		services.ScopeFunctionWrite, services.ScopeFunctionRead,
		services.ScopeMemberWrite, services.ScopeMemberRead,
	)})
	// 只有 app:write，没有计费权限
	db.Create(&models.ApiKey{Name: "apponly", Key: "sk_app", Scopes: services.ScopeAppWrite, Status: 1})

	r := gin.New()
	g := r.Group("/api/openapi")
	{
		g.GET("/me", middleware.RequireAPIKey(""), WhoAmI)
		g.GET("/apps", middleware.RequireAPIKey(services.ScopeAppRead), ListApps)
		g.POST("/apps", middleware.RequireAPIKey(services.ScopeAppWrite), CreateApp)
		g.PUT("/apps/:uuid/billing", middleware.RequireAPIKey(services.ScopeBillingWrite), UpdateAppBilling)
		g.DELETE("/apps/:uuid", middleware.RequireAPIKey(services.ScopeAppWrite), DeleteApp)
		g.POST("/variables", middleware.RequireAPIKey(services.ScopeVariableWrite), CreateVariable)
		g.GET("/variables", middleware.RequireAPIKey(services.ScopeVariableRead), ListVariables)
		g.POST("/functions", middleware.RequireAPIKey(services.ScopeFunctionWrite), CreateFunction)
		g.POST("/members", middleware.RequireAPIKey(services.ScopeMemberWrite), CreateMember)
		g.POST("/members/:uuid/points", middleware.RequireAPIKey(services.ScopeMemberWrite), AdjustMemberPoints)
		g.POST("/members/:uuid/status", middleware.RequireAPIKey(services.ScopeMemberWrite), SetMemberStatus)
		g.GET("/members", middleware.RequireAPIKey(services.ScopeMemberRead), ListMembers)
	}
	return r
}

func joinScopes(s ...string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}

// call 发请求并返回状态码与解析后的 JSON
func call(t *testing.T, r *gin.Engine, method, path, key string, body any) (int, map[string]any) {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// TestAPIKeyScopeFallback 通用档兜底 + 越权拒绝
func TestAPIKeyScopeFallback(t *testing.T) {
	r := setupOpenAPITest(t)

	// 无密钥 → 401
	if code, _ := call(t, r, http.MethodGet, "/api/openapi/apps", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("no key should be 401, got %d", code)
	}
	// api:read 兜底 app:read
	if code, _ := call(t, r, http.MethodGet, "/api/openapi/apps", "sk_ro", nil); code != http.StatusOK {
		t.Fatalf("api:read should allow GET /apps, got %d", code)
	}
	// api:read 不能写
	if code, _ := call(t, r, http.MethodPost, "/api/openapi/apps", "sk_ro", map[string]any{"name": "x"}); code != http.StatusForbidden {
		t.Fatalf("api:read must not write, got %d", code)
	}
	// 只有 app:write 的密钥不能调计费
	if code, _ := call(t, r, http.MethodPut, "/api/openapi/apps/ANY/billing", "sk_app", map[string]any{}); code != http.StatusForbidden {
		t.Fatalf("app:write must not touch billing, got %d", code)
	}
	// /me 自检
	code, out := call(t, r, http.MethodGet, "/api/openapi/me", "sk_ro", nil)
	if code != http.StatusOK || out["code"].(float64) != 0 {
		t.Fatalf("/me failed: %d %+v", code, out)
	}
}

// TestCreateAppAndBilling 创建应用 + 调整计费
func TestCreateAppAndBilling(t *testing.T) {
	r := setupOpenAPITest(t)

	code, out := call(t, r, http.MethodPost, "/api/openapi/apps", "sk_rw", map[string]any{
		"name": "API 建的应用", "plugin_id": "com.test.app",
	})
	if code != http.StatusOK {
		t.Fatalf("create app: %d %+v", code, out)
	}
	data := out["data"].(map[string]any)
	secret := data["secret"].(string)
	if secret == "" {
		t.Fatalf("secret should be returned on create")
	}
	app := data["app"].(map[string]any)
	uuid := app["uuid"].(string)
	// 默认计时模式
	if app["bill_time_enabled"].(float64) != 1 || app["bill_points_enabled"].(float64) != 0 {
		t.Fatalf("default should be time billing: %+v", app)
	}

	// 切成纯点数
	code, out = call(t, r, http.MethodPut, "/api/openapi/apps/"+uuid+"/billing", "sk_rw", map[string]any{
		"bill_time_enabled": false, "bill_points_enabled": true,
		"points_charge_mode": 1, "points_per_period": 2.5, "points_period_minutes": 30,
	})
	if code != http.StatusOK {
		t.Fatalf("update billing: %d %+v", code, out)
	}
	app = out["data"].(map[string]any)
	if app["bill_time_enabled"].(float64) != 0 || app["bill_points_enabled"].(float64) != 1 {
		t.Fatalf("billing flags not updated: %+v", app)
	}
	// billing_mode 应被推导为「点数」(1)
	if app["bill_points_enabled"].(float64) == 1 && app["bill_time_enabled"].(float64) == 0 {
		db, _ := database.GetDB()
		var stored models.App
		db.Where("uuid = ?", uuid).First(&stored)
		if stored.BillingMode != models.BillingModePoints {
			t.Fatalf("billing_mode should derive to points(1), got %d", stored.BillingMode)
		}
	}
}

// TestVariablesFunctions 变量与函数增改查
func TestVariablesFunctions(t *testing.T) {
	r := setupOpenAPITest(t)

	code, out := call(t, r, http.MethodPost, "/api/openapi/variables", "sk_rw", map[string]any{
		"alias": "api_key_var", "data": "v1",
	})
	if code != http.StatusOK {
		t.Fatalf("create variable: %d %+v", code, out)
	}
	// 别名重复应被拒
	code, _ = call(t, r, http.MethodPost, "/api/openapi/variables", "sk_rw", map[string]any{
		"alias": "api_key_var", "data": "v2",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("duplicate alias should be rejected, got %d", code)
	}
	// 列表能查到
	code, out = call(t, r, http.MethodGet, "/api/openapi/variables", "sk_ro", nil)
	if code != http.StatusOK {
		t.Fatalf("list variables: %d %+v", code, out)
	}
	if total := out["data"].(map[string]any)["total"].(float64); total < 1 {
		t.Fatalf("variable should be listed, total=%v", total)
	}

	code, out = call(t, r, http.MethodPost, "/api/openapi/functions", "sk_rw", map[string]any{
		"alias": "api_key_fn", "code": "return 1+1;",
	})
	if code != http.StatusOK {
		t.Fatalf("create function: %d %+v", code, out)
	}
}

// TestMemberOps 账号：创建 / 加点 / 改状态 / 列表
func TestMemberOps(t *testing.T) {
	r := setupOpenAPITest(t)

	code, out := call(t, r, http.MethodPost, "/api/openapi/members", "sk_rw", map[string]any{
		"username": "api_user@test.com", "password": "pw123456",
		"duration_value": 30, "duration_unit": "minute", "points": 5,
	})
	if code != http.StatusOK {
		t.Fatalf("create member: %d %+v", code, out)
	}
	m := out["data"].(map[string]any)
	uuid := m["uuid"].(string)
	if m["points"].(float64) != 5 {
		t.Fatalf("initial points should be 5: %+v", m)
	}

	// 加点
	code, out = call(t, r, http.MethodPost, "/api/openapi/members/"+uuid+"/points", "sk_rw", map[string]any{
		"points": 3,
	})
	if code != http.StatusOK {
		t.Fatalf("add points: %d %+v", code, out)
	}
	if p := out["data"].(map[string]any)["points"].(float64); p != 8 {
		t.Fatalf("points should be 8, got %v", p)
	}
	// 扣点
	code, out = call(t, r, http.MethodPost, "/api/openapi/members/"+uuid+"/points", "sk_rw", map[string]any{
		"points": 2, "mode": "sub",
	})
	if code != http.StatusOK {
		t.Fatalf("sub points: %d %+v", code, out)
	}
	if p := out["data"].(map[string]any)["points"].(float64); p != 6 {
		t.Fatalf("points should be 6, got %v", p)
	}

	// 封停
	code, out = call(t, r, http.MethodPost, "/api/openapi/members/"+uuid+"/status", "sk_rw", map[string]any{
		"status": 0,
	})
	if code != http.StatusOK {
		t.Fatalf("set status: %d %+v", code, out)
	}
	if s := out["data"].(map[string]any)["status"].(float64); s != 0 {
		t.Fatalf("status should be 0, got %v", s)
	}

	// 列表
	code, out = call(t, r, http.MethodGet, "/api/openapi/members?search=api_user", "sk_ro", nil)
	if code != http.StatusOK {
		t.Fatalf("list members: %d %+v", code, out)
	}
	if total := out["data"].(map[string]any)["total"].(float64); total != 1 {
		t.Fatalf("should find 1 member, got %v", total)
	}
}

// TestDeleteApp 删除应用：应用行与它的专属数据都要清掉，全站资源不受影响
func TestDeleteApp(t *testing.T) {
	r := setupOpenAPITest(t)
	db, _ := database.GetDB()

	// 建应用
	code, out := call(t, r, http.MethodPost, "/api/openapi/apps", "sk_rw", map[string]any{
		"name": "待删除应用", "plugin_id": "com.test.del",
	})
	if code != http.StatusOK {
		t.Fatalf("create app: %d %+v", code, out)
	}
	uuid := out["data"].(map[string]any)["app"].(map[string]any)["uuid"].(string)

	// 造两条应用专属数据：版本 + 等级点数
	db.Create(&models.AppVersion{AppUUID: uuid, Version: "1.0.0", Enabled: true, PublishedAt: 1})
	db.Create(&models.AppLevelPoint{AppUUID: uuid, MemberLevelUUID: "LV-1", Points: 2})
	// 造一条全站变量，删应用不应受影响
	db.Create(&models.Variable{Alias: "keep_me", Data: "x"})

	// 删除
	code, out = call(t, r, http.MethodDelete, "/api/openapi/apps/"+uuid, "sk_rw", nil)
	if code != http.StatusOK {
		t.Fatalf("delete app: %d %+v", code, out)
	}
	// 再查应 404
	if code, _ := call(t, r, http.MethodGet, "/api/openapi/apps/"+uuid, "sk_rw", nil); code != http.StatusNotFound {
		t.Fatalf("app should be gone, got %d", code)
	}
	// 专属数据清空
	var versions, levelPoints int64
	db.Model(&models.AppVersion{}).Where("app_uuid = ?", uuid).Count(&versions)
	db.Model(&models.AppLevelPoint{}).Where("app_uuid = ?", uuid).Count(&levelPoints)
	if versions != 0 || levelPoints != 0 {
		t.Fatalf("app-scoped rows should be deleted, versions=%d levelPoints=%d", versions, levelPoints)
	}
	// 全站变量保留
	var vars int64
	db.Model(&models.Variable{}).Where("alias = ?", "keep_me").Count(&vars)
	if vars != 1 {
		t.Fatalf("global variables must not be touched, got %d", vars)
	}

	// 不存在的 uuid → 404
	if code, _ := call(t, r, http.MethodDelete, "/api/openapi/apps/NOT-EXIST", "sk_rw", nil); code != http.StatusNotFound {
		t.Fatalf("deleting missing app should be 404, got %d", code)
	}
	// 无权限 → 403
	if code, _ := call(t, r, http.MethodDelete, "/api/openapi/apps/"+uuid, "sk_ro", nil); code != http.StatusForbidden {
		t.Fatalf("api:read must not delete, got %d", code)
	}
}