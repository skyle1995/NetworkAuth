package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// cascadeDeleteAppData 在事务内删除一批应用的**专属**运行数据：
// 会话与调用审计日志。
//
// 一号通注意：账号(members)、卡密(cards)、接口(apis)、公共变量(variables)、
// 公共函数(functions) 都是全站共享资产，**绝不随应用删除** ——
// 删除应用只应影响「该应用专属」的数据，否则删一个应用就会清空全站会员、
// 卡池、接口定义以及所有插件共用的变量与函数。
func cascadeDeleteAppData(tx *gorm.DB, appUUIDs []string) error {
	if len(appUUIDs) == 0 {
		return nil
	}
	// 只清「属于这个应用」的记录：变量与函数是全站共用资源，删应用不影响它们
	derived := []any{
		&models.AppActivation{}, // 该应用的激活记录
		&models.MemberLog{},     // 该应用产生的审计日志
	}
	for _, m := range derived {
		if err := tx.Where("app_uuid IN ?", appUUIDs).Delete(m).Error; err != nil {
			return err
		}
	}
	return nil
}

// ============================================================================
// 全局变量
// ============================================================================

var appBaseController = controllers.NewBaseController()

// intBool 布尔转 0/1，保持后台接口字段与旧前端一致。
func intBool(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ============================================================================
// API处理器
// ============================================================================

// AppsListHandler 应用列表API处理器
func AppsListHandler(c *gin.Context) {
	// 获取分页参数
	page, limit := appBaseController.GetPaginationParams(c)

	// 获取搜索参数
	search := strings.TrimSpace(c.Query("search"))

	// 构建查询
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	query := db.Model(&models.App{})

	// 如果有搜索条件
	if search != "" {
		// 优化：使用精确匹配提升查询性能
		query = query.Where("name = ? OR uuid = ?", search, search)
	}

	// 泛型分页查询
	apps, total, err := services.Paginate[models.App](query, page, limit, "created_at DESC")
	if err != nil {
		logrus.WithError(err).Error("Failed to query apps")
		appBaseController.HandleInternalError(c, "查询应用列表失败", err)
		return
	}

	// 返回结果
	response := gin.H{
		"code":  0,
		"msg":   "success",
		"count": total,
		"data":  apps,
	}

	c.JSON(http.StatusOK, response)
}

// AppGetAppDataHandler 获取应用数据处理器
func AppGetAppDataHandler(c *gin.Context) {
	// 获取UUID参数
	uuid := c.Query("uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用UUID不能为空",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查找应用
	var app models.App
	if err := db.Where("uuid = ?", uuid).First(&app).Error; err != nil {
		logrus.WithError(err).Error("Failed to find app")
		c.JSON(http.StatusNotFound, gin.H{
			"code": 1,
			"msg":  "应用不存在",
		})
		return
	}

	// 解码base64应用数据内容
	var appData string
	if app.AppData != "" {
		decodedBytes, err := base64.StdEncoding.DecodeString(app.AppData)
		if err != nil {
			logrus.WithError(err).Error("Failed to decode app data")
			// 如果解码失败，返回空字符串
			appData = ""
		} else {
			appData = string(decodedBytes)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "获取成功",
		"data": gin.H{
			"app_data": appData,
		},
	})
}

// AppGetAnnouncementHandler 获取公告处理器
func AppGetAnnouncementHandler(c *gin.Context) {
	// 获取UUID参数
	uuid := c.Query("uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用UUID不能为空",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查找应用
	var app models.App
	if err := db.Where("uuid = ?", uuid).First(&app).Error; err != nil {
		logrus.WithError(err).Error("Failed to find app")
		c.JSON(http.StatusNotFound, gin.H{
			"code": 1,
			"msg":  "应用不存在",
		})
		return
	}

	// 解码base64公告内容
	var announcement string
	if app.Announcement != "" {
		decodedBytes, err := base64.StdEncoding.DecodeString(app.Announcement)
		if err != nil {
			logrus.WithError(err).Error("Failed to decode announcement")
			// 如果解码失败，返回空字符串
			announcement = ""
		} else {
			announcement = string(decodedBytes)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "获取成功",
		"data": gin.H{
			"announcement": announcement,
		},
	})
}

// AppResetSecretHandler 重置应用密钥处理器
func AppResetSecretHandler(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid"`
	}

	if !appBaseController.BindJSON(c, &req) {
		return
	}

	// 验证必填字段
	if strings.TrimSpace(req.UUID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用UUID不能为空",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查找应用
	var app models.App
	if err := db.Where("uuid = ?", req.UUID).First(&app).Error; err != nil {
		logrus.WithError(err).Error("Failed to find app")
		c.JSON(http.StatusNotFound, gin.H{
			"code": 1,
			"msg":  "应用不存在",
		})
		return
	}

	// 生成新的密钥
	bytes := make([]byte, 16) // 16字节 = 32位16进制字符
	rand.Read(bytes)
	newSecret := strings.ToUpper(hex.EncodeToString(bytes))

	// 更新密钥
	if err := db.Model(&app).Update("secret", newSecret).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app secret")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "重置密钥失败",
		})
		return
	}

	logrus.WithField("app_uuid", app.UUID).Debug("Successfully reset app secret")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "重置成功",
		"data": gin.H{
			"secret": newSecret,
		},
	})
}

// AppCreateHandler 创建应用API处理器
func AppCreateHandler(c *gin.Context) {
	var req struct {
		Name                  string  `json:"name"`
		PluginID              *string `json:"plugin_id"`
		Status                int    `json:"status"`
		FreeMode              int    `json:"free_mode"`
		BillingMode           int    `json:"billing_mode"`
		RequiredLevel         int    `json:"required_level"`
		BillingScope          int    `json:"billing_scope"`
		PointsChargeMode      int    `json:"points_charge_mode"`
		PointsPerLogin        float64 `json:"points_per_login"`
		PointsPeriodMinutes   int    `json:"points_period_minutes"`
		PointsPerPeriod       float64 `json:"points_per_period"`
		PointsHeartbeatCharge int    `json:"points_heartbeat_charge"`

		// 外壳 + 插件（二级令牌）
		LeaseTTLMinutes int    `json:"lease_ttl_minutes"` // 激活租约时长，默认 30
	}

	if !appBaseController.BindJSON(c, &req) {
		return
	}

	// 验证必填字段
	if strings.TrimSpace(req.Name) == "" {
		logrus.Error("App name is empty")
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用名称不能为空",
		})
		return
	}

	// 计费字段兜底：创建时不再填计费策略（前端已移出表单），
	// 全部走这里的默认值，后续在「应用管理 → 计费策略」里调整。
	if req.RequiredLevel < 1 {
		req.RequiredLevel = 1
	}
	if req.PointsPerLogin < 0 {
		req.PointsPerLogin = 0
	}
	if req.PointsPerPeriod < 1 {
		req.PointsPerPeriod = 1
	}
	if req.PointsPeriodMinutes < 1 {
		req.PointsPeriodMinutes = 60
	}
	if req.LeaseTTLMinutes < 1 {
		req.LeaseTTLMinutes = 30
	}

	logrus.WithFields(logrus.Fields{
		"name":          req.Name,
		"status":        req.Status,
	}).Debug("Received app create request")

	// 创建应用
	var pluginID *string
	if req.PluginID != nil {
		if trimmed := strings.TrimSpace(*req.PluginID); trimmed != "" {
			pluginID = &trimmed
		}
	}

	app := models.App{
		Name:                  strings.TrimSpace(req.Name),
		PluginID:              pluginID,
		Status:                req.Status,
		FreeMode:              req.FreeMode,
		BillingMode:           req.BillingMode,
		RequiredLevel:         req.RequiredLevel,
		BillingScope:          req.BillingScope,
		PointsChargeMode:      req.PointsChargeMode,
		PointsPerLogin:        req.PointsPerLogin,
		PointsPeriodMinutes:   req.PointsPeriodMinutes,
		PointsPerPeriod:       req.PointsPerPeriod,
		PointsHeartbeatCharge: req.PointsHeartbeatCharge,
		LeaseTTLMinutes:       req.LeaseTTLMinutes,
	}

	// 确保UUID和Secret被设置（虽然BeforeCreate钩子应该处理这些，但为了保险起见）
	if app.UUID == "" {
		app.UUID = strings.ToUpper(uuid.New().String())
	}
	if app.Secret == "" {
		// 生成32位大写16进制随机字符
		bytes := make([]byte, 16) // 16字节 = 32位16进制字符
		rand.Read(bytes)
		app.Secret = strings.ToUpper(hex.EncodeToString(bytes))
	}

	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 开始事务
	tx := db.Begin()
	if tx.Error != nil {
		logrus.WithError(tx.Error).Error("Failed to begin transaction")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "开始事务失败",
		})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建应用
	if err := tx.Create(&app).Error; err != nil {
		tx.Rollback()
		logrus.WithError(err).Error("Failed to create app")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  1,
			"msg":   "创建应用失败: " + err.Error(),
			"error": err.Error(),
		})
		return
	}

	// 一号通：接口条目全站唯一，创建应用时不再生成接口副本。
	// 27 条全局接口由安装流程与启动时的 services.EnsureGlobalAPIs() 负责，
	// 新建应用天然复用同一套接口定义，应用身份由 app_uuid + secret 签名区分。

	// 提交事务

	if err := tx.Commit().Error; err != nil {
		logrus.WithError(err).Error("Failed to commit transaction")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "提交事务失败",
		})
		return
	}

	// 记录操作日志
	operator := c.GetString("admin_username")
	if operator == "" {
		operator = "unknown"
	}
	operatorUUID := c.GetString("admin_uuid")

	services.RecordOperationLog(
		"创建应用",
		operator,
		operatorUUID,
		"创建了应用: "+app.Name,
	)

	logrus.WithField("app_uuid", app.UUID).Debug("Successfully created app with default APIs")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "创建成功",
		"data": app,
	})
}

// AppUpdateHandler 更新应用API处理器
func AppUpdateHandler(c *gin.Context) {
	var req struct {
		ID                    uint    `json:"id"`
		Name                  string  `json:"name"`
		PluginID              *string `json:"plugin_id"`
		Status                int    `json:"status"`
		FreeMode              int    `json:"free_mode"`
		BillingMode           int    `json:"billing_mode"`
		RequiredLevel         int    `json:"required_level"`
		BillingScope          int    `json:"billing_scope"`
		PointsChargeMode      int    `json:"points_charge_mode"`
		PointsPerLogin        float64 `json:"points_per_login"`
		PointsPeriodMinutes   int    `json:"points_period_minutes"`
		PointsPerPeriod       float64 `json:"points_per_period"`
		PointsHeartbeatCharge int    `json:"points_heartbeat_charge"`

		// 外壳 + 插件（二级令牌）
		LeaseTTLMinutes int    `json:"lease_ttl_minutes"` // 激活租约时长，默认 30
	}

	if !appBaseController.BindJSON(c, &req) {
		return
	}

	// 验证必填字段
	if req.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用ID不能为空",
		})
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用名称不能为空",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查找应用
	var app models.App
	if err := db.First(&app, req.ID).Error; err != nil {
		logrus.WithError(err).Error("Failed to find app")
		c.JSON(http.StatusNotFound, gin.H{
			"code": 1,
			"msg":  "应用不存在",
		})
		return
	}

	// 更新应用信息
	app.Name = strings.TrimSpace(req.Name)
	if req.PluginID != nil {
		if trimmed := strings.TrimSpace(*req.PluginID); trimmed != "" {
			app.PluginID = &trimmed
		} else {
			app.PluginID = nil
		}
	}
	app.Status = req.Status
	app.FreeMode = req.FreeMode
	app.BillingMode = req.BillingMode
	app.RequiredLevel = req.RequiredLevel
	app.BillingScope = req.BillingScope
	app.PointsChargeMode = req.PointsChargeMode
	app.PointsPerLogin = req.PointsPerLogin
	app.PointsPeriodMinutes = req.PointsPeriodMinutes
	app.PointsPerPeriod = req.PointsPerPeriod
	app.PointsHeartbeatCharge = req.PointsHeartbeatCharge
	if req.LeaseTTLMinutes > 0 {
		app.LeaseTTLMinutes = req.LeaseTTLMinutes
	}

	if err := db.Save(&app).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "更新应用失败",
		})
		return
	}

	logrus.WithField("app_id", app.ID).Debug("Successfully updated app")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "更新成功",
		"data": app,
	})
}

// AppDeleteHandler 删除应用处理器
func AppDeleteHandler(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}

	if !appBaseController.BindJSON(c, &req) {
		return
	}

	// 验证必填字段
	if req.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用ID不能为空",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查找应用
	var app models.App
	if err := db.First(&app, req.ID).Error; err != nil {
		logrus.WithError(err).Error("Failed to find app")
		c.JSON(http.StatusNotFound, gin.H{
			"code": 1,
			"msg":  "应用不存在",
		})
		return
	}

	// 开始事务
	tx := db.Begin()
	if tx.Error != nil {
		logrus.WithError(tx.Error).Error("Failed to begin transaction")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "开始事务失败",
		})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 级联删除该应用的所有衍生关联数据（接口/卡密/用户/绑定/会话/审计日志/变量/函数）
	if err := cascadeDeleteAppData(tx, []string{app.UUID}); err != nil {
		tx.Rollback()
		logrus.WithError(err).Error("Failed to cascade delete app data")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "删除关联数据失败",
		})
		return
	}

	// 删除应用
	if err := tx.Delete(&app).Error; err != nil {
		tx.Rollback()
		logrus.WithError(err).Error("Failed to delete app")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "删除应用失败",
		})
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logrus.WithError(err).Error("Failed to commit transaction")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "提交事务失败",
		})
		return
	}

	// 记录操作日志
	operator := c.GetString("admin_username")
	if operator == "" {
		operator = "unknown"
	}
	operatorUUID := c.GetString("admin_uuid")

	services.RecordOperationLog(
		"删除应用",
		operator,
		operatorUUID,
		"删除了应用: "+app.Name,
	)

	logrus.WithFields(logrus.Fields{
		"app_id":   app.ID,
		"app_uuid": app.UUID,
	}).Debug("Successfully deleted app and related APIs")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "删除成功",
	})
}

// AppUpdateAppDataHandler 更新应用数据处理器
func AppUpdateAppDataHandler(c *gin.Context) {
	// 解析请求体
	var req struct {
		UUID    string `json:"uuid"`
		AppData string `json:"app_data"`
	}

	if !appBaseController.BindJSON(c, &req) {
		return
	}

	// 验证UUID
	if req.UUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用UUID不能为空",
		})
		return
	}

	// 验证UUID格式
	if _, err := uuid.Parse(req.UUID); err != nil {
		logrus.WithError(err).Error("Invalid UUID format")
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "无效的UUID格式",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查找应用
	var app models.App
	if err := db.Where("uuid = ?", req.UUID).First(&app).Error; err != nil {
		logrus.WithError(err).Error("Failed to find app")
		c.JSON(http.StatusNotFound, gin.H{
			"code": 1,
			"msg":  "应用不存在",
		})
		return
	}

	// 对应用数据内容进行base64编码
	encodedAppData := base64.StdEncoding.EncodeToString([]byte(req.AppData))

	// 更新应用的数据内容
	if err := db.Model(&app).Update("app_data", encodedAppData).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app data")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "更新应用数据失败",
		})
		return
	}

	logrus.WithFields(logrus.Fields{
		"app_uuid": req.UUID,
		"app_name": app.Name,
	}).Debug("App data updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "应用数据更新成功",
	})
}


// AppGetMultiConfigHandler 获取应用多开配置处理器
func AppGetMultiConfigHandler(c *gin.Context) {
	appUUID := c.Query("uuid")
	if appUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用UUID不能为空",
		})
		return
	}

	// 验证UUID格式
	if _, err := uuid.Parse(appUUID); err != nil {
		logrus.WithError(err).Error("Invalid UUID format")
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "无效的UUID格式",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查找应用
	var app models.App
	if err := db.Where("uuid = ?", appUUID).First(&app).Error; err != nil {
		logrus.WithError(err).Error("Failed to find app")
		c.JSON(http.StatusNotFound, gin.H{
			"code": 1,
			"msg":  "应用不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "获取多开配置成功",
		"data": gin.H{
			"login_type":       app.LoginType,
				"clean_interval":   app.CleanInterval,
			"check_interval":   app.CheckInterval,
			"offline_timeout":  app.OfflineTimeout,
			},
	})
}

// AppUpdateMultiConfigHandler 更新应用多开配置处理器
func AppUpdateMultiConfigHandler(c *gin.Context) {
	// 解析请求体
	var req struct {
		UUID           string `json:"uuid"`
		LoginType      int    `json:"login_type"`
		CleanInterval  int    `json:"clean_interval"`
		CheckInterval  int    `json:"check_interval"`
		OfflineTimeout int    `json:"offline_timeout"`
	}

	if !appBaseController.BindJSON(c, &req) {
		return
	}

	// 验证UUID
	if req.UUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用UUID不能为空",
		})
		return
	}

	// 验证UUID格式
	if _, err := uuid.Parse(req.UUID); err != nil {
		logrus.WithError(err).Error("Invalid UUID format")
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "无效的UUID格式",
		})
		return
	}

	// 验证参数范围
	if req.LoginType < 0 || req.LoginType > 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "登录方式参数无效",
		})
		return
	}
	if req.CleanInterval < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "清理间隔必须大于0",
		})
		return
	}
	if req.CheckInterval < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "心跳间隔必须大于0",
		})
		return
	}
	if req.OfflineTimeout < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "自动离线时长必须大于0",
		})
		return
	}
	if req.OfflineTimeout < req.CheckInterval {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "自动离线时长应不小于心跳间隔",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查找应用
	var app models.App
	if err := db.Where("uuid = ?", req.UUID).First(&app).Error; err != nil {
		logrus.WithError(err).Error("Failed to find app")
		c.JSON(http.StatusNotFound, gin.H{
			"code": 1,
			"msg":  "应用不存在",
		})
		return
	}
	// 更新多开配置
	updates := map[string]interface{}{
		"login_type":      req.LoginType,
		"clean_interval":  req.CleanInterval,
		"check_interval":  req.CheckInterval,
		"offline_timeout": req.OfflineTimeout,
	}

	if err := db.Model(&app).Updates(updates).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app multi config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "更新多开配置失败",
		})
		return
	}

	logrus.WithField("app_uuid", req.UUID).Debug("App multi config updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "并发配置更新成功",
	})
}

// ============================================================================
// 账号策略（全站）
// ============================================================================
//
// 这两个接口是**历史路由名**：早期注册/试用/绑定策略按应用配置，路由挂在 /apps 下。
// 一号通改造后策略全部全局化（存 settings 的 account_* 键，见 services.GetAccountPolicy），
// 因此不再接收 uuid，读写的是同一份全站策略。
//
// 路由名保留是为了不破坏已对接的后台前端；语义上它等价于「全局账号策略」。

// AppGetRegisterConfigHandler 读取全站账号策略
func AppGetRegisterConfigHandler(c *gin.Context) {
	p := services.GetAccountPolicy()

	// 前端沿用 0/1 而非 bool，保持与其它配置项一致
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "获取账号策略成功",
		"data": gin.H{
			// 注册
			"register_enabled":       intBool(p.RegisterEnabled),
			"email_verify_enabled":   intBool(p.EmailVerifyEnabled),
			"card_register_enabled":  intBool(p.CardRegisterEnabled),
			// 注册限流
			"register_limit_enabled":        intBool(p.RegisterLimitEnabled),
			"register_device_limit_enabled": intBool(p.RegisterDeviceLimitEnabled),
			"register_limit_time":           p.RegisterLimitTime,
			"register_count":                p.RegisterCount,
			// 卡密
			"card_login_enabled": intBool(p.CardLoginEnabled),
			"recharge_enabled":   intBool(p.RechargeEnabled),
			// 试用
			"trial_enabled":          intBool(p.TrialEnabled),
			"trial_limit_time":       p.TrialLimitTime,
			"trial_points":           p.TrialPoints,
			"trial_duration_minutes": p.TrialDurationMinutes,
			"trial_claim_mode":       p.TrialClaimMode,
			// 绑定与转绑
			"machine_verify":    p.MachineVerify,
			"ip_verify":         p.IPVerify,
			"machine_rebind_enabled": p.MachineRebindEnabled,
			"ip_rebind_enabled":      p.IPRebindEnabled,
			"machine_rebind_free_count": p.MachineRebindFreeCount,
			"machine_rebind_limit_time": p.MachineRebindLimitTime,
			"machine_rebind_max_count":  p.MachineRebindMaxCount,
			"machine_rebind_deduct_minutes": p.MachineRebindDeductMinutes,
			"machine_rebind_deduct_points":  p.MachineRebindDeductPoints,
			"ip_rebind_free_count":      p.IPRebindFreeCount,
			"ip_rebind_limit_time":      p.IPRebindLimitTime,
			"ip_rebind_max_count":       p.IPRebindMaxCount,
			"ip_rebind_deduct_minutes": p.IPRebindDeductMinutes,
			"ip_rebind_deduct_points":  p.IPRebindDeductPoints,
			// 账号级资源配额（跨应用共用）
			"max_devices":       p.MaxDevices,
			"max_concurrent":    p.MaxConcurrent,
		},
	})
}

// AppUpdateRegisterConfigHandler 保存全站账号策略
func AppUpdateRegisterConfigHandler(c *gin.Context) {
	var req struct {
		RegisterEnabled            int     `json:"register_enabled"`
		EmailVerifyEnabled         int     `json:"email_verify_enabled"`
		CardRegisterEnabled        int     `json:"card_register_enabled"`
		RegisterLimitEnabled       int     `json:"register_limit_enabled"`
		RegisterDeviceLimitEnabled int     `json:"register_device_limit_enabled"`
		RegisterLimitTime          int     `json:"register_limit_time"`
		RegisterCount              int     `json:"register_count"`
		CardLoginEnabled           int     `json:"card_login_enabled"`
		RechargeEnabled            int     `json:"recharge_enabled"`
		TrialEnabled               int     `json:"trial_enabled"`
		TrialLimitTime             int     `json:"trial_limit_time"`
		TrialPoints                float64 `json:"trial_points"`
		TrialDurationMinutes       int     `json:"trial_duration_minutes"`
		TrialClaimMode             int     `json:"trial_claim_mode"`

		// 绑定与转绑
		MachineVerify  int `json:"machine_verify"`
		IPVerify       int `json:"ip_verify"`
		MachineRebindEnabled int `json:"machine_rebind_enabled"`
		IPRebindEnabled      int `json:"ip_rebind_enabled"`
		MachineRebindFreeCount int `json:"machine_rebind_free_count"`
		MachineRebindLimitTime int `json:"machine_rebind_limit_time"`
		MachineRebindMaxCount  int `json:"machine_rebind_max_count"`
		MachineRebindDeductMinutes int `json:"machine_rebind_deduct_minutes"`
		MachineRebindDeductPoints  int `json:"machine_rebind_deduct_points"`
		IPRebindFreeCount      int `json:"ip_rebind_free_count"`
		IPRebindLimitTime      int `json:"ip_rebind_limit_time"`
		IPRebindMaxCount       int `json:"ip_rebind_max_count"`
		IPRebindDeductMinutes int `json:"ip_rebind_deduct_minutes"`
		IPRebindDeductPoints  int `json:"ip_rebind_deduct_points"`

		// 账号级资源配额
		MaxDevices      int `json:"max_devices"`
		MaxConcurrent   int `json:"max_concurrent"`
	}

	if !appBaseController.BindJSON(c, &req) {
		return
	}

	// 关键配额不允许为 0：注册次数、设备数、并发数配成 0 会直接把账号锁死
	if req.RegisterCount < 1 {
		appBaseController.HandleValidationError(c, "同一窗口注册次数至少为 1")
		return
	}
	if req.MaxDevices < 1 {
		appBaseController.HandleValidationError(c, "可绑定设备数至少为 1")
		return
	}
	if req.MaxConcurrent < 1 {
		appBaseController.HandleValidationError(c, "同时在线设备数至少为 1")
		return
	}
	if req.MachineRebindFreeCount < 0 || req.MachineRebindMaxCount < 0 ||
		req.MachineRebindDeductMinutes < 0 || req.MachineRebindDeductPoints < 0 ||
		req.IPRebindFreeCount < 0 || req.IPRebindMaxCount < 0 ||
		req.IPRebindDeductMinutes < 0 || req.IPRebindDeductPoints < 0 {
		appBaseController.HandleValidationError(c, "转绑次数与扣除不能为负")
		return
	}

	items := map[string]string{
		models.SettingAccountRegisterEnabled:            strconv.Itoa(req.RegisterEnabled),
		models.SettingAccountEmailVerifyEnabled:         strconv.Itoa(req.EmailVerifyEnabled),
		models.SettingAccountCardRegisterEnabled:        strconv.Itoa(req.CardRegisterEnabled),
		models.SettingAccountRegisterLimitEnabled:       strconv.Itoa(req.RegisterLimitEnabled),
		models.SettingAccountRegisterDeviceLimitEnabled: strconv.Itoa(req.RegisterDeviceLimitEnabled),
		models.SettingAccountRegisterLimitTime:          strconv.Itoa(req.RegisterLimitTime),
		models.SettingAccountRegisterCount:              strconv.Itoa(req.RegisterCount),
		models.SettingAccountCardLoginEnabled:           strconv.Itoa(req.CardLoginEnabled),
		models.SettingAccountRechargeEnabled:            strconv.Itoa(req.RechargeEnabled),
		models.SettingAccountTrialEnabled:               strconv.Itoa(req.TrialEnabled),
		models.SettingAccountTrialLimitTime:             strconv.Itoa(req.TrialLimitTime),
		models.SettingAccountTrialPoints:                strconv.FormatFloat(req.TrialPoints, 'f', -1, 64),
		models.SettingAccountTrialDurationMinutes:       strconv.Itoa(req.TrialDurationMinutes),
		models.SettingAccountTrialClaimMode:             strconv.Itoa(req.TrialClaimMode),
		models.SettingAccountMachineVerify:              strconv.Itoa(req.MachineVerify),
		models.SettingAccountIPVerify:                   strconv.Itoa(req.IPVerify),
		models.SettingAccountMachineRebindEnabled:       strconv.Itoa(req.MachineRebindEnabled),
		models.SettingAccountIPRebindEnabled:            strconv.Itoa(req.IPRebindEnabled),
		models.SettingAccountMachineRebindFreeCount:     strconv.Itoa(req.MachineRebindFreeCount),
		models.SettingAccountMachineRebindLimitTime:     strconv.Itoa(req.MachineRebindLimitTime),
		models.SettingAccountMachineRebindMaxCount:      strconv.Itoa(req.MachineRebindMaxCount),
		models.SettingAccountMachineRebindDeductMinutes: strconv.Itoa(req.MachineRebindDeductMinutes),
		models.SettingAccountMachineRebindDeductPoints:  strconv.Itoa(req.MachineRebindDeductPoints),
		models.SettingAccountIPRebindFreeCount:          strconv.Itoa(req.IPRebindFreeCount),
		models.SettingAccountIPRebindLimitTime:          strconv.Itoa(req.IPRebindLimitTime),
		models.SettingAccountIPRebindMaxCount:           strconv.Itoa(req.IPRebindMaxCount),
		models.SettingAccountIPRebindDeductMinutes: strconv.Itoa(req.IPRebindDeductMinutes),
		models.SettingAccountIPRebindDeductPoints:  strconv.Itoa(req.IPRebindDeductPoints),
		models.SettingAccountMaxDevices:                 strconv.Itoa(req.MaxDevices),
		models.SettingAccountMaxConcurrent:              strconv.Itoa(req.MaxConcurrent),
	}

	if err := services.SaveAccountPolicy(items); err != nil {
		logrus.WithError(err).Error("Failed to save account policy")
		appBaseController.HandleInternalError(c, "保存账号策略失败", err)
		return
	}

	if claims, _, err := GetCurrentAdminUserWithRefresh(c); err == nil && claims != nil {
		services.RecordOperationLog("账号策略", claims.Username, claims.UUID,
			"更新了全站账号策略（注册/卡密/试用/绑定/配额）")
	}
	appBaseController.HandleSuccess(c, "账号策略已保存，全站立即生效", nil)
}

// AppsBatchDeleteHandler 批量删除应用处理器
func AppsBatchDeleteHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}


	if !appBaseController.BindJSON(c, &req) {
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "请选择要删除的应用",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 开始事务
	tx := db.Begin()
	if tx.Error != nil {
		logrus.WithError(tx.Error).Error("Failed to begin transaction")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "开始事务失败",
		})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 首先获取要删除的应用的UUID列表
	var apps []models.App
	if err := tx.Where("id IN ?", req.IDs).Find(&apps).Error; err != nil {
		tx.Rollback()
		logrus.WithError(err).Error("Failed to find apps")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "查找应用失败",
		})
		return
	}

	// 提取UUID列表
	var appUUIDs []string
	for _, app := range apps {
		appUUIDs = append(appUUIDs, app.UUID)
	}

	// 级联删除这些应用的所有衍生关联数据
	if err := cascadeDeleteAppData(tx, appUUIDs); err != nil {
		tx.Rollback()
		logrus.WithError(err).Error("Failed to cascade delete apps data")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "删除关联数据失败",
		})
		return
	}

	// 批量删除应用
	if err := tx.Delete(&models.App{}, req.IDs).Error; err != nil {
		tx.Rollback()
		logrus.WithError(err).Error("Failed to batch delete apps")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "批量删除失败",
		})
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logrus.WithError(err).Error("Failed to commit transaction")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "提交事务失败",
		})
		return
	}

	logrus.WithFields(logrus.Fields{
		"app_ids":   req.IDs,
		"app_uuids": appUUIDs,
	}).Debug("Successfully batch deleted apps and related APIs")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "批量删除成功",
	})
}

// AppsBatchUpdateStatusHandler 批量更新应用状态处理器
func AppsBatchUpdateStatusHandler(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids"`
		Status int    `json:"status"`
	}

	if !appBaseController.BindJSON(c, &req) {
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "请选择要更新的应用",
		})
		return
	}

	if req.Status != 0 && req.Status != 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "状态值无效",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 批量更新状态
	if err := db.Model(&models.App{}).Where("id IN ?", req.IDs).Update("status", req.Status).Error; err != nil {
		logrus.WithError(err).Error("Failed to batch update app status")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "批量更新状态失败",
		})
		return
	}

	statusText := "禁用"
	if req.Status == 1 {
		statusText = "启用"
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "批量" + statusText + "成功",
	})
}

// AppUpdateStatusHandler 更新单个应用状态处理器
func AppUpdateStatusHandler(c *gin.Context) {
	var req struct {
		ID     uint `json:"id"`
		Status int  `json:"status"`
	}

	if !appBaseController.BindJSON(c, &req) {
		return
	}

	if req.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用ID不能为空",
		})
		return
	}

	if req.Status != 0 && req.Status != 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "状态值无效",
		})
		return
	}

	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 检查应用是否存在
	var app models.App
	if err := db.Where("id = ?", req.ID).First(&app).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "应用不存在",
		})
		return
	}

	// 更新状态
	if err := db.Model(&app).Update("status", req.Status).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app status")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "更新状态失败",
		})
		return
	}

	statusText := "禁用"
	if req.Status == 1 {
		statusText = "启用"
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "应用" + statusText + "成功",
	})
}

// AppsSimpleListHandler 简化应用列表API处理器（用于下拉框选择等场景）
func AppsSimpleListHandler(c *gin.Context) {
	// 获取数据库连接
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查询所有启用的应用，只获取必要字段
	// 一号通：账号不再归属应用，此处保留计费模式供前端按需展示/筛选
	var apps []struct {
		ID          uint   `json:"id"`
		UUID        string `json:"uuid"`
		Name        string `json:"name"`
		BillingMode int    `json:"billing_mode"`
		FreeMode    int    `json:"free_mode"`
	}

	if err := db.Model(&models.App{}).
		Select("id, uuid, name, billing_mode, free_mode").
		Where("status = ?", 1). // 只获取启用的应用
		Order("name ASC").
		Find(&apps).Error; err != nil {
		logrus.WithError(err).Error("Failed to query simple apps list")
		appBaseController.HandleInternalError(c, "获取应用列表失败", err)
		return
	}

	// 返回结果
	response := gin.H{
		"code": 0,
		"msg":  "success",
		"data": apps,
	}

	c.JSON(http.StatusOK, response)
}

// ============================================================================
// 高级模式：等级点数配置
// ============================================================================

// AppGetLevelPointsHandler 读取某应用的等级点数配置。
// 返回全部会员等级及其在该应用的点数消耗（未配置的等级返回 app 的默认值并标记 configured=false）。
func AppGetLevelPointsHandler(c *gin.Context) {
	appUUID := strings.TrimSpace(c.Query("uuid"))
	if appUUID == "" {
		appBaseController.HandleValidationError(c, "应用UUID不能为空")
		return
	}
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}
	var app models.App
	if err := db.Where("uuid = ?", appUUID).First(&app).Error; err != nil {
		appBaseController.HandleValidationError(c, "应用不存在")
		return
	}
	levels, err := services.ListMemberLevels()
	if err != nil {
		appBaseController.HandleInternalError(c, "查询会员等级失败", err)
		return
	}
	rows, err := services.ListAppLevelPoints(appUUID)
	if err != nil {
		appBaseController.HandleInternalError(c, "查询等级点数失败", err)
		return
	}
	configured := make(map[string]float64, len(rows))
	for _, r := range rows {
		configured[r.MemberLevelUUID] = r.Points
	}

	type levelPointItem struct {
		LevelUUID  string `json:"level_uuid"`
		Name       string `json:"name"`
		Level      int    `json:"level"`
		Color      string `json:"color"`
		Points     float64    `json:"points"`
		Configured bool   `json:"configured"`
	}
	items := make([]levelPointItem, 0, len(levels))
	for _, lv := range levels {
		pts, ok := configured[lv.UUID]
		items = append(items, levelPointItem{
			LevelUUID:  lv.UUID,
			Name:       lv.Name,
			Level:      lv.Level,
			Color:      lv.Color,
			Points:     pts,
			Configured: ok,
		})
	}

	// 未配置的等级会回退到应用默认点数；默认值要按扣费方式取，
	// 按时模式对应 PointsPerPeriod，按次模式对应 PointsPerLogin。
	defaultPoints := app.PointsPerLogin
	if app.PointsChargeMode == models.PointsChargePerTime {
		defaultPoints = app.PointsPerPeriod
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"billing_mode":    app.BillingMode,
			"billing_name":    models.BillingModeName(app.BillingMode),
			"required_level":  app.RequiredLevel,
			"points_charge_mode": app.PointsChargeMode,
			"default_points":  defaultPoints,
			"levels":          items,
		},
	})
}

// AppSaveLevelPointsHandler 覆盖保存某应用的等级点数配置（高级模式用）。
// 请求体：{ "uuid": "...", "items": [{"level_uuid":"...","points":10}, ...] }
func AppSaveLevelPointsHandler(c *gin.Context) {
	var req struct {
		UUID  string `json:"uuid"`
		Items []struct {
			LevelUUID string `json:"level_uuid"`
			Points    float64    `json:"points"`
		} `json:"items"`
	}
	if !appBaseController.BindJSON(c, &req) {
		return
	}
	appUUID := strings.TrimSpace(req.UUID)
	if appUUID == "" {
		appBaseController.HandleValidationError(c, "应用UUID不能为空")
		return
	}
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}
	var app models.App
	if err := db.Where("uuid = ?", appUUID).First(&app).Error; err != nil {
		appBaseController.HandleValidationError(c, "应用不存在")
		return
	}

	items := make(map[string]float64, len(req.Items))
	for _, it := range req.Items {
		if strings.TrimSpace(it.LevelUUID) == "" {
			continue
		}
		if it.Points < 0 {
			appBaseController.HandleValidationError(c, "点数消耗不能为负")
			return
		}
		items[it.LevelUUID] = it.Points
	}
	if err := services.SaveAppLevelPoints(appUUID, items); err != nil {
		appBaseController.HandleInternalError(c, "保存等级点数失败", err)
		return
	}

	services.RecordOperationLog("保存等级点数", c.GetString("admin_username"),
		c.GetString("admin_uuid"), "应用 "+app.Name+" 共 "+strconv.Itoa(len(items))+" 个等级的计费点数")
	appBaseController.HandleSuccess(c, "保存成功", nil)
}
// ============================================================================
// 计费策略（应用级）
// ============================================================================
//
// 独立成页：创建应用时只填基本信息和内容分发，计费全部走默认值，
// 之后在「应用管理 → 计费策略」按需调整。这样新建应用不必一次面对
// 计费模式/等级/周期/单价十来个字段，也便于横向对比所有应用的定价。

// billingConfig 单个应用的计费配置（列表与更新共用同一组字段）
type billingConfig struct {
	UUID                  string  `json:"uuid"`
	Name                  string  `json:"name"`
	Status                int     `json:"status"`
	FreeMode              int     `json:"free_mode"`
	BillingMode           int     `json:"billing_mode"`
	RequiredLevel         int     `json:"required_level"`
	BillingScope          int     `json:"billing_scope"`
	PointsChargeMode      int     `json:"points_charge_mode"`
	PointsPerLogin        float64 `json:"points_per_login"`
	PointsPeriodMinutes   int     `json:"points_period_minutes"`
	PointsPerPeriod       float64 `json:"points_per_period"`
	PointsHeartbeatCharge int     `json:"points_heartbeat_charge"`
}

// AppBillingListHandler 列出所有应用的计费配置
func AppBillingListHandler(c *gin.Context) {
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	var apps []models.App
	if err := db.Order("name ASC").Find(&apps).Error; err != nil {
		logrus.WithError(err).Error("Failed to list app billing configs")
		appBaseController.HandleInternalError(c, "查询计费策略失败", err)
		return
	}

	list := make([]billingConfig, 0, len(apps))
	for i := range apps {
		a := &apps[i]
		list = append(list, billingConfig{
			UUID:                  a.UUID,
			Name:                  a.Name,
			Status:                a.Status,
			FreeMode:              a.FreeMode,
			BillingMode:           a.BillingMode,
			RequiredLevel:         a.RequiredLevel,
			BillingScope:          a.BillingScope,
			PointsChargeMode:      a.PointsChargeMode,
			PointsPerLogin:        a.PointsPerLogin,
			PointsPeriodMinutes:   a.PointsPeriodMinutes,
			PointsPerPeriod:       a.PointsPerPeriod,
			PointsHeartbeatCharge: a.PointsHeartbeatCharge,
		})
	}

	appBaseController.HandleSuccess(c, "success", list)
}

// AppUpdateBillingHandler 更新单个应用的计费配置（仅计费字段）
func AppUpdateBillingHandler(c *gin.Context) {
	var req billingConfig
	if !appBaseController.BindJSON(c, &req) {
		return
	}
	if !appBaseController.ValidateRequired(c, map[string]interface{}{
		"应用UUID": req.UUID,
	}) {
		return
	}

	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	var app models.App
	if err := db.Where("uuid = ?", strings.TrimSpace(req.UUID)).First(&app).Error; err != nil {
		appBaseController.HandleValidationError(c, "应用不存在")
		return
	}

	// 与创建/编辑时的校验口径保持一致
	if req.RequiredLevel < 1 {
		req.RequiredLevel = 1
	}
	if req.PointsPeriodMinutes < 1 {
		req.PointsPeriodMinutes = 60
	}
	if req.PointsPerLogin < 0 {
		req.PointsPerLogin = 0
	}
	if req.PointsPerPeriod < 1 {
		req.PointsPerPeriod = 1
	}

	updates := map[string]interface{}{
		"free_mode":               req.FreeMode,
		"billing_mode":            req.BillingMode,
		"required_level":          req.RequiredLevel,
		"billing_scope":           req.BillingScope,
		"points_charge_mode":      req.PointsChargeMode,
		"points_per_login":        req.PointsPerLogin,
		"points_period_minutes":   req.PointsPeriodMinutes,
		"points_per_period":       req.PointsPerPeriod,
		"points_heartbeat_charge": req.PointsHeartbeatCharge,
	}
	if err := db.Model(&models.App{}).Where("uuid = ?", app.UUID).Updates(updates).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app billing config")
		appBaseController.HandleInternalError(c, "保存计费策略失败", err)
		return
	}

	services.RecordOperationLog("更新计费策略", c.GetString("admin_username"),
		c.GetString("admin_uuid"), "应用 "+app.Name)
	appBaseController.HandleSuccess(c, "保存成功", nil)
}