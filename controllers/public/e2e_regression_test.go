package public

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ============================================================================
// 全接口真实回归（HTTP 层）
//
// 目的：不走 service 直调，全部经 /api/open 信封（签名 → 分发 → 返回），
// 确保 30 个 api_type 的路由、参数解析、鉴权、错误码都是真实可用的。
//
// 覆盖形态：
//   1. 无需登录的公开接口（1/2/3/4）
//   2. 账号生命周期（21 注册 → 20 登录 → 40 到期 → 50 改密 → 30 登出）
//   3. 卡密（10 登录 → 22 充值）
//   4. 应用激活链（70 激活 → 41 心跳 → 42/43/44/45 → 54 → 53 → 71 释放 → 72 权益）
//   5. 转绑（51）与手动顶号（31）
//   6. 管理类接口（60/61/62）
//   7. 错误路径：签名错、app_uuid 错、未激活直接调应用级接口
// ============================================================================

type e2eEnv struct {
	r      *gin.Engine
	db     *gorm.DB
	acct   string // 账号令牌
	appTok string // 激活令牌
	cardNo string
}

// newE2EEnv 建库、植入应用/接口/变量/函数/卡，返回可直接发请求的环境。
func newE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.App{}, &models.Card{}, &models.CardPackage{}, &models.Member{}, &models.Binding{},
		&models.API{}, &models.MemberLog{}, &models.MemberAccountToken{}, &models.AppActivation{},
		&models.AppAccountBilling{}, &models.Variable{}, &models.Function{}, &models.Settings{},
		&models.MemberLevel{}, &models.AppLevelPoint{}, &models.OperationLog{}, &models.Blacklist{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	database.SetDB(db)
	services.ResetSettingsService()

	// 账号策略：开放注册/卡密登录/充值，试用发 1 点
	for k, v := range map[string]string{
		models.SettingAccountRegisterEnabled:       "1",
		models.SettingAccountCardLoginEnabled:      "1",
		models.SettingAccountRechargeEnabled:       "1",
		models.SettingAccountTrialEnabled:          "1",
		models.SettingAccountTrialPoints:           "1",
		models.SettingAccountTrialClaimMode:        "0",
		models.SettingAccountMaxDevices:            "5",
		models.SettingAccountMaxConcurrent:         "5",
		models.SettingAccountMachineRebindFreeCount: "5",
		models.SettingAccountMachineRebindLimitTime: "1",
		models.SettingAccountIPRebindFreeCount:      "5",
		models.SettingAccountIPRebindLimitTime:      "1",
		models.SettingAccountEmailVerifyEnabled:    "0",
		models.SettingAccountCardRegisterEnabled:   "0",
		models.SettingAccountRegisterLimitEnabled:  "0",
		models.SettingAccountRegisterDeviceLimitEnabled: "0",
		models.SettingAccountRegisterCount:         "10",
	} {
		db.Where("name = ?", k).Delete(&models.Settings{})
		db.Create(&models.Settings{Name: k, Value: v})
	}
	services.ResetSettingsService()

	// 点数模式应用：按次 1 点
	db.Create(&models.App{
		UUID: "APP-1", Name: "回归应用", Secret: "SECRET", Status: 1, Version: "1.0.0",
		BillingMode: models.BillingModePoints, PointsChargeMode: models.PointsChargePerCount,
		PointsPerLogin: 1, CheckInterval: 10, OfflineTimeout: 30,
		CleanInterval: 1, LeaseTTLMinutes: 30,
	})

	// 全部 30 个接口启用、不加密，保证路由可达
	for _, at := range []int{
		models.APITypeGetBulletin, models.APITypeGetUpdateUrl, models.APITypeCheckAppVersion, models.APITypeGetCardInfo,
		models.APITypeSingleLogin, models.APITypeUserLogin, models.APITypeUserRegin, models.APITypeUserRecharge,
		models.APITypeSendEmailCode, models.APITypeClaimTrial, models.APITypeSendResetCode, models.APITypeResetPassword,
		models.APITypeLogOut, models.APITypeKickSession, models.APITypeGetExpired, models.APITypeCheckUserStatus,
		models.APITypeGetAppData, models.APITypeGetVariable, models.APITypeExecuteFunction, models.APITypeGetUserData,
		models.APITypeUpdatePwd, models.APITypeMacChangeBind, models.APITypeDeductPoints, models.APITypeSetUserData,
		models.APITypeDisableUser, models.APITypeBlackUser, models.APITypeUserDeductedTime,
		models.APITypeAppActivate, models.APITypeAppDeactivate, models.APITypeEntitlements,
	} {
		db.Where("api_type = ?", at).Delete(&models.API{})
		db.Create(&models.API{
			APIType: at, Status: 1,
			SubmitAlgorithm: models.AlgorithmNone, ReturnAlgorithm: models.AlgorithmNone,
		})
	}

	// 变量与函数（供 43/44 使用）
	db.Create(&models.Variable{UUID: "v1", Number: "1700000000001", Alias: "server_url", Data: "https://example.test"})
	db.Create(&models.Function{UUID: "f1", Number: "1700000000002", Alias: "ping", Code: "return {ok:true}"})

	// 卡密：60 分钟 + 5 点
	db.Create(&models.Card{CardNo: "KM-E2E", Duration: 60, Points: 5, Status: models.CardStatusUnused, Price: 1000})

	r := gin.New()
	r.POST("/api/open", OpenAPIHandler)
	return &e2eEnv{r: r, db: db, cardNo: "KM-E2E"}
}

// call 发一次带签名的 /api/open 请求，返回 code、结果 JSON、原始响应体。
func (e *e2eEnv) call(t *testing.T, apiType int, payload map[string]any) (int, map[string]any, map[string]any) {
	t.Helper()
	data, _ := json.Marshal(payload)
	ts := time.Now().Unix()
	sign := services.SignOpenRequest("APP-1", apiType, string(data), ts, "SECRET")

	body, _ := json.Marshal(map[string]any{
		"app_uuid": "APP-1", "api_type": apiType,
		"data": string(data), "timestamp": ts, "sign": sign,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	code := -1
	if v, ok := resp["code"].(float64); ok {
		code = int(v)
	}
	// 不加密时返回的 data 是 JSON 字符串
	var result map[string]any
	if s, ok := resp["data"].(string); ok && s != "" {
		_ = json.Unmarshal([]byte(s), &result)
	}
	return code, result, resp
}

// mustCall 要求 code=0，否则带 msg 失败。
func (e *e2eEnv) mustCall(t *testing.T, apiType int, payload map[string]any, what string) map[string]any {
	t.Helper()
	code, result, resp := e.call(t, apiType, payload)
	if code != 0 {
		t.Fatalf("%s: code=%d msg=%v", what, code, resp["msg"])
	}
	return result
}

// TestE2EPublicEndpoints 公开接口：公告 / 更新地址 / 版本检测 / 卡密信息
func TestE2EPublicEndpoints(t *testing.T) {
	e := newE2EEnv(t)

	// 1 公告
	e.mustCall(t, models.APITypeGetBulletin, map[string]any{}, "1 获取公告")

	// 2 更新地址
	e.mustCall(t, models.APITypeGetUpdateUrl, map[string]any{}, "2 获取更新地址")

	// 3 版本检测
	e.mustCall(t, models.APITypeCheckAppVersion, map[string]any{"version": "0.9.0"}, "3 检测版本")

	// 4 卡密信息
	res := e.mustCall(t, models.APITypeGetCardInfo, map[string]any{"card": e.cardNo}, "4 卡密信息")
	if res["card_no"] == nil {
		t.Fatalf("4 卡密信息缺少 card_no 字段: %v", res)
	}
}

// TestE2EAccountLifecycle 账号全生命周期：注册 → 登录 → 到期 → 试用 → 改密 → 登出
func TestE2EAccountLifecycle(t *testing.T) {
	e := newE2EEnv(t)

	// 21 注册
	e.mustCall(t, models.APITypeUserRegin, map[string]any{
		"email": "e2e@test.com", "password": "pw123456", "machine_code": "MC-E2E",
	}, "21 注册")

	// 20 登录（只认证，不扣费）
	login := e.mustCall(t, models.APITypeUserLogin, map[string]any{
		"username": "e2e@test.com", "password": "pw123456", "machine_code": "MC-E2E", "version": "1.0.0",
	}, "20 登录")
	e.acct, _ = login["token"].(string)
	if e.acct == "" {
		t.Fatalf("20 登录未返回 token")
	}

	// 40 查到期（账号级接口，账号令牌可用）
	e.mustCall(t, models.APITypeGetExpired, map[string]any{"token": e.acct}, "40 查到期")

	// 24 领取试用（资源全空，可领）
	e.mustCall(t, models.APITypeClaimTrial, map[string]any{
		"username": "e2e@test.com", "password": "pw123456",
	}, "24 领取试用")

	// 50 改密
	e.mustCall(t, models.APITypeUpdatePwd, map[string]any{
		"token": e.acct, "old_password": "pw123456", "new_password": "pw654321",
	}, "50 改密")

	// 改密后旧令牌失效 → 41 应失败
	code, _, _ := e.call(t, models.APITypeCheckUserStatus, map[string]any{"token": e.acct})
	if code == 0 {
		t.Fatalf("50 改密后旧令牌不应继续可用")
	}

	// 新密码重新登录
	login2 := e.mustCall(t, models.APITypeUserLogin, map[string]any{
		"username": "e2e@test.com", "password": "pw654321", "machine_code": "MC-E2E", "version": "1.0.0",
	}, "20 重新登录")
	e.acct, _ = login2["token"].(string)

	// 30 登出
	e.mustCall(t, models.APITypeLogOut, map[string]any{"token": e.acct}, "30 登出")

	// 登出后令牌失效
	code, _, _ = e.call(t, models.APITypeGetExpired, map[string]any{"token": e.acct})
	if code == 0 {
		t.Fatalf("30 登出后令牌不应继续可用")
	}
}

// TestE2ECardAndRecharge 卡密登录 + 卡密充值
func TestE2ECardAndRecharge(t *testing.T) {
	e := newE2EEnv(t)

	// 10 卡密登录（首次自动建号 + 核销）
	login := e.mustCall(t, models.APITypeSingleLogin, map[string]any{
		"card": e.cardNo, "machine_code": "MC-CARD", "version": "1.0.0",
	}, "10 卡密登录")
	tok, _ := login["token"].(string)
	if tok == "" {
		t.Fatalf("10 卡密登录未返回 token")
	}

	// 22 用另一张卡充值
	e.db.Create(&models.Card{CardNo: "KM-RECHARGE", Duration: 30, Points: 3, Status: models.CardStatusUnused})
	e.mustCall(t, models.APITypeUserRecharge, map[string]any{
		"username": e.cardNo, "card": "KM-RECHARGE",
	}, "22 卡密充值")
}

// TestE2EActivationChain 应用激活全链路：70 → 41 → 41/42/43/44/45 → 54 → 53 → 71 → 72
func TestE2EActivationChain(t *testing.T) {
	e := newE2EEnv(t)

	// 先注册登录拿账号令牌
	e.mustCall(t, models.APITypeUserRegin, map[string]any{
		"email": "chain@test.com", "password": "pw123456", "machine_code": "MC-CHAIN",
	}, "21 注册")
	login := e.mustCall(t, models.APITypeUserLogin, map[string]any{
		"username": "chain@test.com", "password": "pw123456", "machine_code": "MC-CHAIN", "version": "1.0.0",
	}, "20 登录")
	acct, _ := login["token"].(string)

	// 未激活就调应用级接口 → 必须被拒（不兼容旧版隐式激活）
	for _, at := range []int{models.APITypeCheckUserStatus, models.APITypeGetAppData} {
		if code, _, resp := e.call(t, at, map[string]any{"token": acct}); code == 0 {
			t.Fatalf("type %d: 账号令牌不应能直接调应用级接口，msg=%v", at, resp["msg"])
		}
	}

	// 点数模式下激活要扣点：先充值
	e.db.Create(&models.Card{CardNo: "KM-BOOT", Points: 10, Status: models.CardStatusUnused})
	e.mustCall(t, models.APITypeUserRecharge, map[string]any{
		"username": "chain@test.com", "card": "KM-BOOT",
	}, "22 充点")

	// 70 激活
	act := e.mustCall(t, models.APITypeAppActivate, map[string]any{
		"token": acct, "fingerprint": "MC-CHAIN", "version": "1.0.0", "device_name": "回归机",
	}, "70 激活")
	appTok, _ := act["token"].(string)
	if appTok == "" {
		t.Fatalf("70 激活未返回激活令牌")
	}

	// 41 心跳
	e.mustCall(t, models.APITypeCheckUserStatus, map[string]any{"token": appTok}, "41 心跳")

	// 42 程序数据
	e.mustCall(t, models.APITypeGetAppData, map[string]any{"token": appTok}, "42 程序数据")

	// 43 变量
	e.mustCall(t, models.APITypeGetVariable, map[string]any{"token": appTok, "alias": "server_url"}, "43 变量")

	// 44 远程函数
	e.mustCall(t, models.APITypeExecuteFunction, map[string]any{"token": appTok, "alias": "ping"}, "44 远程函数")

	// 45 账号数据
	e.mustCall(t, models.APITypeGetUserData, map[string]any{"token": appTok}, "45 账号数据")

	// 54 设置账号数据
	e.mustCall(t, models.APITypeSetUserData, map[string]any{
		"token": appTok, "data": `{"k":"v"}`,
	}, "54 设置账号数据")

	// 53 功能扣点（余额来自注册默认 0，先用卡充点）
	e.db.Create(&models.Card{CardNo: "KM-PTS", Points: 10, Status: models.CardStatusUnused})
	e.mustCall(t, models.APITypeUserRecharge, map[string]any{
		"username": "chain@test.com", "card": "KM-PTS",
	}, "22 充点")
	e.mustCall(t, models.APITypeDeductPoints, map[string]any{"token": appTok, "points": 1}, "53 扣点")

	// 72 权益
	e.mustCall(t, models.APITypeEntitlements, map[string]any{"token": acct}, "72 权益")

	// 71 释放
	e.mustCall(t, models.APITypeAppDeactivate, map[string]any{"token": appTok}, "71 释放")

	// 释放后激活令牌失效
	if code, _, _ := e.call(t, models.APITypeCheckUserStatus, map[string]any{"token": appTok}); code == 0 {
		t.Fatalf("71 释放后激活令牌不应继续可用")
	}
}

// TestE2ERebindAndKick 转绑（51）与手动顶号（31）
func TestE2ERebindAndKick(t *testing.T) {
	e := newE2EEnv(t)

	login := e.mustCall(t, models.APITypeSingleLogin, map[string]any{
		"card": e.cardNo, "machine_code": "MC-RB1", "version": "1.0.0",
	}, "10 卡密登录")

	// 51 转绑（凭卡号鉴权，无需令牌）
	e.mustCall(t, models.APITypeMacChangeBind, map[string]any{
		"username": e.cardNo, "machine_code": "MC-RB2",
	}, "51 转绑")

	// 31 手动顶号：先激活一个实例，再用凭据踢掉它
	acct, _ := login["token"].(string)
	e.db.Create(&models.Card{CardNo: "KM-KICK", Points: 5, Status: models.CardStatusUnused})
	e.mustCall(t, models.APITypeUserRecharge, map[string]any{
		"username": e.cardNo, "card": "KM-KICK",
	}, "22 充点（顶号用）")
	_ = e.mustCall(t, models.APITypeAppActivate, map[string]any{
		"token": acct, "fingerprint": "MC-RB2", "version": "1.0.0",
	}, "70 激活")

	var row models.AppActivation
	if err := e.db.Where("fingerprint = ?", "MC-RB2").First(&row).Error; err != nil {
		t.Fatalf("load activation: %v", err)
	}
	e.mustCall(t, models.APITypeKickSession, map[string]any{
		"username": e.cardNo, "session_id": row.ID,
	}, "31 手动顶号")
}

// TestE2EAdminOps 管理类：60 封停 / 61 拉黑 / 62 扣时长
func TestE2EAdminOps(t *testing.T) {
	e := newE2EEnv(t)

	e.mustCall(t, models.APITypeUserRegin, map[string]any{
		"email": "adm@test.com", "password": "pw123456", "machine_code": "MC-ADM",
	}, "21 注册")
	e.mustCall(t, models.APITypeUserLogin, map[string]any{
		"username": "adm@test.com", "password": "pw123456", "machine_code": "MC-ADM", "version": "1.0.0",
	}, "20 登录")

	e.mustCall(t, models.APITypeDisableUser, map[string]any{
		"username": "adm@test.com",
	}, "60 封停")
	e.mustCall(t, models.APITypeBlackUser, map[string]any{
		"username": "adm@test.com",
	}, "61 拉黑")
	e.mustCall(t, models.APITypeUserDeductedTime, map[string]any{
		"username": "adm@test.com", "minutes": 10,
	}, "62 扣时长")
}

// TestE2EEmailCodeEndpoints 23/25 邮件验证码：未配置邮件服务时必须是明确的业务错误，而不是 panic 或 500
func TestE2EEmailCodeEndpoints(t *testing.T) {
	e := newE2EEnv(t)

	for _, at := range []int{models.APITypeSendEmailCode /* 23 */, models.APITypeSendResetCode /* 25 */} {
		code, _, resp := e.call(t, at, map[string]any{"email": "nobody@test.com"})
		if code == 0 {
			t.Fatalf("type %d: 未配置邮件服务不应成功", at)
		}
		msg, _ := resp["msg"].(string)
		if msg == "" {
			t.Fatalf("type %d: 失败时必须给出可读原因", at)
		}
	}
}

// TestE2EErrorPaths 错误路径：签名错 / app_uuid 不存在 / 未启用的接口
func TestE2EErrorPaths(t *testing.T) {
	e := newE2EEnv(t)

	data := `{"card":"` + e.cardNo + `"}`
	ts := time.Now().Unix()

	// 签名错
	body, _ := json.Marshal(map[string]any{
		"app_uuid": "APP-1", "api_type": models.APITypeGetBulletin,
		"data": data, "timestamp": ts, "sign": "WRONG",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["code"].(float64)) == 0 {
		t.Fatalf("签名错误不应放行")
	}

	// app_uuid 不存在
	ts2 := time.Now().Unix()
	sign2 := services.SignOpenRequest("NOPE", models.APITypeGetBulletin, data, ts2, "SECRET")
	body2, _ := json.Marshal(map[string]any{
		"app_uuid": "NOPE", "api_type": models.APITypeGetBulletin,
		"data": data, "timestamp": ts2, "sign": sign2,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	e.r.ServeHTTP(w2, req2)
	var resp2 map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if int(resp2["code"].(float64)) == 0 {
		t.Fatalf("不存在的应用不应放行")
	}

	// 接口被停用
	e.db.Model(&models.API{}).Where("api_type = ?", models.APITypeGetBulletin).Update("status", 0)
	code, _, _ := e.call(t, models.APITypeGetBulletin, map[string]any{})
	if code == 0 {
		t.Fatalf("停用的接口不应放行")
	}
	_ = fmt.Sprint()
}
