package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/models"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 全局变量
// ============================================================================

// 创建基础控制器实例
var variableBaseController = controllers.NewBaseController()

// ============================================================================
// 页面处理器
// ============================================================================

// VariableFragmentHandler 公共变量列表页面片段处理器
func VariableFragmentHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "variables.html", gin.H{
		"Title": "公共变量",
	})
}

// ============================================================================
// API处理器
// ============================================================================

// VariableListHandler 变量列表API处理器
func VariableListHandler(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	// 兼容前端使用的page_size参数
	if limit <= 0 {
		limit, _ = strconv.Atoi(c.Query("page_size"))
	}
	if limit <= 0 {
		limit = 10
	}

	// 获取搜索关键词参数（支持编号、别名、数据的综合搜索）
	search := strings.TrimSpace(c.Query("search"))

	// 兼容旧的别名搜索参数
	if search == "" {
		search = strings.TrimSpace(c.Query("alias"))
	}

	// 获取应用筛选参数
	appUUID := strings.TrimSpace(c.Query("app_uuid"))

	// 构建查询
	db, ok := variableBaseController.GetDB(c)
	if !ok {
		return
	}

	// 构建基础查询
	query := db.Model(&models.Variable{})

	// 如果指定了搜索关键词，则在编号、别名、数据、备注中进行精确搜索
	if search != "" {
		// 优化：使用精确匹配提升查询性能
		query = query.Where("number = ? OR alias = ? OR data = ? OR remark = ?",
			search, search, search, search)
	}

	// 如果指定了应用筛选，则按应用UUID筛选
	if appUUID != "" {
		query = query.Where("app_uuid = ?", appUUID)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		logrus.WithError(err).Error("Failed to count variables")
		variableBaseController.HandleInternalError(c, "查询变量总数失败", err)
		return
	}

	// 获取分页数据
	var variables []models.Variable
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&variables).Error; err != nil {
		logrus.WithError(err).Error("Failed to fetch variables")
		variableBaseController.HandleInternalError(c, "查询变量列表失败", err)
		return
	}

	// 构建响应数据
	type VariableResponse struct {
		ID        uint   `json:"id"`
		UUID      string `json:"uuid"`
		Number    string `json:"number"`
		AppUUID   string `json:"app_uuid"`
		Alias     string `json:"alias"`
		Data      string `json:"data"`
		Remark    string `json:"remark"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	var responseData []VariableResponse
	for _, variable := range variables {
		responseData = append(responseData, VariableResponse{
			ID:        variable.ID,
			UUID:      variable.UUID,
			Number:    variable.Number,
			AppUUID:   variable.AppUUID,
			Alias:     variable.Alias,
			Data:      variable.Data,
			Remark:    variable.Remark,
			CreatedAt: variable.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: variable.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	response := gin.H{
		"code":  0,
		"msg":   "success",
		"count": total,
		"data":  responseData,
	}

	c.JSON(http.StatusOK, response)
}

// VariableCreateHandler 新增变量API处理器
func VariableCreateHandler(c *gin.Context) {
	var req struct {
		Alias   string `json:"alias"`
		AppUUID string `json:"app_uuid"`
		Data    string `json:"data"`
		Remark  string `json:"remark"`
	}

	if !variableBaseController.BindJSON(c, &req) {
		return
	}

	// 验证必填字段
	if !variableBaseController.ValidateRequired(c, map[string]interface{}{
		"变量别名": req.Alias,
	}) {
		return
	}

	// 验证别名格式：必须以英文字母开头，只能包含数字和英文字母
	aliasPattern := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`)
	if !aliasPattern.MatchString(req.Alias) {
		variableBaseController.HandleValidationError(c, "别名必须以英文字母开头，只能包含数字和英文字母")
		return
	}

	db, ok := variableBaseController.GetDB(c)
	if !ok {
		return
	}

	// 处理应用UUID：如果为空或"0"，设置为"0"（全局变量）
	updateAppUUID := strings.TrimSpace(req.AppUUID)
	if updateAppUUID == "" {
		updateAppUUID = "0"
	}

	// 如果指定了应用UUID且不是"0"，验证应用是否存在
	if updateAppUUID != "0" {
		var appCount int64
		if err := db.Model(&models.App{}).Where("uuid = ?", updateAppUUID).Count(&appCount).Error; err != nil {
			logrus.WithError(err).Error("Failed to check app existence")
			variableBaseController.HandleInternalError(c, "验证应用失败", err)
			return
		}
		if appCount == 0 {
			variableBaseController.HandleValidationError(c, "指定的应用不存在")
			return
		}
	}

	// 处理应用UUID：如果为空或"0"，设置为"0"（全局变量）
	appUUID := strings.TrimSpace(req.AppUUID)
	if appUUID == "" {
		appUUID = "0"
	}

	// 如果指定了应用UUID且不是"0"，验证应用是否存在
	if appUUID != "0" {
		var appCount int64
		if err := db.Model(&models.App{}).Where("uuid = ?", appUUID).Count(&appCount).Error; err != nil {
			logrus.WithError(err).Error("Failed to check app existence")
			variableBaseController.HandleInternalError(c, "验证应用失败", err)
			return
		}
		if appCount == 0 {
			variableBaseController.HandleValidationError(c, "指定的应用不存在")
			return
		}
	}

	// 创建变量
	variable := models.Variable{
		Alias:   strings.TrimSpace(req.Alias),
		AppUUID: appUUID,
		Data:    req.Data,
		Remark:  strings.TrimSpace(req.Remark),
	}

	if err := db.Create(&variable).Error; err != nil {
		logrus.WithError(err).Error("Failed to create variable")
		variableBaseController.HandleInternalError(c, "创建变量失败", err)
		return
	}

	variableBaseController.HandleSuccess(c, "创建成功", variable)
}

// VariableUpdateHandler 更新变量API处理器
func VariableUpdateHandler(c *gin.Context) {
	var req struct {
		UUID    string `json:"uuid"`
		AppUUID string `json:"app_uuid"`
		Data    string `json:"data"`
		Remark  string `json:"remark"`
	}

	if !variableBaseController.BindJSON(c, &req) {
		return
	}

	// 验证必填字段（移除对alias的验证，因为编辑时不允许修改别名）
	if !variableBaseController.ValidateRequired(c, map[string]interface{}{
		"变量UUID": req.UUID,
	}) {
		return
	}

	db, ok := variableBaseController.GetDB(c)
	if !ok {
		return
	}

	// 处理应用UUID：如果为空或"0"，设置为"0"（全局变量）
	updateAppUUID := strings.TrimSpace(req.AppUUID)
	if updateAppUUID == "" {
		updateAppUUID = "0"
	}

	// 如果指定了应用UUID且不是"0"，验证应用是否存在
	if updateAppUUID != "0" {
		var appCount int64
		if err := db.Model(&models.App{}).Where("uuid = ?", updateAppUUID).Count(&appCount).Error; err != nil {
			logrus.WithError(err).Error("Failed to check app existence")
			variableBaseController.HandleInternalError(c, "验证应用失败", err)
			return
		}
		if appCount == 0 {
			variableBaseController.HandleValidationError(c, "指定的应用不存在")
			return
		}
	}

	// 通过uuid字段查找变量
	var variable models.Variable
	if err := db.Where("uuid = ?", strings.TrimSpace(req.UUID)).First(&variable).Error; err != nil {
		variableBaseController.HandleValidationError(c, "变量不存在")
		return
	}

	// 更新字段（不更新alias，保持原有别名不变）
	variable.AppUUID = updateAppUUID
	variable.Data = req.Data
	variable.Remark = strings.TrimSpace(req.Remark)

	if err := db.Save(&variable).Error; err != nil {
		logrus.WithError(err).Error("Failed to update variable")
		variableBaseController.HandleInternalError(c, "更新变量失败", err)
		return
	}

	variableBaseController.HandleSuccess(c, "更新成功", variable)
}

// VariableDeleteHandler 删除变量API处理器
func VariableDeleteHandler(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}

	if !variableBaseController.BindJSON(c, &req) {
		return
	}

	if req.ID == 0 {
		variableBaseController.HandleValidationError(c, "变量ID不能为空")
		return
	}

	db, ok := variableBaseController.GetDB(c)
	if !ok {
		return
	}

	// 删除变量
	if err := db.Delete(&models.Variable{}, req.ID).Error; err != nil {
		logrus.WithError(err).Error("Failed to delete variable")
		variableBaseController.HandleInternalError(c, "删除变量失败", err)
		return
	}

	logrus.WithField("variable_id", req.ID).Debug("Successfully deleted variable")

	variableBaseController.HandleSuccess(c, "删除成功", nil)
}

// VariablesBatchDeleteHandler 批量删除变量API处理器
func VariablesBatchDeleteHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}

	if !variableBaseController.BindJSON(c, &req) {
		return
	}

	if len(req.IDs) == 0 {
		variableBaseController.HandleValidationError(c, "请选择要删除的变量")
		return
	}

	db, ok := variableBaseController.GetDB(c)
	if !ok {
		return
	}

	// 批量删除变量
	if err := db.Delete(&models.Variable{}, req.IDs).Error; err != nil {
		logrus.WithError(err).Error("Failed to batch delete variables")
		variableBaseController.HandleInternalError(c, "批量删除失败", err)
		return
	}

	logrus.WithField("variable_ids", req.IDs).Debug("Successfully batch deleted variables")

	variableBaseController.HandleSuccess(c, "批量删除成功", nil)
}
