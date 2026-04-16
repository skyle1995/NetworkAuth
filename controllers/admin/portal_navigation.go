package admin

import (
	"NetworkAuth/models"
	"NetworkAuth/services"
	"fmt"

	"github.com/gin-gonic/gin"
)

// PortalNavigationListHandler 查询门户导航列表
// 返回后台管理使用的完整导航数据
func PortalNavigationListHandler(c *gin.Context) {
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	var list []models.PortalNavigation
	if err := db.Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		authBaseController.HandleInternalError(c, "查询门户导航失败", err)
		return
	}

	authBaseController.HandleSuccess(c, "ok", list)
}

// PortalNavigationPublicListHandler 查询公开门户导航列表
// 返回门户首页展示使用的可见导航数据
func PortalNavigationPublicListHandler(c *gin.Context) {
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	var list []models.PortalNavigation
	if err := db.Where("is_hidden = ?", false).Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		authBaseController.HandleInternalError(c, "查询门户导航失败", err)
		return
	}

	authBaseController.HandleSuccess(c, "ok", list)
}

// PortalNavigationCreateHandler 创建门户导航
// 保存新导航并在需要时自动切换唯一首页
func PortalNavigationCreateHandler(c *gin.Context) {
	var body portalNavigationPayload
	if !authBaseController.BindJSON(c, &body) {
		return
	}

	item, valid := buildPortalNavigationFromPayload(c, body)
	if !valid {
		return
	}

	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	if err := services.SavePortalNavigation(db, &item); err != nil {
		authBaseController.HandleInternalError(c, "创建门户导航失败", err)
		return
	}

	recordPortalNavigationOperation(c, "新增门户导航", "新增了门户导航："+item.Name)
	authBaseController.HandleSuccess(c, "创建成功", item)
}

// PortalNavigationUpdateHandler 更新门户导航
// 按ID更新导航信息并维护唯一首页约束
func PortalNavigationUpdateHandler(c *gin.Context) {
	var body portalNavigationPayload
	if !authBaseController.BindJSON(c, &body) {
		return
	}

	switch {
	case body.ID == 0:
		authBaseController.HandleValidationError(c, "导航ID不能为空")
		return
	}

	item, valid := buildPortalNavigationFromPayload(c, body)
	if !valid {
		return
	}

	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	var exists models.PortalNavigation
	if err := db.Where("id = ?", body.ID).First(&exists).Error; err != nil {
		authBaseController.HandleNotFoundError(c, "门户导航")
		return
	}

	item.ID = body.ID
	if err := services.SavePortalNavigation(db, &item, exists); err != nil {
		authBaseController.HandleInternalError(c, "更新门户导航失败", err)
		return
	}

	recordPortalNavigationOperation(c, "修改门户导航", "修改了门户导航："+item.Name)
	authBaseController.HandleSuccess(c, "更新成功", item)
}

// PortalNavigationDeleteHandler 删除门户导航
// 按ID删除指定导航记录
func PortalNavigationDeleteHandler(c *gin.Context) {
	var body struct {
		ID uint `json:"id"`
	}
	if !authBaseController.BindJSON(c, &body) {
		return
	}

	switch {
	case body.ID == 0:
		authBaseController.HandleValidationError(c, "导航ID不能为空")
		return
	}

	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	var item models.PortalNavigation
	if err := db.Where("id = ?", body.ID).First(&item).Error; err != nil {
		authBaseController.HandleNotFoundError(c, "门户导航")
		return
	}

	switch services.IsPortalNavigationAdminEntry(item) {
	case true:
		authBaseController.HandleValidationError(c, "管理员登录导航为系统保留项，不允许删除")
		return
	}

	if err := db.Delete(&item).Error; err != nil {
		authBaseController.HandleInternalError(c, "删除门户导航失败", err)
		return
	}

	recordPortalNavigationOperation(c, "删除门户导航", "删除了门户导航："+item.Name)
	authBaseController.HandleSuccess(c, "删除成功", nil)
}

// portalNavigationPayload 门户导航请求体
type portalNavigationPayload struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Sort       int    `json:"sort"`
	IsHome     bool   `json:"is_home"`
	IsHidden   bool   `json:"is_hidden"`
	IsExternal bool   `json:"is_external"`
}

// buildPortalNavigationFromPayload 构建门户导航实体
// 负责统一做字段校验和数据转换
func buildPortalNavigationFromPayload(c *gin.Context, body portalNavigationPayload) (models.PortalNavigation, bool) {
	item := models.PortalNavigation{
		Name:       body.Name,
		Path:       body.Path,
		Sort:       body.Sort,
		IsHome:     body.IsHome,
		IsHidden:   body.IsHidden,
		IsExternal: body.IsExternal,
	}
	services.NormalizePortalNavigation(&item)

	if err := validatePortalNavigationInput(item); err != nil {
		authBaseController.HandleValidationError(c, err.Error())
		return models.PortalNavigation{}, false
	}

	return item, true
}

// validatePortalNavigationInput 校验门户导航字段
// 保证名称和地址满足基础格式要求
func validatePortalNavigationInput(item models.PortalNavigation) error {
	switch {
	case item.Name == "":
		return fmt.Errorf("名称不能为空")
	case len(item.Name) > 64:
		return fmt.Errorf("名称长度不能超过64个字符")
	case item.Path == "":
		return fmt.Errorf("地址不能为空")
	case len(item.Path) > 255:
		return fmt.Errorf("地址长度不能超过255个字符")
	case item.Sort < 0:
		return fmt.Errorf("排序不能小于0")
	case item.IsHome && item.IsHidden:
		return fmt.Errorf("设为首页后禁止隐藏")
	default:
		return nil
	}
}

// recordPortalNavigationOperation 记录门户导航操作日志
// 统一写入管理员操作日志，便于后台审计
func recordPortalNavigationOperation(c *gin.Context, logType, message string) {
	operator := c.GetString("admin_username")
	operatorUUID := c.GetString("admin_uuid")

	switch {
	case operator == "":
		operator = "system"
	}

	services.RecordOperationLog(logType, operator, operatorUUID, message)
}
