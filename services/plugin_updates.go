package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const pluginManifestPrefix = "KTRWPLUGINPKG1|"

var pluginVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)
var pluginIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)
var updateNoncePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var targetVersionPattern = regexp.MustCompile(`^[0-9]{1,9}(\.[0-9]{1,9}){0,3}$`)
var targetPackagePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+$`)

// PluginUpdateRequest 描述一次带随机挑战值的批量更新查询。
type PluginUpdateRequest struct {
	Token   string `json:"token"`
	Nonce   string `json:"nonce"`
	Channel string `json:"channel"`
	Plugins []struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	} `json:"plugins"`
}

// SignedPluginManifest 保留 JSON 原文的字节序列，客户端先验签再解析。
type SignedPluginManifest struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

// PluginUpdateManifest 将查询挑战、有效期及全部发布字段绑定到同一签名。
type PluginUpdateManifest struct {
	Nonce     string                 `json:"nonce"`
	Channel   string                 `json:"channel"`
	ExpiresAt int64                  `json:"expires_at"`
	Updates   []PluginUpdateRelease  `json:"updates"`
	Revoked   []models.PluginRelease `json:"revoked"`
}

// PluginUpdateRelease 将发布信息与后台应用更新公告组合下发，不重复存储公告。
type PluginUpdateRelease struct {
	models.PluginRelease
	UpdateAnnouncement string `json:"update_announcement"`
}

// SignPluginManifest 用现有租约密钥签署完整更新清单，密钥不可用时拒绝返回。
func SignPluginManifest(manifest PluginUpdateManifest) (*SignedPluginManifest, error) {
	key, err := leasePrivateKey()
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	signature := ed25519.Sign(key, append([]byte(pluginManifestPrefix), raw...))
	return &SignedPluginManifest{
		Payload: base64.StdEncoding.EncodeToString(raw), Signature: hex.EncodeToString(signature),
	}, nil
}

// CheckPluginUpdates 按现有权益过滤发布记录，不申请激活或扣除资源。
func CheckPluginUpdates(req PluginUpdateRequest) (*SignedPluginManifest, error) {
	if !updateNoncePattern.MatchString(req.Nonce) || len(req.Plugins) > 64 {
		return nil, errors.New("更新查询参数无效")
	}
	if req.Channel != "stable" && req.Channel != "beta" {
		return nil, errors.New("不支持的更新渠道")
	}
	requested := make(map[string]string, len(req.Plugins))
	for _, plugin := range req.Plugins {
		if !pluginIDPattern.MatchString(plugin.ID) || !pluginVersionPattern.MatchString(plugin.Version) {
			return nil, errors.New("插件标识或版本无效")
		}
		if _, exists := requested[plugin.ID]; exists {
			return nil, errors.New("更新查询含重复插件")
		}
		requested[plugin.ID] = plugin.Version
	}
	ents, err := EntitlementsByToken(req.Token, "")
	if err != nil {
		return nil, err
	}
	visible := make(map[string]bool)
	for _, ent := range ents.Plugins {
		if ent.Usable == 1 && ent.PluginID != "" {
			visible[ent.PluginID] = true
		}
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		return nil, errors.New("数据库未初始化")
	}
	return pluginUpdatesFromDB(db, req, requested, visible, time.Now())
}

func pluginUpdatesFromDB(db *gorm.DB, req PluginUpdateRequest, requested map[string]string,
	visible map[string]bool, now time.Time) (*SignedPluginManifest, error) {
	ids := make([]string, 0, len(visible))
	for id := range visible {
		if len(requested) == 0 || requested[id] != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	manifest := PluginUpdateManifest{
		Nonce: req.Nonce, Channel: req.Channel, ExpiresAt: now.Add(15 * time.Minute).Unix(),
		Updates: []PluginUpdateRelease{}, Revoked: []models.PluginRelease{},
	}
	if len(ids) == 0 {
		return SignPluginManifest(manifest)
	}
	if len(ids) > 64 {
		return nil, errors.New("可用插件过多，请指定要检查的插件")
	}
	var releases []models.PluginRelease
	if err := db.Where("plugin_id IN ? AND channel = ? AND published_at <= ?", ids, req.Channel, now.Unix()).
		Order("plugin_id ASC").Find(&releases).Error; err != nil {
		return nil, err
	}
	latest := make(map[string]models.PluginRelease)
	for _, release := range releases {
		if release.Revoked {
			if requested[release.PluginID] == release.Version {
				// 撤回响应无需携带下载地址及更新说明，减少签名清单体积。
				manifest.Revoked = append(manifest.Revoked, models.PluginRelease{
					PluginID: release.PluginID, Version: release.Version,
					Revoked: true, RevokedReason: release.RevokedReason,
				})
			}
			continue
		}
		if old, ok := latest[release.PluginID]; !ok || compareVersion(old.Version, release.Version) < 0 {
			latest[release.PluginID] = release
		}
	}
	// 公告由已有应用后台维护；按发布校验使用的应用主键顺序选择绑定应用。
	var apps []models.App
	if err := db.Select("plugin_id", "update_announcement").
		Where("plugin_id IN ? AND status = 1", ids).
		Order(clause.OrderByColumn{Column: clause.Column{Name: clause.PrimaryKey}}).
		Find(&apps).Error; err != nil {
		return nil, err
	}
	announcements := make(map[string]string, len(apps))
	for _, app := range apps {
		if app.PluginID == nil {
			continue
		}
		if _, exists := announcements[*app.PluginID]; !exists {
			announcements[*app.PluginID] = app.UpdateAnnouncement
		}
	}
	for _, id := range ids {
		release, exists := latest[id]
		if exists && (requested[id] == "" || compareVersion(requested[id], release.Version) < 0) {
			manifest.Updates = append(manifest.Updates, PluginUpdateRelease{
				PluginRelease: release, UpdateAnnouncement: announcements[id],
			})
		}
	}
	return SignPluginManifest(manifest)
}

// ValidatePluginRelease 校验发布约束；不支持的版本或兼容规则一律拒绝发布。
func ValidatePluginRelease(release *models.PluginRelease) error {
	if !pluginIDPattern.MatchString(release.PluginID) || !pluginVersionPattern.MatchString(release.Version) {
		return errors.New("插件 id 或版本格式无效（版本须为三段数字）")
	}
	if release.Channel != "stable" && release.Channel != "beta" {
		return errors.New("channel 须为 stable 或 beta")
	}
	// 空下载地址表示「只提示有更新，不提供在线下载」；此时无需校验摘要与大小。
	if release.URL != "" {
		u, err := url.Parse(release.URL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
			u.Fragment != "" || len(release.URL) > 2048 || strings.ContainsAny(release.URL, "\r\n") {
			return errors.New("发布下载地址必须是无凭据的 HTTPS URL（留空表示不提供下载）")
		}
		if release.Size < 76 || release.Size > 256*1024*1024 || len(release.SHA256) != 64 {
			return errors.New("包大小或摘要无效（最大 256 MiB）")
		}
		if hash, err := hex.DecodeString(release.SHA256); err != nil || len(hash) != sha256.Size {
			return errors.New("包摘要不是 SHA256")
		}
	}
	if release.MinModuleVersion != "" && !pluginVersionPattern.MatchString(release.MinModuleVersion) {
		return errors.New("最低模块版本格式无效")
	}
	// 目标包与最低目标版本都是可选项：留空表示不限制。
	if release.Target != "" && !targetPackagePattern.MatchString(release.Target) {
		return errors.New("目标包名无效")
	}
	if release.MinTargetVersion != "" && !targetVersionPattern.MatchString(release.MinTargetVersion) {
		return errors.New("最低目标版本格式无效")
	}
	// 协议、ABI、能力位同属可选项：留空（0）表示不限制，填写时才校验取值。
	if release.LoaderProto > 1 || release.DriverCaps > 0xffff {
		return errors.New("发布要求的协议或能力位不支持")
	}
	if release.ABI != "" && release.ABI != "arm64-v8a" {
		return errors.New("发布要求的 ABI 不支持")
	}
	// level 留空按 optional 处理，兼容只填版本号的最小发布。
	if release.Level == "" {
		release.Level = "optional"
	}
	switch release.Level {
	case "optional", "recommended", "required", "security":
	default:
		return errors.New("level 须为 optional、recommended、required 或 security")
	}
	if len(release.RevokedReason) > 500 {
		return errors.New("撤回原因过长")
	}
	return nil
}

// PublishPluginRelease 仅创建新发布；同版本内容不可覆盖，避免客户端缓存歧义。
func PublishPluginRelease(db *gorm.DB, release *models.PluginRelease) error {
	if err := ValidatePluginRelease(release); err != nil {
		return err
	}
	var app models.App
	if err := db.Where("plugin_id = ? AND status = 1", release.PluginID).First(&app).Error; err != nil {
		return errors.New("插件未绑定启用的授权应用")
	}
	release.ID = 0
	release.Revoked = false
	release.RevokedReason = ""
	release.PublishedAt = time.Now().Unix()
	if err := db.Create(release).Error; err != nil {
		return fmt.Errorf("发布失败（同渠道同版本不可重复发布）: %w", err)
	}
	return nil
}
