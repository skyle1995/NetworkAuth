package admin

import (
	"NetworkAuth/controllers"
	"NetworkAuth/models"
	"NetworkAuth/services"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 全局变量
// ============================================================================

// 创建基础控制器实例
var cardBaseController = controllers.NewBaseController()

// ============================================================================
// 辅助函数
// ============================================================================

// formatCardDuration 将面值分钟数格式化为便于展示的中文文案
func formatCardDuration(minutes int) string {
	if minutes == models.CardDurationPermanent {
		return "永久"
	}
	switch {
	case minutes%(365*24*60) == 0:
		return strconv.Itoa(minutes/(365*24*60)) + "年"
	case minutes%(30*24*60) == 0:
		return strconv.Itoa(minutes/(30*24*60)) + "个月"
	case minutes%(24*60) == 0:
		return strconv.Itoa(minutes/(24*60)) + "天"
	case minutes%60 == 0:
		return strconv.Itoa(minutes/60) + "小时"
	default:
		return strconv.Itoa(minutes) + "分钟"
	}
}

// cardStatusText 卡密状态文案
func cardStatusText(status int) string {
	switch status {
	case models.CardStatusUnused:
		return "未使用"
	case models.CardStatusUsed:
		return "已使用"
	case models.CardStatusFrozen:
		return "已冻结"
	default:
		return "未知"
	}
}

// recordCardLog 记录卡密相关操作日志
func recordCardLog(c *gin.Context, action, details string) {
	operator := c.GetString("admin_username")
	if operator == "" {
		operator = "unknown"
	}
	services.RecordOperationLog(action, operator, c.GetString("admin_uuid"), details)
}

// ============================================================================
// API处理器
// ============================================================================

// CardListHandler 卡密列表API处理器
// 支持按应用、状态、批次号筛选，按卡号精确搜索，分页返回。
func CardListHandler(c *gin.Context) {
	page, limit := cardBaseController.GetPaginationParams(c)

	db, ok := cardBaseController.GetDB(c)
	if !ok {
		return
	}

	query := db.Model(&models.Card{})

	if appUUID := strings.TrimSpace(c.Query("app_uuid")); appUUID != "" {
		query = query.Where("app_uuid = ?", appUUID)
	}
	if batchNo := strings.TrimSpace(c.Query("batch_no")); batchNo != "" {
		query = query.Where("batch_no = ?", batchNo)
	}
	// 状态筛选：需与空串区分，空串表示不筛选
	if statusStr := strings.TrimSpace(c.Query("status")); statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil {
			query = query.Where("status = ?", status)
		}
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query = query.Where("card_no = ?", search)
	}

	cards, total, err := services.Paginate[models.Card](query, page, limit, "created_at DESC")
	if err != nil {
		logrus.WithError(err).Error("Failed to fetch cards")
		cardBaseController.HandleInternalError(c, "查询卡密列表失败", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"msg":   "success",
		"count": total,
		"data":  toCardResponses(cards),
	})
}

// cardResponse 卡密列表/导出的统一返回结构
type cardResponse struct {
	ID           uint   `json:"id"`
	UUID         string `json:"uuid"`
	CardNo       string `json:"card_no"`
	AppUUID      string `json:"app_uuid"`
	BatchNo      string `json:"batch_no"`
	Duration     int    `json:"duration"`
	DurationText string `json:"duration_text"`
	Points       int    `json:"points"`
	Status       int    `json:"status"`
	StatusText   string `json:"status_text"`
	UsedByMember string `json:"used_by_member"`
	UsedAt       string `json:"used_at"`
	Remark       string `json:"remark"`
	CreatedAt    string `json:"created_at"`
}

// toCardResponses 将卡密模型批量转换为返回结构
func toCardResponses(cards []models.Card) []cardResponse {
	list := make([]cardResponse, 0, len(cards))
	for _, card := range cards {
		usedAt := ""
		if card.UsedAt != nil {
			usedAt = card.UsedAt.Format("2006-01-02 15:04:05")
		}
		list = append(list, cardResponse{
			ID:           card.ID,
			UUID:         card.UUID,
			CardNo:       card.CardNo,
			AppUUID:      card.AppUUID,
			BatchNo:      card.BatchNo,
			Duration:     card.Duration,
			DurationText: formatCardDuration(card.Duration),
			Points:       card.Points,
			Status:       card.Status,
			StatusText:   cardStatusText(card.Status),
			UsedByMember: card.UsedByMember,
			UsedAt:       usedAt,
			Remark:       card.Remark,
			CreatedAt:    card.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return list
}

// CardExportHandler 卡密导出API处理器
// 勾选了 ids 则导出指定卡密；否则按筛选条件（应用/状态/批次/卡号）导出全部（不分页）。
func CardExportHandler(c *gin.Context) {
	var req struct {
		IDs     []uint `json:"ids"`
		AppUUID string `json:"app_uuid"`
		BatchNo string `json:"batch_no"`
		Status  string `json:"status"`
		Search  string `json:"search"`
	}
	if !cardBaseController.BindJSON(c, &req) {
		return
	}

	db, ok := cardBaseController.GetDB(c)
	if !ok {
		return
	}

	query := db.Model(&models.Card{})
	if len(req.IDs) > 0 {
		// 导出选中：仅按 ids 过滤，忽略其它筛选
		query = query.Where("id IN ?", req.IDs)
	} else {
		if appUUID := strings.TrimSpace(req.AppUUID); appUUID != "" {
			query = query.Where("app_uuid = ?", appUUID)
		}
		if batchNo := strings.TrimSpace(req.BatchNo); batchNo != "" {
			query = query.Where("batch_no = ?", batchNo)
		}
		if statusStr := strings.TrimSpace(req.Status); statusStr != "" {
			if status, err := strconv.Atoi(statusStr); err == nil {
				query = query.Where("status = ?", status)
			}
		}
		if search := strings.TrimSpace(req.Search); search != "" {
			query = query.Where("card_no = ?", search)
		}
	}

	var cards []models.Card
	if err := query.Order("created_at DESC").Find(&cards).Error; err != nil {
		logrus.WithError(err).Error("Failed to export cards")
		cardBaseController.HandleInternalError(c, "导出卡密失败", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": toCardResponses(cards),
	})
}

// CardCreateHandler 批量制卡API处理器
func CardCreateHandler(c *gin.Context) {
	var req struct {
		AppUUID       string `json:"app_uuid"`
		Prefix        string `json:"prefix"`
		Length        int    `json:"length"`
		Count         int    `json:"count"`
		DurationValue int    `json:"duration_value"`
		DurationUnit  string `json:"duration_unit"`
		Points        int    `json:"points"`
		Remark        string `json:"remark"`
	}

	if !cardBaseController.BindJSON(c, &req) {
		return
	}

	if !cardBaseController.ValidateRequired(c, map[string]interface{}{
		"应用UUID": req.AppUUID,
	}) {
		return
	}

	if req.Count <= 0 {
		cardBaseController.HandleValidationError(c, "生成数量必须大于0")
		return
	}
	if req.Length <= 0 {
		req.Length = 16
	}

	// 时长模式换算面值时长；点数模式不传时长单位，durationMinutes 置 0
	durationMinutes := 0
	if req.DurationUnit != "" {
		var err error
		durationMinutes, err = services.CardDurationToMinutes(req.DurationValue, req.DurationUnit)
		if err != nil {
			cardBaseController.HandleValidationError(c, err.Error())
			return
		}
	}

	cards, batchNo, err := services.BatchCreateCards(
		strings.TrimSpace(req.AppUUID),
		strings.TrimSpace(req.Prefix),
		req.Length,
		req.Count,
		durationMinutes,
		req.Points,
		strings.TrimSpace(req.Remark),
	)
	if err != nil {
		logrus.WithError(err).Error("Failed to batch create cards")
		cardBaseController.HandleValidationError(c, err.Error())
		return
	}

	recordCardLog(c, "制卡", fmt.Sprintf("为应用 %s 生成 %d 张卡密（批次 %s）", req.AppUUID, len(cards), batchNo))

	cardBaseController.HandleSuccess(c, "制卡成功", gin.H{
		"batch_no": batchNo,
		"count":    len(cards),
	})
}

// CardFreezeHandler 批量冻结卡密API处理器
func CardFreezeHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if !cardBaseController.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		cardBaseController.HandleValidationError(c, "请选择要冻结的卡密")
		return
	}

	if err := services.FreezeCards(req.IDs); err != nil {
		logrus.WithError(err).Error("Failed to freeze cards")
		cardBaseController.HandleInternalError(c, "冻结卡密失败", err)
		return
	}

	recordCardLog(c, "冻结卡密", fmt.Sprintf("冻结了 %d 张卡密", len(req.IDs)))
	cardBaseController.HandleSuccess(c, "冻结成功", nil)
}

// CardUnfreezeHandler 批量解冻卡密API处理器
func CardUnfreezeHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if !cardBaseController.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		cardBaseController.HandleValidationError(c, "请选择要解冻的卡密")
		return
	}

	if err := services.UnfreezeCards(req.IDs); err != nil {
		logrus.WithError(err).Error("Failed to unfreeze cards")
		cardBaseController.HandleInternalError(c, "解冻卡密失败", err)
		return
	}

	recordCardLog(c, "解冻卡密", fmt.Sprintf("解冻了 %d 张卡密", len(req.IDs)))
	cardBaseController.HandleSuccess(c, "解冻成功", nil)
}

// CardsBatchDeleteHandler 批量删除卡密API处理器
func CardsBatchDeleteHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if !cardBaseController.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		cardBaseController.HandleValidationError(c, "请选择要删除的卡密")
		return
	}

	if err := services.DeleteCards(req.IDs); err != nil {
		logrus.WithError(err).Error("Failed to batch delete cards")
		cardBaseController.HandleInternalError(c, "批量删除失败", err)
		return
	}

	recordCardLog(c, "删除卡密", fmt.Sprintf("批量删除了 %d 张卡密", len(req.IDs)))
	cardBaseController.HandleSuccess(c, "批量删除成功", nil)
}

// CardDeleteByBatchHandler 按批次号删除整批卡密API处理器
func CardDeleteByBatchHandler(c *gin.Context) {
	var req struct {
		AppUUID string `json:"app_uuid"`
		BatchNo string `json:"batch_no"`
	}
	if !cardBaseController.BindJSON(c, &req) {
		return
	}
	if !cardBaseController.ValidateRequired(c, map[string]interface{}{
		"应用UUID": req.AppUUID,
		"批次号":    req.BatchNo,
	}) {
		return
	}

	affected, err := services.DeleteCardsByBatch(strings.TrimSpace(req.AppUUID), strings.TrimSpace(req.BatchNo))
	if err != nil {
		logrus.WithError(err).Error("Failed to delete cards by batch")
		cardBaseController.HandleInternalError(c, "按批次删除失败", err)
		return
	}

	recordCardLog(c, "删除卡密", fmt.Sprintf("删除了批次 %s 共 %d 张卡密", req.BatchNo, affected))
	cardBaseController.HandleSuccess(c, "删除成功", gin.H{"count": affected})
}
