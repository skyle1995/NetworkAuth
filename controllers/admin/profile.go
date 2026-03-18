package admin

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"NetworkAuth/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ProfileFragmentHandler 个人资料片段渲染
// - 渲染个人资料与修改密码表单
func ProfileFragmentHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "profile.html", map[string]interface{}{})
}

// ProfileInfoHandler 查询当前登录管理员的基本信息
// - 返回 username 字段
func ProfileInfoHandler(c *gin.Context) {
	_, _, err := GetCurrentAdminUserWithRefresh(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "未登录或会话已过期",
			"data": nil,
		})
		return
	}

	// 获取最新设置
	settingsService := services.GetSettingsService()
	username := settingsService.GetString("admin_username", "admin")

	authBaseController.HandleSuccess(c, "ok", map[string]interface{}{
		"username": username,
	})
}

// ProfilePasswordUpdateHandler 修改当前登录管理员的密码
// - 接收 JSON: {old_password, new_password, confirm_password}
// - 校验旧密码正确性、新密码与确认一致性
// - 成功后更新密码哈希
func ProfilePasswordUpdateHandler(c *gin.Context) {
	_, _, err := GetCurrentAdminUserWithRefresh(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "未登录或会话已过期",
			"data": nil,
		})
		return
	}

	var body struct {
		OldPassword     string `json:"old_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if !authBaseController.BindJSON(c, &body) {
		return
	}

	// 基础校验
	if body.OldPassword == "" || body.NewPassword == "" || body.ConfirmPassword == "" {
		authBaseController.HandleValidationError(c, "旧密码/新密码/确认密码均不能为空")
		return
	}
	if len(body.NewPassword) < 6 {
		authBaseController.HandleValidationError(c, "新密码长度不能少于6位")
		return
	}
	if body.NewPassword != body.ConfirmPassword {
		authBaseController.HandleValidationError(c, "两次输入的新密码不一致")
		return
	}
	if body.NewPassword == body.OldPassword {
		authBaseController.HandleValidationError(c, "新密码不能与旧密码相同")
		return
	}

	// 获取当前密码设置
	settingsService := services.GetSettingsService()
	currentHash := settingsService.GetString("admin_password", "")
	currentSalt := settingsService.GetString("admin_password_salt", "")

	// 校验旧密码
	if !utils.VerifyPasswordWithSalt(body.OldPassword, currentSalt, currentHash) {
		authBaseController.HandleValidationError(c, "旧密码不正确")
		return
	}

	// 生成新盐值和哈希
	newSalt, err := utils.GenerateRandomSalt()
	if err != nil {
		authBaseController.HandleInternalError(c, "生成盐值失败", err)
		return
	}

	newHash, err := utils.HashPasswordWithSalt(body.NewPassword, newSalt)
	if err != nil {
		authBaseController.HandleInternalError(c, "生成密码哈希失败", err)
		return
	}

	// 更新数据库
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	// 更新 admin_password
	if err := updateSetting(db, "admin_password", newHash); err != nil {
		authBaseController.HandleInternalError(c, "更新密码失败", err)
		return
	}

	// 更新 admin_password_salt
	if err := updateSetting(db, "admin_password_salt", newSalt); err != nil {
		authBaseController.HandleInternalError(c, "更新盐值失败", err)
		return
	}

	// 刷新缓存
	settingsService.RefreshCache()

	// 清除相关缓存键
	_ = utils.RedisDel(c.Request.Context(), "setting:admin_password", "setting:admin_password_salt")

	// 获取当前用户名
	currentUsername := settingsService.GetString("admin_username", "admin")

	// 重新签发JWT并写入Cookie
	token, err := generateJWTTokenForAdmin(currentUsername, newHash)
	if err != nil {
		authBaseController.HandleInternalError(c, "生成新令牌失败", err)
		return
	}

	secure, sameSite, domain, maxAge := settingsService.GetCookieConfig()
	cookie := utils.CreateSecureCookie("admin_session", token, maxAge, domain, secure, sameSite)
	c.SetCookie(cookie.Name, cookie.Value, cookie.MaxAge, cookie.Path, cookie.Domain, cookie.Secure, cookie.HttpOnly)

	authBaseController.HandleSuccess(c, "密码修改成功", nil)
}

// ProfileUpdateHandler 修改当前登录管理员的用户名
// - 接收 JSON: {username}
// - 校验用户名非空、长度
// - 更新数据库后重新签发JWT并写入 Cookie，保持前端展示的一致性
func ProfileUpdateHandler(c *gin.Context) {
	_, _, err := GetCurrentAdminUserWithRefresh(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "未登录或会话已过期",
			"data": nil,
		})
		return
	}

	var body struct {
		Username    string `json:"username"`
		OldPassword string `json:"old_password"`
	}
	if !authBaseController.BindJSON(c, &body) {
		return
	}

	username := strings.TrimSpace(body.Username)
	if username == "" {
		authBaseController.HandleValidationError(c, "用户名不能为空")
		return
	}
	if len(username) > 64 {
		authBaseController.HandleValidationError(c, "用户名长度不能超过64字符")
		return
	}

	settingsService := services.GetSettingsService()
	currentUsername := settingsService.GetString("admin_username", "admin")

	// 如果未变化则直接返回成功
	if strings.EqualFold(username, currentUsername) {
		authBaseController.HandleSuccess(c, "保存成功", map[string]interface{}{
			"username": username,
		})
		return
	}

	// 修改用户名需要进行当前密码校验
	if strings.TrimSpace(body.OldPassword) == "" {
		authBaseController.HandleValidationError(c, "修改用户名需要提供当前密码")
		return
	}

	currentHash := settingsService.GetString("admin_password", "")
	currentSalt := settingsService.GetString("admin_password_salt", "")

	// 校验旧密码
	if !utils.VerifyPasswordWithSalt(body.OldPassword, currentSalt, currentHash) {
		authBaseController.HandleValidationError(c, "当前密码不正确")
		return
	}

	// 更新数据库
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	if err := updateSetting(db, "admin_username", username); err != nil {
		authBaseController.HandleInternalError(c, "更新用户名失败", err)
		return
	}

	// 重新签发JWT并写入Cookie
	token, err := generateJWTTokenForAdmin(username, currentHash)
	if err != nil {
		authBaseController.HandleInternalError(c, "生成新令牌失败", err)
		return
	}

	secure, sameSite, domain, maxAge := settingsService.GetCookieConfig()
	cookie := utils.CreateSecureCookie("admin_session", token, maxAge, domain, secure, sameSite)
	c.SetCookie(cookie.Name, cookie.Value, cookie.MaxAge, cookie.Path, cookie.Domain, cookie.Secure, cookie.HttpOnly)

	// 刷新缓存
	settingsService.RefreshCache()
	_ = utils.RedisDel(c.Request.Context(), "setting:admin_username")

	authBaseController.HandleSuccess(c, "用户名修改成功", map[string]interface{}{
		"username": username,
	})
}

// 辅助函数：更新设置项
func updateSetting(db interface{}, name, value string) error {
	// 类型断言
	gormDB, ok := db.(*gorm.DB)
	if !ok {
		// 如果断言失败，尝试重新获取连接
		var err error
		gormDB, err = database.GetDB()
		if err != nil {
			return err
		}
	}

	var setting models.Settings
	if err := gormDB.Where("name = ?", name).First(&setting).Error; err != nil {
		// 如果不存在则创建
		setting = models.Settings{Name: name, Value: value}
		return gormDB.Create(&setting).Error
	}

	// 存在则更新
	return gormDB.Model(&setting).Update("value", value).Error
}
