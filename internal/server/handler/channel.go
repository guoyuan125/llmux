package handler

import (
	"net/http"

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

// DeleteChannel deletes a channel and its nested BaseURLs and Keys.
func (h *Handler) DeleteChannel(c *gin.Context) {
	id := c.Param("id")
	err := h.db.Transaction(func(tx *gorm.DB) error {
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
