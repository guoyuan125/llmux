package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/model"
)

// StatsOverview returns aggregated statistics including entity counts.
func (h *Handler) StatsOverview(c *gin.Context) {
	var channelCount, groupCount, apiKeyCount int64
	h.db.Model(&model.Channel{}).Where("enabled = ?", true).Count(&channelCount)
	h.db.Model(&model.Group{}).Count(&groupCount)
	h.db.Model(&model.APIKey{}).Where("enabled = ?", true).Count(&apiKeyCount)

	today := time.Now().Format("2006-01-02")
	var todayStats model.StatsDaily
	h.db.Where("date = ?", today).First(&todayStats)

	// Calculate success rate and avg latency from today's stats
	var successRate float64
	var avgLatency int64
	if todayStats.TotalRequests > 0 {
		successRate = float64(todayStats.TotalRequests-todayStats.FailedRequests) / float64(todayStats.TotalRequests) * 100
		avgLatency = todayStats.TotalLatencyMs / todayStats.TotalRequests
	}

	c.JSON(http.StatusOK, gin.H{
		"channels":        channelCount,
		"groups":          groupCount,
		"api_keys":        apiKeyCount,
		"requests_today":  todayStats.TotalRequests,
		"tokens_today":    todayStats.InputTokens + todayStats.OutputTokens,
		"input_tokens":    todayStats.InputTokens,
		"output_tokens":   todayStats.OutputTokens,
		"cost_today":      todayStats.InputCost + todayStats.OutputCost,
		"failed_today":    todayStats.FailedRequests,
		"success_rate":    successRate,
		"avg_latency_ms":  avgLatency,
		"total_latency_ms": todayStats.TotalLatencyMs,
	})
}

// StatsDaily returns daily statistics.
func (h *Handler) StatsDaily(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days < 1 || days > 90 {
		days = 7
	}
	var stats []model.StatsDaily
	h.db.Where("date >= ?", time.Now().AddDate(0, 0, -days).Format("2006-01-02")).Order("date ASC").Find(&stats)
	c.JSON(http.StatusOK, stats)
}

// StatsHourly returns hourly statistics for rolling 24h.
func (h *Handler) StatsHourly(c *gin.Context) {
	var stats []model.StatsHourly
	today := time.Now().Format("2006-01-02")
	h.db.Where("date = ?", today).Order("hour ASC").Find(&stats)
	c.JSON(http.StatusOK, stats)
}

// StatsModels returns per-model statistics.
func (h *Handler) StatsModels(c *gin.Context) {
	var stats []model.StatsModel
	h.db.Order("total_requests DESC").Limit(50).Find(&stats)
	c.JSON(http.StatusOK, stats)
}

// StatsChannels returns per-channel statistics with channel names.
func (h *Handler) StatsChannels(c *gin.Context) {
	type ChannelStats struct {
		model.StatsChannel
		ChannelName string `json:"channel_name"`
	}
	var results []ChannelStats
	h.db.Table("stats_channels").
		Select("stats_channels.*, channels.name as channel_name").
		Joins("LEFT JOIN channels ON channels.id = stats_channels.channel_id").
		Order("stats_channels.total_requests DESC").
		Find(&results)
	c.JSON(http.StatusOK, results)
}

// ListAuditLogs returns audit logs with pagination and filtering.
func (h *Handler) ListAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
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
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"data":      logs,
	})
}
