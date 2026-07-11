package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"errors"
	"strconv"
	"strings"
)

// ============================================================================
// 应用信息类接口（更新地址 / 版本检测 / 卡密信息）
// ============================================================================

// GetUpdateInfo 获取更新地址（type 2）：返回更新方式与下载地址。
func GetUpdateInfo(appUUID string) (any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"download_type": app.DownloadType,
		"download_url":  app.DownloadURL,
	}, nil
}

// CheckVersion 检测最新版本（type 3）：比对客户端版本与应用版本。
func CheckVersion(appUUID, clientVersion string) (any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	needUpdate := compareVersion(clientVersion, app.Version) < 0
	return map[string]any{
		"latest_version": app.Version,
		"need_update":    needUpdate,
		"download_type":  app.DownloadType,
		"download_url":   app.DownloadURL,
	}, nil
}

// GetCardInfo 获取卡密信息（type 4）：返回卡密的状态与面值。
// 同时返回 mode/duration/points：时长模式看 duration，点数模式看 points。
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
	if err := db.Where("app_uuid = ? AND card_no = ?", app.UUID, cardNo).First(&card).Error; err != nil {
		return nil, errors.New("卡号不存在")
	}
	usedAt := ""
	if card.UsedAt != nil {
		usedAt = card.UsedAt.Format("2006-01-02 15:04:05")
	}
	return map[string]any{
		"card_no":     card.CardNo,
		"status":      card.Status,
		"status_text": cardStatusName(card.Status),
		"mode":        app.OperationMode,
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
