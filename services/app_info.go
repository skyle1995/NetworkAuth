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

// latestVersion 取应用的当前发布版本：版本列表里「已启用」的最高版本。
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
//
// 查的是**请求签名对应的应用**。插件更新检查要走 CheckVersionByPlugin，
// 否则会拿插件版本去和宿主模块应用的版本比大小。
func CheckVersion(appUUID, clientVersion string) (any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	return versionPayload(db, app, clientVersion), nil
}

// CheckVersionByPlugin 按插件 ID 检测版本（type 3 传 plugin_id 时走这条）。
//
// 插件与应用的对应关系存在 apps.plugin_id 上（唯一索引），版本仍按应用维度维护
// （app_versions.app_uuid），所以这里先按 plugin_id 定位应用，再取它的最高版本。
//
// 找不到应用（后台还没登记这个插件，或该应用已停用）时返回 plugin_found=false 的
// 「无更新」结果而不是报错：插件没登记不该让客户端更新流程直接失败。
func CheckVersionByPlugin(pluginID, clientVersion string) (any, error) {
	pluginID = strings.TrimSpace(pluginID)
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	var app models.App
	if err := db.Where("plugin_id = ? AND status = 1", pluginID).First(&app).Error; err != nil {
		return map[string]any{
			"plugin_id":           pluginID,
			"plugin_found":        false,
			"latest_version":      "",
			"need_update":         false,
			"download_type":       models.DownloadTypeDisabled,
			"download_url":        "",
			"update_announcement": "",
		}, nil
	}

	payload := versionPayload(db, &app, clientVersion)
	payload["plugin_id"] = pluginID
	payload["plugin_found"] = true
	return payload, nil
}

// versionPayload 组装版本检测的返回体（两个入口共用，保证字段一致）。
func versionPayload(db *gorm.DB, app *models.App, clientVersion string) map[string]any {
	ver := latestVersion(db, app)
	needUpdate := strings.TrimSpace(ver.Version) != "" &&
		compareVersion(clientVersion, ver.Version) < 0 && ver.DownloadType != models.DownloadTypeDisabled
	return map[string]any{
		"latest_version":      ver.Version,
		"need_update":         needUpdate,
		"download_type":       ver.DownloadType,
		"download_url":        ver.DownloadURL,
		"update_announcement": ver.Changelog,
	}
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
	if app == nil {
		return ""
	}
	db, err := database.GetDB()
	if err != nil {
		return ""
	}
	return latestVersion(db, app).Version
}

// LatestAppUpdateSnapshot 返回当前版本列表项的更新策略快照，供公告/登录接口下发。
func LatestAppUpdateSnapshot(app *models.App) map[string]any {
	if app == nil {
		return map[string]any{"download_type": 0, "download_url": ""}
	}
	db, err := database.GetDB()
	if err != nil {
		return map[string]any{"download_type": 0, "download_url": ""}
	}
	ver := latestVersion(db, app)
	return map[string]any{
		"download_type": ver.DownloadType,
		"download_url":  ver.DownloadURL,
		"version":       ver.Version,
	}
}
