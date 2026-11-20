package admin

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestExportBoxPublicKeyOnly 验证导出不依赖应用表，包含 Box 公钥但不含私钥或应用信息。
func TestExportBoxPublicKeyOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.API{}, &models.OperationLog{}); err != nil {
		t.Fatal(err)
	}
	prev, _ := database.GetDB()
	database.SetDB(db)
	t.Cleanup(func() { database.SetDB(prev) })
	pub, priv, err := services.GenerateBoxKeys()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.API{APIType: 1, SubmitAlgorithm: models.AlgorithmBox, SubmitPublicKey: pub, SubmitPrivateKey: priv}).Error; err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	APIExportKeysHandler(c)
	if bytes.Contains(w.Body.Bytes(), []byte(`"apps"`)) || bytes.Contains(w.Body.Bytes(), []byte(`"app_count"`)) {
		t.Fatal("接口导出不应包含应用列表或数量")
	}
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(pub)) || bytes.Contains(w.Body.Bytes(), []byte(priv)) {
		t.Fatal("导出未包含公钥，或泄露了服务端私钥")
	}
}

// TestAPIListSupportedTypes 验证旧记录不占列表名额，当前接口仍返回正确名称。
func TestAPIListSupportedTypes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.API{}); err != nil {
		t.Fatal(err)
	}
	prev, _ := database.GetDB()
	database.SetDB(db)
	t.Cleanup(func() { database.SetDB(prev) })
	for _, n := range []int{10, 61, 62, 80} {
		if err := db.Create(&models.API{APIType: n}).Error; err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/admin/apis/list", nil)
	APIListHandler(c)
	var resp struct {
		Count int
		Data []struct {
			Type int `json:"api_type"`
			Name string `json:"api_type_name"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	expected := map[int]string{10: "卡密登录", 61: "添加黑名单", 62: "扣除时间"}
	if resp.Count != len(expected) || len(resp.Data) != len(expected) {
		t.Fatalf("应返回 3 个有效接口，实际总数 %d，本页 %d", resp.Count, len(resp.Data))
	}
	for _, row := range resp.Data {
		if name, ok := expected[row.Type]; !ok || name != row.Name {
			t.Fatalf("接口编号 %d 的名称不正确：%s", row.Type, row.Name)
		}
		delete(expected, row.Type)
	}
}
