package public

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/nacl/box"
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
		&models.Binding{}, &models.AppActivation{}, &models.API{}, &models.MemberLog{}, &models.MemberAccountToken{}, &models.AppActivation{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	db.Create(&models.App{
		UUID: "APP-1", Name: "测试", Secret: "SECRET", Status: 1,
	})
	// 启用卡密登录接口(type 10)，提交/返回均不加密（一号通：接口条目全站唯一）
	db.Create(&models.API{
		APIType: models.APITypeSingleLogin, Status: 1,
		SubmitAlgorithm: models.AlgorithmNone, ReturnAlgorithm: models.AlgorithmNone,
	})
	db.Create(&models.Card{CardNo: "KM-1", Duration: 60, Status: models.CardStatusUnused})
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

// TestOpenAPIDuplexBox 验证本次请求的临时私钥能解开成功结果和业务错误。
func TestOpenAPIDuplexBox(t *testing.T) {
	r := setupOpenAPITest(t)
	db, err := database.GetDB()
	if err != nil {
		t.Fatal(err)
	}
	server, sk, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.API{}).Where("api_type = ?", models.APITypeSingleLogin).Updates(map[string]any{
		"submit_algorithm": models.AlgorithmBox, "return_algorithm": models.AlgorithmBox,
		"submit_private_key": hex.EncodeToString(sk[:]), "submit_public_key": hex.EncodeToString(server[:]),
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		params string
		success bool
	}{
		{`{"card":"KM-1","machine_code":"MC-1","version":"1.0.0"}`, true},
		{`{}`, false},
	} {
		pub, priv, err := box.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		var nonce [24]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			t.Fatal(err)
		}
		data := base64.StdEncoding.EncodeToString(box.Seal(append(nonce[:], pub[:]...), []byte(tc.params), &nonce, server, priv))
		ts := time.Now().Unix()
		_, resp := doOpen(r, map[string]any{"app_uuid": "APP-1", "api_type": models.APITypeSingleLogin, "data": data, "timestamp": ts,
			"sign": services.SignOpenRequest("APP-1", models.APITypeSingleLogin, data, ts, "SECRET")})
		encoded, ok := resp["data"].(string)
		if !ok || len(resp) != 2 || resp["code"] != float64(0) {
			t.Fatal("未返回加密信封")
		}
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(raw) < 24+box.Overhead {
			t.Fatal("响应密文格式错误")
		}
		responseNonce := (*[24]byte)(raw[:24])
		plain, ok := box.Open(nil, raw[24:], responseNonce, server, priv)
		prefix := append([]byte("NetworkAuth/response/v1\x00"), nonce[:]...)
		var inner map[string]any
		if !ok || *responseNonce == nonce || !bytes.HasPrefix(plain, prefix) || json.Unmarshal(plain[len(prefix):], &inner) != nil {
			t.Fatal("响应认证或请求绑定错误")
		}
		if (inner["code"] == float64(0)) != tc.success || (tc.success && inner["data"] == nil) || (!tc.success && inner["msg"] == nil) {
			t.Fatal("内层业务状态错误")
		}
	}
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
		t.Fatalf("http status = %v", status)
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

	db, err := database.GetDB()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.API{}).Where("api_type = ?", models.APITypeSingleLogin).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	_, disabled := doOpen(r, map[string]any{
		"app_uuid": "APP-1", "api_type": models.APITypeSingleLogin,
		"data": data, "timestamp": ts, "sign": sign,
	})
	if disabled["msg"] != "接口已停用" || disabled["data"] != nil {
		t.Fatal("停用接口不应返回业务数据")
	}
}
