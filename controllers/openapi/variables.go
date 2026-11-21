package openapi

import (
	"strings"

	"NetworkAuth/database"
	"NetworkAuth/models"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 变量（variables）
// ============================================================================
//
// 变量可按应用隔离：app_uuid 为空表示全站变量；同一应用内别名唯一。

type variableDTO struct {
	UUID      string `json:"uuid"`
	Number    string `json:"number"`
	AppUUID   string `json:"app_uuid"`
	Alias     string `json:"alias"`
	Data      string `json:"data"`
	Remark    string `json:"remark"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toVariableDTO(v *models.Variable) variableDTO {
	return variableDTO{
		UUID:      v.UUID,
		Number:    v.Number,
		AppUUID:   v.AppUUID,
		Alias:     v.Alias,
		Data:      v.Data,
		Remark:    v.Remark,
		CreatedAt: fmtTime(v.CreatedAt),
		UpdatedAt: fmtTime(v.UpdatedAt),
	}
}

// ListVariables GET /api/openapi/variables
// 查询参数：app_uuid（global 或留空=不限）、alias、page、limit
func ListVariables(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	page, limit := pageParams(c)

	q := db.Model(&models.Variable{})
	switch v := strings.TrimSpace(c.Query("app_uuid")); v {
	case "":
	case "global", "0":
		q = q.Where("app_uuid = '' OR app_uuid IS NULL")
	default:
		q = q.Where("app_uuid = ?", v)
	}
	if alias := strings.TrimSpace(c.Query("alias")); alias != "" {
		q = q.Where("alias = ?", alias)
	}
	if s := strings.TrimSpace(c.Query("search")); s != "" {
		q = q.Where("alias LIKE ?", "%"+s+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		internal(c, "统计变量失败", err)
		return
	}
	var rows []models.Variable
	if err := q.Order("id ASC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error; err != nil {
		internal(c, "查询变量失败", err)
		return
	}
	list := make([]variableDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toVariableDTO(&rows[i]))
	}
	ok(c, "success", gin.H{"list": list, "total": total, "page": page, "limit": limit})
}

// GetVariable GET /api/openapi/variables/:uuid
func GetVariable(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	var v models.Variable
	if err := db.Where("uuid = ?", strings.TrimSpace(c.Param("uuid"))).First(&v).Error; err != nil {
		notFound(c, "变量不存在")
		return
	}
	ok(c, "success", toVariableDTO(&v))
}

// CreateVariable POST /api/openapi/variables
// 请求：{ alias, data, remark?, app_uuid? }
func CreateVariable(c *gin.Context) {
	var req struct {
		Alias   string `json:"alias"`
		Data    string `json:"data"`
		Remark  string `json:"remark"`
		AppUUID string `json:"app_uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	alias := strings.TrimSpace(req.Alias)
	if alias == "" {
		badRequest(c, "变量别名不能为空")
		return
	}
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}

	appUUID := strings.TrimSpace(req.AppUUID)
	if appUUID != "" {
		var cnt int64
		if err := db.Model(&models.App{}).Where("uuid = ?", appUUID).Count(&cnt).Error; err != nil {
			internal(c, "校验应用失败", err)
			return
		}
		if cnt == 0 {
			badRequest(c, "应用不存在: "+appUUID)
			return
		}
	}

	var dup int64
	if err := db.Model(&models.Variable{}).
		Where("alias = ? AND (app_uuid = ? OR (app_uuid IS NULL AND ? = ''))", alias, appUUID, appUUID).
		Count(&dup).Error; err != nil {
		internal(c, "校验别名失败", err)
		return
	}
	if dup > 0 {
		badRequest(c, "该应用下已存在同名变量: "+alias)
		return
	}

	v := models.Variable{
		AppUUID: appUUID,
		Alias:   alias,
		Data:    req.Data,
		Remark:  strings.TrimSpace(req.Remark),
	}
	if err := db.Create(&v).Error; err != nil {
		internal(c, "创建变量失败", err)
		return
	}
	logOp(c, "API创建变量", "创建了变量: "+v.Alias)
	ok(c, "创建成功", toVariableDTO(&v))
}

// UpdateVariable PUT /api/openapi/variables/:uuid
// 请求：{ data?, remark?, app_uuid? }（alias 不允许改，避免调用方代码失联）
func UpdateVariable(c *gin.Context) {
	var req struct {
		Data    *string `json:"data"`
		Remark  *string `json:"remark"`
		AppUUID *string `json:"app_uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	var v models.Variable
	if err := db.Where("uuid = ?", strings.TrimSpace(c.Param("uuid"))).First(&v).Error; err != nil {
		notFound(c, "变量不存在")
		return
	}

	updates := map[string]interface{}{}
	if req.Data != nil {
		updates["data"] = *req.Data
	}
	if req.Remark != nil {
		updates["remark"] = *req.Remark
	}
	if req.AppUUID != nil {
		target := strings.TrimSpace(*req.AppUUID)
		if target != "" {
			var cnt int64
			if err := db.Model(&models.App{}).Where("uuid = ?", target).Count(&cnt).Error; err != nil {
				internal(c, "校验应用失败", err)
				return
			}
			if cnt == 0 {
				badRequest(c, "应用不存在: "+target)
				return
			}
		}
		updates["app_uuid"] = target
	}
	if len(updates) == 0 {
		ok(c, "无改动", toVariableDTO(&v))
		return
	}
	if err := db.Model(&v).Updates(updates).Error; err != nil {
		internal(c, "更新变量失败", err)
		return
	}
	if err := db.Where("uuid = ?", v.UUID).First(&v).Error; err != nil {
		internal(c, "读取变量失败", err)
		return
	}
	logOp(c, "API更新变量", "更新了变量: "+v.Alias)
	ok(c, "已保存", toVariableDTO(&v))
}

// DeleteVariable DELETE /api/openapi/variables/:uuid
func DeleteVariable(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	var v models.Variable
	if err := db.Where("uuid = ?", strings.TrimSpace(c.Param("uuid"))).First(&v).Error; err != nil {
		notFound(c, "变量不存在")
		return
	}
	if err := db.Delete(&v).Error; err != nil {
		internal(c, "删除变量失败", err)
		return
	}
	logOp(c, "API删除变量", "删除了变量: "+v.Alias)
	ok(c, "已删除", gin.H{"uuid": v.UUID})
}
