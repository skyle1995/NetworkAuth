package openapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"NetworkAuth/middleware"
	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 对外 API 公共设施
// ============================================================================
//
// 这一层供脚本/第三方系统调用，用 API 密钥（X-API-Key）鉴权，不是后台 JWT。
// 响应结构与后台接口保持一致：{ code, msg, data }，code=0 表示成功。

// ok 成功响应
func ok(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": msg, "data": data})
}

// fail 失败响应（status 同时作为 code，便于调用方按 HTTP 状态判断）
func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": status, "msg": msg})
}

func badRequest(c *gin.Context, msg string) { fail(c, http.StatusBadRequest, msg) }
func notFound(c *gin.Context, msg string)   { fail(c, http.StatusNotFound, msg) }
func internal(c *gin.Context, msg string, err error) {
	fail(c, http.StatusInternalServerError, msg+": "+err.Error())
}

// operator 取当前密钥的标识，写操作日志用
func operator(c *gin.Context) string {
	if k := middleware.CurrentAPIKey(c); k != nil {
		if strings.TrimSpace(k.Name) != "" {
			return "apikey:" + k.Name
		}
		return "apikey#" + strconv.FormatUint(k.ID, 10)
	}
	return "apikey"
}

// logOp 记录操作日志（后台日志页可见，操作者标记为 apikey:名称）
func logOp(c *gin.Context, action, detail string) {
	services.RecordOperationLog(action, operator(c), "", detail)
}

// pageParams 解析分页参数（默认 1/20，上限 200）
func pageParams(c *gin.Context) (page, limit int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	return page, limit
}

// fmtTime 时间格式统一为后台一致的可读格式。
// 入参支持 time.Time / *time.Time（nil 指针返回空串，避免对 nil 调 Format 崩溃）。
func fmtTime(v any) string {
	var t time.Time
	switch x := v.(type) {
	case time.Time:
		t = x
	case *time.Time:
		if x == nil {
			return ""
		}
		t = *x
	case nil:
		return ""
	default:
		return ""
	}
	if t.IsZero() || t.Year() == 1 {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// intFlag 把 bool 指针/值转成 0/1，便于请求体里既收 true 也收 1
func intFlag(v *bool, fallback int) int {
	if v == nil {
		return fallback
	}
	if *v {
		return 1
	}
	return 0
}

// formatFloat 去掉多余小数位，日志里展示数字用
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
// WhoAmI GET /api/openapi/me
// 返回当前密钥的名称与能力，供调用方自检是否配置正确。
func WhoAmI(c *gin.Context) {
	k := middleware.CurrentAPIKey(c)
	if k == nil {
		fail(c, http.StatusUnauthorized, "缺少 API 密钥")
		return
	}
	scopes := make([]string, 0, 4)
	for _, s := range strings.Split(k.Scopes, ",") {
		if v := strings.TrimSpace(s); v != "" {
			scopes = append(scopes, v)
		}
	}
	ok(c, "success", gin.H{
		"name":     k.Name,
		"scopes":   scopes,
		"expire_at": fmtTime(k.ExpireAt),
	})
}