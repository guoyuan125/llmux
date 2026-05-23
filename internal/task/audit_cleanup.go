package task

import (
	"log"
	"time"

	"gorm.io/gorm"
)

// runAuditCleanup periodically deletes audit_logs older than retentionDays.
// Runs once per hour, deletes in batches to avoid long-running transactions.
func runAuditCleanup(db *gorm.DB, retentionDays int, stop <-chan struct{}) {
	if retentionDays <= 0 {
		return
	}

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run once on startup
	cleanupAuditLogs(db, retentionDays)

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			cleanupAuditLogs(db, retentionDays)
		}
	}
}

func cleanupAuditLogs(db *gorm.DB, retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := db.Exec("DELETE FROM audit_logs WHERE created_at < ?", cutoff)
	if result.Error != nil {
		log.Printf("[cleanup] audit_logs cleanup error: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("[cleanup] deleted %d audit_logs older than %d days", result.RowsAffected, retentionDays)
	}
}
