package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/liuguoyuan/llmux/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSetModelSyncAuthHeaders(t *testing.T) {
	tests := []struct {
		name        string
		channelType model.ChannelType
		wantHeader  string
		wantValue   string
	}{
		{name: "openai bearer", channelType: model.ChannelTypeOpenAI, wantHeader: "Authorization", wantValue: "Bearer test-key"},
		{name: "anthropic x api key", channelType: model.ChannelTypeAnthropic, wantHeader: "x-api-key", wantValue: "test-key"},
		{name: "gemini google api key", channelType: model.ChannelTypeGemini, wantHeader: "x-goog-api-key", wantValue: "test-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			setModelSyncAuthHeaders(req, tt.channelType, "test-key")

			if got := req.Header.Get(tt.wantHeader); got != tt.wantValue {
				t.Fatalf("%s = %q, want %q", tt.wantHeader, got, tt.wantValue)
			}
		})
	}
}

func TestDuplicateChannelCopiesNestedConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newChannelTestDB(t)
	h := &Handler{db: db}

	src := model.Channel{
		Name:          "deepseek",
		Type:          model.ChannelTypeAnthropic,
		Enabled:       true,
		Models:        "upstream-model",
		CustomModels:  "deepseek-v4-pro",
		AutoSync:      true,
		Proxy:         "http://proxy.local",
		ParamOverride: `{"temperature":0}`,
		BaseURLs: []model.ChannelURL{
			{URL: "https://api.deepseek.com/anthropic", Latency: 42},
		},
		Keys: []model.ChannelKey{
			{Key: "sk-test", Enabled: true, Remark: "primary"},
		},
	}
	if err := db.Session(&gorm.Session{FullSaveAssociations: true}).Create(&src).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}

	router := gin.New()
	router.POST("/channels/:id/duplicate", h.DuplicateChannel)

	req := httptest.NewRequest(http.MethodPost, "/channels/"+strconv.FormatUint(uint64(src.ID), 10)+"/duplicate", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var dup model.Channel
	if err := db.Preload("BaseURLs").Preload("Keys").Where("name = ?", "deepseek (copy)").First(&dup).Error; err != nil {
		t.Fatalf("load duplicate: %v", err)
	}
	if dup.ID == src.ID {
		t.Fatal("duplicate reused source ID")
	}
	if dup.Type != src.Type || dup.CustomModels != src.CustomModels || dup.Proxy != src.Proxy || dup.ParamOverride != src.ParamOverride {
		t.Fatalf("duplicate fields not preserved: %#v", dup)
	}
	if len(dup.BaseURLs) != 1 || dup.BaseURLs[0].URL != src.BaseURLs[0].URL || dup.BaseURLs[0].ChannelID != dup.ID {
		t.Fatalf("duplicate URLs not copied correctly: %#v", dup.BaseURLs)
	}
	if len(dup.Keys) != 1 || dup.Keys[0].Key != src.Keys[0].Key || dup.Keys[0].Remark != src.Keys[0].Remark || dup.Keys[0].ChannelID != dup.ID {
		t.Fatalf("duplicate keys not copied correctly: %#v", dup.Keys)
	}
}

func newChannelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Channel{}, &model.ChannelURL{}, &model.ChannelKey{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}
