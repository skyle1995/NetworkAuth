package admin

import (
	"net/http"
	"networkDev/controllers"
	"networkDev/models"
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
var functionBaseController = controllers.NewBaseController()

// ============================================================================
// 页面处理器
// ============================================================================

// FunctionFragmentHandler 公共函数列表页面片段处理器
func FunctionFragmentHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "functions.html", gin.H{
		"Title": "公共函数",
	})
}

// ============================================================================
// API处理器
// ============================================================================

// FunctionListHandler 函数列表API处理器
func FunctionListHandler(c *gin.Context) {
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

	// 获取搜索关键词参数（支持编号、别名、代码的综合搜索）
	search := strings.TrimSpace(c.Query("search"))

	// 兼容旧的别名搜索参数
	if search == "" {
		search = strings.TrimSpace(c.Query("alias"))
	}

	// 获取应用筛选参数
	appUUID := strings.TrimSpace(c.Query("app_uuid"))

	// 构建查询
	db, ok := functionBaseController.GetDB(c)
	if !ok {
		return
	}

	// 构建基础查询
	query := db.Model(&models.Function{})

	// 如果指定了搜索关键词，则在编号、别名、代码、备注中进行精确搜索
	if search != "" {
		// 优化：使用精确匹配提升查询性能
		query = query.Where("number = ? OR alias = ? OR code = ? OR remark = ?",
			search, search, search, search)
	}

	// 如果指定了应用筛选，则按应用UUID筛选
	if appUUID != "" {
		query = query.Where("app_uuid = ?", appUUID)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		logrus.WithError(err).Error("Failed to count functions")
		functionBaseController.HandleInternalError(c, "查询函数总数失败", err)
		return
	}

	// 获取分页数据
	var functions []models.Function
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&functions).Error; err != nil {
		logrus.WithError(err).Error("Failed to fetch functions")
		functionBaseController.HandleInternalError(c, "查询函数列表失败", err)
		return
	}

	// 构建响应数据
	type FunctionResponse struct {
		ID        uint   `json:"id"`
		UUID      string `json:"uuid"`
		Number    string `json:"number"`
		AppUUID   string `json:"app_uuid"`
		Alias     string `json:"alias"`
		Code      string `json:"code"`
		Remark    string `json:"remark"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	var responseData []FunctionResponse
	for _, function := range functions {
		responseData = append(responseData, FunctionResponse{
			ID:        function.ID,
			UUID:      function.UUID,
			Number:    function.Number,
			AppUUID:   function.AppUUID,
			Alias:     function.Alias,
			Code:      function.Code,
			Remark:    function.Remark,
			CreatedAt: function.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: function.UpdatedAt.Format("2006-01-02 15:04:05"),
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

// FunctionCreateHandler 新增函数API处理器
func FunctionCreateHandler(c *gin.Context) {
	var req struct {
		Alias   string `json:"alias"`
		AppUUID string `json:"app_uuid"`
		Code    string `json:"code"`
		Remark  string `json:"remark"`
	}

	if !functionBaseController.BindJSON(c, &req) {
		return
	}

	// 验证必填字段
	if !functionBaseController.ValidateRequired(c, map[string]interface{}{
		"函数别名": req.Alias,
	}) {
		return
	}

	// 验证别名格式：必须以英文字母开头，只能包含数字和英文字母
	aliasPattern := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`)
	if !aliasPattern.MatchString(req.Alias) {
		functionBaseController.HandleValidationError(c, "别名必须以英文字母开头，只能包含数字和英文字母")
		return
	}

	db, ok := functionBaseController.GetDB(c)
	if !ok {
		return
	}

	// 处理应用UUID：如果为空或"0"，设置为"0"（全局函数）
	appUUID := strings.TrimSpace(req.AppUUID)
	if appUUID == "" {
		appUUID = "0"
	}

	// 如果指定了应用UUID且不是"0"，验证应用是否存在
	if appUUID != "0" {
		var appCount int64
		if err := db.Model(&models.App{}).Where("uuid = ?", appUUID).Count(&appCount).Error; err != nil {
			logrus.WithError(err).Error("Failed to check app existence")
			functionBaseController.HandleInternalError(c, "验证应用失败", err)
			return
		}
		if appCount == 0 {
			functionBaseController.HandleValidationError(c, "指定的应用不存在")
			return
		}
	}

	// 创建函数
	function := models.Function{
		Alias:   strings.TrimSpace(req.Alias),
		AppUUID: appUUID,
		Code:    req.Code,
		Remark:  strings.TrimSpace(req.Remark),
	}

	if err := db.Create(&function).Error; err != nil {
		logrus.WithError(err).Error("Failed to create function")
		functionBaseController.HandleInternalError(c, "创建函数失败", err)
		return
	}

	functionBaseController.HandleSuccess(c, "创建成功", function)
}

// FunctionUpdateHandler 更新函数API处理器
func FunctionUpdateHandler(c *gin.Context) {
	var req struct {
		UUID    string `json:"uuid"`
		AppUUID string `json:"app_uuid"`
		Code    string `json:"code"`
		Remark  string `json:"remark"`
	}

	if !functionBaseController.BindJSON(c, &req) {
		return
	}

	// 验证必填字段（移除对alias的验证，因为编辑时不允许修改别名）
	if !functionBaseController.ValidateRequired(c, map[string]interface{}{
		"函数UUID": req.UUID,
	}) {
		return
	}

	db, ok := functionBaseController.GetDB(c)
	if !ok {
		return
	}

	// 处理应用UUID：如果为空或"0"，设置为"0"（全局函数）
	updateAppUUID := strings.TrimSpace(req.AppUUID)
	if updateAppUUID == "" {
		updateAppUUID = "0"
	}

	// 如果指定了应用UUID且不是"0"，验证应用是否存在
	if updateAppUUID != "0" {
		var appCount int64
		if err := db.Model(&models.App{}).Where("uuid = ?", updateAppUUID).Count(&appCount).Error; err != nil {
			logrus.WithError(err).Error("Failed to check app existence")
			functionBaseController.HandleInternalError(c, "验证应用失败", err)
			return
		}
		if appCount == 0 {
			functionBaseController.HandleValidationError(c, "指定的应用不存在")
			return
		}
	}

	// 通过uuid字段查找函数
	var function models.Function
	if err := db.Where("uuid = ?", strings.TrimSpace(req.UUID)).First(&function).Error; err != nil {
		functionBaseController.HandleValidationError(c, "函数不存在")
		return
	}

	// 更新函数信息（不允许修改别名）
	function.AppUUID = updateAppUUID
	function.Code = req.Code
	function.Remark = strings.TrimSpace(req.Remark)

	if err := db.Save(&function).Error; err != nil {
		logrus.WithError(err).Error("Failed to update function")
		functionBaseController.HandleInternalError(c, "更新函数失败", err)
		return
	}

	functionBaseController.HandleSuccess(c, "更新成功", function)
}

// FunctionDeleteHandler 删除函数API处理器
func FunctionDeleteHandler(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}

	if !functionBaseController.BindJSON(c, &req) {
		return
	}

	if req.ID == 0 {
		functionBaseController.HandleValidationError(c, "函数ID不能为空")
		return
	}

	db, ok := functionBaseController.GetDB(c)
	if !ok {
		return
	}

	// 删除函数
	if err := db.Delete(&models.Function{}, req.ID).Error; err != nil {
		logrus.WithError(err).Error("Failed to delete function")
		functionBaseController.HandleInternalError(c, "删除函数失败", err)
		return
	}

	logrus.WithField("function_id", req.ID).Info("Successfully deleted function")

	functionBaseController.HandleSuccess(c, "删除成功", nil)
}

// FunctionsBatchDeleteHandler 批量删除函数API处理器
func FunctionsBatchDeleteHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}

	if !functionBaseController.BindJSON(c, &req) {
		return
	}

	if len(req.IDs) == 0 {
		functionBaseController.HandleValidationError(c, "请选择要删除的函数")
		return
	}

	db, ok := functionBaseController.GetDB(c)
	if !ok {
		return
	}

	// 批量删除函数
	if err := db.Delete(&models.Function{}, req.IDs).Error; err != nil {
		logrus.WithError(err).Error("Failed to batch delete functions")
		functionBaseController.HandleInternalError(c, "批量删除失败", err)
		return
	}

	logrus.WithField("function_ids", req.IDs).Info("Successfully batch deleted functions")

	functionBaseController.HandleSuccess(c, "批量删除成功", nil)
}
