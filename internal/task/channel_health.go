package task

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"time"

	"github.com/liuguoyuan/llmux/internal/model"
	"gorm.io/gorm"
)

const (
	healthCheckInterval = 5 * time.Minute
	healthCheckTimeout  = 5 * time.Second
)

// runChannelHealthCheck probes all enabled channel URLs periodically via TCP connect
// and updates ChannelURL.Latency in the database.
func runChannelHealthCheck(db *gorm.DB, stop <-chan struct{}) {
	// Run once immediately at startup, then on interval
	checkAllChannels(db)

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			checkAllChannels(db)
		}
	}
}

func checkAllChannels(db *gorm.DB) {
	var channels []model.Channel
	db.Preload("BaseURLs").Where("enabled = ?", true).Find(&channels)

	for _, ch := range channels {
		for _, u := range ch.BaseURLs {
			latency, err := tcpProbe(u.URL)
			if err != nil {
				log.Printf("[health] channel=%s url=%s error=%v", ch.Name, u.URL, err)
				// Set latency to 0 to indicate unknown/unreachable
				db.Model(&model.ChannelURL{}).Where("id = ?", u.ID).Update("latency", 0)
				continue
			}
			ms := int(latency.Milliseconds())
			db.Model(&model.ChannelURL{}).Where("id = ?", u.ID).Update("latency", ms)
			log.Printf("[health] channel=%s url=%s latency=%dms", ch.Name, u.URL, ms)
		}
	}
}

// tcpProbe measures the TCP connection time to the host derived from rawURL.
func tcpProbe(rawURL string) (time.Duration, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0, fmt.Errorf("invalid URL: %w", err)
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), healthCheckTimeout)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return time.Since(start), nil
}
