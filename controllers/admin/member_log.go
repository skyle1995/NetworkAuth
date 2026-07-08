package admin

import (
	"NetworkAuth/models"
	"NetworkAuth/services"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// MemberLogListHandler 终端用户调用审计日志列表
func MemberLogListHandler(c *gin.Context) {
	page, limit := memberBaseController.GetPaginationParams(c)

	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}

	query := db.Model(&models.MemberLog{})
	if appUUID := strings.TrimSpace(c.Query("app_uuid")); appUUID != "" {
		query = query.Where("app_uuid = ?", appUUID)
	}
	if username := strings.TrimSpace(c.Query("username")); username != "" {
		query = query.Where("username = ?", username)
	}
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		query = query.Where("action = ?", action)
	}
	query = memberBaseController.ApplyTimeRangeQuery(c, query, "created_at")

	logs, total, err := services.Paginate[models.MemberLog](query, page, limit, "created_at DESC")
	if err != nil {
		logrus.WithError(err).Error("Failed to fetch member logs")
		memberBaseController.HandleInternalError(c, "查询审计日志失败", err)
		return
	}

	type LogResponse struct {
		ID        uint   `json:"id"`
		AppUUID   string `json:"app_uuid"`
		Username  string `json:"username"`
		Action    string `json:"action"`
		Detail    string `json:"detail"`
		IP        string `json:"ip"`
		CreatedAt string `json:"created_at"`
	}
	data := make([]LogResponse, 0, len(logs))
	for _, l := range logs {
		data = append(data, LogResponse{
			ID:        l.ID,
			AppUUID:   l.AppUUID,
			Username:  l.Username,
			Action:    l.Action,
			Detail:    l.Detail,
			IP:        l.IP,
			CreatedAt: l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"msg":   "success",
		"count": total,
		"data":  data,
	})
}

// MemberLogClearHandler 清空终端用户审计日志
func MemberLogClearHandler(c *gin.Context) {
	db, ok := memberBaseController.GetDB(c)
	if !ok {
		return
	}
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).
		Where("1 = 1").Delete(&models.MemberLog{}).Error; err != nil {
		memberBaseController.HandleInternalError(c, "清空失败", err)
		return
	}
	recordMemberLog(c, "清空审计日志", "清空了终端用户调用审计日志")
	memberBaseController.HandleSuccess(c, "已清空", nil)
}
