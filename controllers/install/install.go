package install

import (
	"NetworkAuth/config"
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"NetworkAuth/utils"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// InstallSubmitHandler 处理安装表单提交
func InstallSubmitHandler(c *gin.Context) {
	// 二次安全校验：检查系统是否已经安装
	isInstalledStr := services.GetSettingsService().GetString("is_installed", "0")
	if isInstalledStr == "1" {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "系统已安装，禁止重复初始化"})
		return
	}

	var req struct {
		// 数据库配置
		DbType string `json:"db_type" binding:"required,oneof=sqlite mysql"`
		DbHost string `json:"db_host"`
		DbPort int    `json:"db_port"`
		DbName string `json:"db_name"`
		DbUser string `json:"db_user"`
		DbPass string `json:"db_pass"`

		// 站点和管理员配置
		SiteTitle     string `json:"site_title" binding:"required"`
		AdminUsername string `json:"admin_username" binding:"required"`
		AdminPassword string `json:"admin_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "参数错误: " + err.Error()})
		return
	}

	// 1. 更新配置文件
	err := config.UpdateConfig(func(cfg *config.AppConfig) {
		cfg.Database.Type = req.DbType
		if req.DbType == "mysql" {
			cfg.Database.MySQL.Host = req.DbHost
			cfg.Database.MySQL.Port = req.DbPort
			cfg.Database.MySQL.Database = req.DbName
			cfg.Database.MySQL.Username = req.DbUser
			cfg.Database.MySQL.Password = req.DbPass
		}
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "更新配置文件失败: " + err.Error()})
		return
	}

	// 2. 使用新配置尝试连接数据库
	var testDB *gorm.DB
	if req.DbType == "mysql" {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			req.DbUser, req.DbPass, req.DbHost, req.DbPort, req.DbName)
		testDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "连接 MySQL 数据库失败，请检查配置是否正确: " + err.Error()})
			return
		}
		sqlDB, err := testDB.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "连接 MySQL 数据库失败，无法 Ping 通，请检查配置是否正确"})
			return
		}
	}

	// 3. 重新初始化全局数据库连接并执行迁移
	db, err := database.ReInit()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "连接数据库失败: " + err.Error()})
		return
	}

	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "获取数据库实例失败，请检查数据库配置是否正确"})
		return
	}

	// 强制执行迁移确保表存在
	if err := database.AutoMigrate(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "初始化数据表失败: " + err.Error()})
		return
	}

	// 初始化系统默认设置
	database.SeedDefaultSettings()
	database.SeedDefaultPortalNavigation()

	// 3. 生成新的管理员密码哈希和盐值
	adminSalt, err := utils.GenerateRandomSalt()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "生成盐值失败"})
		return
	}
	adminPasswordHash, err := utils.HashPasswordWithSalt(req.AdminPassword, adminSalt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "加密密码失败"})
		return
	}

	// 开启事务进行更新
	tx := db.Begin()

	// 更新或创建超级管理员账号
	var adminUser models.User
	if err := tx.Where("role = ?", 0).First(&adminUser).Error; err != nil {
		// 如果不存在则创建
		adminUser = models.User{
			Username:     strings.TrimSpace(req.AdminUsername),
			Password:     adminPasswordHash,
			PasswordSalt: adminSalt,
			Nickname:     "超级管理员",
			Avatar:       "",
			Role:         0,
			Status:       1,
			Remark:       "系统默认超级管理员",
		}
		// 使用 Select("Role") 确保 Role 字段（值为0时是零值）被显式插入，避免使用数据库默认值 1
		if err := tx.Select("UUID", "Username", "Password", "PasswordSalt", "Nickname", "Avatar", "Role", "Status", "Remark").Create(&adminUser).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "创建管理员账号失败"})
			return
		}
	} else {
		// 存在则更新
		adminUser.Username = strings.TrimSpace(req.AdminUsername)
		adminUser.Password = adminPasswordHash
		adminUser.PasswordSalt = adminSalt
		adminUser.Nickname = "超级管理员"
		adminUser.Role = 0
		if err := tx.Save(&adminUser).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "更新管理员账号失败"})
			return
		}
		// 确保角色被更新为0（GORM的Save可能忽略零值，所以额外Update一次）
		tx.Model(&adminUser).Update("Role", 0)
	}

	// 如果是新创建的，再额外确保一次 Role 为 0，避免 default 标签导致的零值问题
	tx.Model(&adminUser).Update("Role", 0)

	// 4. 更新设置表
	settingsToUpdate := map[string]string{
		"site_title":   req.SiteTitle,
		"is_installed": "1", // 标记为已安装
	}

	for name, value := range settingsToUpdate {
		// 先尝试更新，如果没有该记录，则忽略（因为 AutoMigrate 已经创建了默认记录）
		if err := tx.Model(&models.Settings{}).Where("name = ?", name).Update("value", value).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "保存设置失败: " + name})
			return
		}
	}
	tx.Commit()

	// 5. 更新内存缓存
	services.ResetSettingsService()

	// 6. 动态初始化核心组件
	// 在系统安装完成后，执行本来在 server.go 中需要已安装才能执行的初始化逻辑
	encryptionKey := services.GetSettingsService().GetEncryptionKey()
	if err := utils.InitEncryption(encryptionKey); err != nil {
		logrus.WithError(err).Error("安装完成后加密管理器初始化失败")
	}

	// 启动日志清理定时任务
	services.StartLogCleanupTask()

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "安装成功"})
}
