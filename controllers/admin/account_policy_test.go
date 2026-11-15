package admin

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestAccountPolicyHandlersAreGlobal 验证「账号策略」接口已全局化：
// 不再要求 app_uuid，读写的都是 settings 里的 account_* 全局键。
//
// 历史背景：这两个接口的路由名仍挂在 /apps 下（为避免破坏已对接的后台前端），
// 但早期实现会校验 uuid 并要求应用存在；一号通改造后若不改，
// 前端传空就会得到「应用UUID不能为空」，账号策略页直接打不开。
func TestAccountPolicyHandlersAreGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Settings{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	// 让服务层使用同一个内存库
	database.SetDB(db)
	t.Cleanup(func() { database.SetDB(nil) })

	// ---------- 读取：不带任何参数也必须成功 ----------
	router := gin.New()
	router.GET("/api/admin/apps/get_register_config", AppGetRegisterConfigHandler)
	router.POST("/api/admin/apps/update_register_config", AppUpdateRegisterConfigHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/apps/get_register_config", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET should not require app uuid, got status=%v body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if got.Code != 0 {
		t.Fatalf("GET should succeed, got code=%v body=%s", got.Code, w.Body.String())
	}
	// 默认值：注册开、卡密登录开、充值开、试用关
	for _, k := range []string{
		"register_enabled", "email_verify_enabled", "card_register_enabled",
		"register_limit_enabled", "register_device_limit_enabled",
		"register_limit_time", "register_count",
		"card_login_enabled", "recharge_enabled",
		"trial_enabled", "trial_limit_time", "trial_points",
		"trial_duration_minutes", "trial_claim_mode",
	} {
		if _, ok := got.Data[k]; !ok {
			t.Fatalf("GET response missing field %q", k)
		}
	}

	// ---------- 写入：不带 uuid，且必须真正落到 settings ----------
	payload := map[string]any{
		"register_enabled":              0,
		"email_verify_enabled":          1,
		"card_register_enabled":         1,
		"register_limit_enabled":        1,
		"register_device_limit_enabled": 1,
		"register_limit_time":           0,
		"register_count":                5,
		"card_login_enabled":            0,
		"recharge_enabled":              0,
		"trial_enabled":                 1,
		"trial_limit_time":              0,
		"trial_points":                  100,
		"trial_duration_minutes":        1440,
		"trial_claim_mode":              0,
		// 账号级资源配额
		"max_devices":       3,
		"max_concurrent":    1,
		"rebind_free_count": 1,
		"rebind_limit_time": 1,
	}
	body, _ := json.Marshal(payload)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/apps/update_register_config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST should not require app uuid, got status=%v body=%s", w.Code, w.Body.String())
	}
	var saveResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	json.Unmarshal(w.Body.Bytes(), &saveResp)
	if saveResp.Code != 0 {
		t.Fatalf("POST should succeed, got code=%v msg=%s", saveResp.Code, saveResp.Msg)
	}

	// settings 表里应有对应值。
	// 注意：每个断言都用**新的** struct —— GORM 的 First 会带上 struct 里已有的主键条件，
	// 复用同一个变量会叠加上一次的 id，导致后续查询恒为 not found。
	readSetting := func(key string) string {
		var s models.Settings
		if err := db.Where("name = ?", key).First(&s).Error; err != nil {
			t.Fatalf("%s not persisted: %v", key, err)
		}
		return s.Value
	}
	if v := readSetting(models.SettingAccountRegisterCount); v != "5" {
		t.Fatalf("register_count should be 5, got %q", v)
	}
	if v := readSetting(models.SettingAccountTrialPoints); v != "100" {
		t.Fatalf("trial_points should be 100, got %q", v)
	}
	if v := readSetting(models.SettingAccountTrialDurationMinutes); v != "1440" {
		t.Fatalf("trial_duration_minutes should be 1440, got %q", v)
	}
	if v := readSetting(models.SettingAccountCardLoginEnabled); v != "0" {
		t.Fatalf("card_login_enabled should be 0, got %q", v)
	}
	if v := readSetting(models.SettingAccountRechargeEnabled); v != "0" {
		t.Fatalf("recharge_enabled should be 0, got %q", v)
	}
	if v := readSetting(models.SettingAccountRegisterEnabled); v != "0" {
		t.Fatalf("register_enabled should be 0, got %q", v)
	}

	// ---------- 再读回来，应与写入一致 ----------
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/apps/get_register_config", nil)
	router.ServeHTTP(w, req)
	got = struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}{}
	json.Unmarshal(w.Body.Bytes(), &got)
	if v, _ := got.Data["register_count"].(float64); int(v) != 5 {
		t.Fatalf("read back register_count want 5, got %v", got.Data["register_count"])
	}
	if v, _ := got.Data["trial_points"].(float64); int(v) != 100 {
		t.Fatalf("read back trial_points want 100, got %v", got.Data["trial_points"])
	}
	if v, _ := got.Data["card_login_enabled"].(float64); int(v) != 0 {
		t.Fatalf("read back card_login_enabled want 0, got %v", got.Data["card_login_enabled"])
	}
}

// TestAccountPolicyRejectsZeroRegisterCount 注册次数必须 > 0，
// 否则同一 IP 注册过第一个账号后就再也注册不了。
func TestAccountPolicyRejectsZeroRegisterCount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(&models.Settings{})
	database.SetDB(db)
	t.Cleanup(func() { database.SetDB(nil) })

	router := gin.New()
	router.POST("/api/admin/apps/update_register_config", AppUpdateRegisterConfigHandler)

	body, _ := json.Marshal(map[string]any{"register_count": 0})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/apps/update_register_config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code == 0 {
		t.Fatalf("register_count=0 should be rejected")
	}
}
