package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"time"

	"github.com/sirupsen/logrus"
)

// ============================================================================
// 在线会话定时清理
// ============================================================================
//
// 后台按各应用的 CleanInterval（清理间隔，小时）周期性扫描，删除超过该应用
// CheckInterval（校验间隔，分钟）未活跃的会话——即使用户不再登录也能主动腾出
// 多开名额、还原真实在线数。同时清理归属应用已不存在的孤儿会话。

// 每个应用上次清理时间，用于按 CleanInterval 控制各自的清理节奏。
var lastSessionSweep = map[string]time.Time{}

// StartSessionCleanupTask 启动在线会话清理定时任务。
func StartSessionCleanupTask() {
	go func() {
		// 基础节拍每分钟一次，具体某应用是否清理由其 CleanInterval 决定
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			sweepSessions()
		}
	}()
}

func sweepSessions() {
	db, err := database.GetDB()
	if err != nil {
		return
	}

	var apps []struct {
		UUID          string
		CleanInterval int
		CheckInterval int
	}
	if err := db.Model(&models.App{}).
		Select("uuid, clean_interval, check_interval").Find(&apps).Error; err != nil {
		return
	}

	now := time.Now()
	liveApps := make(map[string]struct{}, len(apps))
	for _, app := range apps {
		liveApps[app.UUID] = struct{}{}

		cleanHours := app.CleanInterval
		if cleanHours <= 0 {
			cleanHours = 1
		}
		// 未到该应用的清理周期则跳过
		if last, ok := lastSessionSweep[app.UUID]; ok &&
			now.Sub(last) < time.Duration(cleanHours)*time.Hour {
			continue
		}
		lastSessionSweep[app.UUID] = now

		checkMin := app.CheckInterval
		if checkMin <= 0 {
			checkMin = 10
		}
		deadline := now.Add(-time.Duration(checkMin) * time.Minute)
		res := db.Where("app_uuid = ? AND last_active_at < ?", app.UUID, deadline).
			Delete(&models.MemberSession{})
		if res.Error == nil && res.RowsAffected > 0 {
			logrus.Debugf("会话清理: 应用 %s 删除 %d 个失效会话", app.UUID, res.RowsAffected)
		}
	}

	// 清理归属应用已删除的孤儿会话
	uuids := make([]string, 0, len(liveApps))
	for u := range liveApps {
		uuids = append(uuids, u)
	}
	if len(uuids) > 0 {
		db.Where("app_uuid NOT IN ?", uuids).Delete(&models.MemberSession{})
	}
}
