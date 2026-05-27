package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/model"
	"gorm.io/gorm"
)

// ListChannels returns all channels with their URLs and keys.
func (h *Handler) ListChannels(c *gin.Context) {
	var channels []model.Channel
	if err := h.db.Preload("BaseURLs").Preload("Keys").Find(&channels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, channels)
}

// CreateChannel creates a new channel and its nested BaseURLs and Keys.
func (h *Handler) CreateChannel(c *gin.Context) {
	var ch model.Channel
	if err := c.ShouldBindJSON(&ch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Session(&gorm.Session{FullSaveAssociations: true}).Create(&ch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ch)
}

// UpdateChannel replaces an existing channel's fields, BaseURLs, and Keys.
func (h *Handler) UpdateChannel(c *gin.Context) {
	id := c.Param("id")
	var ch model.Channel
	if err := h.db.First(&ch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	var input model.Channel
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ch).Updates(map[string]interface{}{
			"name":           input.Name,
			"type":           int(input.Type),
			"enabled":        input.Enabled,
			"models":         input.Models,
			"custom_models":  input.CustomModels,
			"auto_sync":      input.AutoSync,
			"proxy":          input.Proxy,
			"param_override": input.ParamOverride,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", ch.ID).Delete(&model.ChannelURL{}).Error; err != nil {
			return err
		}
		for i := range input.BaseURLs {
			input.BaseURLs[i].ID = 0
			input.BaseURLs[i].ChannelID = ch.ID
		}
		if len(input.BaseURLs) > 0 {
			if err := tx.Create(&input.BaseURLs).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("channel_id = ?", ch.ID).Delete(&model.ChannelKey{}).Error; err != nil {
			return err
		}
		for i := range input.Keys {
			input.Keys[i].ID = 0
			input.Keys[i].ChannelID = ch.ID
		}
		if len(input.Keys) > 0 {
			if err := tx.Create(&input.Keys).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("BaseURLs").Preload("Keys").First(&ch, ch.ID)
	c.JSON(http.StatusOK, ch)
}

// SyncChannelModels fetches the model list from the upstream provider and returns it.
// It does NOT modify the channel's custom_models — the frontend handles selection and save.
func (h *Handler) SyncChannelModels(c *gin.Context) {
	id := c.Param("id")
	var ch model.Channel
	if err := h.db.Preload("BaseURLs").Preload("Keys").First(&ch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}
	if len(ch.BaseURLs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel has no base URLs configured"})
		return
	}
	// Pick first enabled key; fall back to first key if none are enabled.
	var apiKey string
	for _, k := range ch.Keys {
		if k.Enabled {
			apiKey = k.Key
			break
		}
	}
	if apiKey == "" && len(ch.Keys) > 0 {
		apiKey = ch.Keys[0].Key
	}
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel has no API keys configured"})
		return
	}

	upstreamURL := strings.TrimRight(ch.BaseURLs[0].URL, "/") + "/v1/models"

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request: " + err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	setModelSyncAuthHeaders(req, ch.Type, apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read upstream response"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream returned " + resp.Status, "body": string(body)})
		return
	}

	// Parse OpenAI models list format: {"object":"list","data":[{"id":"model-name",...},...]}
	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to parse upstream response: " + err.Error()})
		return
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	sort.Strings(models)

	c.JSON(http.StatusOK, gin.H{"models": models})
}

func setModelSyncAuthHeaders(req *http.Request, channelType model.ChannelType, apiKey string) {
	switch channelType {
	case model.ChannelTypeAnthropic:
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case model.ChannelTypeGemini:
		req.Header.Set("x-goog-api-key", apiKey)
	default:
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

// DuplicateChannel copies an existing channel, including URLs, keys, and model config.
func (h *Handler) DuplicateChannel(c *gin.Context) {
	id := c.Param("id")
	var src model.Channel
	if err := h.db.Preload("BaseURLs").Preload("Keys").First(&src, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	newChannel := model.Channel{
		Name:          src.Name + " (copy)",
		Type:          src.Type,
		Enabled:       src.Enabled,
		Models:        src.Models,
		CustomModels:  src.CustomModels,
		AutoSync:      src.AutoSync,
		Proxy:         src.Proxy,
		ParamOverride: src.ParamOverride,
	}
	for _, u := range src.BaseURLs {
		newChannel.BaseURLs = append(newChannel.BaseURLs, model.ChannelURL{
			URL:     u.URL,
			Latency: u.Latency,
		})
	}
	for _, k := range src.Keys {
		newChannel.Keys = append(newChannel.Keys, model.ChannelKey{
			Key:     k.Key,
			Enabled: k.Enabled,
			Remark:  k.Remark,
		})
	}

	if err := h.db.Session(&gorm.Session{FullSaveAssociations: true}).Create(&newChannel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("BaseURLs").Preload("Keys").First(&newChannel, newChannel.ID)
	c.JSON(http.StatusCreated, newChannel)
}

// DeleteChannel deletes a channel, its nested BaseURLs and Keys, and any GroupItems referencing it.
// If ?check=true is passed, returns affected group names without deleting.
func (h *Handler) DeleteChannel(c *gin.Context) {
	id := c.Param("id")

	// Collect affected groups regardless (used for check and for cascade delete).
	type affectedGroup struct {
		ID   uint   `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	var affected []affectedGroup
	h.db.Raw(`
		SELECT g.id, g.name FROM groups g
		INNER JOIN group_items gi ON gi.group_id = g.id
		WHERE gi.channel_id = ?`, id).Scan(&affected)

	if c.Query("check") == "true" {
		names := make([]string, len(affected))
		for i, g := range affected {
			names[i] = g.Name
		}
		c.JSON(http.StatusOK, gin.H{"groups": names})
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", id).Delete(&model.GroupItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", id).Delete(&model.ChannelURL{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", id).Delete(&model.ChannelKey{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Channel{}, id).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
