package task

import (
	"log"
	"time"

	"github.com/liuguoyuan/llmux/internal/model"
	"gorm.io/gorm"
)

// knownPrices is a curated list of well-known model prices in USD per million tokens.
// Source: public provider pricing pages (as of 2024-Q4).
var knownPrices = []model.ModelPrice{
	// OpenAI
	{ModelName: "gpt-4o", InputPrice: 5.0, OutputPrice: 15.0, Source: "manual"},
	{ModelName: "gpt-4o-2024-11-20", InputPrice: 2.5, OutputPrice: 10.0, Source: "manual"},
	{ModelName: "gpt-4o-mini", InputPrice: 0.15, OutputPrice: 0.60, Source: "manual"},
	{ModelName: "gpt-4-turbo", InputPrice: 10.0, OutputPrice: 30.0, Source: "manual"},
	{ModelName: "gpt-4-turbo-preview", InputPrice: 10.0, OutputPrice: 30.0, Source: "manual"},
	{ModelName: "gpt-4", InputPrice: 30.0, OutputPrice: 60.0, Source: "manual"},
	{ModelName: "gpt-3.5-turbo", InputPrice: 0.5, OutputPrice: 1.5, Source: "manual"},
	{ModelName: "o1", InputPrice: 15.0, OutputPrice: 60.0, Source: "manual"},
	{ModelName: "o1-mini", InputPrice: 3.0, OutputPrice: 12.0, Source: "manual"},
	{ModelName: "o3-mini", InputPrice: 1.1, OutputPrice: 4.4, Source: "manual"},
	// OpenAI Embeddings
	{ModelName: "text-embedding-3-small", InputPrice: 0.02, OutputPrice: 0, Source: "manual"},
	{ModelName: "text-embedding-3-large", InputPrice: 0.13, OutputPrice: 0, Source: "manual"},
	{ModelName: "text-embedding-ada-002", InputPrice: 0.10, OutputPrice: 0, Source: "manual"},
	// Anthropic
	{ModelName: "claude-3-5-sonnet-20241022", InputPrice: 3.0, OutputPrice: 15.0, Source: "manual"},
	{ModelName: "claude-3-5-sonnet-20240620", InputPrice: 3.0, OutputPrice: 15.0, Source: "manual"},
	{ModelName: "claude-3-5-haiku-20241022", InputPrice: 0.8, OutputPrice: 4.0, Source: "manual"},
	{ModelName: "claude-3-opus-20240229", InputPrice: 15.0, OutputPrice: 75.0, Source: "manual"},
	{ModelName: "claude-3-sonnet-20240229", InputPrice: 3.0, OutputPrice: 15.0, Source: "manual"},
	{ModelName: "claude-3-haiku-20240307", InputPrice: 0.25, OutputPrice: 1.25, Source: "manual"},
	{ModelName: "claude-opus-4-5", InputPrice: 15.0, OutputPrice: 75.0, Source: "manual"},
	{ModelName: "claude-sonnet-4-5", InputPrice: 3.0, OutputPrice: 15.0, Source: "manual"},
	// Google Gemini
	{ModelName: "gemini-1.5-pro", InputPrice: 1.25, OutputPrice: 5.0, Source: "manual"},
	{ModelName: "gemini-1.5-flash", InputPrice: 0.075, OutputPrice: 0.30, Source: "manual"},
	{ModelName: "gemini-2.0-flash", InputPrice: 0.10, OutputPrice: 0.40, Source: "manual"},
}

// SeedModelPrices inserts well-known model prices if the ModelPrice table is empty.
// Existing records are not overwritten.
func SeedModelPrices(db *gorm.DB) error {
	var count int64
	db.Model(&model.ModelPrice{}).Count(&count)
	if count > 0 {
		return nil
	}

	now := time.Now().Unix()
	prices := make([]model.ModelPrice, len(knownPrices))
	copy(prices, knownPrices)
	for i := range prices {
		prices[i].UpdatedAt = now
	}

	if err := db.Create(&prices).Error; err != nil {
		return err
	}
	log.Printf("[task] seeded %d model prices", len(prices))
	return nil
}
