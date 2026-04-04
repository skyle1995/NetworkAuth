package database

import (
	"NetworkAuth/config"
	"NetworkAuth/models"

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

	isInstalledDefault := "0"

	// 定义默认设置项
	var defaultSettings []models.Settings

	// ===== 系统安装状态 =====
	defaultSettings = append(defaultSettings, []models.Settings{
		{
			Name:        "is_installed",
			Value:       isInstalledDefault,
			Description: "系统是否已初始化安装，0=未安装，1=已安装",
		},
	}...)

	// ===== 系统和安全相关默认项 =====
	defaultSettings = append(defaultSettings, []models.Settings{
		{
			Name:        "maintenance_mode",
			Value:       "0",
			Description: "维护模式，0=关闭维护模式，1=开启维护模式",
		},
		{
			Name:        "hide_login_entrance",
			Value:       "0",
			Description: "隐藏登录入口，0=显示，1=隐藏（门户中不显示管理员或子账号登录入口）",
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
			Value:       "10",
			Description: "文件上传最大尺寸",
		},
		{
			Name:        "max_upload_size_unit",
			Value:       "MB",
			Description: "文件上传大小单位（B/KB/MB/GB）",
		},
	}...)

	// ===== 日志清理策略默认项 =====
	defaultSettings = append(defaultSettings, []models.Settings{
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
	}...)

	// ===== Cookie相关默认项 =====
	defaultSettings = append(defaultSettings, []models.Settings{
		{
			Name:        "cookie_secure",
			Value:       "false",
			Description: "是否启用安全Cookie（仅HTTPS），开启后HTTP访问可能导致登录失败",
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
	}...)

	// ===== 站点基本信息默认项 =====
	defaultSettings = append(defaultSettings, []models.Settings{
		{
			Name:        "site_title",
			Value:       "NetworkAuth",
			Description: "网站标题，显示在浏览器标题栏和页面顶部",
		},
		{
			Name:        "site_keywords",
			Value:       "NetworkAuth,网络授权服务,GoLang,Web服务",
			Description: "网站关键词，用于SEO优化，多个关键词用逗号分隔",
		},
		{
			Name:        "site_description",
			Value:       "网络授权服务 (NetworkAuth) 是一个专注于应用鉴权、接口管理和动态逻辑分发的后端系统",
			Description: "网站描述，用于SEO优化和社交媒体分享",
		},
		{
			Name:        "site_logo",
			Value:       "/logo.svg",
			Description: "网站Logo图片路径",
		},
		{
			Name:        "contact_email",
			Value:       "admin@example.com",
			Description: "联系邮箱，用于客服和业务咨询",
		},
	}...)

	// ===== 页脚与备案相关默认项 =====
	defaultSettings = append(defaultSettings, []models.Settings{
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
	}...)

	// ===== 前端平台配置相关默认项 =====
	defaultSettings = append(defaultSettings, []models.Settings{
		{
			Name:        "platform_fixed_header",
			Value:       "1",
			Description: "是否固定页头 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_hidden_side_bar",
			Value:       "0",
			Description: "是否隐藏侧边栏 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_multi_tags_cache",
			Value:       "0",
			Description: "是否开启多标签页缓存 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_keep_alive",
			Value:       "1",
			Description: "是否开启组件缓存 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_layout",
			Value:       "vertical",
			Description: "布局模式 (vertical/horizontal/mix/comprehensive)",
		},
		{
			Name:        "platform_theme",
			Value:       "light",
			Description: "主题配色 (light/dark)",
		},
		{
			Name:        "platform_dark_mode",
			Value:       "0",
			Description: "是否开启暗黑模式 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_overall_style",
			Value:       "light",
			Description: "整体风格",
		},
		{
			Name:        "platform_grey",
			Value:       "0",
			Description: "是否开启灰色模式 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_weak",
			Value:       "0",
			Description: "是否开启色弱模式 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_hide_tabs",
			Value:       "0",
			Description: "是否隐藏标签页 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_hide_footer",
			Value:       "0",
			Description: "是否隐藏页脚 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_stretch",
			Value:       "0",
			Description: "是否开启页面宽度拉伸 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_sidebar_status",
			Value:       "1",
			Description: "侧边栏状态 (0 = 折叠，1 = 展开)",
		},
		{
			Name:        "platform_ep_theme_color",
			Value:       "#409EFF",
			Description: "Element Plus 主题色",
		},
		{
			Name:        "platform_show_logo",
			Value:       "1",
			Description: "是否显示Logo (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_show_model",
			Value:       "smart",
			Description: "显示模式 (smart等)",
		},
		{
			Name:        "platform_menu_arrow_icon_no_transition",
			Value:       "0",
			Description: "菜单箭头图标是否取消过渡动画 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_caching_async_routes",
			Value:       "0",
			Description: "是否缓存异步路由 (0 = 关闭，1 = 开启)",
		},
		{
			Name:        "platform_tooltip_effect",
			Value:       "light",
			Description: "提示框效果 (light/dark)",
		},
		{
			Name:        "platform_responsive_storage_name_space",
			Value:       "responsive-",
			Description: "响应式存储命名空间",
		},
		{
			Name:        "platform_menu_search_history",
			Value:       "6",
			Description: "菜单搜索历史最大记录数",
		},
	}...)

	// 逐个检查并创建不存在的设置项
	for _, setting := range defaultSettings {
		var count int64
		if err := db.Model(&models.Settings{}).Where("name = ?", setting.Name).Count(&count).Error; err != nil {
			return err
		}

		if count == 0 {
			if err := db.Create(&setting).Error; err != nil {
				logrus.WithError(err).WithField("name", setting.Name).Error("创建系统设置失败")
				return err
			}
			logrus.WithField("name", setting.Name).WithField("value", setting.Value).Debug("创建系统设置项")
		}
	}

	logrus.Info("系统设置初始化完成")
	return nil
}
