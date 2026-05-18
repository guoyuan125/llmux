package task

import (
	"log"

	"github.com/liuguoyuan/llmux/internal/model"
	"gorm.io/gorm"
)

// Runner manages background task goroutines.
type Runner struct {
	db   *gorm.DB
	stop chan struct{}
}

// New creates a new task runner and seeds static data synchronously.
func New(db *gorm.DB) *Runner {
	r := &Runner{db: db, stop: make(chan struct{})}

	// Seed model prices on startup (synchronous, fast — only runs when table is empty)
	if err := SeedModelPrices(db); err != nil {
		log.Printf("[task] model price seed error: %v", err)
	}

	return r
}

// Start launches all background goroutines.
func (r *Runner) Start() {
	log.Println("[task] starting background tasks")
	go runChannelHealthCheck(r.db, r.stop)
}

// Stop signals all background goroutines to exit.
func (r *Runner) Stop() {
	log.Println("[task] stopping background tasks")
	close(r.stop)
}

// suppress unused import warning — model is used by other files in this package
var _ = model.ModelPrice{}
