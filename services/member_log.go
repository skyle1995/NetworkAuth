package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"

	"github.com/sirupsen/logrus"
)

// ============================================================================
// 终端用户调用审计日志
// ============================================================================

// AddMemberLog 记录一条终端用户调用审计日志（尽力而为，失败不影响主流程）。
func AddMemberLog(appUUID, memberUUID, username, action, detail, ip string) {
	db, err := database.GetDB()
	if err != nil {
		return
	}
	log := models.MemberLog{
		AppUUID:    appUUID,
		MemberUUID: memberUUID,
		Username:   username,
		Action:     action,
		Detail:     detail,
		IP:         ip,
	}
	if err := db.Create(&log).Error; err != nil {
		logrus.WithError(err).Warn("failed to record member log")
	}
}
