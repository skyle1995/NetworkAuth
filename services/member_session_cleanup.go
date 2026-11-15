package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"time"

	"github.com/sirupsen/logrus"
)

// ============================================================================
// 授权状态定时清理（dead machine pruner）
// ============================================================================
//
// 二级令牌改造后，这里承担四件事，都按各应用的 CleanInterval（小时）节流：
//
//  1. **回收过期租约**：把 lease_expires_at 已过的激活标为 released，归还并发名额。
//     这是「客户端崩溃/被强杀、没调 71」的唯一兜底 —— 没有它，名额要等用户重装才释放。
//  2. **物理删除**：已释放超过 7 天的激活记录，避免表无限增长（保留期内仍可查历史）。
//  3. **过期令牌**：账号令牌 expires_at 已过的行直接删除。
//
// 另外清理归属应用已删除的孤儿激活。

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
		UUID           string
		CleanInterval  int
		OfflineTimeout int
	}
	if err := db.Model(&models.App{}).
		Select("uuid, clean_interval, offline_timeout").Find(&apps).Error; err != nil {
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

		// ① 二级令牌：把租约已过期的激活标为「已释放」，归还并发名额。
		//    这一步是必须的 —— 客户端崩溃/被强杀时不会调 71，
		//    只能靠租约到期 + 这里的兜底扫描回收。
		resAct := db.Model(&models.AppActivation{}).
			Where("app_uuid = ? AND deactivated_at IS NULL AND lease_expires_at < ?", app.UUID, now).
			Updates(map[string]any{"deactivated_at": now, "updated_at": now})
		if resAct.Error == nil && resAct.RowsAffected > 0 {
			logrus.Debugf("激活清理: 应用 %s 回收 %d 个过期租约", app.UUID, resAct.RowsAffected)
		}

		purgeBefore := now.Add(-7 * 24 * time.Hour)
		db.Where("app_uuid = ? AND deactivated_at IS NOT NULL AND deactivated_at < ?", app.UUID, purgeBefore).
			Delete(&models.AppActivation{})
	}

	// 清理归属应用已删除的孤儿会话与激活
	uuids := make([]string, 0, len(liveApps))
	for u := range liveApps {
		uuids = append(uuids, u)
	}
	if len(uuids) > 0 {
		db.Where("app_uuid NOT IN ?", uuids).Delete(&models.AppActivation{})
	}

	// 清理过期账号令牌（登录态过期后不再可用）
	if res := db.Where("expires_at < ?", now).Delete(&models.MemberAccountToken{}); res.Error == nil && res.RowsAffected > 0 {
		logrus.Debugf("令牌清理: 删除 %d 个过期账号令牌", res.RowsAffected)
	}
}
