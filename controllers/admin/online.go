package admin

import (
	"net/http"
	"strings"
	"time"

	"NetworkAuth/models"
	"NetworkAuth/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ============================================================================
// 在线管理：跨用户列出当前所有在线会话，供集中管理（查看归属地、踢下线）
// ============================================================================
//
// 「在线」= member_sessions 表中现存的会话行（后台 sweep 会周期清理超时会话，
// 与仪表盘 online_sessions 统计口径一致）。支持按应用筛选、按用户名/IP/机器码 搜索、分页。

// OnlineSessionsHandler 列出在线会话（分页 + app_uuid 筛选 + username/ip/机器码 搜索）。
// 关联 members 取用户名，登录 IP 实时解析归属地。
func OnlineSessionsHandler(c *gin.Context) {
	page, limit := memberBaseController.GetPaginationParams(c)

	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}

	appUUID := strings.TrimSpace(c.Query("app_uuid"))
	search := strings.TrimSpace(c.Query("search"))

	// 统一的表关联与筛选条件；count 与取数各自独立构建，避免多列 Select 干扰 count(*)。
	applyScope := func(q *gorm.DB) *gorm.DB {
		q = q.Table(models.MemberSession{}.TableName() + " AS s").
			Joins("LEFT JOIN " + models.Member{}.TableName() + " AS m ON m.uuid = s.member_uuid")
		if appUUID != "" {
			q = q.Where("s.app_uuid = ?", appUUID)
		}
		if search != "" {
			like := "%" + search + "%"
			q = q.Where("m.username LIKE ? OR s.ip LIKE ? OR s.machine_code LIKE ?", like, like, like)
		}
		return q
	}

	var total int64
	if err := applyScope(db).Count(&total).Error; err != nil {
		logrus.WithError(err).Error("Failed to count online sessions")
		memberBaseController.HandleInternalError(c, "查询在线会话失败", err)
		return
	}

	// 会话与用户名合并后的行结构（供扫描）
	type sessionRow struct {
		ID           uint
		MemberUUID   string
		AppUUID      string
		MachineCode  string
		IP           string
		LastActiveAt time.Time
		CreatedAt    time.Time
		Username     string
	}

	offset := (page - 1) * limit
	var rows []sessionRow
	if err := applyScope(db).
		Select("s.id, s.member_uuid, s.app_uuid, s.machine_code, s.ip, s.last_active_at, s.created_at, m.username").
		Order("s.last_active_at DESC").
		Offset(offset).Limit(limit).
		Scan(&rows).Error; err != nil {
		logrus.WithError(err).Error("Failed to fetch online sessions")
		memberBaseController.HandleInternalError(c, "查询在线会话失败", err)
		return
	}

	type OnlineResponse struct {
		ID           uint   `json:"id"`
		MemberUUID   string `json:"member_uuid"`
		AppUUID      string `json:"app_uuid"`
		Username     string `json:"username"`
		MachineCode  string `json:"machine_code"`
		IP           string `json:"ip"`
		Province     string `json:"province"`
		City         string `json:"city"`
		LastActiveAt string `json:"last_active_at"`
		CreatedAt    string `json:"created_at"`
	}
	list := make([]OnlineResponse, 0, len(rows))
	for _, r := range rows {
		province, city := services.ResolveIPRegion(r.IP)
		list = append(list, OnlineResponse{
			ID:           r.ID,
			MemberUUID:   r.MemberUUID,
			AppUUID:      r.AppUUID,
			Username:     r.Username,
			MachineCode:  r.MachineCode,
			IP:           r.IP,
			Province:     province,
			City:         city,
			LastActiveAt: r.LastActiveAt.Format("2006-01-02 15:04:05"),
			CreatedAt:    r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"msg":   "success",
		"count": total,
		"data":  list,
	})
}
