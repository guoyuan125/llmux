package task

import (
	"log"

	"github.com/liuguoyuan/llmux/internal/config"
	"github.com/liuguoyuan/llmux/internal/model"
	"gorm.io/gorm"
)

// Runner manages background task goroutines.
type Runner struct {
	db   *gorm.DB
	cfg  *config.Config
	stop chan struct{}
}

// New creates a new task runner and seeds static data synchronously.
func New(db *gorm.DB, cfg *config.Config) *Runner {
	r := &Runner{db: db, cfg: cfg, stop: make(chan struct{})}

	if err := SeedModelPrices(db); err != nil {
		log.Printf("[task] model price seed error: %v", err)
	}

	return r
}

// Start launches all background goroutines.
func (r *Runner) Start() {
	log.Println("[task] starting background tasks")
	go runChannelHealthCheck(r.db, r.stop)
	go runAuditCleanup(r.db, r.cfg.Log.AuditRetentionDays, r.stop)
}

// Stop signals all background goroutines to exit.
func (r *Runner) Stop() {
	log.Println("[task] stopping background tasks")
	close(r.stop)
}

// suppress unused import warning — model is used by other files in this package
var _ = model.ModelPrice{}
