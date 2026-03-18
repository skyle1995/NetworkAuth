package admin

import (
	"encoding/hex"
	"net/http"
	"NetworkAuth/controllers"
	"NetworkAuth/models"
	"NetworkAuth/utils/encrypt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 全局变量
// ============================================================================

// 创建基础控制器实例
var apiBaseController = controllers.NewBaseController()

// ============================================================================
// 页面处理器
// ============================================================================

// APIFragmentHandler 接口列表页面片段处理器
func APIFragmentHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "apis.html", gin.H{
		"Title": "接口设置",
	})
}

// ============================================================================
// API处理器
// ============================================================================

// APIListHandler 接口列表API处理器
func APIListHandler(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = 10
	}

	// 获取应用UUID参数（用于按应用筛选接口）
	appUUID := strings.TrimSpace(c.Query("app_uuid"))

	// 获取接口类型参数（用于按接口类型筛选）
	apiTypeStr := strings.TrimSpace(c.Query("api_type"))
	var apiType int
	if apiTypeStr != "" {
		apiType, _ = strconv.Atoi(apiTypeStr)
	}

	// 构建查询
	db, ok := apiBaseController.GetDB(c)
	if !ok {
		return
	}

	// 构建基础查询
	query := db.Model(&models.API{})

	// 如果指定了应用UUID，则按应用筛选
	if appUUID != "" {
		query = query.Where("app_uuid = ?", appUUID)
	}

	// 如果指定了接口类型，则按接口类型筛选
	if apiType > 0 {
		query = query.Where("api_type = ?", apiType)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		logrus.WithError(err).Error("Failed to count APIs")
		apiBaseController.HandleInternalError(c, "获取接口总数失败", err)
		return
	}

	// 获取分页数据
	var apis []models.API
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&apis).Error; err != nil {
		logrus.WithError(err).Error("Failed to fetch APIs")
		apiBaseController.HandleInternalError(c, "获取接口列表失败", err)
		return
	}

	// 获取关联的应用信息
	var appUUIDs []string
	for _, api := range apis {
		appUUIDs = append(appUUIDs, api.AppUUID)
	}

	var apps []models.App
	if len(appUUIDs) > 0 {
		if err := db.Where("uuid IN ?", appUUIDs).Find(&apps).Error; err != nil {
			logrus.WithError(err).Error("Failed to fetch related apps")
		}
	}

	// 创建应用UUID到应用名称的映射
	appMap := make(map[string]string)
	for _, app := range apps {
		appMap[app.UUID] = app.Name + "(ID:" + strconv.Itoa(int(app.ID)) + ")"
	}

	// 构建响应数据
	type APIResponse struct {
		models.API
		AppName        string `json:"app_name"`
		APITypeName    string `json:"api_type_name"`
		StatusName     string `json:"status_name"`
		AlgorithmNames struct {
			Submit string `json:"submit"`
			Return string `json:"return"`
		} `json:"algorithm_names"`
	}

	var responseAPIs []APIResponse
	for _, api := range apis {
		responseAPI := APIResponse{
			API:         api,
			AppName:     appMap[api.AppUUID],
			APITypeName: models.GetAPITypeName(api.APIType),
			StatusName:  getAPIStatusName(api.Status),
		}
		responseAPI.AlgorithmNames.Submit = models.GetAlgorithmName(api.SubmitAlgorithm)
		responseAPI.AlgorithmNames.Return = models.GetAlgorithmName(api.ReturnAlgorithm)
		responseAPIs = append(responseAPIs, responseAPI)
	}

	// 计算分页信息
	totalPages := (total + int64(limit) - 1) / int64(limit)

	response := gin.H{
		"success": true,
		"data": gin.H{
			"apis":        responseAPIs,
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": totalPages,
		},
	}

	c.JSON(http.StatusOK, response)
}

// ============================================================================
// 辅助函数
// ============================================================================

// getAPIStatusName 获取API状态名称
func getAPIStatusName(status int) string {
	switch status {
	case 1:
		return "启用"
	case 0:
		return "禁用"
	default:
		return "未知"
	}
}

// APIUpdateHandler 更新接口处理器
func APIUpdateHandler(c *gin.Context) {
	var req struct {
		UUID             string `json:"uuid"`
		Status           int    `json:"status"`
		SubmitAlgorithm  int    `json:"submit_algorithm"`
		ReturnAlgorithm  int    `json:"return_algorithm"`
		SubmitPublicKey  string `json:"submit_public_key"`
		SubmitPrivateKey string `json:"submit_private_key"`
		ReturnPublicKey  string `json:"return_public_key"`
		ReturnPrivateKey string `json:"return_private_key"`
	}

	if !apiBaseController.BindJSON(c, &req) {
		return
	}

	// 验证必填字段
	if strings.TrimSpace(req.UUID) == "" {
		apiBaseController.HandleValidationError(c, "接口UUID不能为空")
		return
	}

	if req.Status != 0 && req.Status != 1 {
		apiBaseController.HandleValidationError(c, "无效的状态值")
		return
	}

	if !models.IsValidAlgorithm(req.SubmitAlgorithm) || !models.IsValidAlgorithm(req.ReturnAlgorithm) {
		apiBaseController.HandleValidationError(c, "无效的算法类型")
		return
	}

	// 获取数据库连接
	db, ok := apiBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查找并更新API记录
	var api models.API
	if err := db.Where("uuid = ?", strings.TrimSpace(req.UUID)).First(&api).Error; err != nil {
		apiBaseController.HandleValidationError(c, "接口不存在")
		return
	}

	// 更新字段（不允许修改 APIType）
	api.Status = req.Status
	api.SubmitAlgorithm = req.SubmitAlgorithm
	api.ReturnAlgorithm = req.ReturnAlgorithm

	// 可选更新密钥/证书（当提供时）
	if req.SubmitPublicKey != "" || req.SubmitPrivateKey != "" {
		api.SubmitPublicKey = req.SubmitPublicKey
		api.SubmitPrivateKey = req.SubmitPrivateKey
	}
	if req.ReturnPublicKey != "" || req.ReturnPrivateKey != "" {
		api.ReturnPublicKey = req.ReturnPublicKey
		api.ReturnPrivateKey = req.ReturnPrivateKey
	}

	if err := db.Save(&api).Error; err != nil {
		logrus.WithError(err).Error("Failed to update API")
		apiBaseController.HandleInternalError(c, "更新接口失败", err)
		return
	}

	apiBaseController.HandleSuccess(c, "接口更新成功", api)
}

// APIGetTypesHandler 获取接口类型列表API处理器
func APIGetTypesHandler(c *gin.Context) {
	// 构建接口类型列表
	type APITypeItem struct {
		Value int    `json:"value"`
		Name  string `json:"name"`
	}

	var apiTypes []APITypeItem

	// 获取所有有效的API类型
	validTypes := []int{
		models.APITypeGetBulletin, models.APITypeGetUpdateUrl, models.APITypeCheckAppVersion, models.APITypeGetCardInfo,
		models.APITypeSingleLogin,
		models.APITypeUserLogin, models.APITypeUserRegin, models.APITypeUserRecharge,
		models.APITypeLogOut,
		models.APITypeGetExpired, models.APITypeCheckUserStatus, models.APITypeGetAppData, models.APITypeGetVariable,
		models.APITypeUpdatePwd, models.APITypeMacChangeBind, models.APITypeIPChangeBind,
		models.APITypeDisableUser, models.APITypeBlackUser, models.APITypeUserDeductedTime,
	}

	for _, apiType := range validTypes {
		apiTypes = append(apiTypes, APITypeItem{
			Value: apiType,
			Name:  models.GetAPITypeName(apiType),
		})
	}

	apiBaseController.HandleSuccess(c, "获取接口类型列表成功", apiTypes)
}

// APIUpdateStatusHandler 更新单个接口状态处理器
func APIUpdateStatusHandler(c *gin.Context) {
	var req struct {
		ID     uint `json:"id"`
		Status int  `json:"status"`
	}

	if !apiBaseController.BindJSON(c, &req) {
		return
	}

	if req.ID == 0 {
		apiBaseController.HandleValidationError(c, "接口ID不能为空")
		return
	}

	if req.Status != 0 && req.Status != 1 {
		apiBaseController.HandleValidationError(c, "状态值无效")
		return
	}

	// 获取数据库连接
	db, ok := apiBaseController.GetDB(c)
	if !ok {
		return
	}

	// 检查接口是否存在
	var api models.API
	if err := db.Where("id = ?", req.ID).First(&api).Error; err != nil {
		apiBaseController.HandleValidationError(c, "接口不存在")
		return
	}

	// 更新状态
	if err := db.Model(&api).Update("status", req.Status).Error; err != nil {
		logrus.WithError(err).Error("Failed to update API status")
		apiBaseController.HandleInternalError(c, "更新状态失败", err)
		return
	}

	statusText := "禁用"
	if req.Status == 1 {
		statusText = "启用"
	}

	apiBaseController.HandleSuccess(c, "接口"+statusText+"成功", nil)
}

func APIGenerateKeysHandler(c *gin.Context) {
	var req struct {
		Side      string `json:"side"`      // submit | return
		Algorithm int    `json:"algorithm"` // 与 models.Algorithm* 对应
	}

	if !apiBaseController.BindJSON(c, &req) {
		return
	}

	if req.Side != "submit" && req.Side != "return" {
		apiBaseController.HandleValidationError(c, "side参数必须为submit或return")
		return
	}
	if !models.IsValidAlgorithm(req.Algorithm) {
		apiBaseController.HandleValidationError(c, "无效的算法类型")
		return
	}

	// 根据算法生成密钥/证书
	result := map[string]interface{}{}

	switch req.Algorithm {
	case models.AlgorithmNone:
		// 不加密不生成任何密钥
		result["public_key"] = ""
		result["private_key"] = ""
	case models.AlgorithmRC4:
		// 生成16字节随机密钥并返回16位十六进制（大写）
		key, err := encrypt.GenerateRC4Key(8) // 生成8字节密钥
		if err != nil {
			logrus.WithError(err).Error("Failed to generate RC4 key")
			apiBaseController.HandleInternalError(c, "生成RC4密钥失败", err)
			return
		}
		result["public_key"] = ""
		result["private_key"] = strings.ToUpper(hex.EncodeToString(key))
	case models.AlgorithmRSA:
		// 生成标准RSA 2048密钥对，返回PEM明文字符串
		publicKey, privateKey, err := encrypt.GenerateRSAKeyPair(2048)
		if err != nil {
			logrus.WithError(err).Error("Failed to generate RSA key pair")
			apiBaseController.HandleInternalError(c, "生成RSA密钥失败", err)
			return
		}

		// 转换为PEM格式
		publicKeyPEM, err := encrypt.PublicKeyToPEM(publicKey)
		if err != nil {
			logrus.WithError(err).Error("Failed to convert public key to PEM")
			apiBaseController.HandleInternalError(c, "转换公钥格式失败", err)
			return
		}

		privateKeyPEM, err := encrypt.PrivateKeyToPEM(privateKey)
		if err != nil {
			logrus.WithError(err).Error("Failed to convert private key to PEM")
			apiBaseController.HandleInternalError(c, "转换私钥格式失败", err)
			return
		}

		result["public_key"] = publicKeyPEM
		result["private_key"] = privateKeyPEM
	case models.AlgorithmRSADynamic:
		// 生成RSA动态加密密钥对，返回PEM明文字符串
		publicKeyPEM, privateKeyPEM, err := encrypt.GenerateRSADynamicKeyPair(2048)
		if err != nil {
			logrus.WithError(err).Error("Failed to generate RSA dynamic key pair")
			apiBaseController.HandleInternalError(c, "生成RSA动态密钥失败", err)
			return
		}

		result["public_key"] = publicKeyPEM
		result["private_key"] = privateKeyPEM
	case models.AlgorithmEasy:
		// 生成易加密密钥对，返回逗号分隔的整数数组字符串
		encryptKey, _, err := encrypt.GenerateEasyKey()
		if err != nil {
			logrus.WithError(err).Error("Failed to generate Easy encryption key")
			apiBaseController.HandleInternalError(c, "生成易加密密钥失败", err)
			return
		}
		result["public_key"] = ""
		result["private_key"] = encrypt.FormatKeyAsString(encryptKey)
	default:
		apiBaseController.HandleValidationError(c, "不支持的算法类型")
		return
	}

	apiBaseController.HandleSuccess(c, "生成成功", result)
}
