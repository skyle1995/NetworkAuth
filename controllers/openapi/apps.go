package openapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"NetworkAuth/database"
	"NetworkAuth/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// 应用（apps）
// ============================================================================

// appDTO 对外返回的应用信息。
// 默认**不返回 secret**：密钥只在创建时返回一次，之后想看请到后台或调重置接口，
// 避免只读密钥就能拿到全部应用的签名密钥。
type appDTO struct {
	UUID              string `json:"uuid"`
	Name              string `json:"name"`
	PluginID          string `json:"plugin_id"`
	Status            int    `json:"status"`
	FreeMode          int    `json:"free_mode"`
	BillTimeEnabled   int    `json:"bill_time_enabled"`
	BillPointsEnabled int    `json:"bill_points_enabled"`
	LevelGateEnabled  int    `json:"level_gate_enabled"`
	RequiredLevel     int    `json:"required_level"`
	BillingScope      int    `json:"billing_scope"`
	PointsChargeMode  int    `json:"points_charge_mode"`
	PointsPerLogin    float64 `json:"points_per_login"`
	PointsPeriodMinutes int   `json:"points_period_minutes"`
	PointsPerPeriod   float64 `json:"points_per_period"`
	PointsHeartbeatCharge int `json:"points_heartbeat_charge"`
	LeaseTTLMinutes   int    `json:"lease_ttl_minutes"`
	CreatedAt         string `json:"created_at"`
}

func toAppDTO(a *models.App) appDTO {
	d := appDTO{
		UUID:                  a.UUID,
		Name:                  a.Name,
		Status:                a.Status,
		FreeMode:              a.FreeMode,
		BillTimeEnabled:       a.BillTimeEnabled,
		BillPointsEnabled:     a.BillPointsEnabled,
		LevelGateEnabled:      a.LevelGateEnabled,
		RequiredLevel:         a.RequiredLevel,
		BillingScope:          a.BillingScope,
		PointsChargeMode:      a.PointsChargeMode,
		PointsPerLogin:        a.PointsPerLogin,
		PointsPeriodMinutes:   a.PointsPeriodMinutes,
		PointsPerPeriod:       a.PointsPerPeriod,
		PointsHeartbeatCharge: a.PointsHeartbeatCharge,
		LeaseTTLMinutes:       a.LeaseTTLMinutes,
		CreatedAt:             fmtTime(a.CreatedAt),
	}
	if a.PluginID != nil {
		d.PluginID = *a.PluginID
	}
	return d
}

// ListApps GET /api/openapi/apps
// 查询参数：search（名称或 uuid 精确）、status、page、limit
func ListApps(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	page, limit := pageParams(c)

	q := db.Model(&models.App{})
	if s := strings.TrimSpace(c.Query("search")); s != "" {
		q = q.Where("name = ? OR uuid = ?", s, s)
	}
	if v := strings.TrimSpace(c.Query("status")); v != "" {
		q = q.Where("status = ?", v)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		internal(c, "统计应用失败", err)
		return
	}
	var apps []models.App
	if err := q.Order("id ASC").Offset((page - 1) * limit).Limit(limit).Find(&apps).Error; err != nil {
		internal(c, "查询应用失败", err)
		return
	}
	list := make([]appDTO, 0, len(apps))
	for i := range apps {
		list = append(list, toAppDTO(&apps[i]))
	}
	ok(c, "success", gin.H{"list": list, "total": total, "page": page, "limit": limit})
}

// GetApp GET /api/openapi/apps/:uuid
func GetApp(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	var app models.App
	if err := db.Where("uuid = ?", strings.TrimSpace(c.Param("uuid"))).First(&app).Error; err != nil {
		notFound(c, "应用不存在")
		return
	}
	ok(c, "success", toAppDTO(&app))
}

// CreateApp POST /api/openapi/apps
//
// 请求：{ name, plugin_id?, status?, free_mode?, bill_time_enabled?, bill_points_enabled?,
//         level_gate_enabled?, required_level?, lease_ttl_minutes? }
//
// 计费三项都不传时默认「计时模式」，与后台新建应用的默认一致。
// 创建成功会返回 secret（只此一次）。
func CreateApp(c *gin.Context) {
	var req struct {
		Name              string `json:"name"`
		PluginID          *string `json:"plugin_id"`
		Status            *int   `json:"status"`
		FreeMode          *int   `json:"free_mode"`
		BillTimeEnabled   *bool  `json:"bill_time_enabled"`
		BillPointsEnabled *bool  `json:"bill_points_enabled"`
		LevelGateEnabled  *bool  `json:"level_gate_enabled"`
		RequiredLevel     *int   `json:"required_level"`
		LeaseTTLMinutes   *int   `json:"lease_ttl_minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		badRequest(c, "应用名称不能为空")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}

	var pluginID *string
	if req.PluginID != nil {
		if v := strings.TrimSpace(*req.PluginID); v != "" {
			var cnt int64
			if err := db.Model(&models.App{}).Where("plugin_id = ?", v).Count(&cnt).Error; err != nil {
				internal(c, "校验插件标识失败", err)
				return
			}
			if cnt > 0 {
				badRequest(c, "插件标识已被其他应用占用: "+v)
				return
			}
			pluginID = &v
		}
	}

	app := models.App{
		UUID:              strings.ToUpper(uuid.New().String()),
		Name:              name,
		PluginID:          pluginID,
		Status:            1,
		FreeMode:          intOr(req.FreeMode, 0),
		BillTimeEnabled:   intFlag(req.BillTimeEnabled, 1),
		BillPointsEnabled: intFlag(req.BillPointsEnabled, 0),
		LevelGateEnabled:  intFlag(req.LevelGateEnabled, 0),
		RequiredLevel:     intOr(req.RequiredLevel, 1),
		PointsChargeMode:  models.PointsChargePerCount,
		PointsPerLogin:    1,
		PointsPerPeriod:   1,
		PointsPeriodMinutes: 60,
		LeaseTTLMinutes:   intOr(req.LeaseTTLMinutes, 30),
	}
	if req.Status != nil {
		app.Status = *req.Status
	}
	// 兼容展示值：由三个开关推导
	app.BillingMode = models.DeriveBillingMode(
		app.BillTimeEnabled == 1, app.BillPointsEnabled == 1, app.LevelGateEnabled == 1)

	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		internal(c, "生成密钥失败", err)
		return
	}
	app.Secret = strings.ToUpper(hex.EncodeToString(secret))

	if err := db.Create(&app).Error; err != nil {
		internal(c, "创建应用失败", err)
		return
	}

	logOp(c, "API创建应用", "创建了应用: "+app.Name)
	out := toAppDTO(&app)
	ok(c, "创建成功", gin.H{"app": out, "secret": app.Secret})
}

// UpdateAppBilling PUT /api/openapi/apps/:uuid/billing
//
// 只改计费相关字段，其余保持不动。字段留空表示不修改。
func UpdateAppBilling(c *gin.Context) {
	var req struct {
		FreeMode          *int     `json:"free_mode"`
		BillTimeEnabled   *bool    `json:"bill_time_enabled"`
		BillPointsEnabled *bool    `json:"bill_points_enabled"`
		LevelGateEnabled  *bool    `json:"level_gate_enabled"`
		RequiredLevel     *int     `json:"required_level"`
		BillingScope      *int     `json:"billing_scope"`
		PointsChargeMode  *int     `json:"points_charge_mode"`
		PointsPerLogin    *float64 `json:"points_per_login"`
		PointsPeriodMinutes *int   `json:"points_period_minutes"`
		PointsPerPeriod   *float64 `json:"points_per_period"`
		PointsHeartbeatCharge *int `json:"points_heartbeat_charge"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	var app models.App
	if err := db.Where("uuid = ?", strings.TrimSpace(c.Param("uuid"))).First(&app).Error; err != nil {
		notFound(c, "应用不存在")
		return
	}

	// 未提交的开关沿用当前值，保证推导出来的 billing_mode 与真实状态一致
	timeOn := app.BillTimeEnabled == 1
	pointsOn := app.BillPointsEnabled == 1
	levelOn := app.LevelGateEnabled == 1
	if req.BillTimeEnabled != nil {
		timeOn = *req.BillTimeEnabled
	}
	if req.BillPointsEnabled != nil {
		pointsOn = *req.BillPointsEnabled
	}
	if req.LevelGateEnabled != nil {
		levelOn = *req.LevelGateEnabled
	}

	updates := map[string]interface{}{
		"bill_time_enabled":   boolInt(timeOn),
		"bill_points_enabled": boolInt(pointsOn),
		"level_gate_enabled":  boolInt(levelOn),
		"billing_mode":        models.DeriveBillingMode(timeOn, pointsOn, levelOn),
	}
	if req.FreeMode != nil {
		updates["free_mode"] = *req.FreeMode
	}
	if req.RequiredLevel != nil {
		v := *req.RequiredLevel
		if v < 1 {
			v = 1
		}
		updates["required_level"] = v
	}
	if req.BillingScope != nil {
		updates["billing_scope"] = *req.BillingScope
	}
	if req.PointsChargeMode != nil {
		updates["points_charge_mode"] = *req.PointsChargeMode
	}
	if req.PointsPerLogin != nil {
		v := *req.PointsPerLogin
		if v < 0 {
			v = 0
		}
		updates["points_per_login"] = v
	}
	if req.PointsPerPeriod != nil {
		v := *req.PointsPerPeriod
		if v < 1 {
			v = 1
		}
		updates["points_per_period"] = v
	}
	if req.PointsPeriodMinutes != nil {
		v := *req.PointsPeriodMinutes
		if v < 1 {
			v = 60
		}
		updates["points_period_minutes"] = v
	}
	if req.PointsHeartbeatCharge != nil {
		updates["points_heartbeat_charge"] = *req.PointsHeartbeatCharge
	}

	if err := db.Model(&app).Updates(updates).Error; err != nil {
		internal(c, "更新计费策略失败", err)
		return
	}
	if err := db.Where("uuid = ?", app.UUID).First(&app).Error; err != nil {
		internal(c, "读取应用失败", err)
		return
	}
	logOp(c, "API调整计费", "调整了应用计费策略: "+app.Name)
	ok(c, "已保存", toAppDTO(&app))
}

// ResetAppSecret POST /api/openapi/apps/:uuid/reset-secret
// 重置签名密钥，旧密钥立即失效（需要 app:write）。
func ResetAppSecret(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	var app models.App
	if err := db.Where("uuid = ?", strings.TrimSpace(c.Param("uuid"))).First(&app).Error; err != nil {
		notFound(c, "应用不存在")
		return
	}
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		internal(c, "生成密钥失败", err)
		return
	}
	app.Secret = strings.ToUpper(hex.EncodeToString(secret))
	if err := db.Model(&app).Update("secret", app.Secret).Error; err != nil {
		internal(c, "重置密钥失败", err)
		return
	}
	logOp(c, "API重置密钥", "重置了应用密钥: "+app.Name)
	ok(c, "已重置", gin.H{"uuid": app.UUID, "secret": app.Secret})
}

// DeleteApp DELETE /api/openapi/apps/:uuid
//
// 删除应用并清理**该应用专属**的数据；账号、卡密、接口定义、变量、函数都是全站资源，不受影响。
// 清理范围：激活记录、会员日志、版本列表、等级点数、账号级周期账本。
func DeleteApp(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		internal(c, "数据库不可用", err)
		return
	}
	var app models.App
	if err := db.Where("uuid = ?", strings.TrimSpace(c.Param("uuid"))).First(&app).Error; err != nil {
		notFound(c, "应用不存在")
		return
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		scoped := []any{
			&models.AppActivation{},     // 该应用的激活/租约
			&models.MemberLog{},         // 该应用产生的账号事件日志
			&models.AppVersion{},        // 版本列表
			&models.AppLevelPoint{},     // 等级点数配置
			&models.AppAccountBilling{}, // 账号合并计费的周期账本
		}
		for _, m := range scoped {
			if err := tx.Where("app_uuid = ?", app.UUID).Delete(m).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&app).Error
	})
	if err != nil {
		internal(c, "删除应用失败", err)
		return
	}

	logOp(c, "API删除应用", "删除了应用: "+app.Name)
	ok(c, "已删除", gin.H{"uuid": app.UUID})
}
// ============================================================================
// 小工具
// ============================================================================

func intOr(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// isNotFound 判断 gorm 的「记录不存在」
func isNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
