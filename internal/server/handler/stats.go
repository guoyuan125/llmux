package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/model"
)

// StatsOverview returns aggregated statistics.
func (h *Handler) StatsOverview(c *gin.Context) {
	var total model.Stats
	h.db.Model(&model.StatsDaily{}).
		Select("SUM(input_tokens) as input_tokens, SUM(output_tokens) as output_tokens, "+
			"SUM(input_cost) as input_cost, SUM(output_cost) as output_cost, "+
			"SUM(total_requests) as total_requests, SUM(failed_requests) as failed_requests").
		Scan(&total)
	c.JSON(http.StatusOK, total)
}

// StatsDaily returns daily statistics.
func (h *Handler) StatsDaily(c *gin.Context) {
	var stats []model.StatsDaily
	h.db.Order("date DESC").Limit(30).Find(&stats)
	c.JSON(http.StatusOK, stats)
}

// StatsModels returns per-model statistics.
func (h *Handler) StatsModels(c *gin.Context) {
	var stats []model.StatsModel
	h.db.Order("total_requests DESC").Limit(50).Find(&stats)
	c.JSON(http.StatusOK, stats)
}

// ListAuditLogs returns audit logs with pagination and filtering.
func (h *Handler) ListAuditLogs(c *gin.Context) {
	page := 1
	pageSize := 50
	modelFilter := c.Query("model")

	query := h.db.Model(&model.AuditLog{}).Order("created_at DESC")
	if modelFilter != "" {
		query = query.Where("model = ?", modelFilter)
	}

	var total int64
	query.Count(&total)

	var logs []model.AuditLog
	query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"data":  logs,
	})
}
