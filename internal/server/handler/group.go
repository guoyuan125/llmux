package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/model"
	"gorm.io/gorm"
)

// ListGroups returns all groups with their items.
func (h *Handler) ListGroups(c *gin.Context) {
	var groups []model.Group
	if err := h.db.Preload("Items").Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

// CreateGroup creates a new group and its nested Items.
func (h *Handler) CreateGroup(c *gin.Context) {
	var g model.Group
	if err := c.ShouldBindJSON(&g); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Session(&gorm.Session{FullSaveAssociations: true}).Create(&g).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, g)
}

// UpdateGroup replaces an existing group's fields and Items.
func (h *Handler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	var g model.Group
	if err := h.db.First(&g, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	var input model.Group
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&g).Updates(map[string]interface{}{
			"name":                input.Name,
			"models":              input.Models,
			"mode":                int(input.Mode),
			"context_size":        input.ContextSize,
			"first_token_timeout": input.FirstTokenTimeout,
			"session_keep_time":   input.SessionKeepTime,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", g.ID).Delete(&model.GroupItem{}).Error; err != nil {
			return err
		}
		for i := range input.Items {
			input.Items[i].ID = 0
			input.Items[i].GroupID = g.ID
		}
		if len(input.Items) > 0 {
			if err := tx.Create(&input.Items).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("Items").First(&g, g.ID)
	c.JSON(http.StatusOK, g)
}

// DuplicateGroup copies an existing group (and its items) with name appended " (copy)".
func (h *Handler) DuplicateGroup(c *gin.Context) {
	id := c.Param("id")
	var src model.Group
	if err := h.db.Preload("Items").First(&src, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	newGroup := model.Group{
		Name:               src.Name + " (copy)",
		Models:             src.Models,
		Mode:               src.Mode,
		ContextSize:        src.ContextSize,
		FirstTokenTimeout:  src.FirstTokenTimeout,
		SessionKeepTime:    src.SessionKeepTime,
	}
	for _, it := range src.Items {
		newGroup.Items = append(newGroup.Items, model.GroupItem{
			ChannelID: it.ChannelID,
			ModelName: it.ModelName,
			Priority:  it.Priority,
			Weight:    it.Weight,
		})
	}

	if err := h.db.Session(&gorm.Session{FullSaveAssociations: true}).Create(&newGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Preload("Items").First(&newGroup, newGroup.ID)
	c.JSON(http.StatusCreated, newGroup)
}

// DeleteGroup deletes a group and its nested Items.
func (h *Handler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&model.GroupItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Group{}, id).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
