package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"NetworkAuth/models"
	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 黑名单管理：设备(机器码) / IP / 地区 维度封禁
// ============================================================================

// blacklistTypeText 类型中文名。
func blacklistTypeText(t int) string {
	switch t {
	case models.BlacklistTypeMachine:
		return "设备"
	case models.BlacklistTypeIP:
		return "IP"
	case models.BlacklistTypeRegion:
		return "地区"
	default:
		return "未知"
	}
}

// MemberBlacklistHandler 拉黑账号：账号置黑 + 清会话，并按所选维度把其设备/IP/地区加入黑名单。
func MemberBlacklistHandler(c *gin.Context) {
	var req struct {
		ID             uint `json:"id"`
		BlacklistDevice bool `json:"blacklist_device"`
		BlacklistIP     bool `json:"blacklist_ip"`
		BlacklistRegion bool `json:"blacklist_region"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if req.ID == 0 {
		memberBaseController.HandleValidationError(c, "账号ID不能为空")
		return
	}

	res, err := services.BlacklistMemberFull(req.ID, services.BlacklistOptions{
		Device: req.BlacklistDevice,
		IP:     req.BlacklistIP,
		Region: req.BlacklistRegion,
	})
	if err != nil {
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}

	recordMemberLog(c, "拉黑账号", fmt.Sprintf(
		"拉黑账号 %v（设备+%v / IP+%v / 地区+%v）",
		res["username"], res["device"], res["ip"], res["region"],
	))
	memberBaseController.HandleSuccess(c, "已拉黑", res)
}

// SessionBlacklistHandler 从在线会话拉黑：按会话具体值封禁 设备/IP/地区（可连带拉黑账号），
// 并踢掉命中的在线会话。
func SessionBlacklistHandler(c *gin.Context) {
	var req struct {
		AppUUID         string `json:"app_uuid"`
		MemberUUID      string `json:"member_uuid"`
		Username        string `json:"username"`
		MachineCode     string `json:"machine_code"`
		IP              string `json:"ip"`
		Province        string `json:"province"`
		City            string `json:"city"`
		BlacklistDevice  bool `json:"blacklist_device"`
		BlacklistIP      bool `json:"blacklist_ip"`
		BlacklistRegion  bool `json:"blacklist_region"`
		BlacklistAccount bool `json:"blacklist_account"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if !req.BlacklistDevice && !req.BlacklistIP && !req.BlacklistRegion && !req.BlacklistAccount {
		memberBaseController.HandleValidationError(c, "请至少选择一个拉黑维度")
		return
	}

	res, err := services.BlacklistFromSession(
		req.AppUUID, req.MemberUUID, req.Username, req.MachineCode, req.IP, req.Province, req.City,
		services.SessionBlacklistOptions{
			Device:  req.BlacklistDevice,
			IP:      req.BlacklistIP,
			Region:  req.BlacklistRegion,
			Account: req.BlacklistAccount,
		},
	)
	if err != nil {
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}

	recordMemberLog(c, "拉黑", fmt.Sprintf(
		"在线会话拉黑 %s（设备+%v/IP+%v/地区+%v/账号%v，踢下线%v）",
		req.Username, res["device"], res["ip"], res["region"], res["account"], res["kicked"],
	))
	memberBaseController.HandleSuccess(c, "已拉黑", res)
}

// BlacklistListHandler 黑名单列表（分页 + app_uuid / type 筛选 + value/username 搜索）。
func BlacklistListHandler(c *gin.Context) {
	page, limit := memberBaseController.GetPaginationParams(c)
	appUUID := strings.TrimSpace(c.Query("app_uuid"))
	search := strings.TrimSpace(c.Query("search"))

	var typ *int
	if ts := strings.TrimSpace(c.Query("type")); ts != "" {
		if t, err := strconv.Atoi(ts); err == nil {
			typ = &t
		}
	}

	items, total, err := services.ListBlacklist(appUUID, typ, search, page, limit)
	if err != nil {
		logrus.WithError(err).Error("Failed to fetch blacklist")
		memberBaseController.HandleInternalError(c, "查询黑名单失败", err)
		return
	}

	type BlacklistResponse struct {
		ID        uint   `json:"id"`
		AppUUID   string `json:"app_uuid"`
		Type      int    `json:"type"`
		TypeText  string `json:"type_text"`
		Value     string `json:"value"`
		Province  string `json:"province"`
		City      string `json:"city"`
		Username  string `json:"username"`
		Remark    string `json:"remark"`
		CreatedAt string `json:"created_at"`
	}
	list := make([]BlacklistResponse, 0, len(items))
	for _, b := range items {
		list = append(list, BlacklistResponse{
			ID:        b.ID,
			AppUUID:   b.AppUUID,
			Type:      b.Type,
			TypeText:  blacklistTypeText(b.Type),
			Value:     b.Value,
			Province:  b.Province,
			City:      b.City,
			Username:  b.Username,
			Remark:    b.Remark,
			CreatedAt: b.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "count": total, "data": list})
}

// BlacklistAddHandler 后台手动新增黑名单条目。
func BlacklistAddHandler(c *gin.Context) {
	var req struct {
		AppUUID  string `json:"app_uuid"`
		Type     int    `json:"type"`
		Value    string `json:"value"`
		Province string `json:"province"`
		City     string `json:"city"`
		Remark   string `json:"remark"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.AppUUID) == "" {
		memberBaseController.HandleValidationError(c, "请选择应用")
		return
	}
	if err := services.AddBlacklistManual(req.AppUUID, req.Type, req.Value, req.Province, req.City, req.Remark); err != nil {
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}
	recordMemberLog(c, "新增黑名单", fmt.Sprintf("新增「%s」黑名单", blacklistTypeText(req.Type)))
	memberBaseController.HandleSuccess(c, "已加入黑名单", nil)
}

// BlacklistDeleteHandler 按ID批量移除黑名单（解封设备/IP/地区）。
func BlacklistDeleteHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		memberBaseController.HandleValidationError(c, "请选择要移除的条目")
		return
	}
	if err := services.RemoveBlacklist(req.IDs); err != nil {
		memberBaseController.HandleInternalError(c, "移除失败", err)
		return
	}
	recordMemberLog(c, "移除黑名单", fmt.Sprintf("移除 %d 条黑名单", len(req.IDs)))
	memberBaseController.HandleSuccess(c, "已移除", nil)
}
