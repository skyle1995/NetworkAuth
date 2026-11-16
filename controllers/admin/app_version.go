package admin

import (
	"net/http"
	"strings"
	"time"

	"NetworkAuth/database"
	"NetworkAuth/models"

	"github.com/gin-gonic/gin"
)

// 版本列表管理：一个应用可以维护多条历史版本，更新检查取「已启用」中的最高版本。

// AppVersionsListHandler 列出某个应用的全部版本。
func AppVersionsListHandler(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "数据库未初始化"})
		return
	}
	uuid := strings.TrimSpace(c.Query("app_uuid"))
	if uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "应用UUID不能为空"})
		return
	}
	var rows []models.AppVersion
	if err := db.Where("app_uuid = ?", uuid).Order("published_at DESC, id DESC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "查询版本列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"list": rows}})
}

// AppVersionSaveHandler 新增或更新一条版本记录。
func AppVersionSaveHandler(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "数据库未初始化"})
		return
	}
	var req struct {
		ID           uint   `json:"id"`
		AppUUID      string `json:"app_uuid"`
		Version      string `json:"version"`
		DownloadType int    `json:"download_type"`
		DownloadURL  string `json:"download_url"`
		Changelog    string `json:"changelog"`
		Enabled      *bool  `json:"enabled"`
	}
	if !appBaseController.BindJSON(c, &req) {
		return
	}
	req.AppUUID = strings.TrimSpace(req.AppUUID)
	req.Version = strings.TrimSpace(req.Version)
	if req.AppUUID == "" || req.Version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "应用UUID与版本号不能为空"})
		return
	}
	if req.DownloadType < models.DownloadTypeDisabled || req.DownloadType > models.DownloadTypeFree {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "更新方式须为 0/1/2"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	row := models.AppVersion{
		AppUUID: req.AppUUID, Version: req.Version, DownloadType: req.DownloadType,
		DownloadURL: strings.TrimSpace(req.DownloadURL), Changelog: req.Changelog,
		Enabled: enabled, PublishedAt: time.Now().Unix(),
	}
	if req.ID > 0 {
		if err := db.Model(&models.AppVersion{}).Where("id = ?", req.ID).Updates(map[string]any{
			"version": row.Version, "download_type": row.DownloadType,
			"download_url": row.DownloadURL, "changelog": row.Changelog,
			"enabled": row.Enabled,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "更新版本失败（版本号可能重复）"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已更新"})
		return
	}
	if err := db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "新增版本失败（版本号可能重复）"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已新增", "data": row})
}

// AppVersionDeleteHandler 删除一条版本记录。
func AppVersionDeleteHandler(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "数据库未初始化"})
		return
	}
	var req struct {
		ID uint `json:"id"`
	}
	if !appBaseController.BindJSON(c, &req) || req.ID == 0 {
		return
	}
	if err := db.Delete(&models.AppVersion{}, req.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}
