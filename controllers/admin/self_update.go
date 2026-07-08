package admin

import (
	selfupdate "NetworkAuth/services/selfupdate"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 自更新控制器
// ============================================================================

// SelfUpdateCheckHandler 触发异步检查更新（带防抖）
func SelfUpdateCheckHandler(c *gin.Context) {
	mgr := selfupdate.GetSelfUpdateManager()
	st := mgr.CheckAsync(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": st,
	})
}

// SelfUpdateCheckForceHandler 手动强制检查更新（绕过节流缓存，每次强拉最新）
func SelfUpdateCheckForceHandler(c *gin.Context) {
	mgr := selfupdate.GetSelfUpdateManager()
	st := mgr.CheckForceAsync(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": st,
	})
}

// SelfUpdateRestartHandler 管理员手动触发重启以加载已安装的新版本
func SelfUpdateRestartHandler(c *gin.Context) {
	mgr := selfupdate.GetSelfUpdateManager()
	mgr.RestartNow()
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "正在重启，请稍候…",
	})
}

// SelfUpdateStatusHandler 获取当前自更新状态
func SelfUpdateStatusHandler(c *gin.Context) {
	mgr := selfupdate.GetSelfUpdateManager()
	st := mgr.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": st,
	})
}

// SelfUpdateVersionsHandler 扫描存储桶获取版本列表
func SelfUpdateVersionsHandler(c *gin.Context) {
	mgr := selfupdate.GetSelfUpdateManager()
	// 未配置存储桶（type=0）属正常的"未启用"状态，返回空列表而非 400，
	// 避免前端打开页面自动扫描时收到错误；用户可在「更新配置」Tab 配置后再扫描
	if cfg := mgr.LoadConfig(); cfg.Type == 0 {
		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"data": []selfupdate.SelfUpdateVersionItem{},
		})
		return
	}
	items, err := mgr.ScanVersions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": items,
	})
}

// SelfUpdatePrepareRequest 更新准备请求
type SelfUpdatePrepareRequest struct {
	Version     string `json:"version" binding:"required"`      // 目标版本号
	DownloadURL string `json:"download_url" binding:"required"` // 下载链接
	SHA256      string `json:"sha256" binding:"required"`       // SHA256 哈希
}

// SelfUpdatePrepareHandler 下载并准备指定版本的更新
func SelfUpdatePrepareHandler(c *gin.Context) {
	var req SelfUpdatePrepareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": "请求参数无效",
		})
		return
	}

	mgr := selfupdate.GetSelfUpdateManager()
	st := mgr.Prepare(c.Request.Context(), req.Version, req.DownloadURL, req.SHA256)
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": st,
	})
}

// SelfUpdateGetConfigHandler 获取自更新存储桶配置
func SelfUpdateGetConfigHandler(c *gin.Context) {
	mgr := selfupdate.GetSelfUpdateManager()
	cfg := mgr.LoadConfig()
	// 脱敏：不返回完整密钥
	maskedCfg := gin.H{
		"type":       cfg.Type,
		"secret_id":  maskSecret(cfg.SecretID),
		"secret_key": maskSecret(cfg.SecretKey),
		"region":     cfg.Region,
		"bucket":     cfg.Bucket,
		"prefix":     cfg.Prefix,
		"base_url":   cfg.BaseURL,
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": maskedCfg,
	})
}

// SelfUpdateConfigRequest 更新配置请求
type SelfUpdateConfigRequest struct {
	Type      int    `json:"type"`
	SecretID  string `json:"secret_id"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	BaseURL   string `json:"base_url"`
}

// SelfUpdateUpdateConfigHandler 保存自更新存储桶配置
func SelfUpdateUpdateConfigHandler(c *gin.Context) {
	var req SelfUpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": "请求参数无效",
		})
		return
	}

	mgr := selfupdate.GetSelfUpdateManager()
	cfg := mgr.LoadConfig()

	// 只更新非空字段
	if req.Type > 0 || req.Type == 0 {
		cfg.Type = req.Type
	}
	if req.SecretID != "" && !strings.Contains(req.SecretID, "****") {
		cfg.SecretID = req.SecretID
	}
	if req.SecretKey != "" && !strings.Contains(req.SecretKey, "****") {
		cfg.SecretKey = req.SecretKey
	}
	if req.Region != "" {
		cfg.Region = req.Region
	}
	if req.Bucket != "" {
		cfg.Bucket = req.Bucket
	}
	if req.Prefix != "" {
		cfg.Prefix = req.Prefix
	}
	cfg.BaseURL = req.BaseURL // 允许清空

	if err := mgr.SaveConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": "保存配置失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "配置已保存",
	})
}

// SelfUpdateTestConfigHandler 测试存储桶连接
func SelfUpdateTestConfigHandler(c *gin.Context) {
	mgr := selfupdate.GetSelfUpdateManager()
	items, err := mgr.ScanVersions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": "连接失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "连接成功",
		"data": gin.H{
			"versions_count": len(items),
		},
	})
}

// maskSecret 对密钥进行脱敏处理，仅保留前4位和后4位
func maskSecret(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
