package openapi

import (
	"strings"

	"NetworkAuth/database"
	"NetworkAuth/models"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 函数（functions）
// ============================================================================
//
// 函数全站共用、别名全局唯一，在 goja 沙箱里执行（`44 执行远程函数`）。

type functionDTO struct {
	UUID      string `json:"uuid"`
	Number    string `json:"number"`
	Alias     string `json:"alias"`
	Code      string `json:"code"`
	Remark    string `json:"remark"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toFunctionDTO(f *models.Function) functionDTO {
	return functionDTO{
		UUID:      f.UUID,
		Number:    f.Number,
		Alias:     f.Alias,
		Code:      f.Code,
		Remark:    f.Remark,
		CreatedAt: fmtTime(f.CreatedAt),
		UpdatedAt: fmtTime(f.UpdatedAt),
	}
}

// ListFunctions GET /api/openapi/functions
// 查询参数：alias（精确）、search（别名模糊）、page、limit
func ListFunctions(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	page, limit := pageParams(c)

	q := db.Model(&models.Function{})
	if alias := strings.TrimSpace(c.Query("alias")); alias != "" {
		q = q.Where("alias = ?", alias)
	}
	if s := strings.TrimSpace(c.Query("search")); s != "" {
		q = q.Where("alias LIKE ?", "%"+s+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		internal(c, "统计函数失败", err)
		return
	}
	var rows []models.Function
	if err := q.Order("id ASC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error; err != nil {
		internal(c, "查询函数失败", err)
		return
	}
	list := make([]functionDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toFunctionDTO(&rows[i]))
	}
	ok(c, "success", gin.H{"list": list, "total": total, "page": page, "limit": limit})
}

// GetFunction GET /api/openapi/functions/:uuid
func GetFunction(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	var f models.Function
	if err := db.Where("uuid = ?", strings.TrimSpace(c.Param("uuid"))).First(&f).Error; err != nil {
		notFound(c, "函数不存在")
		return
	}
	ok(c, "success", toFunctionDTO(&f))
}

// CreateFunction POST /api/openapi/functions
// 请求：{ alias, code, remark? }
func CreateFunction(c *gin.Context) {
	var req struct {
		Alias  string `json:"alias"`
		Code   string `json:"code"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	alias := strings.TrimSpace(req.Alias)
	if alias == "" {
		badRequest(c, "函数别名不能为空")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		badRequest(c, "函数代码不能为空")
		return
	}
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	var dup int64
	if err := db.Model(&models.Function{}).Where("alias = ?", alias).Count(&dup).Error; err != nil {
		internal(c, "校验别名失败", err)
		return
	}
	if dup > 0 {
		badRequest(c, "已存在同名函数: "+alias)
		return
	}

	f := models.Function{Alias: alias, Code: req.Code, Remark: strings.TrimSpace(req.Remark)}
	if err := db.Create(&f).Error; err != nil {
		internal(c, "创建函数失败", err)
		return
	}
	logOp(c, "API创建函数", "创建了函数: "+f.Alias)
	ok(c, "创建成功", toFunctionDTO(&f))
}

// UpdateFunction PUT /api/openapi/functions/:uuid
// 请求：{ code?, remark? }（alias 不允许改）
func UpdateFunction(c *gin.Context) {
	var req struct {
		Code   *string `json:"code"`
		Remark *string `json:"remark"`
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
	var f models.Function
	if err := db.Where("uuid = ?", strings.TrimSpace(c.Param("uuid"))).First(&f).Error; err != nil {
		notFound(c, "函数不存在")
		return
	}
	updates := map[string]interface{}{}
	if req.Code != nil {
		updates["code"] = *req.Code
	}
	if req.Remark != nil {
		updates["remark"] = *req.Remark
	}
	if len(updates) == 0 {
		ok(c, "无改动", toFunctionDTO(&f))
		return
	}
	if err := db.Model(&f).Updates(updates).Error; err != nil {
		internal(c, "更新函数失败", err)
		return
	}
	if err := db.Where("uuid = ?", f.UUID).First(&f).Error; err != nil {
		internal(c, "读取函数失败", err)
		return
	}
	logOp(c, "API更新函数", "更新了函数: "+f.Alias)
	ok(c, "已保存", toFunctionDTO(&f))
}

// DeleteFunction DELETE /api/openapi/functions/:uuid
func DeleteFunction(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	var f models.Function
	if err := db.Where("uuid = ?", strings.TrimSpace(c.Param("uuid"))).First(&f).Error; err != nil {
		notFound(c, "函数不存在")
		return
	}
	if err := db.Delete(&f).Error; err != nil {
		internal(c, "删除函数失败", err)
		return
	}
	logOp(c, "API删除函数", "删除了函数: "+f.Alias)
	ok(c, "已删除", gin.H{"uuid": f.UUID})
}
