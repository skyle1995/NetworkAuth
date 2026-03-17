package admin

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"networkDev/controllers"
	"networkDev/models"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 全局变量
// ============================================================================

var appBaseController = controllers.NewBaseController()

// ============================================================================
// 页面处理器
// ============================================================================

// AppsFragmentHandler 应用列表页面片段处理器
func AppsFragmentHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "apps.html", gin.H{
		"Title": "应用程序",
	})
}

// ============================================================================
// API处理器
// ============================================================================

// AppsListHandler 应用列表API处理器
func AppsListHandler(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = 10
	}

	// 获取搜索参数
	search := strings.TrimSpace(c.Query("search"))

	// 构建查询
	db, ok := appBaseController.GetDB(c)
	if !ok {
		return
	}

	var apps []models.App
	var total int64

	query := db.Model(&models.App{})

	// 如果有搜索条件
	if search != "" {
		// 优化：使用精确匹配提升查询性能
		query = query.Where("name = ? OR uuid = ?", search, search)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		logrus.WithError(err).Error("Failed to count apps")
		appBaseController.HandleInternalError(c, "获取应用总数失败", err)
		return
	}

	// 分页查询
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&apps).Error; err != nil {
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

	logrus.WithField("app_uuid", app.UUID).Info("Successfully reset app secret")

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
		Name         string `json:"name"`
		Version      string `json:"version"`
		Status       int    `json:"status"`
		DownloadType int    `json:"download_type"`
		ForceUpdate  int    `json:"force_update"`
		DownloadURL  string `json:"download_url"`
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

	// 设置默认值
	if req.Version == "" {
		req.Version = "1.0.0"
	}

	logrus.WithFields(logrus.Fields{
		"name":          req.Name,
		"version":       req.Version,
		"status":        req.Status,
		"download_type": req.DownloadType,
		"download_url":  req.DownloadURL,
		"force_update":  req.ForceUpdate,
	}).Info("Received app create request")

	// 创建应用
	app := models.App{
		Name:         strings.TrimSpace(req.Name),
		Version:      req.Version,
		Status:       req.Status,
		DownloadType: req.DownloadType,
		DownloadURL:  strings.TrimSpace(req.DownloadURL),
		ForceUpdate:  req.ForceUpdate,
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
			"code": 1,
			"msg":  "创建应用失败",
		})
		return
	}

	// 为应用创建所有默认接口
	defaultAPITypes := models.GetDefaultAPITypes()

	// 批量创建默认接口
	for _, apiType := range defaultAPITypes {
		api := models.API{
			APIType:         apiType,
			AppUUID:         app.UUID,
			Status:          0,                    // 默认禁用
			SubmitAlgorithm: models.AlgorithmNone, // 默认不加密
			ReturnAlgorithm: models.AlgorithmNone, // 默认不加密
		}

		if err := tx.Create(&api).Error; err != nil {
			tx.Rollback()
			logrus.WithError(err).WithField("api_type", apiType).Error("Failed to create default API")
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 1,
				"msg":  "创建默认接口失败",
			})
			return
		}
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

	logrus.WithField("app_uuid", app.UUID).Info("Successfully created app with default APIs")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "创建成功",
		"data": app,
	})
}

// AppUpdateHandler 更新应用API处理器
func AppUpdateHandler(c *gin.Context) {
	var req struct {
		ID           uint   `json:"id"`
		Name         string `json:"name"`
		Version      string `json:"version"`
		Status       int    `json:"status"`
		DownloadType int    `json:"download_type"`
		DownloadURL  string `json:"download_url"`
		ForceUpdate  int    `json:"force_update"`
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
	app.Version = req.Version
	app.Status = req.Status
	app.DownloadType = req.DownloadType
	app.DownloadURL = strings.TrimSpace(req.DownloadURL)
	app.ForceUpdate = req.ForceUpdate

	if err := db.Save(&app).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "更新应用失败",
		})
		return
	}

	logrus.WithField("app_id", app.ID).Info("Successfully updated app")

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

	// 删除相关的API记录
	if err := tx.Where("app_uuid = ?", app.UUID).Delete(&models.API{}).Error; err != nil {
		tx.Rollback()
		logrus.WithError(err).Error("Failed to delete related APIs")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "删除相关接口失败",
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

	logrus.WithFields(logrus.Fields{
		"app_id":   app.ID,
		"app_uuid": app.UUID,
	}).Info("Successfully deleted app and related APIs")

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
	}).Info("App data updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "应用数据更新成功",
	})
}

// AppUpdateAnnouncementHandler 更新应用程序公告处理器
func AppUpdateAnnouncementHandler(c *gin.Context) {
	// 解析请求体
	var req struct {
		UUID         string `json:"uuid"`
		Announcement string `json:"announcement"`
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

	// 对公告内容进行base64编码
	encodedAnnouncement := base64.StdEncoding.EncodeToString([]byte(req.Announcement))

	// 更新应用的公告内容
	if err := db.Model(&app).Update("announcement", encodedAnnouncement).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app announcement")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "更新程序公告失败",
		})
		return
	}

	logrus.WithFields(logrus.Fields{
		"app_uuid": req.UUID,
		"app_name": app.Name,
	}).Info("App announcement updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "程序公告更新成功",
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
			"multi_open_scope": app.MultiOpenScope,
			"clean_interval":   app.CleanInterval,
			"check_interval":   app.CheckInterval,
			"multi_open_count": app.MultiOpenCount,
		},
	})
}

// AppUpdateMultiConfigHandler 更新应用多开配置处理器
func AppUpdateMultiConfigHandler(c *gin.Context) {
	// 解析请求体
	var req struct {
		UUID           string `json:"uuid"`
		LoginType      int    `json:"login_type"`
		MultiOpenScope int    `json:"multi_open_scope"`
		CleanInterval  int    `json:"clean_interval"`
		CheckInterval  int    `json:"check_interval"`
		MultiOpenCount int    `json:"multi_open_count"`
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
	if req.LoginType < 0 || req.LoginType > 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "登录方式参数无效",
		})
		return
	}
	if req.MultiOpenScope < 0 || req.MultiOpenScope > 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "多开范围参数无效",
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
			"msg":  "校验间隔必须大于0",
		})
		return
	}
	if req.MultiOpenCount < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 1,
			"msg":  "多开数量必须大于0",
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
		"login_type":       req.LoginType,
		"multi_open_scope": req.MultiOpenScope,
		"clean_interval":   req.CleanInterval,
		"check_interval":   req.CheckInterval,
		"multi_open_count": req.MultiOpenCount,
	}

	if err := db.Model(&app).Updates(updates).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app multi config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "更新多开配置失败",
		})
		return
	}

	logrus.WithFields(logrus.Fields{
		"app_uuid": req.UUID,
		"app_name": app.Name,
	}).Info("App multi config updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "多开配置更新成功",
	})
}

// AppGetBindConfigHandler 获取应用绑定配置处理器
func AppGetBindConfigHandler(c *gin.Context) {
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
		"msg":  "获取绑定配置成功",
		"data": gin.H{
			"machine_verify":         app.MachineVerify,
			"machine_rebind_enabled": app.MachineRebindEnabled,
			"machine_rebind_limit":   app.MachineRebindLimit,
			"machine_free_count":     app.MachineFreeCount,
			"machine_rebind_count":   app.MachineRebindCount,
			"machine_rebind_deduct":  app.MachineRebindDeduct,
			"ip_verify":              app.IPVerify,
			"ip_rebind_enabled":      app.IPRebindEnabled,
			"ip_rebind_limit":        app.IPRebindLimit,
			"ip_free_count":          app.IPFreeCount,
			"ip_rebind_count":        app.IPRebindCount,
			"ip_rebind_deduct":       app.IPRebindDeduct,
		},
	})
}

// AppUpdateBindConfigHandler 更新应用绑定配置处理器
func AppUpdateBindConfigHandler(c *gin.Context) {
	// 解析请求体
	var req struct {
		UUID                 string `json:"uuid"`
		MachineVerify        int    `json:"machine_verify"`
		MachineRebindEnabled int    `json:"machine_rebind_enabled"`
		MachineRebindLimit   int    `json:"machine_rebind_limit"`
		MachineFreeCount     int    `json:"machine_free_count"`
		MachineRebindCount   int    `json:"machine_rebind_count"`
		MachineRebindDeduct  int    `json:"machine_rebind_deduct"`
		IPVerify             int    `json:"ip_verify"`
		IPRebindEnabled      int    `json:"ip_rebind_enabled"`
		IPRebindLimit        int    `json:"ip_rebind_limit"`
		IPFreeCount          int    `json:"ip_free_count"`
		IPRebindCount        int    `json:"ip_rebind_count"`
		IPRebindDeduct       int    `json:"ip_rebind_deduct"`
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

	// 更新绑定配置
	updates := map[string]interface{}{
		"machine_verify":         req.MachineVerify,
		"machine_rebind_enabled": req.MachineRebindEnabled,
		"machine_rebind_limit":   req.MachineRebindLimit,
		"machine_free_count":     req.MachineFreeCount,
		"machine_rebind_count":   req.MachineRebindCount,
		"machine_rebind_deduct":  req.MachineRebindDeduct,
		"ip_verify":              req.IPVerify,
		"ip_rebind_enabled":      req.IPRebindEnabled,
		"ip_rebind_limit":        req.IPRebindLimit,
		"ip_free_count":          req.IPFreeCount,
		"ip_rebind_count":        req.IPRebindCount,
		"ip_rebind_deduct":       req.IPRebindDeduct,
	}

	if err := db.Model(&app).Updates(updates).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app bind config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "更新绑定配置失败",
		})
		return
	}

	logrus.WithFields(logrus.Fields{
		"app_uuid": req.UUID,
		"app_name": app.Name,
	}).Info("App bind config updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "绑定配置更新成功",
	})
}

// AppGetRegisterConfigHandler 获取应用注册配置处理器
func AppGetRegisterConfigHandler(c *gin.Context) {
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
		"msg":  "获取注册配置成功",
		"data": gin.H{
			"register_enabled":       app.RegisterEnabled,
			"register_limit_enabled": app.RegisterLimitEnabled,
			"register_limit_time":    app.RegisterLimitTime,
			"register_count":         app.RegisterCount,
			"trial_enabled":          app.TrialEnabled,
			"trial_limit_time":       app.TrialLimitTime,
			"trial_duration":         app.TrialDuration,
		},
	})
}

// AppUpdateRegisterConfigHandler 更新应用注册配置处理器
func AppUpdateRegisterConfigHandler(c *gin.Context) {
	// 解析请求体
	var req struct {
		UUID                 string `json:"uuid"`
		RegisterEnabled      int    `json:"register_enabled"`
		RegisterLimitEnabled int    `json:"register_limit_enabled"`
		RegisterLimitTime    int    `json:"register_limit_time"`
		RegisterCount        int    `json:"register_count"`
		TrialEnabled         int    `json:"trial_enabled"`
		TrialLimitTime       int    `json:"trial_limit_time"`
		TrialDuration        int    `json:"trial_duration"`
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

	// 更新注册配置
	updates := map[string]interface{}{
		"register_enabled":       req.RegisterEnabled,
		"register_limit_enabled": req.RegisterLimitEnabled,
		"register_limit_time":    req.RegisterLimitTime,
		"register_count":         req.RegisterCount,
		"trial_enabled":          req.TrialEnabled,
		"trial_limit_time":       req.TrialLimitTime,
		"trial_duration":         req.TrialDuration,
	}

	if err := db.Model(&app).Updates(updates).Error; err != nil {
		logrus.WithError(err).Error("Failed to update app register config")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 1,
			"msg":  "更新注册配置失败",
		})
		return
	}

	logrus.WithFields(logrus.Fields{
		"app_uuid": req.UUID,
		"app_name": app.Name,
	}).Info("App register config updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "注册配置更新成功",
	})
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

	// 删除这些应用的所有相关接口
	if len(appUUIDs) > 0 {
		if err := tx.Where("app_uuid IN ?", appUUIDs).Delete(&models.API{}).Error; err != nil {
			tx.Rollback()
			logrus.WithError(err).Error("Failed to delete related APIs")
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 1,
				"msg":  "删除相关接口失败",
			})
			return
		}
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
	}).Info("Successfully batch deleted apps and related APIs")

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
	var apps []struct {
		ID   uint   `json:"id"`
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}

	if err := db.Model(&models.App{}).
		Select("id, uuid, name").
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
