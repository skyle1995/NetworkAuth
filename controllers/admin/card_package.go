package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 卡密套餐管理：制卡的售卖单元（面值 + 售价）
// ============================================================================

var packageBaseController = controllers.NewBaseController()

// CardPackageListHandler 列出套餐（按 app_uuid 筛选；enabled=1 只列启用的，供制卡下拉用）
func CardPackageListHandler(c *gin.Context) {
	appUUID := strings.TrimSpace(c.Query("app_uuid"))
	onlyEnabled := c.Query("enabled") == "1"

	pkgs, err := services.ListCardPackages(appUUID, onlyEnabled)
	if err != nil {
		logrus.WithError(err).Error("Failed to list card packages")
		packageBaseController.HandleInternalError(c, "查询套餐失败", err)
		return
	}
	packageBaseController.HandleSuccess(c, "success", pkgs)
}

// CardPackageSaveHandler 新增/更新套餐（uuid 为空则新增）
func CardPackageSaveHandler(c *gin.Context) {
	var req models.CardPackage
	if !packageBaseController.BindJSON(c, &req) {
		return
	}
	if !packageBaseController.ValidateRequired(c, map[string]interface{}{
		"应用UUID": req.AppUUID,
		"套餐名称":   req.Name,
	}) {
		return
	}

	isNew := strings.TrimSpace(req.UUID) == ""
	if err := services.SaveCardPackage(&req); err != nil {
		packageBaseController.HandleValidationError(c, err.Error())
		return
	}

	action := "更新套餐"
	if isNew {
		action = "新增套餐"
	}
	recordCardLog(c, action, fmt.Sprintf("%s：%s（应用 %s）", action, req.Name, req.AppUUID))
	packageBaseController.HandleSuccess(c, "保存成功", req)
}

// CardPackageDeleteHandler 删除套餐（已售出的卡已快照面值，不受影响）
func CardPackageDeleteHandler(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid"`
	}
	if !packageBaseController.BindJSON(c, &req) {
		return
	}
	if !packageBaseController.ValidateRequired(c, map[string]interface{}{"套餐UUID": req.UUID}) {
		return
	}
	if err := services.DeleteCardPackage(req.UUID); err != nil {
		packageBaseController.HandleValidationError(c, err.Error())
		return
	}
	recordCardLog(c, "删除套餐", fmt.Sprintf("删除套餐 %s", req.UUID))
	packageBaseController.HandleSuccess(c, "删除成功", nil)
}
