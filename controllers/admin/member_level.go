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
// 会员等级管理：累充门槛 + 充值返利
// ============================================================================

var levelBaseController = controllers.NewBaseController()

// MemberLevelListHandler 列出会员等级（按 app_uuid 筛选）
func MemberLevelListHandler(c *gin.Context) {
	levels, err := services.ListMemberLevels()
	if err != nil {
		logrus.WithError(err).Error("Failed to list member levels")
		levelBaseController.HandleInternalError(c, "查询会员等级失败", err)
		return
	}
	// 时间统一成「2006-01-02 15:04:05」：直接返回 model 会被序列化成 RFC3339
	list := make([]gin.H, 0, len(levels))
	for _, lv := range levels {
		list = append(list, gin.H{
			"id":               lv.ID,
			"uuid":             lv.UUID,
			"name":             lv.Name,
			"level":            lv.Level,
			"threshold":        lv.Threshold,
			"rebate_rate":      lv.RebateRate,
			"extra_devices":    lv.ExtraDevices,
			"extra_concurrent": lv.ExtraConcurrent,
			"extra_rebind_count": lv.ExtraRebindCount,
			"color":            lv.Color,
			"sort":             lv.Sort,
			"status":           lv.Status,
			"remark":           lv.Remark,
			"created_at":       fmtTime(lv.CreatedAt),
			"updated_at":       fmtTime(lv.UpdatedAt),
		})
	}
	levelBaseController.HandleSuccess(c, "success", list)
}

// MemberLevelSaveHandler 新增/更新会员等级（uuid 为空则新增）
func MemberLevelSaveHandler(c *gin.Context) {
	var req models.MemberLevel
	if !levelBaseController.BindJSON(c, &req) {
		return
	}
	if !levelBaseController.ValidateRequired(c, map[string]interface{}{
		"等级名称":   req.Name,
	}) {
		return
	}

	isNew := strings.TrimSpace(req.UUID) == ""
	if err := services.SaveMemberLevel(&req); err != nil {
		levelBaseController.HandleValidationError(c, err.Error())
		return
	}

	action := "更新会员等级"
	if isNew {
		action = "新增会员等级"
	}
	recordMemberLog(c, action, fmt.Sprintf("%s：%s", action, req.Name))
	levelBaseController.HandleSuccess(c, "保存成功", req)
}

// MemberLevelDeleteHandler 删除会员等级，并清除账号对该等级的引用
func MemberLevelDeleteHandler(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid"`
	}
	if !levelBaseController.BindJSON(c, &req) {
		return
	}
	if !levelBaseController.ValidateRequired(c, map[string]interface{}{"等级UUID": req.UUID}) {
		return
	}
	if err := services.DeleteMemberLevel(req.UUID); err != nil {
		levelBaseController.HandleValidationError(c, err.Error())
		return
	}
	recordMemberLog(c, "删除会员等级", fmt.Sprintf("删除等级 %s", req.UUID))
	levelBaseController.HandleSuccess(c, "删除成功", nil)
}
