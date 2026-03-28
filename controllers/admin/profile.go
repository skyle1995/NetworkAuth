package admin

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"NetworkAuth/utils"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ProfileQueryHandler 获取当前登录管理员的用户名和昵称等信息
// - 返回 JSON: {username, nickname, avatar}
// - 从数据库获取最新信息
func ProfileQueryHandler(c *gin.Context) {
	claims, _, err := GetCurrentAdminUserWithRefresh(c)
	if err != nil {
		authBaseController.HandleValidationError(c, "未登录或会话已过期")
		return
	}

	// 获取最新设置
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}
	var adminUser models.User
	if err := db.Where("uuid = ?", claims.UUID).First(&adminUser).Error; err != nil {
		authBaseController.HandleInternalError(c, "获取管理员信息失败", err)
		return
	}
	username := adminUser.Username
	nickname := adminUser.Nickname
	avatar := adminUser.Avatar

	authBaseController.HandleSuccess(c, "ok", gin.H{
		"username": username,
		"nickname": nickname,
		"avatar":   avatar,
	})
}

// ProfilePasswordUpdateHandler 修改当前登录管理员的密码
// - 接收 JSON: {old_password, new_password, confirm_password}
// - 校验旧密码正确性、新密码与确认一致性
// - 成功后更新密码哈希
// - 自动刷新接近过期的JWT令牌
func ProfilePasswordUpdateHandler(c *gin.Context) {
	var body struct {
		OldPassword     string `json:"old_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}

	if !authBaseController.BindJSON(c, &body) {
		return
	}

	// 获取当前用户信息用于日志记录
	claims, _, err := GetCurrentAdminUserWithRefresh(c)
	if err != nil {
		authBaseController.HandleValidationError(c, "未登录或会话已过期")
		return
	}

	// 基础校验
	if !authBaseController.ValidateRequired(c, map[string]interface{}{
		"旧密码":  body.OldPassword,
		"新密码":  body.NewPassword,
		"确认密码": body.ConfirmPassword,
	}) {
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

	// 注释：由于使用了AdminAuthRequired中间件，已确保是管理员用户

	// 获取数据库连接
	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	// 从数据库获取当前管理员信息
	var adminUser models.User
	if err := db.Where("uuid = ?", claims.UUID).First(&adminUser).Error; err != nil {
		authBaseController.HandleInternalError(c, "获取管理员信息失败", err)
		return
	}

	currentHash := adminUser.Password
	currentSalt := adminUser.PasswordSalt

	// 检查必要的设置是否存在
	if currentHash == "" || currentSalt == "" {
		authBaseController.HandleInternalError(c, "管理员密码设置不完整", nil)
		return
	}

	// 校验旧密码
	if !utils.VerifyPasswordWithSalt(body.OldPassword, currentSalt, currentHash) {
		authBaseController.HandleValidationError(c, "旧密码不正确")
		return
	}

	// 生成新的密码盐值
	newSalt, err := utils.GenerateRandomSalt()
	if err != nil {
		authBaseController.HandleInternalError(c, "生成密码盐失败", err)
		return
	}

	// 生成新密码哈希
	newHash, err := utils.HashPasswordWithSalt(body.NewPassword, newSalt)
	if err != nil {
		authBaseController.HandleInternalError(c, "生成密码哈希失败", err)
		return
	}

	// 更新到数据库
	err = db.Transaction(func(tx *gorm.DB) error {
		// 更新密码和盐值
		return tx.Model(&models.User{}).Where("uuid = ?", claims.UUID).Updates(map[string]interface{}{
			"password":      newHash,
			"password_salt": newSalt,
		}).Error
	})

	if err != nil {
		authBaseController.HandleInternalError(c, "更新密码失败", err)
		return
	}

	// 记录操作日志
	services.RecordOperationLog("修改密码", claims.Username, claims.UUID, "管理员修改了登录密码")

	authBaseController.HandleSuccess(c, "密码修改成功，请重新登录", gin.H{
		"redirect": "/admin/login",
	})
}

// ProfileUpdateHandler 修改当前登录管理员的资料（用户名、昵称、头像）
// - 接收 JSON: {username, nickname, avatar, old_password}
// - 校验旧密码正确性
// - 更新数据库后重新签发JWT并写入 Cookie，保持前端展示的一致性
// - 自动刷新接近过期的JWT令牌
func ProfileUpdateHandler(c *gin.Context) {
	claims, _, err := GetCurrentAdminUserWithRefresh(c)
	if err != nil {
		authBaseController.HandleValidationError(c, "未登录或会话已过期")
		return
	}

	var body struct {
		Username    string `json:"username"`
		Nickname    string `json:"nickname"`
		Avatar      string `json:"avatar"`
		OldPassword string `json:"old_password"`
	}
	if !authBaseController.BindJSON(c, &body) {
		return
	}

	username := strings.TrimSpace(body.Username)
	nickname := strings.TrimSpace(body.Nickname)
	avatar := strings.TrimSpace(body.Avatar)

	if username == "" {
		authBaseController.HandleValidationError(c, "用户名不能为空")
		return
	}
	if len(username) > 64 {
		authBaseController.HandleValidationError(c, "用户名长度不能超过64字符")
		return
	}
	if len(nickname) > 64 {
		authBaseController.HandleValidationError(c, "昵称长度不能超过64字符")
		return
	}
	if len(avatar) > 255 {
		authBaseController.HandleValidationError(c, "头像URL长度不能超过255字符")
		return
	}

	db, ok := authBaseController.GetDB(c)
	if !ok {
		return
	}

	// 注释：由于使用了AdminAuthRequired中间件，已确保是管理员用户

	// 从数据库获取当前管理员信息
	var adminUser models.User
	if err := db.Where("uuid = ?", claims.UUID).First(&adminUser).Error; err != nil {
		authBaseController.HandleInternalError(c, "获取管理员信息失败", err)
		return
	}

	adminUsername := adminUser.Username
	adminNickname := adminUser.Nickname
	adminAvatar := adminUser.Avatar
	adminPassword := adminUser.Password
	adminPasswordSalt := adminUser.PasswordSalt

	// 检查必要的设置是否存在
	if adminUsername == "" || adminPassword == "" || adminPasswordSalt == "" {
		authBaseController.HandleInternalError(c, "管理员设置不完整", nil)
		return
	}

	// 如果用户名、昵称和头像都未变化则直接返回成功（无需校验旧密码）
	if strings.EqualFold(username, adminUsername) && nickname == adminNickname && avatar == adminAvatar {
		authBaseController.HandleSuccess(c, "保存成功", gin.H{
			"username": username,
			"nickname": nickname,
			"avatar":   avatar,
		})
		return
	}

	// 如果只修改昵称或头像，不需要验证密码
	if !strings.EqualFold(username, adminUsername) {
		// 修改用户名需要进行当前密码校验
		if strings.TrimSpace(body.OldPassword) == "" {
			authBaseController.HandleValidationError(c, "修改账号需要提供当前密码")
			return
		}

		// 使用盐值验证当前密码
		if !utils.VerifyPasswordWithSalt(body.OldPassword, adminPasswordSalt, adminPassword) {
			authBaseController.HandleValidationError(c, "当前密码不正确")
			return
		}
	}

	// 更新管理员资料
	if dbErr := db.Model(&models.User{}).Where("uuid = ?", claims.UUID).Updates(map[string]interface{}{
		"username": username,
		"nickname": nickname,
		"avatar":   avatar,
	}).Error; dbErr != nil {
		authBaseController.HandleInternalError(c, "更新管理员资料失败", dbErr)
		return
	}

	// 获取当前管理员并刷新Token（这会生成包含新用户名的Token并更新Cookie）
	_, _, _ = GetCurrentAdminUserWithRefresh(c)

	// 记录操作日志
	services.RecordOperationLog("修改资料", claims.Username, claims.UUID, fmt.Sprintf("管理员修改资料为 用户名: %s, 昵称: %s, 头像: %s", username, nickname, avatar))

	authBaseController.HandleSuccess(c, "保存成功", gin.H{
		"username": username,
		"nickname": nickname,
		"avatar":   avatar,
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
