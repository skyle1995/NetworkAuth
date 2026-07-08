package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 全局变量
// ============================================================================

var memberBaseController = controllers.NewBaseController()

// ============================================================================
// 辅助函数
// ============================================================================

func memberTypeText(t int) string {
	switch t {
	case models.MemberTypeRegister:
		return "注册账号"
	case models.MemberTypeCard:
		return "卡密账号"
	default:
		return "未知"
	}
}

func memberStatusText(status int) string {
	switch status {
	case models.MemberStatusDisabled:
		return "已封停"
	case models.MemberStatusNormal:
		return "正常"
	case models.MemberStatusBlack:
		return "黑名单"
	default:
		return "未知"
	}
}

func recordMemberLog(c *gin.Context, action, details string) {
	operator := c.GetString("admin_username")
	if operator == "" {
		operator = "unknown"
	}
	services.RecordOperationLog(action, operator, c.GetString("admin_uuid"), details)
}

// ============================================================================
// API处理器
// ============================================================================

// MemberListHandler 终端用户列表API处理器
func MemberListHandler(c *gin.Context) {
	page, limit := memberBaseController.GetPaginationParams(c)

	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}

	query := db.Model(&models.Member{})

	if appUUID := strings.TrimSpace(c.Query("app_uuid")); appUUID != "" {
		query = query.Where("app_uuid = ?", appUUID)
	}
	if typeStr := strings.TrimSpace(c.Query("type")); typeStr != "" {
		if t, err := strconv.Atoi(typeStr); err == nil {
			query = query.Where("type = ?", t)
		}
	}
	if statusStr := strings.TrimSpace(c.Query("status")); statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil {
			query = query.Where("status = ?", status)
		}
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query = query.Where("username = ?", search)
	}

	members, total, err := services.Paginate[models.Member](query, page, limit, "created_at DESC")
	if err != nil {
		logrus.WithError(err).Error("Failed to fetch members")
		memberBaseController.HandleInternalError(c, "查询终端用户列表失败", err)
		return
	}

	// 批量取这些用户所属应用的运营模式，用于前端展示到期/点数
	modeByApp := make(map[string]int)
	if len(members) > 0 {
		appUUIDs := make([]string, 0, len(members))
		for _, m := range members {
			appUUIDs = append(appUUIDs, m.AppUUID)
		}
		var appModes []struct {
			UUID          string
			OperationMode int
		}
		db.Model(&models.App{}).Select("uuid, operation_mode").
			Where("uuid IN ?", appUUIDs).Find(&appModes)
		for _, a := range appModes {
			modeByApp[a.UUID] = a.OperationMode
		}
	}

	type MemberResponse struct {
		ID                uint   `json:"id"`
		UUID              string `json:"uuid"`
		AppUUID           string `json:"app_uuid"`
		Username          string `json:"username"`
		Type              int    `json:"type"`
		TypeText          string `json:"type_text"`
		Status            int    `json:"status"`
		StatusText        string `json:"status_text"`
		Mode              int    `json:"mode"`
		ExpiredAt         string `json:"expired_at"`
		Points            int    `json:"points"`
		Email             string `json:"email"`
		CardUUID          string `json:"card_uuid"`
		RegisterIP        string `json:"register_ip"`
		MachineRebindUsed int    `json:"machine_rebind_used"`
		IPRebindUsed      int    `json:"ip_rebind_used"`
		TrialUsed         int    `json:"trial_used"`
		TrialDate         string `json:"trial_date"`
		LastLoginAt       string `json:"last_login_at"`
		LastLoginIP       string `json:"last_login_ip"`
		Remark            string `json:"remark"`
		CreatedAt         string `json:"created_at"`
	}

	responseData := make([]MemberResponse, 0, len(members))
	for _, m := range members {
		expiredAt := "永久"
		if !m.ExpiredAt.Equal(models.PermanentTime) {
			expiredAt = m.ExpiredAt.Format("2006-01-02 15:04:05")
		}
		lastLogin := ""
		if m.LastLoginAt != nil {
			lastLogin = m.LastLoginAt.Format("2006-01-02 15:04:05")
		}
		responseData = append(responseData, MemberResponse{
			ID:                m.ID,
			UUID:              m.UUID,
			AppUUID:           m.AppUUID,
			Username:          m.Username,
			Type:              m.Type,
			TypeText:          memberTypeText(m.Type),
			Status:            m.Status,
			StatusText:        memberStatusText(m.Status),
			Mode:              modeByApp[m.AppUUID],
			ExpiredAt:         expiredAt,
			Points:            m.Points,
			Email:             m.Email,
			CardUUID:          m.CardUUID,
			RegisterIP:        m.RegisterIP,
			MachineRebindUsed: m.MachineRebindUsed,
			IPRebindUsed:      m.IPRebindUsed,
			TrialUsed:         m.TrialUsed,
			TrialDate:         m.TrialDate,
			LastLoginAt:       lastLogin,
			LastLoginIP:       m.LastLoginIP,
			Remark:            m.Remark,
			CreatedAt:         m.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"msg":   "success",
		"count": total,
		"data":  responseData,
	})
}

// MemberCreateHandler 后台创建注册型终端用户API处理器
func MemberCreateHandler(c *gin.Context) {
	var req struct {
		AppUUID       string `json:"app_uuid"`
		Username      string `json:"username"`
		Password      string `json:"password"`
		DurationValue int    `json:"duration_value"`
		DurationUnit  string `json:"duration_unit"`
		Points        int    `json:"points"`
		Remark        string `json:"remark"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if !memberBaseController.ValidateRequired(c, map[string]interface{}{
		"应用UUID": req.AppUUID,
		"用户名":    req.Username,
		"密码":     req.Password,
	}) {
		return
	}

	// 时长模式换算初始时长；点数模式不传单位，durationMinutes 置 0
	durationMinutes := 0
	if req.DurationUnit != "" {
		var err error
		durationMinutes, err = services.CardDurationToMinutes(req.DurationValue, req.DurationUnit)
		if err != nil {
			memberBaseController.HandleValidationError(c, err.Error())
			return
		}
	}

	member, err := services.CreateMember(req.AppUUID, req.Username, req.Password, durationMinutes, req.Points, strings.TrimSpace(req.Remark))
	if err != nil {
		logrus.WithError(err).Error("Failed to create member")
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}

	recordMemberLog(c, "创建终端用户", fmt.Sprintf("为应用 %s 创建用户 %s", req.AppUUID, member.Username))
	memberBaseController.HandleSuccess(c, "创建成功", gin.H{"id": member.ID, "uuid": member.UUID})
}

// MemberSetStatusHandler 批量设置终端用户状态API处理器（正常/封停/黑名单）
func MemberSetStatusHandler(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids"`
		Status int    `json:"status"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		memberBaseController.HandleValidationError(c, "请选择要操作的用户")
		return
	}

	if err := services.SetMembersStatus(req.IDs, req.Status); err != nil {
		logrus.WithError(err).Error("Failed to set members status")
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}

	recordMemberLog(c, "设置用户状态", fmt.Sprintf("将 %d 个用户状态设为「%s」", len(req.IDs), memberStatusText(req.Status)))
	memberBaseController.HandleSuccess(c, "操作成功", nil)
}

// MemberRechargeHandler 终端用户充值API处理器（时长模式加时长/永久，点数模式加点数）
func MemberRechargeHandler(c *gin.Context) {
	var req struct {
		ID            uint   `json:"id"`
		DurationValue int    `json:"duration_value"`
		DurationUnit  string `json:"duration_unit"`
		Points        int    `json:"points"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if req.ID == 0 {
		memberBaseController.HandleValidationError(c, "用户ID不能为空")
		return
	}

	// 点数模式：加点数
	if mode, err := services.GetMemberAppMode(req.ID); err == nil && mode == models.OperationModePoints {
		if err := services.RechargeMemberPoints(req.ID, req.Points); err != nil {
			memberBaseController.HandleValidationError(c, err.Error())
			return
		}
		recordMemberLog(c, "用户充值", fmt.Sprintf("为用户ID %d 充值 %d 点", req.ID, req.Points))
		memberBaseController.HandleSuccess(c, "充值成功", nil)
		return
	}

	if req.DurationUnit == "permanent" {
		if err := services.SetMemberExpiry(req.ID, models.PermanentTime); err != nil {
			memberBaseController.HandleInternalError(c, "设置永久失败", err)
			return
		}
		recordMemberLog(c, "用户充值", fmt.Sprintf("将用户ID %d 设为永久", req.ID))
		memberBaseController.HandleSuccess(c, "已设为永久", nil)
		return
	}

	minutes, err := services.CardDurationToMinutes(req.DurationValue, req.DurationUnit)
	if err != nil {
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}
	if err := services.RechargeMemberTime(req.ID, minutes); err != nil {
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}

	recordMemberLog(c, "用户充值", fmt.Sprintf("为用户ID %d 充值 %d 分钟", req.ID, minutes))
	memberBaseController.HandleSuccess(c, "充值成功", nil)
}

// MemberDeductHandler 终端用户扣减API处理器（时长模式扣时长，点数模式扣点数）
func MemberDeductHandler(c *gin.Context) {
	var req struct {
		ID            uint   `json:"id"`
		DurationValue int    `json:"duration_value"`
		DurationUnit  string `json:"duration_unit"`
		Points        int    `json:"points"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if req.ID == 0 {
		memberBaseController.HandleValidationError(c, "用户ID不能为空")
		return
	}

	// 点数模式：扣点数
	if mode, err := services.GetMemberAppMode(req.ID); err == nil && mode == models.OperationModePoints {
		if err := services.DeductMemberPoints(req.ID, req.Points); err != nil {
			memberBaseController.HandleValidationError(c, err.Error())
			return
		}
		recordMemberLog(c, "用户扣减", fmt.Sprintf("为用户ID %d 扣除 %d 点", req.ID, req.Points))
		memberBaseController.HandleSuccess(c, "扣减成功", nil)
		return
	}

	minutes, err := services.CardDurationToMinutes(req.DurationValue, req.DurationUnit)
	if err != nil {
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}
	if err := services.DeductMemberTime(req.ID, minutes); err != nil {
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}

	recordMemberLog(c, "用户扣时", fmt.Sprintf("为用户ID %d 扣除 %d 分钟", req.ID, minutes))
	memberBaseController.HandleSuccess(c, "扣时成功", nil)
}

// MemberResetPasswordHandler 重置终端用户密码API处理器
func MemberResetPasswordHandler(c *gin.Context) {
	var req struct {
		ID       uint   `json:"id"`
		Password string `json:"password"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if req.ID == 0 {
		memberBaseController.HandleValidationError(c, "用户ID不能为空")
		return
	}
	if !memberBaseController.ValidateRequired(c, map[string]interface{}{"新密码": req.Password}) {
		return
	}

	if err := services.ResetMemberPassword(req.ID, req.Password); err != nil {
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}

	recordMemberLog(c, "重置用户密码", fmt.Sprintf("重置了用户ID %d 的密码", req.ID))
	memberBaseController.HandleSuccess(c, "密码重置成功", nil)
}

// MemberUpdateRemarkHandler 更新终端用户备注API处理器
func MemberUpdateRemarkHandler(c *gin.Context) {
	var req struct {
		ID     uint   `json:"id"`
		Remark string `json:"remark"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if req.ID == 0 {
		memberBaseController.HandleValidationError(c, "用户ID不能为空")
		return
	}

	if err := services.UpdateMemberRemark(req.ID, strings.TrimSpace(req.Remark)); err != nil {
		memberBaseController.HandleInternalError(c, "更新备注失败", err)
		return
	}
	memberBaseController.HandleSuccess(c, "更新成功", nil)
}

// MemberBindingsHandler 查询终端用户的机器码/IP绑定列表API处理器
func MemberBindingsHandler(c *gin.Context) {
	memberUUID := strings.TrimSpace(c.Query("member_uuid"))
	if memberUUID == "" {
		memberBaseController.HandleValidationError(c, "终端用户UUID不能为空")
		return
	}

	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}

	var bindings []models.Binding
	if err := db.Where("member_uuid = ?", memberUUID).Order("created_at DESC").Find(&bindings).Error; err != nil {
		memberBaseController.HandleInternalError(c, "查询绑定列表失败", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": bindings,
	})
}

// MemberGetDataHandler 获取终端用户的用户数据
func MemberGetDataHandler(c *gin.Context) {
	idStr := strings.TrimSpace(c.Query("id"))
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		memberBaseController.HandleValidationError(c, "用户ID无效")
		return
	}
	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}
	var member models.Member
	if err := db.Select("id, data").First(&member, id).Error; err != nil {
		memberBaseController.HandleNotFoundError(c, "终端用户")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": gin.H{"data": member.Data}})
}

// MemberUpdateDataHandler 更新终端用户的用户数据
func MemberUpdateDataHandler(c *gin.Context) {
	var req struct {
		ID   uint   `json:"id"`
		Data string `json:"data"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if req.ID == 0 {
		memberBaseController.HandleValidationError(c, "用户ID不能为空")
		return
	}
	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}
	if err := db.Model(&models.Member{}).Where("id = ?", req.ID).Update("data", req.Data).Error; err != nil {
		memberBaseController.HandleInternalError(c, "更新用户数据失败", err)
		return
	}
	recordMemberLog(c, "更新用户数据", fmt.Sprintf("更新了用户ID %d 的用户数据", req.ID))
	memberBaseController.HandleSuccess(c, "保存成功", nil)
}

// MemberSessionsHandler 查询终端用户的在线会话API处理器
func MemberSessionsHandler(c *gin.Context) {
	memberUUID := strings.TrimSpace(c.Query("member_uuid"))
	if memberUUID == "" {
		memberBaseController.HandleValidationError(c, "终端用户UUID不能为空")
		return
	}

	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}

	var sessions []models.MemberSession
	if err := db.Where("member_uuid = ?", memberUUID).Order("last_active_at DESC").Find(&sessions).Error; err != nil {
		memberBaseController.HandleInternalError(c, "查询会话列表失败", err)
		return
	}

	type SessionResponse struct {
		ID           uint   `json:"id"`
		MachineCode  string `json:"machine_code"`
		IP           string `json:"ip"`
		LastActiveAt string `json:"last_active_at"`
		CreatedAt    string `json:"created_at"`
	}
	list := make([]SessionResponse, 0, len(sessions))
	for _, s := range sessions {
		list = append(list, SessionResponse{
			ID:           s.ID,
			MachineCode:  s.MachineCode,
			IP:           s.IP,
			LastActiveAt: s.LastActiveAt.Format("2006-01-02 15:04:05"),
			CreatedAt:    s.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": list})
}

// MemberKickSessionHandler 踢下线：删除指定会话（id）或某用户全部会话（member_uuid）
func MemberKickSessionHandler(c *gin.Context) {
	var req struct {
		ID         uint   `json:"id"`
		MemberUUID string `json:"member_uuid"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}

	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}

	q := db.Model(&models.MemberSession{})
	if req.ID > 0 {
		q = q.Where("id = ?", req.ID)
	} else if strings.TrimSpace(req.MemberUUID) != "" {
		q = q.Where("member_uuid = ?", strings.TrimSpace(req.MemberUUID))
	} else {
		memberBaseController.HandleValidationError(c, "请指定会话ID或用户UUID")
		return
	}

	res := q.Delete(&models.MemberSession{})
	if res.Error != nil {
		memberBaseController.HandleInternalError(c, "踢下线失败", res.Error)
		return
	}

	recordMemberLog(c, "踢下线", fmt.Sprintf("下线了 %d 个会话", res.RowsAffected))
	memberBaseController.HandleSuccess(c, "操作成功", gin.H{"count": res.RowsAffected})
}

// MemberClearBindingsHandler 清空终端用户绑定API处理器（后台解绑）
func MemberClearBindingsHandler(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if !memberBaseController.ValidateRequired(c, map[string]interface{}{"终端用户UUID": req.UUID}) {
		return
	}

	if err := services.ClearMemberBindings(req.UUID); err != nil {
		memberBaseController.HandleValidationError(c, err.Error())
		return
	}

	recordMemberLog(c, "清空用户绑定", fmt.Sprintf("清空了用户 %s 的机器码/IP绑定", req.UUID))
	memberBaseController.HandleSuccess(c, "解绑成功", nil)
}

// MembersBatchDeleteHandler 批量删除终端用户API处理器
func MembersBatchDeleteHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		memberBaseController.HandleValidationError(c, "请选择要删除的用户")
		return
	}

	if err := services.DeleteMembers(req.IDs); err != nil {
		logrus.WithError(err).Error("Failed to batch delete members")
		memberBaseController.HandleInternalError(c, "批量删除失败", err)
		return
	}

	recordMemberLog(c, "删除终端用户", fmt.Sprintf("批量删除了 %d 个终端用户", len(req.IDs)))
	memberBaseController.HandleSuccess(c, "批量删除成功", nil)
}
