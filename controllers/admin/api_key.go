package admin

import (
	"strconv"
	"strings"
	"time"

	"NetworkAuth/models"
	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
)

// parseAPIKeyExpire 解析过期时间字符串（空=永久，返回 nil）。
func parseAPIKeyExpire(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return &t
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return &t
	}
	return nil
}

// recordAPIKeyOperation 记录 API 密钥相关操作日志，统一取当前管理员身份。
func recordAPIKeyOperation(c *gin.Context, logType, message string) {
	operator := c.GetString("admin_username")
	operatorUUID := c.GetString("admin_uuid")
	if operator == "" {
		operator = "system"
	}
	services.RecordOperationLog(logType, operator, operatorUUID, message)
}

// GetApiKeyScopes 返回所有可分配的能力（供后台选择）。
func GetApiKeyScopes(c *gin.Context) {
	authBaseController.HandleSuccess(c, "ok", gin.H{"list": services.AvailableAPIScopes()})
}

// GetApiKeyList 密钥列表（可按名称模糊搜索）。
func GetApiKeyList(c *gin.Context) {
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}
	q := db.Model(&models.ApiKey{})
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		q = q.Where("name LIKE ?", "%"+kw+"%")
	}
	var list []models.ApiKey
	q.Order("id desc").Find(&list)
	authBaseController.HandleSuccess(c, "ok", gin.H{"list": list})
}

// CreateApiKey 新建密钥（密钥串由服务端生成）。
func CreateApiKey(c *gin.Context) {
	var req struct {
		Name     string   `json:"name"`
		Scopes   []string `json:"scopes"`
		ExpireAt string   `json:"expire_at"`
	}
	if !authBaseController.BindJSON(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		authBaseController.HandleValidationError(c, "请填写用途名称")
		return
	}
	if len(req.Scopes) == 0 {
		authBaseController.HandleValidationError(c, "请至少选择一项能力")
		return
	}
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}
	key := models.ApiKey{
		Name:     req.Name,
		Key:      services.GenerateAPIKeyString(),
		Scopes:   strings.Join(req.Scopes, ","),
		Status:   1,
		ExpireAt: parseAPIKeyExpire(req.ExpireAt),
	}
	if err := db.Create(&key).Error; err != nil {
		authBaseController.HandleInternalError(c, "创建失败", err)
		return
	}
	recordAPIKeyOperation(c, "新建API密钥", "新建密钥："+key.Name)
	authBaseController.HandleSuccess(c, "创建成功", key)
}

// UpdateApiKey 编辑密钥（名称/能力/状态/过期；不改密钥串）。
func UpdateApiKey(c *gin.Context) {
	var req struct {
		ID       uint64   `json:"id"`
		Name     string   `json:"name"`
		Scopes   []string `json:"scopes"`
		Status   int      `json:"status"`
		ExpireAt string   `json:"expire_at"`
	}
	if !authBaseController.BindJSON(c, &req) {
		return
	}
	if req.ID == 0 {
		authBaseController.HandleValidationError(c, "密钥ID不能为空")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		authBaseController.HandleValidationError(c, "请填写用途名称")
		return
	}
	if len(req.Scopes) == 0 {
		authBaseController.HandleValidationError(c, "请至少选择一项能力")
		return
	}
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}
	if err := db.Model(&models.ApiKey{}).Where("id = ?", req.ID).Updates(map[string]interface{}{
		"name":      strings.TrimSpace(req.Name),
		"scopes":    strings.Join(req.Scopes, ","),
		"status":    req.Status,
		"expire_at": parseAPIKeyExpire(req.ExpireAt),
	}).Error; err != nil {
		authBaseController.HandleInternalError(c, "更新失败", err)
		return
	}
	recordAPIKeyOperation(c, "编辑API密钥", "编辑密钥 ID："+strconv.FormatUint(req.ID, 10))
	authBaseController.HandleSuccess(c, "更新成功", nil)
}

// RegenerateApiKey 重置密钥串（吊销旧串，生成新串）。
func RegenerateApiKey(c *gin.Context) {
	var req struct {
		ID uint64 `json:"id"`
	}
	if !authBaseController.BindJSON(c, &req) {
		return
	}
	if req.ID == 0 {
		authBaseController.HandleValidationError(c, "密钥ID不能为空")
		return
	}
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}
	newKey := services.GenerateAPIKeyString()
	if err := db.Model(&models.ApiKey{}).Where("id = ?", req.ID).Update("key", newKey).Error; err != nil {
		authBaseController.HandleInternalError(c, "重置失败", err)
		return
	}
	recordAPIKeyOperation(c, "重置API密钥", "重置密钥 ID："+strconv.FormatUint(req.ID, 10))
	authBaseController.HandleSuccess(c, "已重置", gin.H{"key": newKey})
}

// DeleteApiKey 删除密钥。
func DeleteApiKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		authBaseController.HandleValidationError(c, "ID 格式不正确")
		return
	}
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}
	db.Delete(&models.ApiKey{}, id)
	recordAPIKeyOperation(c, "删除API密钥", "删除密钥 ID："+c.Param("id"))
	authBaseController.HandleSuccess(c, "删除成功", nil)
}
