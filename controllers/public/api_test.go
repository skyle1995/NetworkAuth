package public

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupOpenAPITest 建库并植入应用、启用的卡密登录接口(不加密)、一张卡，返回 gin 引擎。
func setupOpenAPITest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.App{}, &models.Card{}, &models.Member{},
		&models.Binding{}, &models.MemberSession{}, &models.API{}, &models.MemberLog{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	db.Create(&models.App{
		UUID: "APP-1", Name: "测试", Secret: "SECRET", Status: 1,
		CardLoginEnabled: 1, MultiOpenCount: 1,
	})
	// 启用卡密登录接口(type 10)，提交/返回均不加密
	db.Create(&models.API{
		AppUUID: "APP-1", APIType: models.APITypeSingleLogin, Status: 1,
		SubmitAlgorithm: models.AlgorithmNone, ReturnAlgorithm: models.AlgorithmNone,
	})
	db.Create(&models.Card{CardNo: "KM-1", AppUUID: "APP-1", Duration: 60, Status: models.CardStatusUnused})
	database.SetDB(db)

	r := gin.New()
	r.POST("/api/open", OpenAPIHandler)
	return r
}

// doOpen 发起一次 /api/open 请求并解析响应
func doOpen(r *gin.Engine, body map[string]any) (int, map[string]any) {
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp
}

func TestOpenAPIEnvelopeCardLogin(t *testing.T) {
	r := setupOpenAPITest(t)

	data := `{"card":"KM-1","machine_code":"MC-1","version":"1.0.0"}`
	ts := time.Now().Unix()
	sign := services.SignOpenRequest("APP-1", models.APITypeSingleLogin, data, ts, "SECRET")

	// 正常：签名 + 不加密载荷 → 分发到卡密登录 → 返回 token
	status, resp := doOpen(r, map[string]any{
		"app_uuid": "APP-1", "api_type": models.APITypeSingleLogin,
		"data": data, "timestamp": ts, "sign": sign,
	})
	if status != http.StatusOK {
		t.Fatalf("http status = %d", status)
	}
	if resp["code"].(float64) != 0 {
		t.Fatalf("expected code 0, got %v (msg=%v)", resp["code"], resp["msg"])
	}
	// 不加密时 data 为结果 JSON 文本，解析应含 token
	var result map[string]any
	if err := json.Unmarshal([]byte(resp["data"].(string)), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result["token"] == nil || result["token"] == "" {
		t.Fatalf("login result missing token: %v", result)
	}

	// 错误签名 → code 1
	_, bad := doOpen(r, map[string]any{
		"app_uuid": "APP-1", "api_type": models.APITypeSingleLogin,
		"data": data, "timestamp": ts, "sign": "WRONGSIGN",
	})
	if bad["code"].(float64) != 1 {
		t.Fatalf("bad sign should return code 1")
	}

	// 未启用/不存在的接口 → code 1（type 99 未配置）
	ts2 := time.Now().Unix()
	sign2 := services.SignOpenRequest("APP-1", 99, data, ts2, "SECRET")
	_, notcfg := doOpen(r, map[string]any{
		"app_uuid": "APP-1", "api_type": 99,
		"data": data, "timestamp": ts2, "sign": sign2,
	})
	if notcfg["code"].(float64) != 1 {
		t.Fatalf("unconfigured interface should return code 1")
	}
}
