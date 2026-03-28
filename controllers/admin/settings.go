package admin

import (
	"NetworkAuth/config"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"NetworkAuth/utils"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SubAccountSimpleListHandler 子账号简单列表API处理器 (Mock)
func SubAccountSimpleListHandler(c *gin.Context) {
	// Mock implementation for NetworkAuth which has no subaccounts
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": []interface{}{},
	})
}

// SettingsQueryHandler 设置查询API
// - 返回所有设置项的 name:value 映射
func SettingsQueryHandler(c *gin.Context) {
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	var list []models.Settings
	if err := db.Find(&list).Error; err != nil {
		authBaseController.HandleInternalError(c, "查询失败", err)
		return
	}
	res := map[string]string{}
	for _, s := range list {
		res[s.Name] = s.Value
	}
	authBaseController.HandleSuccess(c, "ok", res)
}

// SettingsUpdateHandler 更新系统设置处理器
// - 接收JSON格式的设置数据，支持两种格式：
//  1. 直接字段格式: {"site_title": "值", "site_keywords": "值"}
//  2. 嵌套格式: {"settings": {"site_title": "值", "site_keywords": "值"}}
//
// - 自动创建不存在的设置项
// - 更新已存在的设置项
// - 更新完成后：
//  1. 删除对应的Redis缓存键，确保后续读取走数据库并重建缓存
//  2. 刷新SettingsService内存缓存
func SettingsUpdateHandler(c *gin.Context) {
	// 先尝试解析为直接字段格式
	var directBody map[string]interface{}
	if err := c.ShouldBindJSON(&directBody); err != nil {
		authBaseController.HandleValidationError(c, "请求体错误")
		return
	}

	var categoryStr string
	if category, ok := directBody["category"].(string); ok {
		categoryStr = category
	}

	// 提取设置数据
	var settingsData map[string]string

	// 检查是否为嵌套格式（包含settings字段）
	if settings, exists := directBody["settings"]; exists {
		if settingsMap, ok := settings.(map[string]interface{}); ok {
			settingsData = make(map[string]string)
			for k, v := range settingsMap {
				if str, ok := v.(string); ok {
					settingsData[k] = str
				}
			}
		} else {
			authBaseController.HandleValidationError(c, "settings字段格式错误")
			return
		}
	} else {
		// 直接字段格式
		settingsData = make(map[string]string)
		for k, v := range directBody {
			if k == "category" {
				continue // 忽略 category 字段，不保存到设置表
			}
			if str, ok := v.(string); ok {
				settingsData[k] = str
			} else if v != nil {
				// 转换其他类型为字符串
				settingsData[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	if len(settingsData) == 0 {
		authBaseController.HandleValidationError(c, "无设置项")
		return
	}

	// 验证设置项值
	for k, v := range settingsData {
		if err := validateSettingValue(k, v); err != nil {
			authBaseController.HandleValidationError(c, err.Error())
			return
		}
	}

	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	// 记录需要失效的缓存键，统一删除，减少与Redis交互次数
	keysToDel := make([]string, 0, len(settingsData))

	// 批量处理设置项
	for k, v := range settingsData {
		var s models.Settings
		if err := db.Where("name = ?", k).First(&s).Error; err != nil {
			// 不存在则创建
			s = models.Settings{Name: k, Value: v}
			if err := db.Create(&s).Error; err != nil {
				logrus.WithError(err).WithField("setting_name", k).Error("创建设置失败")
				authBaseController.HandleInternalError(c, fmt.Sprintf("保存设置 %s 失败", k), err)
				return
			}

		} else {
			// 存在则更新
			if err := db.Model(&models.Settings{}).Where("id = ?", s.ID).Update("value", v).Error; err != nil {
				logrus.WithError(err).WithField("setting_name", k).Error("更新设置失败")
				authBaseController.HandleInternalError(c, fmt.Sprintf("更新设置 %s 失败", k), err)
				return
			}

		}
		// 收集对应的Redis缓存键（与services/query.go中的键命名保持一致）
		keysToDel = append(keysToDel, fmt.Sprintf("setting:%s", k))
	}

	// 删除Redis缓存键（如果Redis不可用则静默跳过）
	_ = utils.RedisDel(c.Request.Context(), keysToDel...)

	// 刷新内存中的设置缓存，保证后续读取一致
	services.GetSettingsService().RefreshCache()

	// 获取当前操作人信息
	claims, _, err := GetCurrentAdminUserWithRefresh(c)
	var operator, operatorUUID string
	if err == nil && claims != nil {
		operator = claims.Username
		operatorUUID = claims.UUID
	} else {
		operator = "system"
	}

	// 记录操作日志
	logType := "系统设置"
	if categoryStr != "" {
		logType = fmt.Sprintf("系统设置-%s", categoryStr)
	}
	services.RecordOperationLog(logType, operator, operatorUUID, fmt.Sprintf("管理员更新了系统设置，包含 %d 个配置项", len(settingsData)))

	authBaseController.HandleSuccess(c, "保存成功", nil)
}

// validateSettingValue 验证设置项值的合法性
func validateSettingValue(key, value string) error {
	switch key {
	case "jwt_refresh":
		// 验证JWT刷新时间：至少1小时
		hours, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("JWT刷新阈值必须是整数")
		}
		if hours < 1 {
			return fmt.Errorf("JWT刷新阈值必须至少为1小时")
		}
	case "jwt_expire":
		// 验证JWT有效期：至少1小时
		hours, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("JWT有效期必须是整数")
		}
		if hours < 1 {
			return fmt.Errorf("JWT有效期必须至少为1小时")
		}
	}
	return nil
}

// SettingsGenerateKeyHandler 生成安全密钥API
// - type: "jwt" 或 "encryption"
func SettingsGenerateKeyHandler(c *gin.Context) {
	keyType := c.Query("type")
	var key string
	var err error

	switch keyType {
	case "jwt":
		key, err = config.GenerateSecureJWTSecret()
	case "encryption":
		key, err = config.GenerateSecureEncryptionKey()
	default:
		authBaseController.HandleValidationError(c, "无效的密钥类型")
		return
	}

	if err != nil {
		authBaseController.HandleInternalError(c, "生成密钥失败: "+err.Error(), err)
		return
	}

	authBaseController.HandleSuccess(c, "生成成功", map[string]string{"key": key})
}

// SettingsPublicHandler 公开设置查询API
// - 仅返回允许公开的设置项以及所有前端平台配置
func SettingsPublicHandler(c *gin.Context) {
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	var list []models.Settings
	// 查询公开的基本信息、维护模式和所有前端平台配置
	if err := db.Where("name IN ? OR name LIKE ?", []string{"site_title", "site_description", "site_keywords", "site_logo", "contact_email", "maintenance_mode"}, "platform_%").Find(&list).Error; err != nil {
		authBaseController.HandleInternalError(c, "查询失败", err)
		return
	}
	res := map[string]string{}
	for _, s := range list {
		res[s.Name] = s.Value
	}
	authBaseController.HandleSuccess(c, "ok", res)
}
