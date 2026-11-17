package public

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"NetworkAuth/models"
	"NetworkAuth/services"
)

// TestSecurityDeactivateAudience 验证跨应用心跳、释放与登出被拒绝，正确应用释放保持幂等。
func TestSecurityDeactivateAudience(t *testing.T) {
	e := newE2EEnv(t)
	m := models.Member{Username: "security-audience", Status: models.MemberStatusNormal, Points: 10}
	if err := e.db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	acct, err := services.IssueAccountToken(e.db, m.UUID, "FP", "127.0.0.1", "1", "test")
	if err != nil {
		t.Fatal(err)
	}
	act, err := services.ActivateByToken("APP-1", acct.Token, "FP", "127.0.0.1", "1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.db.Create(&models.App{UUID: "APP-2", Secret: "TEST-SECRET", Status: 1}).Error; err != nil {
		t.Fatal(err)
	}
	for _, apiType := range []int{30, 41, 71} {
		data, _ := json.Marshal(map[string]any{"token": act.Token})
		ts := time.Now().Unix()
		body, _ := json.Marshal(map[string]any{
			"app_uuid": "APP-2", "api_type": apiType, "data": string(data), "timestamp": ts,
			"sign": services.SignOpenRequest("APP-2", apiType, string(data), ts, "TEST-SECRET"),
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		e.r.ServeHTTP(w, req)
		var resp struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Code == 0 {
			t.Fatalf("跨应用操作被放行：type=%d", apiType)
		}
	}
	if _, _, err := services.AuthActivation(e.db, act.Token); err != nil {
		t.Fatalf("拒绝跨应用操作后原激活必须保持有效：%v", err)
	}
	if code, _, _ := e.call(t, 71, map[string]any{"token": acct.Token}); code == 0 {
		t.Fatal("释放应用接口不得接受账号令牌")
	}
	for i := 0; i < 2; i++ {
		e.mustCall(t, 71, map[string]any{"token": act.Token}, "正确应用幂等释放")
	}
}

// TestSecurityLogoutDeviceScope 验证真实 HTTP 登出只撤销当前登录名下的激活。
func TestSecurityLogoutDeviceScope(t *testing.T) {
	e := newE2EEnv(t)
	m := models.Member{Username: "security-logout", Status: models.MemberStatusNormal, Points: 10}
	if err := e.db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	a, err := services.IssueAccountToken(e.db, m.UUID, "FP-A", "127.0.0.1", "1", "A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := services.IssueAccountToken(e.db, m.UUID, "FP-B", "127.0.0.1", "1", "B")
	if err != nil {
		t.Fatal(err)
	}
	actA, err := services.ActivateByToken("APP-1", a.Token, "FP-A", "127.0.0.1", "1", "A")
	if err != nil {
		t.Fatal(err)
	}
	actB, err := services.ActivateByToken("APP-1", b.Token, "FP-B", "127.0.0.1", "1", "B")
	if err != nil {
		t.Fatal(err)
	}
	e.mustCall(t, 30, map[string]any{"token": a.Token}, "单设备登出")
	if _, err := services.AuthAccountToken(e.db, a.Token); err == nil {
		t.Fatal("当前账号令牌未吊销")
	}
	if _, _, err := services.AuthActivation(e.db, actA.Token); err == nil {
		t.Fatal("当前登录的激活未吊销")
	}
	if _, err := services.AuthAccountToken(e.db, b.Token); err != nil {
		t.Fatalf("其他设备账号令牌受影响：%v", err)
	}
	if _, _, err := services.AuthActivation(e.db, actB.Token); err != nil {
		t.Fatalf("其他设备激活受影响：%v", err)
	}
}

// TestSecurityPackageKeyScope 验证包密钥禁止全局回落，普通全局变量仍可读取。
func TestSecurityPackageKeyScope(t *testing.T) {
	e := newE2EEnv(t)
	m := models.Member{Username: "security-variable", Status: models.MemberStatusNormal, Points: 10}
	if err := e.db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	acct, err := services.IssueAccountToken(e.db, m.UUID, "FP", "127.0.0.1", "1", "test")
	if err != nil {
		t.Fatal(err)
	}
	act, err := services.ActivateByToken("APP-1", acct.Token, "FP", "127.0.0.1", "1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.db.Create(&models.Variable{UUID: "security-key", Number: "9900000000001", Alias: "zex_key", Data: "TEST-NONSECRET"}).Error; err != nil {
		t.Fatal(err)
	}
	if code, _, _ := e.call(t, 43, map[string]any{"token": act.Token, "alias": "zex_key"}); code == 0 {
		t.Fatal("全局包密钥不应被返回")
	}
	e.mustCall(t, 43, map[string]any{"token": act.Token, "alias": "server_url"}, "普通全局变量")
}

// TestSecurityResetParameter 验证管理器实际请求字段到达验证码校验，而非缺少新密码。
func TestSecurityResetParameter(t *testing.T) {
	e := newE2EEnv(t)
	code, _, resp := e.call(t, 26, map[string]any{
		"email": "security@example.test", "code": "000000", "new_password": "test-password",
	})
	if code == 0 || resp["msg"] == "新密码不能为空" {
		t.Fatalf("找回密码字段契约异常：code=%d msg=%v", code, resp["msg"])
	}
}
