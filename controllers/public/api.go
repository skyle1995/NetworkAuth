package public

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 公开 API 统一入口
// ============================================================================
//
// 客户端请求信封（明文路由字段 + 密文载荷）：
//
//	POST /api/open
//	{ "app_uuid": "...", "api_type": 10, "data": "<按接口提交算法加密的参数>" }
//
// 服务端流程：定位接口配置 → 解密 data → 按 api_type 分发业务 →
// 将结果按接口返回算法加密后返回 { code:0, data:"<密文>" }。
// 出错时返回明文 { code:1, msg:"..." }，便于客户端直接展示。

// fail 返回明文错误
func fail(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, gin.H{"code": 1, "msg": msg})
}

// OpenAPIHandler 公开 API 分发入口
func OpenAPIHandler(c *gin.Context) {
	var envelope struct {
		AppUUID   string `json:"app_uuid"`
		APIType   int    `json:"api_type"`
		Data      string `json:"data"`
		Timestamp int64  `json:"timestamp"`
		Sign      string `json:"sign"`
	}
	if err := c.ShouldBindJSON(&envelope); err != nil {
		fail(c, "请求参数错误")
		return
	}
	if envelope.AppUUID == "" {
		fail(c, "应用标识不能为空")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		fail(c, "服务暂不可用")
		return
	}

	// 校验应用存在且启用
	var app models.App
	if err := db.Where("uuid = ?", envelope.AppUUID).First(&app).Error; err != nil {
		fail(c, "应用不存在")
		return
	}
	if app.Status != 1 {
		fail(c, "应用已停用")
		return
	}

	// 校验请求签名（防重放 + 完整性 + 应用鉴权）
	if err := services.VerifyOpenSign(envelope.AppUUID, envelope.APIType, envelope.Data, envelope.Timestamp, envelope.Sign, app.Secret); err != nil {
		fail(c, err.Error())
		return
	}

	// 定位接口配置（决定加解密算法与密钥），并校验接口已启用
	var api models.API
	if err := db.Where("app_uuid = ? AND api_type = ?", envelope.AppUUID, envelope.APIType).First(&api).Error; err != nil {
		fail(c, "接口未配置")
		return
	}
	if api.Status != 1 {
		fail(c, "接口已停用")
		return
	}

	// 解密请求载荷
	codec := services.NewAPICodec(&api)
	plainParams, err := codec.DecryptRequest(envelope.Data)
	if err != nil {
		logrus.WithError(err).Warn("public api decrypt request failed")
		fail(c, "请求解密失败")
		return
	}

	// 按接口类型分发业务
	result, bizErr := dispatch(c, &app, envelope.APIType, plainParams)
	if bizErr != nil {
		fail(c, bizErr.Error())
		return
	}

	// 加密业务结果后返回
	respondEncrypted(c, codec, result)
}

// dispatch 按接口类型分发到对应业务，返回业务结果或错误
func dispatch(c *gin.Context, app *models.App, apiType int, plainParams string) (any, error) {
	switch apiType {
	case models.APITypeGetBulletin:
		return handleBulletin(app)
	case models.APITypeGetUpdateUrl:
		return services.GetUpdateInfo(app.UUID)
	case models.APITypeCheckAppVersion:
		return handleCheckVersion(app, plainParams)
	case models.APITypeGetCardInfo:
		return handleGetCardInfo(app, plainParams)
	case models.APITypeSingleLogin:
		return handleCardLogin(c, app, plainParams)
	case models.APITypeUserLogin:
		return handleAccountLogin(c, app, plainParams)
	case models.APITypeUserRegin:
		return handleAccountRegister(c, app, plainParams)
	case models.APITypeSendEmailCode:
		return handleSendEmailCode(app, plainParams)
	case models.APITypeClaimTrial:
		return handleClaimTrial(app, plainParams)
	case models.APITypeSendResetCode:
		return handleSendResetCode(app, plainParams)
	case models.APITypeResetPassword:
		return handleResetPassword(app, plainParams)
	case models.APITypeUserRecharge:
		return handleRecharge(app, plainParams)
	case models.APITypeCheckUserStatus:
		return handleCheckStatus(app, plainParams)
	case models.APITypeGetExpired:
		return handleGetExpired(app, plainParams)
	case models.APITypeGetAppData:
		return handleGetAppData(app, plainParams)
	case models.APITypeGetVariable:
		return handleGetVariable(app, plainParams)
	case models.APITypeExecuteFunction:
		return handleExecuteFunction(app, plainParams)
	case models.APITypeGetUserData:
		return handleGetUserData(app, plainParams)
	case models.APITypeSetUserData:
		return handleSetUserData(app, plainParams)
	case models.APITypeUpdatePwd:
		return handleUpdatePassword(app, plainParams)
	case models.APITypeMacChangeBind:
		// 统一转绑（机器码 + IP）
		return handleRebind(c, app, plainParams)
	case models.APITypeDeductPoints:
		return handleDeductPoints(app, plainParams)
	case models.APITypeDisableUser:
		return handleRiskAction(app, plainParams, models.APITypeDisableUser)
	case models.APITypeBlackUser:
		return handleRiskAction(app, plainParams, models.APITypeBlackUser)
	case models.APITypeUserDeductedTime:
		return handleRiskDeduct(app, plainParams)
	case models.APITypeLogOut:
		return handleLogout(app, plainParams)
	default:
		return nil, errUnsupported
	}
}

// respondEncrypted 将业务结果序列化并按返回算法加密后返回
func respondEncrypted(c *gin.Context, codec *services.APICodec, result any) {
	payload, err := json.Marshal(result)
	if err != nil {
		fail(c, "结果序列化失败")
		return
	}
	cipher, err := codec.EncryptResponse(string(payload))
	if err != nil {
		logrus.WithError(err).Warn("public api encrypt response failed")
		fail(c, "返回加密失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cipher})
}

// parseParams 将解密后的明文参数解析到目标结构（空参数视为空对象）
func parseParams(plain string, dst any) error {
	if plain == "" {
		return nil
	}
	return json.Unmarshal([]byte(plain), dst)
}

// ============================================================================
// 业务处理器
// ============================================================================

// handleBulletin 获取程序公告（type 1）
// 作为客户端「启动初始化」入口：一并返回公告 + 应用能力开关 + 运营模式 + 更新策略，
// 客户端据此渲染登录/注册界面（是否显示验证码/卡密登录、按模式显示到期或点数等），免登录。
func handleBulletin(app *models.App) (any, error) {
	content := app.Announcement
	// 公告以 base64 存储，解码失败则原样返回
	if decoded, err := base64.StdEncoding.DecodeString(app.Announcement); err == nil {
		content = string(decoded)
	}
	// 各接口启用状态：api_type -> 1启用/0禁用，客户端据此判断某功能是否可调
	interfaces := map[string]int{}
	if db, err := database.GetDB(); err == nil {
		var apis []models.API
		if err := db.Model(&models.API{}).Select("api_type, status").
			Where("app_uuid = ?", app.UUID).Find(&apis).Error; err == nil {
			for _, a := range apis {
				interfaces[strconv.Itoa(a.APIType)] = a.Status
			}
		}
	}

	return gin.H{
		"title":   app.Name,
		"version": app.Version,
		"content": content,
		// 能力/模式开关：客户端据此决定界面（如注册是否需验证码、是否显示卡密登录）
		"config": gin.H{
			"operation_mode":          app.OperationMode,
			"points_charge_mode":      app.PointsChargeMode,
			"points_heartbeat_charge": app.PointsHeartbeatCharge,
			"card_login_enabled":      app.CardLoginEnabled,
			"register_enabled":        app.RegisterEnabled,
			"email_verify_enabled":    app.EmailVerifyEnabled,
			// 设备注册限制：=1 时客户端注册须收集并提交 machine_code，否则会被拒
			"register_device_required": app.RegisterDeviceLimitEnabled,
			"recharge_enabled":         app.RechargeEnabled,
			"trial_enabled":            app.TrialEnabled,
			// 验证与多开
			"machine_verify":   app.MachineVerify,
			"ip_verify":        app.IPVerify,
			"multi_open_scope": app.MultiOpenScope,
			"multi_open_count": app.MultiOpenCount,
		},
		// 换绑能力：客户端据此显示换绑入口及提示扣费/次数
		"rebind": gin.H{
			"machine": gin.H{
				"enabled": app.MachineRebindEnabled,
				"limit":   app.MachineRebindLimit, // 0每天 1永久
				"count":   app.MachineRebindCount,
				"deduct":  app.MachineRebindDeduct, // 每次换绑扣除分钟
			},
			"ip": gin.H{
				"enabled": app.IPRebindEnabled,
				"limit":   app.IPRebindLimit,
				"count":   app.IPRebindCount,
				"deduct":  app.IPRebindDeduct,
			},
		},
		// 各接口启用状态映射：{ "20": 1, "21": 0, ... }
		"interfaces": interfaces,
		// 更新策略：启动即可判断是否强制更新/下载方式
		"update": gin.H{
			"force_update":  app.ForceUpdate == 1,
			"download_type": app.DownloadType,
			"download_url":  app.DownloadURL,
		},
	}, nil
}

// handleCardLogin 卡密登录（type 10）
func handleCardLogin(c *gin.Context, app *models.App, plainParams string) (any, error) {
	var params struct {
		Card        string `json:"card"`
		MachineCode string `json:"machine_code"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.CardLogin(app.UUID, params.Card, params.MachineCode, c.ClientIP())
}

// handleAccountLogin 账号登录（type 20）
func handleAccountLogin(c *gin.Context, app *models.App, plainParams string) (any, error) {
	var params struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		MachineCode string `json:"machine_code"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.AccountLogin(app.UUID, params.Username, params.Password, params.MachineCode, c.ClientIP())
}

// handleAccountRegister 账号注册（type 21，邮箱即账号）
func handleAccountRegister(c *gin.Context, app *models.App, plainParams string) (any, error) {
	var params struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		Code        string `json:"code"`
		MachineCode string `json:"machine_code"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.AccountRegister(app.UUID, params.Email, params.Password, params.Code, c.ClientIP(), params.MachineCode)
}

// handleSendEmailCode 发送注册验证码（type 23）
func handleSendEmailCode(app *models.App, plainParams string) (any, error) {
	var params struct {
		Email string `json:"email"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.SendRegisterCode(app.UUID, params.Email)
}

// handleClaimTrial 领取试用（type 24）
func handleClaimTrial(app *models.App, plainParams string) (any, error) {
	var params struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.ClaimTrial(app.UUID, params.Username, params.Password)
}

// handleSendResetCode 发送找回密码验证码（type 25）
func handleSendResetCode(app *models.App, plainParams string) (any, error) {
	var params struct {
		Email string `json:"email"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.SendResetCode(app.UUID, params.Email)
}

// handleResetPassword 找回密码（type 26）：邮箱验证码校验后重设密码，无需登录
func handleResetPassword(app *models.App, plainParams string) (any, error) {
	var params struct {
		Email       string `json:"email"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.ResetPasswordByCode(app.UUID, params.Email, params.Code, params.NewPassword)
}

// handleRecharge 用户充值（type 22）：用一张卡为账号充值
func handleRecharge(app *models.App, plainParams string) (any, error) {
	var params struct {
		Username string `json:"username"`
		Card     string `json:"card"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.RechargeByCard(app.UUID, params.Username, params.Card)
}

// handleCheckStatus 检测账号状态/心跳（type 41）
// charge：点数-按时且「心跳触发扣费」模式下，本次心跳是否触发扣费（用功能A传false、用功能B传true）。
func handleCheckStatus(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token  string `json:"token"`
		Charge bool   `json:"charge"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.CheckMemberStatus(app.UUID, params.Token, params.Charge)
}

// handleGetExpired 获取到期时间（type 40）
func handleGetExpired(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token string `json:"token"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.GetMemberExpiry(app.UUID, params.Token)
}

// handleGetAppData 获取程序数据（type 42）
func handleGetAppData(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token string `json:"token"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.GetAppData(app.UUID, params.Token)
}

// handleGetVariable 获取变量数据（type 43）
func handleGetVariable(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token string `json:"token"`
		Alias string `json:"alias"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.GetVariable(app.UUID, params.Token, params.Alias)
}

// handleExecuteFunction 执行远程函数（type 44）：服务端 goja 沙箱执行，返回结果
func handleExecuteFunction(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token  string `json:"token"`
		Alias  string `json:"alias"`
		Params any    `json:"params"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	result, err := services.ExecuteFunction(app.UUID, params.Token, params.Alias, params.Params)
	if err != nil {
		return nil, err
	}
	return map[string]any{"result": result}, nil
}

// handleGetUserData 获取账号数据（type 45）
func handleGetUserData(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token string `json:"token"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.GetUserData(app.UUID, params.Token)
}

// handleSetUserData 设置账号数据（type 54）
func handleSetUserData(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token string `json:"token"`
		Data  string `json:"data"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.SetUserData(app.UUID, params.Token, params.Data)
}

// handleUpdatePassword 修改账号密码（type 50）
func handleUpdatePassword(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token       string `json:"token"`
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.ChangeMemberPassword(app.UUID, params.Token, params.OldPassword, params.NewPassword)
}

// handleCheckVersion 检测最新版本（type 3）
func handleCheckVersion(app *models.App, plainParams string) (any, error) {
	var params struct {
		Version string `json:"version"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.CheckVersion(app.UUID, params.Version)
}

// handleGetCardInfo 获取卡密信息（type 4）
func handleGetCardInfo(app *models.App, plainParams string) (any, error) {
	var params struct {
		Card string `json:"card"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.GetCardInfo(app, params.Card)
}

// handleRebind 统一转绑（type 51/52）：凭账号密码鉴权（卡密账号用卡号），
// 开启机器码转绑时须带 machine_code；IP 转绑用当前请求 IP。不需登录令牌，避免死循环。
func handleRebind(c *gin.Context, app *models.App, plainParams string) (any, error) {
	var params struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		MachineCode string `json:"machine_code"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.Rebind(app.UUID, params.Username, params.Password, params.MachineCode, c.ClientIP())
}

// handleDeductPoints 功能扣点（type 53，点数模式）
func handleDeductPoints(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token  string `json:"token"`
		Points int    `json:"points"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.DeductPoints(app.UUID, params.Token, params.Points)
}

// handleRiskAction 封停/拉黑（type 60/61）：按用户名操作
func handleRiskAction(app *models.App, plainParams string, apiType int) (any, error) {
	var params struct {
		Username string `json:"username"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	if apiType == models.APITypeBlackUser {
		return services.RiskBlacklistMember(app.UUID, params.Username)
	}
	return services.RiskDisableMember(app.UUID, params.Username)
}

// handleRiskDeduct 扣除时间（type 62）：按用户名扣除分钟数
func handleRiskDeduct(app *models.App, plainParams string) (any, error) {
	var params struct {
		Username string `json:"username"`
		Minutes  int    `json:"minutes"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.RiskDeductMember(app.UUID, params.Username, params.Minutes)
}

// handleLogout 退出登录（type 30）
func handleLogout(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token string `json:"token"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	if err := services.MemberLogout(app.UUID, params.Token); err != nil {
		return nil, err
	}
	return gin.H{"message": "已退出登录"}, nil
}
