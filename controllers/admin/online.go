package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"NetworkAuth/models"
	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// OnlineSessionsHandler 应用运行列表（当前在线的激活）。
//
// 数据源是 app_activations：一行 = 一个账号在某应用的一个活跃实例。
// 旧 member_sessions 表已移除，不再有"登录会话"这个概念。
func OnlineSessionsHandler(c *gin.Context) {
	page, limit := memberBaseController.GetPaginationParams(c)
	_ = limit

	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}

	appUUID := strings.TrimSpace(c.Query("app_uuid"))
	search := strings.TrimSpace(c.Query("search"))

	// 统一的关联与筛选；count 与取数各自构建，避免多列 Select 干扰 count(*)。
	applyScope := func(q *gorm.DB) *gorm.DB {
		q = q.Table(models.AppActivation{}.TableName() + " AS a").
			Joins("LEFT JOIN " + models.Member{}.TableName() + " AS m ON m.uuid = a.member_uuid").
			Where("a.deactivated_at IS NULL").
			Where("a.lease_expires_at > ?", time.Now())
		if appUUID != "" {
			q = q.Where("a.app_uuid = ?", appUUID)
		}
		if search != "" {
			like := "%" + search + "%"
			q = q.Where("m.username LIKE ? OR a.ip LIKE ? OR a.fingerprint LIKE ?", like, like, like)
		}
		return q
	}

	var total int64
	if err := applyScope(db).Count(&total).Error; err != nil {
		logrus.WithError(err).Error("Failed to count online activations")
		memberBaseController.HandleInternalError(c, "查询在线实例失败", err)
		return
	}

	type activationRow struct {
		ID             uint
		MemberUUID     string
		AppUUID        string
		Fingerprint    string
		IP             string
		Version        string
		DeviceName     string
		LeaseExpiresAt time.Time
		LastActiveAt   time.Time
		CreatedAt      time.Time
		OnlineAt       time.Time
		Username       string
	}

	var rows []activationRow
	if err := applyScope(db).
		Select("a.id, a.member_uuid, a.app_uuid, a.fingerprint, a.ip, a.version, a.device_name, " +
			"a.lease_expires_at, a.last_active_at, a.created_at, a.online_at, m.username").
		Order("a.last_active_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Scan(&rows).Error; err != nil {
		logrus.WithError(err).Error("Failed to list online activations")
		memberBaseController.HandleInternalError(c, "查询在线实例失败", err)
		return
	}

	list := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		province, city := services.ResolveIPRegion(r.IP)
		list = append(list, gin.H{
			"id":               r.ID,
			"member_uuid":      r.MemberUUID,
			"username":         r.Username,
			"app_uuid":         r.AppUUID,
			// 对外统一叫 machine_code（设备码的业务名），DB 列名是 fingerprint
			"machine_code":     r.Fingerprint,
			"ip":               r.IP,
			"province":         province,
			"city":             city,
			"version":          r.Version,
			"device_name":      r.DeviceName,
			// 时间统一成「2006-01-02 15:04:05」：裸 time.Time 会被序列化成
			// 2026-11-15T21:49:06.0156506+08:00，表格里读不了
			"lease_expires_at": fmtTime(r.LeaseExpiresAt),
			"last_active_at":   fmtTime(r.LastActiveAt),
			"created_at":       fmtTime(r.CreatedAt),
			"online_at":        fmtTime(r.OnlineAt),
		})
	}

	// 与全站分页约定一致：data 是数组，count 是总数（前端据此渲染表格与分页）
	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"msg":   "success",
		"count": total,
		"data":  list,
	})
}

// OnlineKickHandler 强制下线某个在线实例。
func OnlineKickHandler(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if !memberBaseController.BindJSON(c, &req) {
		return
	}
	if req.ID == 0 {
		memberBaseController.HandleValidationError(c, "实例ID不能为空")
		return
	}
	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}
	var act models.AppActivation
	if err := db.Where("id = ?", req.ID).First(&act).Error; err != nil {
		memberBaseController.HandleValidationError(c, "实例不存在")
		return
	}
	if err := services.Deactivate(db, act.Token); err != nil {
		memberBaseController.HandleInternalError(c, "强制下线失败", err)
		return
	}
	operator := c.GetString("admin_username")
	if operator == "" {
		operator = "unknown"
	}
	services.RecordOperationLog("强制下线", operator, c.GetString("admin_uuid"), "下线实例 #"+strconv.FormatUint(uint64(req.ID), 10))
	memberBaseController.HandleSuccess(c, "已下线", nil)
}

// fmtTime 把时间统一成表格可读的「2006-01-02 15:04:05」。
// 直接返回 time.Time 会被 encoding/json 序列化成 RFC3339（带纳秒和时区），
// 管理端表格里读不了；零值返回空串，前端才能用 "-" 占位。
func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}