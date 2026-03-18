package controllers

import (
	"net/http"
	"strconv"

	"NetworkAuth/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ============================================================================
// 结构体定义
// ============================================================================

// BaseController 基础控制器结构体
// 提供通用的数据库访问和响应处理方法
type BaseController struct{}

// ============================================================================
// 构造函数
// ============================================================================

// NewBaseController 创建基础控制器实例
func NewBaseController() *BaseController {
	return &BaseController{}
}

// ============================================================================
// 数据库相关方法
// ============================================================================

// GetDB 获取数据库连接，统一错误处理
func (bc *BaseController) GetDB(c *gin.Context) (*gorm.DB, bool) {
	db, err := database.GetDB()
	if err != nil {
		bc.HandleDatabaseError(c, err)
		return nil, false
	}
	return db, true
}

// ============================================================================
// 错误处理方法
// ============================================================================

// HandleDatabaseError 统一处理数据库连接错误
func (bc *BaseController) HandleDatabaseError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"code": 1,
		"msg":  "数据库连接失败",
		"data": nil,
	})
}

// HandleValidationError 统一处理验证错误
func (bc *BaseController) HandleValidationError(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"code": 1,
		"msg":  message,
		"data": nil,
	})
}

// HandleNotFoundError 统一处理资源未找到错误
func (bc *BaseController) HandleNotFoundError(c *gin.Context, resource string) {
	c.JSON(http.StatusNotFound, gin.H{
		"code": 1,
		"msg":  resource + "不存在",
		"data": nil,
	})
}

// HandleInternalError 统一处理内部服务器错误
func (bc *BaseController) HandleInternalError(c *gin.Context, message string, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"code": 1,
		"msg":  message,
		"data": nil,
	})
}

// ============================================================================
// 成功响应方法
// ============================================================================

// HandleSuccess 统一处理成功响应
func (bc *BaseController) HandleSuccess(c *gin.Context, message string, data interface{}) {
	resp := gin.H{
		"code": 0,
		"msg":  message,
		"data": data,
	}

	// 检查是否有刷新的Token
	if newToken, exists := c.Get("new_token"); exists {
		resp["token"] = newToken
	}

	c.JSON(http.StatusOK, resp)
}

// HandleCreated 统一处理创建成功响应
func (bc *BaseController) HandleCreated(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{
		"code": 0,
		"msg":  message,
		"data": data,
	})
}

// ============================================================================
// 辅助方法
// ============================================================================

// ValidateRequired 验证必填字段
func (bc *BaseController) ValidateRequired(c *gin.Context, fields map[string]interface{}) bool {
	for fieldName, fieldValue := range fields {
		if fieldValue == nil || fieldValue == "" {
			bc.HandleValidationError(c, fieldName+"不能为空")
			return false
		}
	}
	return true
}

// GetPaginationParams 获取分页参数
func (bc *BaseController) GetPaginationParams(c *gin.Context) (int, int) {
	page := 1
	pageSize := 10

	if p := c.Query("page"); p != "" {
		if pageInt, err := strconv.Atoi(p); err == nil && pageInt > 0 {
			page = pageInt
		}
	}

	if ps := c.Query("page_size"); ps != "" {
		if pageSizeInt, err := strconv.Atoi(ps); err == nil && pageSizeInt > 0 && pageSizeInt <= 100 {
			pageSize = pageSizeInt
		}
	}

	return page, pageSize
}

// CalculateOffset 计算数据库查询偏移量
func (bc *BaseController) CalculateOffset(page, pageSize int) int {
	return (page - 1) * pageSize
}

// BindJSON 绑定JSON数据并处理错误
func (bc *BaseController) BindJSON(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		bc.HandleValidationError(c, "请求参数错误: "+err.Error())
		return false
	}
	return true
}

// BindQuery 绑定查询参数并处理错误
func (bc *BaseController) BindQuery(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		bc.HandleValidationError(c, "查询参数错误: "+err.Error())
		return false
	}
	return true
}

// BindURI 绑定URI参数并处理错误
func (bc *BaseController) BindURI(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindUri(obj); err != nil {
		bc.HandleValidationError(c, "URI参数绑定失败: "+err.Error())
		return false
	}
	return true
}

// GetDefaultTemplateData 获取默认模板数据
// 返回包含系统基础信息的数据映射，包括站点标题、页脚文本、备案信息等
func (bc *BaseController) GetDefaultTemplateData() gin.H {
	return gin.H{
		"Title":         "NetworkAuth",
		"SystemName":    "NetworkAuth",
		"FooterText":    "© 2026 NetworkAuth 保留所有权利",
		"ICPRecord":     "",
		"ICPRecordLink": "https://beian.miit.gov.cn",
		"PSBRecord":     "",
		"PSBRecordLink": "https://www.beian.gov.cn",
	}
}
