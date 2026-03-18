package install

import (
	"NetworkAuth/config"
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"NetworkAuth/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// InstallPageHandler 渲染安装页面
func InstallPageHandler(c *gin.Context) {
	// 由于前端是通过模板渲染的，我们返回一个安装页面
	c.HTML(http.StatusOK, "install.html", gin.H{
		"title": "NetworkAuth 系统初始化",
	})
}

// InstallSubmitHandler 处理安装表单提交
func InstallSubmitHandler(c *gin.Context) {
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

	// 2. 重新初始化数据库连接并执行迁移
	db, err := database.ReInit()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "连接数据库失败: " + err.Error()})
		return
	}

	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "获取数据库实例失败"})
		return
	}

	// 强制执行迁移确保表存在
	if err := database.AutoMigrate(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "初始化数据表失败: " + err.Error()})
		return
	}

	// 初始化系统默认设置
	database.SeedDefaultSettings()

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

	// 4. 更新设置表
	settingsToUpdate := map[string]string{
		"site_title":          req.SiteTitle,
		"admin_username":      strings.TrimSpace(req.AdminUsername),
		"admin_password":      adminPasswordHash,
		"admin_password_salt": adminSalt,
		"is_installed":        "1", // 标记为已安装
	}

	// 开启事务进行更新
	tx := db.Begin()
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
	settingsService := services.GetSettingsService()
	for name, value := range settingsToUpdate {
		settingsService.Set(name, value)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "安装成功"})
}
