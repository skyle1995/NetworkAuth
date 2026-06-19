package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"NetworkAuth/utils"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ============================================================================
// 全局变量
// ============================================================================

// 创建BaseController实例
var authBaseController = controllers.NewBaseController()

// ============================================================================
// API处理器
// ============================================================================

// CSRFTokenHandler 获取CSRF令牌接口
func CSRFTokenHandler(c *gin.Context) {
	// 尝试从Cookie获取
	var token string
	if cookie, err := c.Cookie(CSRFCookieName); err == nil && cookie != "" {
		token = cookie
	} else {
		newToken, err := utils.GenerateCSRFToken()
		if err != nil {
			authBaseController.HandleInternalError(c, "生成CSRF令牌失败", err)
			return
		}
		token = newToken
		setCSRFToken(c, token)
	}

	authBaseController.HandleSuccess(c, "success", gin.H{
		"csrf_token": token,
	})
}

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
		recordLoginLog(c, "", body.Username, 0, "验证码错误或已过期")
		authBaseController.HandleValidationError(c, "验证码错误或已过期")
		return
	}

	// 从数据库中查找对应的用户
	db, err := database.GetDB()
	if err != nil {
		recordLoginLog(c, "", body.Username, 0, "数据库连接失败")
		authBaseController.HandleInternalError(c, "数据库连接失败", err)
		return
	}

	var user models.User
	if err := db.Where("username = ?", body.Username).First(&user).Error; err != nil {
		// 用户不存在时执行一次等价耗时的密码校验，消除时序差异，缓解用户名枚举
		utils.PerformDummyPasswordCheck(body.Password)
		recordLoginLog(c, user.UUID, body.Username, 0, "用户不存在")
		authBaseController.HandleValidationError(c, "用户不存在或密码错误")
		return
	}

	// 检查账号状态 (Status=1 表示启用，否则禁止登录)
	if user.Status != 1 {
		recordLoginLog(c, user.UUID, body.Username, 0, "账号已被禁用")
		authBaseController.HandleValidationError(c, "该账号已被禁用，请联系超级管理员")
		return
	}

	// 检查是否允许登录 (role=0 或 role=1 允许登录，role=2 不允许)
	if user.Role > 1 {
		recordLoginLog(c, user.UUID, body.Username, 0, "权限不足")
		authBaseController.HandleValidationError(c, "权限不足，禁止登录")
		return
	}

	// 验证密码（使用盐值校验）
	if !utils.VerifyPasswordWithSalt(body.Password, user.PasswordSalt, user.Password) {
		recordLoginLog(c, user.UUID, body.Username, 0, "密码错误")
		authBaseController.HandleValidationError(c, "用户不存在或密码错误")
		return
	}

	// 生成 access JWT
	token, err := generateJWTTokenForAdmin(user.Username, user.Password, user.UUID, user.Role)
	if err != nil {
		recordLoginLog(c, user.UUID, body.Username, 0, "生成令牌失败")
		authBaseController.HandleInternalError(c, "生成令牌失败", err)
		return
	}

	// 签发 refreshToken（新 family）
	settingsService := services.GetSettingsService()
	refreshTokenSvc := services.GetRefreshTokenService()
	refreshDays := settingsService.GetRefreshTokenExpireDays()
	absoluteDays := settingsService.GetSessionAbsoluteExpireDays()
	if absoluteDays < refreshDays {
		absoluteDays = refreshDays
	}
	refreshExpiresAt := time.Now().Add(time.Duration(refreshDays) * 24 * time.Hour)
	absoluteExpiresAt := time.Now().Add(time.Duration(absoluteDays) * 24 * time.Hour)
	jti := refreshTokenSvc.NewJTI()
	familyID := refreshTokenSvc.NewFamilyID()
	refreshToken, err := generateRefreshTokenForAdmin(user.Username, user.Password, user.UUID, user.Role, jti, familyID, refreshExpiresAt)
	if err != nil {
		recordLoginLog(c, user.UUID, body.Username, 0, "生成刷新令牌失败")
		authBaseController.HandleInternalError(c, "生成刷新令牌失败", err)
		return
	}
	if err := refreshTokenSvc.Create(jti, familyID, user.UUID, "admin",
		refreshExpiresAt, absoluteExpiresAt, c.Request.UserAgent(), c.ClientIP()); err != nil {
		recordLoginLog(c, user.UUID, body.Username, 0, "持久化刷新令牌失败")
		authBaseController.HandleInternalError(c, "持久化刷新令牌失败", err)
		return
	}

	accessExpiresAt := time.Now().Add(time.Duration(settingsService.GetJWTExpire()) * time.Hour)

	recordLoginLog(c, user.UUID, body.Username, 1, "登录成功")
	authBaseController.HandleSuccess(c, "登录成功", gin.H{
		"redirect":     "/admin",
		"avatar":       user.Avatar,
		"nickname":     user.Nickname,
		"username":     user.Username,
		"role":         user.Role,
		"token":        token,
		"accessToken":  token,
		"refreshToken": refreshToken,
		"expires":      accessExpiresAt,
	})
}

// recordLoginLog 记录登录日志
// status: 1-成功, 0-失败
func recordLoginLog(c *gin.Context, uuid string, username string, status int, message string) {
	db, err := database.GetDB()
	if err != nil {
		// 记录日志失败不应影响主流程，但可以记录到系统日志
		logrus.WithError(err).Error("记录登录日志失败：获取数据库连接失败")
		return
	}

	log := models.LoginLog{
		Type:      "admin",
		UUID:      uuid,
		Username:  username,
		IP:        c.ClientIP(),
		Status:    status,
		Message:   "登录管理 - " + message,
		UserAgent: c.Request.UserAgent(),
		CreatedAt: time.Now(),
	}

	if err := db.Create(&log).Error; err != nil {
		logrus.WithError(err).Error("写入登录日志失败")
	}
}

// LogoutHandler 管理员登出
// - 清理JWT Cookie会话
// - 撤销当前 refreshToken family
func LogoutHandler(c *gin.Context) {
	// 尝试解析当前 access token，提取 family（通过 refresh DB 反查）
	if token, err := getJWTCookie(c); err == nil && token != "" {
		if claims, err := parseJWTToken(token); err == nil {
			// access token 不带 family，需通过 user uuid 撤销该用户全部活跃 refresh
			revokeAllRefreshOfUser(claims.UUID)
		}
	}

	// 清理JWT Cookie
	clearInvalidJWTCookie(c)

	authBaseController.HandleSuccess(c, "已退出登录", gin.H{
		"redirect": "/admin/login",
	})
}

// revokeAllRefreshOfUser 撤销该用户全部未撤销的 refreshToken
func revokeAllRefreshOfUser(userUUID string) {
	db, err := database.GetDB()
	if err != nil {
		return
	}
	db.Model(&models.RefreshToken{}).
		Where("user_uuid = ? AND user_type = ? AND revoked = ?", userUUID, "admin", false).
		Update("revoked", true)
}

// RefreshTokenHandler 刷新管理员会话令牌
// - 校验请求体中的 refreshToken（OAuth2 风格）
// - DB 校验：jti 存在、未撤销、未过期、未超绝对上限
// - 轮换：旧 jti 标记 revoked + replaced_by；签发新 access + 新 refresh
// - 重用检测：旧已撤销 token 再次提交 -> 整 family 撤销
func RefreshTokenHandler(c *gin.Context) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = c.ShouldBindJSON(&body)
	refreshTokenStr := strings.TrimSpace(body.RefreshToken)
	if refreshTokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "缺少刷新令牌",
			"data": nil,
		})
		return
	}

	// 1. 解析 JWT
	claims, err := parseJWTToken(refreshTokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "无效的刷新令牌",
			"data": nil,
		})
		return
	}

	// 2. 必须是 refresh 类型
	if claims.TokenType != TokenTypeRefresh {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "令牌类型错误",
			"data": nil,
		})
		return
	}

	// 3. DB 查询 jti
	refreshSvc := services.GetRefreshTokenService()
	rec, err := refreshSvc.FindByJTI(claims.ID)
	if err != nil {
		// 找不到 = 已被清理或伪造 -> 拒绝
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "刷新令牌不存在或已失效",
			"data": nil,
		})
		return
	}

	// 4. 重用检测：已撤销的 token 被再次使用 -> 整族撤销
	if rec.Revoked {
		_ = refreshSvc.RevokeFamily(rec.FamilyID)
		clearInvalidJWTCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "检测到刷新令牌重用，会话已强制失效",
			"data": nil,
		})
		return
	}

	now := time.Now()

	// 5. 过期检查
	if now.After(rec.ExpiresAt) {
		_ = refreshSvc.RevokeByJTI(rec.JTI)
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "刷新令牌已过期，请重新登录",
			"data": nil,
		})
		return
	}

	// 6. 绝对上限检查
	if now.After(rec.AbsoluteExpiresAt) {
		_ = refreshSvc.RevokeFamily(rec.FamilyID)
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "会话已达最长有效期，请重新登录",
			"data": nil,
		})
		return
	}

	// 7. 校验用户依然有效 + 密码未变（复用此处加载的用户，避免后续重复查询）
	adminUserPtr, ok := loadAndValidateAdmin(claims, c)
	if !ok {
		_ = refreshSvc.RevokeFamily(rec.FamilyID)
		clearInvalidJWTCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 1,
			"msg":  "会话已失效，请重新登录",
			"data": nil,
		})
		return
	}
	adminUser := *adminUserPtr

	// 8. 签发新 access + 新 refresh（一次一换，继承 absolute）
	settingsService := services.GetSettingsService()
	newAccess, err := generateJWTTokenForAdmin(adminUser.Username, adminUser.Password, adminUser.UUID, adminUser.Role)
	if err != nil {
		authBaseController.HandleInternalError(c, "生成令牌失败", err)
		return
	}
	refreshDays := settingsService.GetRefreshTokenExpireDays()
	newRefreshExpiresAt := now.Add(time.Duration(refreshDays) * 24 * time.Hour)
	if newRefreshExpiresAt.After(rec.AbsoluteExpiresAt) {
		newRefreshExpiresAt = rec.AbsoluteExpiresAt
	}
	newJTI := refreshSvc.NewJTI()
	newRefresh, err := generateRefreshTokenForAdmin(adminUser.Username, adminUser.Password, adminUser.UUID, adminUser.Role,
		newJTI, rec.FamilyID, newRefreshExpiresAt)
	if err != nil {
		authBaseController.HandleInternalError(c, "生成刷新令牌失败", err)
		return
	}
	// 在单个事务内插入新令牌并撤销旧令牌，保证轮换的原子性
	if err := refreshSvc.CreateAndRotate(newJTI, rec.FamilyID, adminUser.UUID, "admin",
		newRefreshExpiresAt, rec.AbsoluteExpiresAt, c.Request.UserAgent(), c.ClientIP(), rec.JTI); err != nil {
		authBaseController.HandleInternalError(c, "持久化刷新令牌失败", err)
		return
	}

	// 9. access token 通过响应体返回，不再同步到 Cookie

	authBaseController.HandleSuccess(c, "刷新成功", gin.H{
		"accessToken":  newAccess,
		"refreshToken": newRefresh,
		"expires":      now.Add(time.Duration(settingsService.GetJWTExpire()) * time.Hour),
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
// - HttpOnly 必须为 false：前端采用 double-submit 模式，需通过 JS 读取该 Cookie 并回填到请求头
// - Secure 在 HTTPS 连接下自动开启，避免明文链路泄露令牌
func setCSRFToken(c *gin.Context, token string) {
	secure := isSecureRequest(c)
	c.SetCookie(CSRFCookieName, token, 3600*24, "/", "", secure, false)
	c.Header(CSRFHeaderName, token)
}

// isSecureRequest 判断当前请求是否经由 HTTPS（含反向代理场景）
func isSecureRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	// 兼容反向代理：优先看 X-Forwarded-Proto
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		return strings.EqualFold(proto, "https")
	}
	return false
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

	// 使用常量时间比较，避免逐字节比较带来的时序泄露
	return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(requestToken)) == 1
}

// ============================================================================
// 辅助函数
// ============================================================================

// clearInvalidJWTCookie 已废弃：当前使用纯 Bearer Token 模式，无需清理 Cookie
// 保留空实现以兼容历史调用
func clearInvalidJWTCookie(c *gin.Context) {
	_ = c
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

	// 3. 如果仍未获取到，则记录严重错误并 panic，拒绝使用空/不安全密钥签名。
	//    这里使用 panic 而非 logrus.Fatal：panic 会被 gin.Recovery 捕获，
	//    仅令当前请求返回 500，而不会让单个异常请求拖垮整个服务进程。
	logrus.Error("致命安全错误: 无法获取有效的 JWT 密钥，请检查数据库设置或重新安装系统。系统拒绝以不安全模式运行。")
	panic("JWT secret 不可用，拒绝以不安全模式签发/校验令牌")
}

// ============================================================================
// 结构体定义
// ============================================================================

// Token 类型常量
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// JWTClaims JWT载荷结构体
type JWTClaims struct {
	Username     string `json:"username"`
	UUID         string `json:"uuid"`          // 用户UUID
	Role         int    `json:"role"`          // 用户角色
	PasswordHash string `json:"password_hash"` // 密码哈希摘要，用于验证密码是否被修改
	TokenType    string `json:"typ,omitempty"` // access | refresh，旧版无此字段视为 access
	FamilyID     string `json:"fid,omitempty"` // refresh 专用：会话族 ID
	jwt.RegisteredClaims
}

// generateJWTTokenForAdmin 生成管理员 access JWT 令牌
// - 包含管理员用户名信息和密码哈希
// - 设置过期时间
// - 使用HMAC-SHA256签名
func generateJWTTokenForAdmin(username, passwordHash string, adminUUID string, role int) (string, error) {
	passwordHashDigest := utils.GenerateSHA256Hash(passwordHash)

	claims := JWTClaims{
		Username:     username,
		UUID:         adminUUID,
		Role:         role,
		PasswordHash: passwordHashDigest,
		TokenType:    TokenTypeAccess,
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

// generateRefreshTokenForAdmin 生成管理员 refresh JWT 令牌
// - 携带 jti / family_id 用于持久化与轮换
// - 过期时间使用 settings.refresh_token_expire_days
func generateRefreshTokenForAdmin(username, passwordHash, adminUUID string, role int,
	jti, familyID string, expiresAt time.Time) (string, error) {
	passwordHashDigest := utils.GenerateSHA256Hash(passwordHash)
	claims := JWTClaims{
		Username:     username,
		UUID:         adminUUID,
		Role:         role,
		PasswordHash: passwordHashDigest,
		TokenType:    TokenTypeRefresh,
		FamilyID:     familyID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
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

// getJWTCookie 从 Authorization Bearer 头中读取 access token
// 注意：函数名保留以兼容历史调用，已不再读取 Cookie
func getJWTCookie(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("未找到会话信息")
}

// loadAndValidateAdmin 加载并校验管理员：账号存在、启用、角色合法、密码未变更
// - 返回已加载的用户对象，便于调用方复用，避免在同一请求内重复查询数据库
// - 校验不通过时返回 (nil, false)
func loadAndValidateAdmin(claims *JWTClaims, c *gin.Context) (*models.User, bool) {
	// 【安全修复】验证数据库中的当前密码哈希
	// 这确保了密码修改后，旧的JWT令牌会失效
	db, err := database.GetDB()
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"username": claims.Username, "ip": c.ClientIP(),
		}).Warn("鉴权失败：数据库连接异常")
		return nil, false
	}

	// 获取当前数据库中的管理员用户
	var adminUser models.User
	if err := db.Where("uuid = ?", claims.UUID).First(&adminUser).Error; err != nil {
		logrus.WithFields(logrus.Fields{
			"uuid": claims.UUID, "ip": c.ClientIP(),
		}).Warn("鉴权失败：管理员用户不存在")
		return nil, false
	}

	// 检查账号状态 (Status=1 表示启用，否则强制下线)
	if adminUser.Status != 1 {
		logrus.WithFields(logrus.Fields{
			"uuid": claims.UUID, "ip": c.ClientIP(),
		}).Warn("鉴权失败：管理员账号已被禁用")
		return nil, false
	}

	// 检查是否允许登录 (role=0 或 role=1 允许，role=2不允许访问admin后台)
	if adminUser.Role > 1 {
		logrus.WithFields(logrus.Fields{
			"uuid": claims.UUID, "ip": c.ClientIP(),
		}).Warn("鉴权失败：管理员角色权限不足")
		return nil, false
	}

	// 生成当前数据库密码的哈希摘要
	currentPasswordHash := utils.GenerateSHA256Hash(adminUser.Password)

	// 验证JWT中的密码哈希是否与当前数据库中的密码哈希一致
	if claims.PasswordHash != currentPasswordHash {
		logrus.WithFields(logrus.Fields{
			"username": claims.Username, "ip": c.ClientIP(),
		}).Warn("鉴权失败：密码哈希不匹配，令牌已失效")
		return nil, false
	}

	// 【安全修复】用数据库实时角色覆盖 JWT 载荷中的角色，防止角色被降级后旧令牌权限残留(TOCTOU)。
	// claims 为指针，此处同步后，所有读取 claims.Role 的路径（含 AdminAuthRequired 写入 admin_role）均使用 DB 实时值。
	claims.Role = adminUser.Role

	return &adminUser, true
}

// validateAdminPasswordHash 验证管理员密码哈希的通用函数（仅返回校验结果）
func validateAdminPasswordHash(claims *JWTClaims, c *gin.Context) bool {
	_, ok := loadAndValidateAdmin(claims, c)
	return ok
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
	token := ""
	cookie, err := r.Cookie("admin_session")
	if err == nil && cookie.Value != "" {
		token = cookie.Value
	} else {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if token == "" {
		return false
	}

	// 解析并验证JWT令牌
	claims, err := parseJWTToken(token)
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

	var adminUser models.User
	if err := db.Where("uuid = ?", claims.UUID).First(&adminUser).Error; err != nil {
		return false
	}

	// 检查账号状态 (Status=1 表示启用，否则强制下线)
	if adminUser.Status != 1 {
		return false
	}

	// 检查是否允许登录 (role=0 或 role=1 允许，role=2不允许访问admin后台)
	if adminUser.Role > 1 {
		return false
	}

	// 验证密码哈希
	currentPasswordHash := utils.GenerateSHA256Hash(adminUser.Password)
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
	token := ""
	cookie, err := r.Cookie("admin_session")
	if err == nil && cookie.Value != "" {
		token = cookie.Value
	} else {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if token == "" {
		return nil, fmt.Errorf("未找到会话信息")
	}

	claims, err := parseJWTToken(token)
	if err != nil {
		return nil, fmt.Errorf("无效的会话信息")
	}

	return claims, nil
}

// GetCurrentAdminUserWithRefresh 获取当前登录的管理员用户信息
// - 仅校验 access token 是否有效（不再做滑动续期）
// - 续期统一由前端调用 /refresh-token 完成（OAuth2 风格）
// - 第二个返回值保留为 false 以兼容历史调用方
func GetCurrentAdminUserWithRefresh(c *gin.Context) (*JWTClaims, bool, error) {
	cookie, err := getJWTCookie(c)
	if err != nil {
		return nil, false, fmt.Errorf("未找到会话信息")
	}

	claims, err := parseJWTToken(cookie)
	if err != nil {
		return nil, false, fmt.Errorf("无效的会话信息")
	}

	// access token 必须是 access 类型
	if claims.TokenType != "" && claims.TokenType != TokenTypeAccess {
		return nil, false, fmt.Errorf("令牌类型错误")
	}

	if !validateAdminPasswordHash(claims, c) {
		return nil, false, fmt.Errorf("会话已失效，请重新登录")
	}

	return claims, false, nil
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

			// API 请求直接返回 401 JSON
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "未登录或会话已过期",
				"data":    nil,
			})
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

// SuperAdminRequired 超级管理员专属接口拦截中间件 (Gin Middleware)
// - 必须在 AdminAuthRequired 之后使用，依赖其写入的 admin_role 上下文（已同步为数据库实时角色）。
// - 仅放行 role==0 的超级管理员，其余角色返回 403，阻止普通管理员访问系统级敏感接口。
func SuperAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("admin_role")
		if !exists || role.(int) != 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "权限不足，仅超级管理员可访问",
				"data":    nil,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
