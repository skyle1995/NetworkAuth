package database

import (
	"NetworkAuth/config"
	"NetworkAuth/models"
	"NetworkAuth/utils"

	"github.com/sirupsen/logrus"
)

// ============================================================================
// 公共函数
// ============================================================================

// SeedDefaultSettings 初始化默认系统设置
// - 检查各项设置是否已存在，如不存在则创建默认值
// - 包含站点基本信息、SEO设置等常用配置项
func SeedDefaultSettings() error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	// 生成安全的随机密钥
	jwtSecret, err := config.GenerateSecureJWTSecret()
	if err != nil {
		return err
	}
	encryptionKey, err := config.GenerateSecureEncryptionKey()
	if err != nil {
		return err
	}

	// 生成默认管理员密码（admin123）的盐值和哈希
	// 这样可以确保admin_password和admin_password_salt在初始化时就有值
	adminSalt, err := utils.GenerateRandomSalt()
	if err != nil {
		return err
	}
	adminPasswordHash, err := utils.HashPasswordWithSalt("admin123", adminSalt)
	if err != nil {
		return err
	}

	// 检查是否已有 admin_password，如果有，说明是旧版本升级，应该把 is_installed 默认设为 1
	var adminPwdCount int64
	db.Model(&models.Settings{}).Where("name = ?", "admin_password").Count(&adminPwdCount)
	isInstalledDefault := "0"
	if adminPwdCount > 0 {
		isInstalledDefault = "1"
	}

	// 定义默认设置项
	defaultSettings := []models.Settings{
		// ===== 系统安装状态 =====
		{
			Name:        "is_installed",
			Value:       isInstalledDefault,
			Description: "系统是否已初始化安装，0=未安装，1=已安装",
		},
		// ===== 管理员账号相关默认项 =====
		{
			Name:        "admin_username",
			Value:       "admin",
			Description: "管理员用户名",
		},
		{
			Name:        "admin_password",
			Value:       adminPasswordHash,
			Description: "管理员密码哈希值",
		},
		{
			Name:        "admin_password_salt",
			Value:       adminSalt,
			Description: "管理员密码加密盐值",
		},
		// ===== 系统和安全相关默认项 =====
		{
			Name:        "maintenance_mode",
			Value:       "0",
			Description: "维护模式，0=关闭维护模式，1=开启维护模式",
		},
		{
			Name:        "encryption_key",
			Value:       encryptionKey,
			Description: "数据加密密钥",
		},
		{
			Name:        "jwt_secret",
			Value:       jwtSecret,
			Description: "JWT签名密钥",
		},
		{
			Name:        "jwt_refresh",
			Value:       "6",
			Description: "JWT令牌刷新阈值（小时）",
		},
		{
			Name:        "jwt_expire",
			Value:       "24",
			Description: "JWT令牌有效期（小时）",
		},
		{
			Name:        "session_timeout",
			Value:       "3600",
			Description: "会话超时时间（秒），默认1小时",
		},
		{
			Name:        "max_upload_size",
			Value:       "10485760",
			Description: "文件上传最大尺寸（字节），默认10MB",
		},
		{
			Name:        "default_user_role",
			Value:       "1",
			Description: "新用户默认角色，0=管理员，1=普通用户",
		},
		// ===== 日志清理策略默认项 =====
		{
			Name:        "login_log_cleanup_days",
			Value:       "30",
			Description: "登录日志保留天数（0表示不按天清理）",
		},
		{
			Name:        "login_log_cleanup_limit",
			Value:       "10000",
			Description: "登录日志保留条数（0表示不按数量清理）",
		},
		{
			Name:        "operation_log_cleanup_days",
			Value:       "30",
			Description: "操作日志保留天数（0表示不按天清理）",
		},
		{
			Name:        "operation_log_cleanup_limit",
			Value:       "10000",
			Description: "操作日志保留条数（0表示不按数量清理）",
		},
		// ===== Cookie相关默认项 =====
		{
			Name:        "cookie_secure",
			Value:       "true",
			Description: "Cookie Secure属性（是否只在HTTPS下发送）",
		},
		{
			Name:        "cookie_same_site",
			Value:       "Lax",
			Description: "Cookie SameSite属性（Strict/Lax/None）",
		},
		{
			Name:        "cookie_domain",
			Value:       "",
			Description: "Cookie域名",
		},
		{
			Name:        "cookie_max_age",
			Value:       "86400",
			Description: "Cookie最大存活时间（秒）",
		},
		// ===== 站点基本信息默认项 =====
		{
			Name:        "site_title",
			Value:       "NetworkAuth",
			Description: "网站标题，显示在浏览器标题栏和页面顶部",
		},
		{
			Name:        "site_keywords",
			Value:       "NetworkAuth,鉴权,API管理,GoLang",
			Description: "网站关键词，用于SEO优化，多个关键词用逗号分隔",
		},
		{
			Name:        "site_description",
			Value:       "NetworkAuth 网络授权服务，专注于应用鉴权与接口管理",
			Description: "网站描述，用于SEO优化和社交媒体分享",
		},
		{
			Name:        "site_logo",
			Value:       "/static/logo.png",
			Description: "网站Logo图片路径",
		},
		{
			Name:        "contact_email",
			Value:       "admin@example.com",
			Description: "联系邮箱，用于客服和业务咨询",
		},
		// ===== 页脚与备案相关默认项 =====
		{
			Name:        "footer_text",
			Value:       "Copyright © 2026 NetworkAuth. All Rights Reserved.",
			Description: "页脚展示的版权或说明信息",
		},
		{
			Name:        "icp_record",
			Value:       "",
			Description: "ICP备案号，留空则不显示",
		},
		{
			Name:        "icp_record_link",
			Value:       "https://beian.miit.gov.cn",
			Description: "工信部ICP备案查询链接，留空则不显示",
		},
		{
			Name:        "psb_record",
			Value:       "",
			Description: "公安备案号，留空则不显示",
		},
		{
			Name:        "psb_record_link",
			Value:       "",
			Description: "公安备案查询链接，留空则不显示",
		},
	}

	// 逐个检查并创建不存在的设置项
	for _, setting := range defaultSettings {
		var count int64
		if err := db.Model(&models.Settings{}).Where("name = ?", setting.Name).Count(&count).Error; err != nil {
			return err
		}

		if count == 0 {
			if err := db.Create(&setting).Error; err != nil {
				logrus.WithError(err).WithField("name", setting.Name).Error("创建默认设置失败")
				return err
			}
			logrus.WithField("name", setting.Name).WithField("value", setting.Value).Debug("创建默认设置项")
		}
	}

	logrus.Info("默认系统设置初始化完成")
	return nil
}
