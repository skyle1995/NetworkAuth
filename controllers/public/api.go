package public

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"encoding/base64"
	"encoding/json"
	"net/http"

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
		return handleRebindMachine(app, plainParams)
	case models.APITypeIPChangeBind:
		return handleRebindIP(c, app, plainParams)
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
func handleBulletin(app *models.App) (any, error) {
	content := app.Announcement
	// 公告以 base64 存储，解码失败则原样返回
	if decoded, err := base64.StdEncoding.DecodeString(app.Announcement); err == nil {
		content = string(decoded)
	}
	return gin.H{
		"title":   app.Name,
		"version": app.Version,
		"content": content,
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
		Email    string `json:"email"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.AccountRegister(app.UUID, params.Email, params.Password, params.Code, c.ClientIP())
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
func handleCheckStatus(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token string `json:"token"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.CheckMemberStatus(app.UUID, params.Token)
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

// handleRebindMachine 机器码转绑（type 51）
func handleRebindMachine(app *models.App, plainParams string) (any, error) {
	var params struct {
		Token       string `json:"token"`
		MachineCode string `json:"machine_code"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.RebindMachine(app.UUID, params.Token, params.MachineCode)
}

// handleRebindIP IP转绑（type 52），以服务端识别的客户端 IP 为准。
func handleRebindIP(c *gin.Context, app *models.App, plainParams string) (any, error) {
	var params struct {
		Token string `json:"token"`
	}
	if err := parseParams(plainParams, &params); err != nil {
		return nil, errBadParams
	}
	return services.RebindIP(app.UUID, params.Token, c.ClientIP())
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
