package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"errors"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// ============================================================================
// 应用信息类接口（更新地址 / 版本检测 / 卡密信息）
// ============================================================================

// latestVersion 取应用的当前发布版本：优先版本列表里已启用的最高版本，
// 列表为空时回退到 apps.version / download_url（兼容旧配置）。
func latestVersion(db *gorm.DB, app *models.App) models.AppVersion {
	/* 版本号、下载地址、更新公告一律以版本列表为准；
	 * 列表没有已启用记录时视为「没有可更新版本」，不再回退 apps 表字段。 */
	var rows []models.AppVersion
	if err := db.Where("app_uuid = ? AND enabled = ?", app.UUID, true).
		Order("published_at DESC, id DESC").Limit(100).Find(&rows).Error; err != nil || len(rows) == 0 {
		return models.AppVersion{AppUUID: app.UUID, DownloadType: models.DownloadTypeDisabled}
	}
	best := rows[0]
	for _, row := range rows[1:] {
		if compareVersion(best.Version, row.Version) < 0 {
			best = row
		}
	}
	return best
}

// GetUpdateInfo 获取更新地址（type 2）：返回当前版本的更新方式与下载地址。
func GetUpdateInfo(appUUID string) (any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	ver := latestVersion(db, app)
	return map[string]any{
		"latest_version":      ver.Version,
		"download_type":       ver.DownloadType,
		"download_url":        ver.DownloadURL,
		"update_announcement": ver.Changelog,
	}, nil
}

// CheckVersion 检测最新版本（type 3）：比对客户端版本与版本列表里的最高版本。
func CheckVersion(appUUID, clientVersion string) (any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	ver := latestVersion(db, app)
	needUpdate := strings.TrimSpace(ver.Version) != "" &&
		compareVersion(clientVersion, ver.Version) < 0 && ver.DownloadType != models.DownloadTypeDisabled
	return map[string]any{
		"latest_version":      ver.Version,
		"need_update":         needUpdate,
		"download_type":       ver.DownloadType,
		"download_url":        ver.DownloadURL,
		"update_announcement": ver.Changelog,
	}, nil
}

// GetCardInfo 获取卡密信息（type 4）：返回卡密的状态与面值。
// 同时返回 mode/duration/points：时长模式看 duration，点数模式看 points。
// 卡密全站唯一，不再按应用隔离查询；mode 为全局计费模式。
func GetCardInfo(app *models.App, cardNo string) (any, error) {
	cardNo = strings.TrimSpace(cardNo)
	if cardNo == "" {
		return nil, errors.New("卡号不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	var card models.Card
	if err := db.Where("card_no = ?", cardNo).First(&card).Error; err != nil {
		return nil, errors.New("卡号不存在")
	}
	usedAt := ""
	if card.UsedAt != nil {
		usedAt = card.UsedAt.Format("2006-01-02 15:04:05")
	}
	billing := EffectiveBilling(app)
	return map[string]any{
		"card_no":     card.CardNo,
		"status":      card.Status,
		"status_text": cardStatusName(card.Status),
		"mode":        billing.Mode,
		"duration":    card.Duration,
		"points":      card.Points,
		"used_at":     usedAt,
	}, nil
}

// cardStatusName 卡密状态文案（服务层，供公开接口使用）
func cardStatusName(status int) string {
	switch status {
	case models.CardStatusUnused:
		return "未使用"
	case models.CardStatusUsed:
		return "已使用"
	case models.CardStatusFrozen:
		return "已冻结"
	default:
		return "未知"
	}
}

// compareVersion 比较点分版本号，返回 -1(a<b) / 0(相等) / 1(a>b)。
// 非法段按 0 处理，"v1.2" 前缀 v/V 会被忽略。
func compareVersion(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		va, vb := 0, 0
		if i < len(pa) {
			va = pa[i]
		}
		if i < len(pb) {
			vb = pb[i]
		}
		if va != vb {
			if va < vb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// parseVersion 将版本字符串解析为整数段
func parseVersion(v string) []int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(strings.TrimSpace(p))
		out = append(out, n)
	}
	return out
}

// LatestAppVersionField 返回版本列表里的最新版本号，供登录/公告接口展示；
// 列表为空时返回应用自身版本号，保证旧数据仍有值可显示。
func LatestAppVersionField(app *models.App) string {
	db, err := database.GetDB()
	if err != nil || app == nil {
		if app != nil {
			return app.Version
		}
		return ""
	}
	ver := latestVersion(db, app)
	if strings.TrimSpace(ver.Version) == "" {
		return app.Version
	}
	return ver.Version
}
