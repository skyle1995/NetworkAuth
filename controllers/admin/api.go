package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"NetworkAuth/utils/encrypt"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 全局变量
// ============================================================================

// 创建基础控制器实例
var apiBaseController = controllers.NewBaseController()

// ============================================================================
// API处理器
// ============================================================================

// APIListHandler 接口列表API处理器
func APIListHandler(c *gin.Context) {
	// 获取分页参数
	page, limit := apiBaseController.GetPaginationParams(c)

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

	// 泛型分页查询
	apis, total, err := services.Paginate[models.API](query, page, limit, "created_at DESC")
	if err != nil {
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

	// 返回结果
	response := gin.H{
		"code":  0,
		"msg":   "success",
		"count": total,
		"data":  responseAPIs,
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

// APIExportKeysHandler 导出指定应用的对接密钥（应用密钥 + 各接口算法与密钥）
// 供开发者一次性拿到全部加密配置，避免逐个复制
func APIExportKeysHandler(c *gin.Context) {
	appUUID := strings.TrimSpace(c.Query("app_uuid"))
	if appUUID == "" {
		apiBaseController.HandleValidationError(c, "请先选择要导出的应用")
		return
	}

	db, ok := apiBaseController.GetDB(c)
	if !ok {
		return
	}

	// 查询应用
	var app models.App
	if err := db.Where("uuid = ?", appUUID).First(&app).Error; err != nil {
		apiBaseController.HandleValidationError(c, "应用不存在")
		return
	}

	// 查询该应用的全部接口，按接口类型升序
	var apis []models.API
	if err := db.Where("app_uuid = ?", appUUID).Order("api_type ASC").Find(&apis).Error; err != nil {
		logrus.WithError(err).Error("Failed to fetch APIs for export")
		apiBaseController.HandleInternalError(c, "查询接口失败", err)
		return
	}

	type ifaceExport struct {
		APIType             int    `json:"api_type"`
		APITypeName         string `json:"api_type_name"`
		Status              int    `json:"status"`
		StatusName          string `json:"status_name"`
		SubmitAlgorithm     int    `json:"submit_algorithm"`
		SubmitAlgorithmName string `json:"submit_algorithm_name"`
		ReturnAlgorithm     int    `json:"return_algorithm"`
		ReturnAlgorithmName string `json:"return_algorithm_name"`
		SubmitPublicKey     string `json:"submit_public_key"`
		SubmitPrivateKey    string `json:"submit_private_key"`
		ReturnPublicKey     string `json:"return_public_key"`
		ReturnPrivateKey    string `json:"return_private_key"`
	}

	interfaces := make([]ifaceExport, 0, len(apis))
	for _, api := range apis {
		interfaces = append(interfaces, ifaceExport{
			APIType:             api.APIType,
			APITypeName:         models.GetAPITypeName(api.APIType),
			Status:              api.Status,
			StatusName:          getAPIStatusName(api.Status),
			SubmitAlgorithm:     api.SubmitAlgorithm,
			SubmitAlgorithmName: models.GetAlgorithmName(api.SubmitAlgorithm),
			ReturnAlgorithm:     api.ReturnAlgorithm,
			ReturnAlgorithmName: models.GetAlgorithmName(api.ReturnAlgorithm),
			SubmitPublicKey:     api.SubmitPublicKey,
			SubmitPrivateKey:    api.SubmitPrivateKey,
			ReturnPublicKey:     api.ReturnPublicKey,
			ReturnPrivateKey:    api.ReturnPrivateKey,
		})
	}

	// 根据当前访问请求推导服务器地址（兼容反向代理）
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	}
	host := c.Request.Host
	if fwdHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); fwdHost != "" {
		host = fwdHost
	}
	baseURL := scheme + "://" + host

	payload := gin.H{
		"server": gin.H{
			"base_url":     baseURL,
			"api_endpoint": baseURL + "/api/open",
			"method":       "POST",
		},
		"app": gin.H{
			"name":    app.Name,
			"uuid":    app.UUID,
			"secret":  app.Secret,
			"version": app.Version,
		},
		"sign_rule":   "sign = SHA256(app_uuid|api_type|data|timestamp|app_secret) 转大写HEX",
		"exported_at": time.Now().Format("2006-01-02 15:04:05"),
		"interfaces":  interfaces,
	}

	// 导出含应用密钥与私钥，属敏感操作，记入操作日志
	operator := c.GetString("admin_username")
	if operator == "" {
		operator = "unknown"
	}
	services.RecordOperationLog("导出对接密钥", operator, c.GetString("admin_uuid"),
		fmt.Sprintf("导出应用 %s(%s) 的对接密钥", app.Name, app.UUID))

	apiBaseController.HandleSuccess(c, "导出成功", payload)
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
	// 直接取全部默认接口类型，避免此处硬编码遗漏
	validTypes := models.GetDefaultAPITypes()

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

	pub, priv, err := generateAlgorithmKeys(req.Algorithm)
	if err != nil {
		logrus.WithError(err).Error("Failed to generate keys")
		apiBaseController.HandleInternalError(c, "生成密钥失败", err)
		return
	}

	apiBaseController.HandleSuccess(c, "生成成功", map[string]interface{}{
		"public_key":  pub,
		"private_key": priv,
	})
}

// generateAlgorithmKeys 按算法生成一对(公钥, 私钥)明文；不加密返回空串。
// RC4/易加密只有私钥；RSA/RSA动态返回 PEM 公私钥。
func generateAlgorithmKeys(algorithm int) (string, string, error) {
	switch algorithm {
	case models.AlgorithmNone:
		return "", "", nil
	case models.AlgorithmRC4:
		key, err := encrypt.GenerateRC4Key(8)
		if err != nil {
			return "", "", err
		}
		return "", strings.ToUpper(hex.EncodeToString(key)), nil
	case models.AlgorithmRSA:
		publicKey, privateKey, err := encrypt.GenerateRSAKeyPair(2048)
		if err != nil {
			return "", "", err
		}
		pubPEM, err := encrypt.PublicKeyToPEM(publicKey)
		if err != nil {
			return "", "", err
		}
		privPEM, err := encrypt.PrivateKeyToPEM(privateKey)
		if err != nil {
			return "", "", err
		}
		return pubPEM, privPEM, nil
	case models.AlgorithmRSADynamic:
		pubPEM, privPEM, err := encrypt.GenerateRSADynamicKeyPair(2048)
		if err != nil {
			return "", "", err
		}
		return pubPEM, privPEM, nil
	case models.AlgorithmEasy:
		encryptKey, _, err := encrypt.GenerateEasyKey()
		if err != nil {
			return "", "", err
		}
		return "", encrypt.FormatKeyAsString(encryptKey), nil
	default:
		return "", "", errors.New("不支持的算法类型")
	}
}

// APIBatchSetAlgorithmHandler 批量设置所选接口的加密方式，并自动(重新)生成密钥。
// 无论原接口是否已生成密钥，都会按新算法重新生成并覆盖。
func APIBatchSetAlgorithmHandler(c *gin.Context) {
	var req struct {
		IDs             []uint `json:"ids"`
		SubmitAlgorithm int    `json:"submit_algorithm"`
		ReturnAlgorithm int    `json:"return_algorithm"`
	}
	if !apiBaseController.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		apiBaseController.HandleValidationError(c, "请选择要设置的接口")
		return
	}
	if !models.IsValidAlgorithm(req.SubmitAlgorithm) || !models.IsValidAlgorithm(req.ReturnAlgorithm) {
		apiBaseController.HandleValidationError(c, "无效的算法类型")
		return
	}

	db, ok := apiBaseController.GetDB(c)
	if !ok {
		return
	}

	var apis []models.API
	if err := db.Where("id IN ?", req.IDs).Find(&apis).Error; err != nil {
		apiBaseController.HandleInternalError(c, "查询接口失败", err)
		return
	}

	updated := 0
	for i := range apis {
		submitPub, submitPriv, err := generateAlgorithmKeys(req.SubmitAlgorithm)
		if err != nil {
			apiBaseController.HandleInternalError(c, "生成提交算法密钥失败", err)
			return
		}
		returnPub, returnPriv, err := generateAlgorithmKeys(req.ReturnAlgorithm)
		if err != nil {
			apiBaseController.HandleInternalError(c, "生成返回算法密钥失败", err)
			return
		}
		// 用 map 更新以保证空字符串（不加密）也能写入，覆盖旧密钥
		if err := db.Model(&models.API{}).Where("id = ?", apis[i].ID).Updates(map[string]interface{}{
			"submit_algorithm":   req.SubmitAlgorithm,
			"return_algorithm":   req.ReturnAlgorithm,
			"submit_public_key":  submitPub,
			"submit_private_key": submitPriv,
			"return_public_key":  returnPub,
			"return_private_key": returnPriv,
		}).Error; err != nil {
			apiBaseController.HandleInternalError(c, "更新接口失败", err)
			return
		}
		updated++
	}

	operator := c.GetString("admin_username")
	if operator == "" {
		operator = "unknown"
	}
	services.RecordOperationLog("批量设置接口算法", operator, c.GetString("admin_uuid"),
		fmt.Sprintf("批量设置 %d 个接口的加密算法并重新生成密钥", updated))

	apiBaseController.HandleSuccess(c, fmt.Sprintf("已设置 %d 个接口", updated), gin.H{"updated": updated})
}
