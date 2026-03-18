package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"NetworkAuth/utils"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

// ============================================================================
// 全局变量
// ============================================================================

// 创建BaseController实例
var authBaseController = controllers.NewBaseController()

// ============================================================================
// 页面处理器
// ============================================================================

// LoginPageHandler 管理员登录页渲染处理器
// - 如果已登录则重定向到 /admin
// - 否则渲染 web/template/admin/login.html 模板
// - 自动清理失效的JWT Cookie，避免刷新时的问题
func LoginPageHandler(c *gin.Context) {
	// 使用带清理功能的JWT校验，避免失效Cookie在登录页面造成问题
	if IsAdminAuthenticatedWithCleanup(c) {
		c.Redirect(http.StatusFound, "/admin")
		return
	}

	// 获取或生成CSRF令牌
	var token string
	// 尝试从Cookie获取
	if cookie, err := c.Cookie(CSRFCookieName); err == nil && cookie != "" {
		token = cookie
	} else {
		// 生成新的CSRF令牌并设置到Cookie
		newToken, err := utils.GenerateCSRFToken()
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "生成CSRF令牌失败",
			})
			return
		}
		token = newToken
		setCSRFToken(c, token)
	}

	// 准备模板数据
	data := authBaseController.GetDefaultTemplateData()
	data["Title"] = "管理员登录"
	data["CSRFToken"] = token

	c.HTML(http.StatusOK, "login.html", data)
}

// ============================================================================
// API处理器
// ============================================================================

// LoginHandler 管理员登录接口
// - 接收JSON: {username, password, captcha, csrf_token}
// - 验证CSRF令牌
// - 验证验证码
// - 验证用户存在与密码正确性
// - 仅允许管理员登录
// - 成功后设置JWT Cookie
func LoginHandler(c *gin.Context) {
	var body struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		Captcha   string `json:"captcha"`
		CSRFToken string `json:"csrf_token"`
	}

	if !authBaseController.BindJSON(c, &body) {
		return
	}

	// 1. 验证CSRF令牌 (Gin 方式)
	if !validateCSRFToken(c, body.CSRFToken) {
		authBaseController.HandleValidationError(c, "CSRF令牌验证失败")
		return
	}

	if !authBaseController.ValidateRequired(c, map[string]interface{}{
		"用户名": body.Username,
		"密码":  body.Password,
		"验证码": body.Captcha,
	}) {
		return
	}

	// 验证验证码
	if !VerifyCaptcha(c, body.Captcha) {
		recordLoginLog(c, body.Username, 0, "验证码错误")
		authBaseController.HandleValidationError(c, "验证码错误")
		return
	}

	// 获取系统设置服务
	settingsService := services.GetSettingsService()
	adminUsername := settingsService.GetString("admin_username", "admin")
	adminPasswordHash := settingsService.GetString("admin_password", "")
	adminPasswordSalt := settingsService.GetString("admin_password_salt", "")

	// 验证密码为空的情况（首次登录需要初始化）
	if adminPasswordHash == "" || adminPasswordSalt == "" {
		recordLoginLog(c, body.Username, 0, "管理员账号未初始化")
		authBaseController.HandleInternalError(c, "管理员账号未初始化，请联系系统管理员", nil)
		return
	}

	// 验证用户名
	if body.Username != adminUsername {
		recordLoginLog(c, body.Username, 0, "用户名错误")
		authBaseController.HandleValidationError(c, "用户不存在或密码错误")
		return
	}

	// 验证密码（使用盐值校验）
	if !utils.VerifyPasswordWithSalt(body.Password, adminPasswordSalt, adminPasswordHash) {
		recordLoginLog(c, body.Username, 0, "密码错误")
		authBaseController.HandleValidationError(c, "用户不存在或密码错误")
		return
	}

	// 生成JWT令牌
	token, err := generateJWTTokenForAdmin(body.Username, adminPasswordHash)
	if err != nil {
		recordLoginLog(c, body.Username, 0, "生成令牌失败")
		authBaseController.HandleInternalError(c, "生成令牌失败", err)
		return
	}

	// 设置JWT Cookie（HttpOnly，安全）
	// 使用系统配置的Cookie参数
	secure, sameSite, domain, maxAge := settingsService.GetCookieConfig()
	cookie := utils.CreateSecureCookie("admin_session", token, maxAge, domain, secure, sameSite)
	c.SetCookie(cookie.Name, cookie.Value, cookie.MaxAge, cookie.Path, cookie.Domain, cookie.Secure, cookie.HttpOnly)

	recordLoginLog(c, body.Username, 1, "登录成功")
	authBaseController.HandleSuccess(c, "登录成功", gin.H{
		"redirect": "/admin",
	})
}

// recordLoginLog 记录登录日志
// status: 1-成功, 0-失败
func recordLoginLog(c *gin.Context, username string, status int, message string) {
	db, err := database.GetDB()
	if err != nil {
		// 记录日志失败不应影响主流程，但可以记录到系统日志
		fmt.Printf("Failed to connect to database for login log: %v\n", err)
		return
	}

	log := models.LoginLog{
		Type:      "admin",
		Username:  username,
		IP:        c.ClientIP(),
		Status:    status,
		Message:   message,
		UserAgent: c.Request.UserAgent(),
		CreatedAt: time.Now(),
	}

	if err := db.Create(&log).Error; err != nil {
		fmt.Printf("Failed to create login log: %v\n", err)
	}
}

// LogoutHandler 管理员登出
// - 清理JWT Cookie会话
// - 确保令牌完全失效
func LogoutHandler(c *gin.Context) {
	// 清理JWT Cookie
	clearInvalidJWTCookie(c)

	authBaseController.HandleSuccess(c, "已退出登录", gin.H{
		"redirect": "/admin/login",
	})
}

// ============================================================================
// CSRF 相关辅助函数
// ============================================================================

const (
	CSRFCookieName = "csrf_token"
	CSRFHeaderName = "X-CSRF-Token"
	CSRFFormField  = "csrf_token"
)

// setCSRFToken 设置CSRF令牌到Cookie (Gin适配)
func setCSRFToken(c *gin.Context, token string) {
	c.SetCookie(CSRFCookieName, token, 3600*24, "/", "", false, false)
	c.Header(CSRFHeaderName, token)
}

// validateCSRFToken 验证CSRF令牌 (Gin适配)
func validateCSRFToken(c *gin.Context, requestToken string) bool {
	// 获取Cookie中的令牌
	cookie, err := c.Cookie(CSRFCookieName)
	if err != nil || cookie == "" {
		return false
	}
	cookieToken := cookie

	// 如果请求体中没有提供token，尝试从Header获取
	if requestToken == "" {
		requestToken = c.GetHeader(CSRFHeaderName)
	}

	if requestToken == "" {
		return false
	}

	// 使用常量时间比较
	return strings.Compare(cookieToken, requestToken) == 0
}

// ============================================================================
// 辅助函数
// ============================================================================

// clearInvalidJWTCookie 清理无效的JWT Cookie
// - 统一的Cookie清理函数，确保一致性
// - 在JWT校验失败时自动调用，提升安全性和用户体验
func clearInvalidJWTCookie(c *gin.Context) {
	_, _, domain, _ := services.GetSettingsService().GetCookieConfig()
	cookie := utils.CreateExpiredCookie("admin_session", domain)
	c.SetCookie(cookie.Name, cookie.Value, cookie.MaxAge, cookie.Path, cookie.Domain, cookie.Secure, cookie.HttpOnly)
}

// getJWTSecret 动态获取当前的JWT密钥
// 修复安全漏洞：确保每次都从最新配置中获取密钥，而不是使用启动时的全局变量
func getJWTSecret() []byte {
	// 1. 尝试从数据库设置获取
	settingsService := services.GetSettingsService()
	if secret := settingsService.GetJWTSecret(); secret != "" {
		return []byte(secret)
	}

	// 2. 尝试从配置文件获取（兼容旧配置）
	if secret := viper.GetString("security.jwt_secret"); secret != "" {
		return []byte(secret)
	}

	// 3. 使用默认不安全密钥（仅开发环境）
	return []byte("default-insecure-jwt-secret")
}

// ============================================================================
// 结构体定义
// ============================================================================

// JWTClaims JWT载荷结构体
type JWTClaims struct {
	Username     string `json:"username"`
	UUID         string `json:"uuid"`          // 添加虚拟角色UUID
	Role         int    `json:"role"`          // 添加虚拟角色
	PasswordHash string `json:"password_hash"` // 密码哈希摘要，用于验证密码是否被修改
	jwt.RegisteredClaims
}

// generateJWTTokenForAdmin 生成管理员JWT令牌
// - 包含管理员用户名信息和密码哈希
// - 设置过期时间
// - 使用HMAC-SHA256签名
func generateJWTTokenForAdmin(username, passwordHash string) (string, error) {
	// 生成密码哈希摘要（使用SHA256）
	// 注意：传入的 passwordHash 已经是数据库存的 Hash，这里我们再次 Hash 还是直接用？
	// atomicLibrary 的实现是: utils.GenerateSHA256Hash(adminUser.Password)
	// 这里我们直接用数据库里的 Hash 值作为 Token 的一部分即可，或者对它再 Hash 一次。
	// 为了与 validateAdminPasswordHash 对应，我们需要知道验证时怎么比对。
	// validateAdminPasswordHash: currentPasswordHash := utils.GenerateSHA256Hash(adminPassword.Value)
	// 所以这里也应该对数据库里的值进行 Hash。
	passwordHashDigest := utils.GenerateSHA256Hash(passwordHash)

	// 获取虚拟管理员UUID (NetworkAuth 项目默认为 admin-uuid-001)
	adminUUID := services.GetSettingsService().GetString("admin_uuid", "admin-uuid-001")

	claims := JWTClaims{
		Username:     username,
		UUID:         adminUUID,
		Role:         0,                  // 0表示超级管理员
		PasswordHash: passwordHashDigest, // 包含密码哈希摘要
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(services.GetSettingsService().GetJWTExpire()) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "NetworkAuth",
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

// parseJWTToken 解析并验证JWT令牌
// - 验证签名有效性
// - 检查过期时间
// - 返回用户信息
func parseJWTToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return getJWTSecret(), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// getJWTCookie 获取JWT cookie的通用函数
func getJWTCookie(c *gin.Context) (string, error) {
	return c.Cookie("admin_session")
}

// validateAdminPasswordHash 验证管理员密码哈希的通用函数
func validateAdminPasswordHash(claims *JWTClaims, c *gin.Context) bool {
	// 【安全修复】验证数据库中的当前密码哈希
	// 这确保了密码修改后，旧的JWT令牌会失效
	db, err := database.GetDB()
	if err != nil {
		fmt.Printf("[SECURITY WARNING] Database connection failed during auth - Username=%s, IP=%s\n",
			claims.Username, c.ClientIP())
		return false
	}

	// 获取当前数据库中的管理员密码
	var adminPassword models.Settings
	if err := db.Where("name = ?", "admin_password").First(&adminPassword).Error; err != nil {
		fmt.Printf("[SECURITY WARNING] Admin password not found in database - Username=%s, IP=%s\n",
			claims.Username, c.ClientIP())
		return false
	}

	// 生成当前数据库密码的哈希摘要
	currentPasswordHash := utils.GenerateSHA256Hash(adminPassword.Value)

	// 验证JWT中的密码哈希是否与当前数据库中的密码哈希一致
	if claims.PasswordHash != currentPasswordHash {
		fmt.Printf("[SECURITY WARNING] Password hash mismatch - JWT token invalidated - Username=%s, IP=%s\n",
			claims.Username, c.ClientIP())
		return false
	}

	return true
}

// IsAdminAuthenticated 判断管理员是否已认证（Gin版本）
// - 检查admin_session Cookie中的JWT令牌
// - 验证令牌签名、过期时间和用户角色
func IsAdminAuthenticated(c *gin.Context) bool {
	cookie, err := getJWTCookie(c)
	if err != nil || cookie == "" {
		return false
	}

	// 解析并验证JWT令牌
	claims, err := parseJWTToken(cookie)
	if err != nil {
		return false
	}

	// 验证密码哈希
	return validateAdminPasswordHash(claims, c)
}

// IsAdminAuthenticatedHttp 判断管理员是否已认证（HTTP兼容版本）
// 保留此方法以兼容未迁移的 Handler
func IsAdminAuthenticatedHttp(r *http.Request) bool {
	cookie, err := r.Cookie("admin_session")
	if err != nil || cookie.Value == "" {
		return false
	}

	// 解析并验证JWT令牌
	claims, err := parseJWTToken(cookie.Value)
	if err != nil {
		return false
	}

	// 注意：HTTP 版本无法方便地获取 ClientIP 用于日志，且无法使用 Gin Context 的功能
	// 这里仅做基本的 Token 验证。如果 Token 包含了 PasswordHash，这里也会解析出来。
	// 但验证 PasswordHash 需要 DB 访问。
	// 为了完整性，我们应该也验证 PasswordHash。
	// 这里的 ClientIP 只能从 r.RemoteAddr 获取。

	db, err := database.GetDB()
	if err != nil {
		return false
	}

	var adminPassword models.Settings
	if err := db.Where("name = ?", "admin_password").First(&adminPassword).Error; err != nil {
		return false
	}

	currentPasswordHash := utils.GenerateSHA256Hash(adminPassword.Value)
	if claims.PasswordHash != currentPasswordHash {
		return false
	}

	return true
}

// IsAdminAuthenticatedWithCleanup 带自动清理功能的JWT校验函数
// - 当JWT校验失败时，自动清理失效的Cookie
// - 适用于API接口等需要清理失效令牌的场景
func IsAdminAuthenticatedWithCleanup(c *gin.Context) bool {
	cookie, err := getJWTCookie(c)
	if err != nil || cookie == "" {
		return false
	}

	// 解析并验证JWT令牌
	claims, err := parseJWTToken(cookie)
	if err != nil {
		// JWT解析失败，清理失效Cookie
		clearInvalidJWTCookie(c)
		return false
	}

	// 验证密码哈希
	if !validateAdminPasswordHash(claims, c) {
		clearInvalidJWTCookie(c)
		return false
	}

	return true
}

// GetCurrentAdminUser 获取当前登录的管理员用户信息 (HTTP 兼容版)
func GetCurrentAdminUser(r *http.Request) (*JWTClaims, error) {
	cookie, err := r.Cookie("admin_session")
	if err != nil {
		return nil, fmt.Errorf("未找到会话信息")
	}

	claims, err := parseJWTToken(cookie.Value)
	if err != nil {
		return nil, fmt.Errorf("无效的会话信息")
	}

	return claims, nil
}

// GetCurrentAdminUserWithRefresh 获取当前登录的管理员用户信息并自动刷新令牌
// - 从JWT令牌中提取用户信息
// - 自动刷新接近过期的令牌（剩余时间少于6小时时刷新）
// - 返回用户ID、用户名、角色和是否刷新了令牌
func GetCurrentAdminUserWithRefresh(c *gin.Context) (*JWTClaims, bool, error) {
	cookie, err := getJWTCookie(c)
	if err != nil {
		return nil, false, fmt.Errorf("未找到会话信息")
	}

	claims, err := parseJWTToken(cookie)
	if err != nil {
		return nil, false, fmt.Errorf("无效的会话信息")
	}

	// 验证密码哈希
	if !validateAdminPasswordHash(claims, c) {
		return nil, false, fmt.Errorf("会话已失效，请重新登录")
	}

	// 检查是否需要刷新令牌
	refreshed := false

	// 动态获取刷新阈值：默认剩余时间少于6小时刷新
	refreshThresholdHours := services.GetSettingsService().GetJWTRefresh()
	if refreshThresholdHours <= 0 {
		refreshThresholdHours = 6 // 默认值
	}
	refreshThreshold := time.Duration(refreshThresholdHours) * time.Hour

	// 动态获取JWT总有效期
	expireHours := services.GetSettingsService().GetJWTExpire()
	if expireHours <= 0 {
		expireHours = 24 // 默认值
	}

	// 动态获取Cookie配置（用于更新Cookie过期时间）
	secure, sameSite, domain, maxAge := services.GetSettingsService().GetCookieConfig()

	// 1. 默认情况下，每次请求都更新Cookie的过期时间（滑动过期）
	tokenToSet := cookie
	shouldUpdateCookie := true

	// 2. 检查是否需要刷新JWT令牌（生成新的Token）
	if time.Until(claims.ExpiresAt.Time) < refreshThreshold {
		// 获取当前的 PasswordHash
		db, _ := database.GetDB()
		var adminPassword models.Settings
		db.Where("name = ?", "admin_password").First(&adminPassword)

		// 使用新的有效期生成令牌
		newToken, err := generateJWTTokenForAdmin(claims.Username, adminPassword.Value)
		if err == nil {
			tokenToSet = newToken
			refreshed = true

			// 更新当前claims的过期时间
			claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour))
			claims.IssuedAt = jwt.NewNumericDate(time.Now())
		}
	}

	// 3. 执行Cookie更新
	if shouldUpdateCookie {
		cookieObj := utils.CreateSecureCookie("admin_session", tokenToSet, maxAge, domain, secure, sameSite)
		c.SetCookie(cookieObj.Name, cookieObj.Value, cookieObj.MaxAge, cookieObj.Path, cookieObj.Domain, cookieObj.Secure, cookieObj.HttpOnly)
	}

	return claims, refreshed, nil
}

// AdminAuthRequired 管理员认证拦截中间件 (Gin Middleware)
// - 未登录：重定向到 /admin/login
// - 已登录：自动刷新接近过期的令牌，然后放行到后续处理器
func AdminAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试获取用户信息并自动刷新令牌
		claims, refreshed, err := GetCurrentAdminUserWithRefresh(c)
		if err != nil {
			// 自动清理失效的JWT Cookie，提升安全性和用户体验
			clearInvalidJWTCookie(c)

			// 中文注释：区分普通页面请求与AJAX/JSON请求
			accept := c.GetHeader("Accept")
			xrw := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Requested-With")))
			if strings.Contains(accept, "application/json") || xrw == "xmlhttprequest" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"message": "未登录或会话已过期",
					"data":    nil,
				})
				c.Abort()
				return
			}
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}

		// 如果令牌被刷新，可以在这里记录日志（可选）
		if refreshed {
			// 可以添加日志记录令牌刷新事件
			_ = claims // 避免未使用变量警告
		}

		// 将解析出的用户信息存入上下文，供后续处理使用
		c.Set("admin_uuid", claims.UUID)
		c.Set("admin_username", claims.Username)
		c.Set("admin_role", claims.Role)

		c.Next()
	}
}
