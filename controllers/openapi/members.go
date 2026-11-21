package openapi

import (
	"strings"
	"time"

	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 终端账号（members）
// ============================================================================
//
// 一号通：账号全站唯一，不归属任何应用。这里按 UUID 操作，避免暴露自增 id。

type memberDTO struct {
	UUID          string  `json:"uuid"`
	Username      string  `json:"username"`
	Email         string  `json:"email"`
	Type          int     `json:"type"`
	Status        int     `json:"status"`
	StatusText    string  `json:"status_text"`
	Points        float64 `json:"points"`
	Permanent     bool    `json:"permanent"`
	ExpiredAt     string  `json:"expired_at"`
	TotalRecharge int     `json:"total_recharge"`
	LevelUUID     string  `json:"level_uuid"`
	Level         int     `json:"level"`
	LevelName     string  `json:"level_name"`
	Remark        string  `json:"remark"`
	CreatedAt     string  `json:"created_at"`
}

func memberStatusText(s int) string {
	switch s {
	case models.MemberStatusNormal:
		return "正常"
	case models.MemberStatusDisabled:
		return "封停"
	case models.MemberStatusBlack:
		return "黑名单"
	default:
		return "未知"
	}
}

// toMemberDTO 组装返回体，顺带解析会员等级名称
func toMemberDTO(m *models.Member) memberDTO {
	d := memberDTO{
		UUID:          m.UUID,
		Username:      m.Username,
		Email:         m.Email,
		Type:          m.Type,
		Status:        m.Status,
		StatusText:    memberStatusText(m.Status),
		Points:        m.Points,
		TotalRecharge: m.TotalRecharge,
		LevelUUID:     m.LevelUUID,
		Remark:        m.Remark,
		CreatedAt:     fmtTime(m.CreatedAt),
	}
	// 永久判定与客户端一致：到期时间远在未来
	if m.ExpiredAt.Year() >= models.PermanentTime.Year() {
		d.Permanent = true
		d.ExpiredAt = "permanent"
	} else {
		d.ExpiredAt = fmtTime(m.ExpiredAt)
	}

	db, err := database.GetDB()
	if err == nil && m.LevelUUID != "" {
		var lv models.MemberLevel
		if err := db.Where("uuid = ?", m.LevelUUID).First(&lv).Error; err == nil {
			d.Level = lv.Level
			d.LevelName = lv.Name
		}
	}
	return d
}

// findMemberByUUID 按 UUID 取账号
func findMemberByUUID(uuid string) (*models.Member, bool) {
	db, err := database.GetDB()
	if err != nil {
		return nil, false
	}
	var m models.Member
	if err := db.Where("uuid = ?", strings.TrimSpace(uuid)).First(&m).Error; err != nil {
		return nil, false
	}
	return &m, true
}

// ListMembers GET /api/openapi/members
// 查询参数：search（用户名模糊）、status、level_uuid、page、limit
func ListMembers(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	page, limit := pageParams(c)

	q := db.Model(&models.Member{})
	if s := strings.TrimSpace(c.Query("search")); s != "" {
		q = q.Where("username LIKE ? OR email LIKE ?", "%"+s+"%", "%"+s+"%")
	}
	if v := strings.TrimSpace(c.Query("status")); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := strings.TrimSpace(c.Query("level_uuid")); v != "" {
		q = q.Where("level_uuid = ?", v)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		internal(c, "统计账号失败", err)
		return
	}
	var rows []models.Member
	if err := q.Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error; err != nil {
		internal(c, "查询账号失败", err)
		return
	}
	list := make([]memberDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toMemberDTO(&rows[i]))
	}
	ok(c, "success", gin.H{"list": list, "total": total, "page": page, "limit": limit})
}

// GetMember GET /api/openapi/members/:uuid
func GetMember(c *gin.Context) {
	m, found := findMemberByUUID(c.Param("uuid"))
	if !found {
		notFound(c, "账号不存在")
		return
	}
	ok(c, "success", toMemberDTO(m))
}

// CreateMember POST /api/openapi/members
// 请求：{ username, password, duration_value?, duration_unit?, points?, remark? }
func CreateMember(c *gin.Context) {
	var req struct {
		Username      string  `json:"username"`
		Password      string  `json:"password"`
		DurationValue float64 `json:"duration_value"`
		DurationUnit  string  `json:"duration_unit"`
		Points        float64 `json:"points"`
		Remark        string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		badRequest(c, "用户名与密码不能为空")
		return
	}

	durationMinutes := 0
	if strings.TrimSpace(req.DurationUnit) != "" {
		v, err := services.CardDurationToMinutes(req.DurationValue, req.DurationUnit)
		if err != nil {
			badRequest(c, err.Error())
			return
		}
		durationMinutes = v
	}

	m, err := services.CreateMember(strings.TrimSpace(req.Username), req.Password,
		durationMinutes, req.Points, strings.TrimSpace(req.Remark))
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	logOp(c, "API创建账号", "创建了账号: "+m.Username)
	ok(c, "创建成功", toMemberDTO(m))
}

// UpdateMember PUT /api/openapi/members/:uuid
// 请求（都可选）：{ points?, permanent?, expired_at?, total_recharge?, remark? }
//   - points 为绝对值覆盖；要做加减请用 /points
//   - permanent=true 设为永久；否则 expired_at 接受 "2006-01-02 15:04:05" 或 RFC3339
func UpdateMember(c *gin.Context) {
	var req struct {
		Points        *float64 `json:"points"`
		Permanent     *bool    `json:"permanent"`
		ExpiredAt     *string  `json:"expired_at"`
		TotalRecharge *int     `json:"total_recharge"`
		Remark        *string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	m, found := findMemberByUUID(c.Param("uuid"))
	if !found {
		notFound(c, "账号不存在")
		return
	}

	up := services.MemberProfileUpdate{
		Points:        req.Points,
		Permanent:     req.Permanent,
		TotalRecharge: req.TotalRecharge,
		Remark:        req.Remark,
	}
	if req.ExpiredAt != nil && (req.Permanent == nil || !*req.Permanent) {
		t, err := parseFlexibleTime(*req.ExpiredAt)
		if err != nil {
			badRequest(c, "expired_at 格式应为 2006-01-02 15:04:05 或 RFC3339")
			return
		}
		up.ExpiredAt = &t
	}

	updated, err := services.UpdateMemberProfile(m.ID, up)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	logOp(c, "API修改账号", "修改了账号资料: "+updated.Username)
	ok(c, "已保存", toMemberDTO(updated))
}

// SetMemberStatus POST /api/openapi/members/:uuid/status
// 请求：{ status } 0=封停 1=正常 2=黑名单
func SetMemberStatus(c *gin.Context) {
	var req struct {
		Status *int `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == nil {
		badRequest(c, "请求体格式错误，需提供 status")
		return
	}
	if *req.Status != models.MemberStatusNormal &&
		*req.Status != models.MemberStatusDisabled &&
		*req.Status != models.MemberStatusBlack {
		badRequest(c, "status 只能是 0 封停 / 1 正常 / 2 黑名单")
		return
	}
	m, found := findMemberByUUID(c.Param("uuid"))
	if !found {
		notFound(c, "账号不存在")
		return
	}
	if err := services.SetMembersStatus([]uint{m.ID}, *req.Status); err != nil {
		badRequest(c, err.Error())
		return
	}
	logOp(c, "API设置账号状态", "账号 "+m.Username+" → "+memberStatusText(*req.Status))
	m.Status = *req.Status
	ok(c, "操作成功", toMemberDTO(m))
}

// AdjustMemberPoints POST /api/openapi/members/:uuid/points
// 请求：{ points, mode? } mode=add（默认）加，sub 扣
func AdjustMemberPoints(c *gin.Context) {
	var req struct {
		Points *float64 `json:"points"`
		Mode   string   `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Points == nil {
		badRequest(c, "请求体格式错误，需提供 points")
		return
	}
	if *req.Points <= 0 {
		badRequest(c, "points 必须大于 0")
		return
	}
	m, found := findMemberByUUID(c.Param("uuid"))
	if !found {
		notFound(c, "账号不存在")
		return
	}
	var err error
	if strings.EqualFold(strings.TrimSpace(req.Mode), "sub") {
		err = services.DeductMemberPoints(m.ID, *req.Points)
	} else {
		err = services.RechargeMemberPoints(m.ID, *req.Points)
	}
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	fresh, _ := findMemberByUUID(c.Param("uuid"))
	logOp(c, "API调整点数", "账号 "+m.Username+" 点数变动 "+formatFloat(*req.Points))
	ok(c, "操作成功", toMemberDTO(fresh))
}

// AdjustMemberTime POST /api/openapi/members/:uuid/time
// 请求：{ value, unit, mode?, permanent? }
//   - permanent=true 直接设为永久（忽略 value/unit/mode）
//   - mode=add（默认）加时长，sub 扣时长
func AdjustMemberTime(c *gin.Context) {
	var req struct {
		Value     float64 `json:"value"`
		Unit      string  `json:"unit"`
		Mode      string  `json:"mode"`
		Permanent *bool   `json:"permanent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	m, found := findMemberByUUID(c.Param("uuid"))
	if !found {
		notFound(c, "账号不存在")
		return
	}

	if req.Permanent != nil && *req.Permanent {
		if err := services.SetMemberExpiry(m.ID, models.PermanentTime); err != nil {
			internal(c, "设置永久失败", err)
			return
		}
		fresh, _ := findMemberByUUID(c.Param("uuid"))
		logOp(c, "API调整时长", "账号 "+m.Username+" 设为永久")
		ok(c, "已设为永久", toMemberDTO(fresh))
		return
	}

	minutes, err := services.CardDurationToMinutes(req.Value, req.Unit)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if minutes == 0 {
		badRequest(c, "时长不能为 0")
		return
	}
	if strings.EqualFold(strings.TrimSpace(req.Mode), "sub") {
		err = services.DeductMemberTime(m.ID, minutes)
	} else {
		err = services.RechargeMemberTime(m.ID, minutes)
	}
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	fresh, _ := findMemberByUUID(c.Param("uuid"))
	logOp(c, "API调整时长", "账号 "+m.Username+" 时长变动 "+formatFloat(req.Value)+req.Unit)
	ok(c, "操作成功", toMemberDTO(fresh))
}

// ResetMemberPassword POST /api/openapi/members/:uuid/password
// 请求：{ password }
func ResetMemberPassword(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		badRequest(c, "请求体格式错误，需提供 password")
		return
	}
	m, found := findMemberByUUID(c.Param("uuid"))
	if !found {
		notFound(c, "账号不存在")
		return
	}
	if err := services.ResetMemberPassword(m.ID, req.Password); err != nil {
		badRequest(c, err.Error())
		return
	}
	logOp(c, "API重置账号密码", "账号 "+m.Username)
	ok(c, "已重置", gin.H{"uuid": m.UUID})
}

// DeleteMember DELETE /api/openapi/members/:uuid
func DeleteMember(c *gin.Context) {
	m, found := findMemberByUUID(c.Param("uuid"))
	if !found {
		notFound(c, "账号不存在")
		return
	}
	if err := services.DeleteMembers([]uint{m.ID}); err != nil {
		internal(c, "删除账号失败", err)
		return
	}
	logOp(c, "API删除账号", "删除账号: "+m.Username)
	ok(c, "已删除", gin.H{"uuid": m.UUID})
}

// ============================================================================
// 小工具
// ============================================================================

// parseFlexibleTime 兼容后台格式与 RFC3339
func parseFlexibleTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.ParseInLocation("2006-01-02", s, time.Local)
}
