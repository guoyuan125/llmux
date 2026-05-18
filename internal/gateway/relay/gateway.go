package relay

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/liuguoyuan/llmux/internal/gateway/balancer"
	"github.com/liuguoyuan/llmux/internal/gateway/circuit"
	"github.com/liuguoyuan/llmux/internal/gateway/session"
	"github.com/liuguoyuan/llmux/internal/metrics"
	"github.com/liuguoyuan/llmux/internal/model"
	"github.com/liuguoyuan/llmux/internal/ratelimit"
	"github.com/liuguoyuan/llmux/internal/transformer/types"
	inboundAnthropic "github.com/liuguoyuan/llmux/internal/transformer/inbound/anthropic"
	inboundOpenAI "github.com/liuguoyuan/llmux/internal/transformer/inbound/openai"
	outboundAnthropic "github.com/liuguoyuan/llmux/internal/transformer/outbound/anthropic"
	outboundOpenAI "github.com/liuguoyuan/llmux/internal/transformer/outbound/openai"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// Gateway is the core relay engine.
type Gateway struct {
	db           *gorm.DB
	circuit      *circuit.Manager
	sessions     *session.Store
	client       *http.Client
	logPublisher func(log *model.AuditLog)
}

// NewGateway creates a new relay gateway.
func NewGateway(db *gorm.DB) *Gateway {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		TLSClientConfig:       &tls.Config{},
		ResponseHeaderTimeout: 60 * time.Second,
	}

	return &Gateway{
		db:       db,
		circuit:  circuit.NewManager(nil),
		sessions: session.NewStore(),
		client:   &http.Client{Transport: transport},
	}
}

// SetLogPublisher sets the callback invoked on each audit log creation.
func (g *Gateway) SetLogPublisher(fn func(log *model.AuditLog)) {
	g.logPublisher = fn
}

// InboundType identifies the inbound protocol.
type InboundType int

const (
	InboundOpenAIChat InboundType = iota
	InboundOpenAIResponses
	InboundAnthropic
	InboundOpenAIEmbedding
)

// HandleRelay is the main entry point for relay handlers.
func (g *Gateway) HandleRelay(c *gin.Context, inboundType InboundType) {
	startTime := time.Now()
	apiKeyID := c.GetUint("api_key_id")
	supportedModels := c.GetString("supported_models")

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Create inbound adapter
	inAdapter := g.getInbound(inboundType)

	// Parse request
	internalReq, err := inAdapter.TransformRequest(c.Request.Context(), body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check model whitelist
	if supportedModels != "" {
		models := strings.Split(supportedModels, ",")
		allowed := false
		for _, m := range models {
			if strings.TrimSpace(m) == internalReq.Model {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "model not allowed for this API key"})
			return
		}
	}

	requestModel := internalReq.Model
	isStream := internalReq.Stream != nil && *internalReq.Stream
	isEmbedding := inboundType == InboundOpenAIEmbedding

	// Find group
	group, err := g.findGroup(requestModel)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("model not found: %s", requestModel)})
		return
	}

	// Get candidates via balancer
	b := balancer.Get(group.Mode)
	items := group.Items
	if group.Mode == model.GroupModeLeastCost || group.Mode == model.GroupModeLeastLatency {
		g.populateRuntimeData(items)
	}
	candidates := b.Candidates(items)
	if len(candidates) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available channels"})
		return
	}

	// Session stickiness: move sticky channel to front
	if group.SessionKeepTime > 0 {
		if chID, _, ok := g.sessions.Get(apiKeyID, requestModel); ok {
			candidates = moveToFront(candidates, chID)
		}
	}

	// Iterate candidates
	var lastErr error
	attempts := 0

	for _, item := range candidates {
		select {
		case <-c.Request.Context().Done():
			log.Printf("request cancelled, stopping retry")
			return
		default:
		}

		attempts++

		// Circuit breaker check
		cbKey := circuit.BreakerKey(item.ChannelID, 0)
		if !g.circuit.Allow(cbKey) {
			log.Printf("[RELAY] circuit breaker OPEN for channel %d, skipping", item.ChannelID)
			continue
		}

		// Load channel
		var channel model.Channel
		if err := g.db.Preload("BaseURLs").Preload("Keys").First(&channel, item.ChannelID).Error; err != nil {
			continue
		}
		if !channel.Enabled {
			continue
		}

		// Select best URL and key
		baseURL := selectBestURL(channel.BaseURLs)
		if baseURL == "" {
			continue
		}
		channelKey := selectBestKey(channel.Keys)
		if channelKey == nil {
			continue
		}

		// Set actual upstream model
		if item.ModelName != "" && item.ModelName != "*" {
			internalReq.Model = item.ModelName
		}

		// Get outbound adapter
		var outAdapter types.Outbound
		if isEmbedding {
			outAdapter = g.getEmbeddingOutbound(channel.Type)
		} else {
			outAdapter = g.getOutbound(channel.Type)
		}

		var outReq *types.OutboundHTTPRequest

		// For Anthropic→Anthropic, passthrough the raw request body with only model name replaced.
		// This preserves thinking blocks, signatures, and other protocol-specific fields.
		if inboundType == InboundAnthropic && channel.Type == model.ChannelTypeAnthropic {
			upstreamModel := item.ModelName
			modifiedBody := replaceModelInBody(body, upstreamModel)
			url := strings.TrimRight(baseURL, "/") + "/v1/messages"
			headers := map[string]string{
				"Content-Type":      "application/json",
				"x-api-key":         channelKey.Key,
				"anthropic-version": "2023-06-01",
			}
			// Forward client's anthropic-beta and anthropic-version headers
			if v := c.GetHeader("anthropic-beta"); v != "" {
				headers["anthropic-beta"] = v
			}
			if v := c.GetHeader("anthropic-version"); v != "" {
				headers["anthropic-version"] = v
			}
			outReq = &types.OutboundHTTPRequest{
				Method:  "POST",
				URL:     url,
				Headers: headers,
				Body:    modifiedBody,
			}
		} else {
			// Build outbound request via transformer
			outReq, err = outAdapter.TransformRequest(c.Request.Context(), internalReq, baseURL, channelKey.Key)
			if err != nil {
				lastErr = err
				continue
			}
		}

		// Apply param override
		if channel.ParamOverride != "" {
			outReq.Body = applyParamOverride(outReq.Body, channel.ParamOverride)
		}

		// Send request
		httpReq, err := http.NewRequestWithContext(c.Request.Context(), outReq.Method, outReq.URL, bytes.NewReader(outReq.Body))
		if err != nil {
			lastErr = err
			continue
		}
		for k, v := range outReq.Headers {
			httpReq.Header.Set(k, v)
		}

		// Forward passthrough headers from client request
		for _, h := range []string{"comate_custom_header"} {
			if v := c.GetHeader(h); v != "" {
				httpReq.Header.Set(h, v)
			}
		}

		// Use channel-specific proxy if configured
		client := g.getHTTPClient(channel.Proxy)

		resp, err := client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("channel %s: request failed: %w", channel.Name, err)
			g.circuit.RecordFailure(cbKey)
			continue
		}

		// Check status
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("channel %s: upstream %d: %s", channel.Name, resp.StatusCode, string(respBody))
			g.circuit.RecordFailure(cbKey)

			// Update key status
			channelKey.StatusCode = resp.StatusCode
			channelKey.LastUsedAt = time.Now().Unix()
			g.db.Save(channelKey)
			continue
		}

		// Success path
		g.circuit.RecordSuccess(cbKey)
		channelKey.StatusCode = resp.StatusCode
		channelKey.LastUsedAt = time.Now().Unix()

		// Update session stickiness
		if group.SessionKeepTime > 0 {
			g.sessions.Set(apiKeyID, requestModel, channel.ID, channelKey.ID, time.Duration(group.SessionKeepTime)*time.Second)
		}

		// Handle response
		if isEmbedding {
			g.handleEmbeddingPassthrough(c, resp, outAdapter, &channel, channelKey, requestModel, startTime, apiKeyID, attempts)
		} else if isStream {
			// When inbound and outbound are both Anthropic, raw passthrough preserves
			// thinking blocks and other protocol-specific features without lossy conversion.
			if inboundType == InboundAnthropic && channel.Type == model.ChannelTypeAnthropic {
				g.handleStreamPassthrough(c, resp, &channel, channelKey, requestModel, startTime, apiKeyID, attempts, group.FirstTokenTimeout)
			} else {
				g.handleStreamResponse(c, resp, inAdapter, outAdapter, &channel, channelKey, requestModel, startTime, apiKeyID, attempts, group.FirstTokenTimeout)
			}
		} else {
			if inboundType == InboundAnthropic && channel.Type == model.ChannelTypeAnthropic {
				g.handleNonStreamPassthrough(c, resp, &channel, channelKey, requestModel, startTime, apiKeyID, attempts)
			} else {
				g.handleNonStreamResponse(c, resp, inAdapter, outAdapter, &channel, channelKey, requestModel, startTime, apiKeyID, attempts)
			}
		}

		return
	}

	// All channels failed
	latencyMs := time.Since(startTime).Milliseconds()
	metrics.RequestsTotal.With(prometheus.Labels{"model": requestModel, "channel": "none", "status": "error"}).Inc()
	metrics.RequestDuration.With(prometheus.Labels{"model": requestModel, "channel": "none"}).Observe(float64(latencyMs) / 1000.0)

	errMsg := "all channels failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	log.Printf("[RELAY ERROR] model=%s attempts=%d error=%s", requestModel, attempts, errMsg)

	// Save error audit log with request body for debugging
	errChannel := &model.Channel{Name: "none"}
	go g.saveAuditLog(nil, errChannel, requestModel, latencyMs, 0, apiKeyID, attempts, fmt.Errorf("%s", errMsg), body)

	c.JSON(http.StatusBadGateway, gin.H{"error": errMsg})
}

// handleNonStreamResponse processes a non-streaming upstream response.
func (g *Gateway) handleNonStreamResponse(c *gin.Context, resp *http.Response, inAdapter types.Inbound, outAdapter types.Outbound, channel *model.Channel, key *model.ChannelKey, requestModel string, startTime time.Time, apiKeyID uint, attempts int) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read upstream response"})
		return
	}

	// Transform outbound response -> internal
	internalResp, err := outAdapter.TransformResponse(c.Request.Context(), resp.StatusCode, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to parse upstream response"})
		return
	}

	// Transform internal -> inbound client format
	clientBody, err := inAdapter.TransformResponse(c.Request.Context(), internalResp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to format response"})
		return
	}

	// Record metrics
	latencyMs := time.Since(startTime).Milliseconds()
	g.recordMetrics(internalResp, channel, requestModel, latencyMs, apiKeyID, attempts, key)

	c.Data(http.StatusOK, "application/json", clientBody)
}

// handleStreamResponse processes a streaming SSE upstream response.
func (g *Gateway) handleStreamResponse(c *gin.Context, resp *http.Response, inAdapter types.Inbound, outAdapter types.Outbound, channel *model.Channel, key *model.ChannelKey, requestModel string, startTime time.Time, apiKeyID uint, attempts int, firstTokenTimeout int) {
	defer resp.Body.Close()

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	metrics.ActiveConnections.Inc()
	defer metrics.ActiveConnections.Dec()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024) // 256KB max line

	firstToken := true
	var firstTokenTime time.Time

	// First token timeout
	type scanResult struct {
		line string
		err  error
		done bool
	}
	results := make(chan scanResult, 1)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			results <- scanResult{line: line}
		}
		if err := scanner.Err(); err != nil {
			results <- scanResult{err: err}
		}
		results <- scanResult{done: true}
	}()

	var firstTokenTimer *time.Timer
	var firstTokenC <-chan time.Time
	if firstTokenTimeout > 0 {
		firstTokenTimer = time.NewTimer(time.Duration(firstTokenTimeout) * time.Second)
		firstTokenC = firstTokenTimer.C
		defer func() {
			if firstTokenTimer != nil {
				firstTokenTimer.Stop()
			}
		}()
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-firstTokenC:
			log.Printf("first token timeout (%ds) for channel %s", firstTokenTimeout, channel.Name)
			return
		case r := <-results:
			if r.done || r.err != nil {
				// Stream ended
				latencyMs := time.Since(startTime).Milliseconds()
				var ftMs int64
				if !firstTokenTime.IsZero() {
					ftMs = firstTokenTime.Sub(startTime).Milliseconds()
				}
				g.recordStreamMetrics(inAdapter, channel, requestModel, latencyMs, ftMs, apiKeyID, attempts, key)
				return
			}

			line := r.line
			// SSE format: "data: {...}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			// Parse via outbound transformer
			internalResp, err := outAdapter.TransformStream(c.Request.Context(), []byte(data))
			if err != nil {
				continue
			}

			// Transform to client format
			clientData, err := inAdapter.TransformStream(c.Request.Context(), internalResp)
			if err != nil || len(clientData) == 0 {
				continue
			}

			// Track first token
			if firstToken && !internalResp.IsDone {
				firstToken = false
				firstTokenTime = time.Now()
				if firstTokenTimer != nil {
					firstTokenTimer.Stop()
					firstTokenTimer = nil
					firstTokenC = nil
				}
				metrics.FirstTokenLatency.With(prometheus.Labels{"model": requestModel, "channel": channel.Name}).Observe(time.Since(startTime).Seconds())
			}

			// Write to client
			c.Writer.Write(clientData)
			flusher.Flush()

			if internalResp.IsDone {
				latencyMs := time.Since(startTime).Milliseconds()
				var ftMs int64
				if !firstTokenTime.IsZero() {
					ftMs = firstTokenTime.Sub(startTime).Milliseconds()
				}
				g.recordStreamMetrics(inAdapter, channel, requestModel, latencyMs, ftMs, apiKeyID, attempts, key)
				return
			}
		}
	}
}

// handleStreamPassthrough relays SSE bytes directly from upstream to client
// without parsing or transforming. Used when inbound and outbound share the same protocol.
func (g *Gateway) handleStreamPassthrough(c *gin.Context, resp *http.Response, channel *model.Channel, key *model.ChannelKey, requestModel string, startTime time.Time, apiKeyID uint, attempts int, firstTokenTimeout int) {
	defer resp.Body.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	metrics.ActiveConnections.Inc()
	defer metrics.ActiveConnections.Dec()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	buf := make([]byte, 32*1024)
	firstByte := true
	var firstTokenTime time.Time

	var firstTokenTimer *time.Timer
	var firstTokenC <-chan time.Time
	if firstTokenTimeout > 0 {
		firstTokenTimer = time.NewTimer(time.Duration(firstTokenTimeout) * time.Second)
		firstTokenC = firstTokenTimer.C
		defer func() {
			if firstTokenTimer != nil {
				firstTokenTimer.Stop()
			}
		}()
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-firstTokenC:
			log.Printf("first token timeout (%ds) for channel %s", firstTokenTimeout, channel.Name)
			return
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if firstByte {
				firstByte = false
				firstTokenTime = time.Now()
				if firstTokenTimer != nil {
					firstTokenTimer.Stop()
					firstTokenTimer = nil
					firstTokenC = nil
				}
				metrics.FirstTokenLatency.With(prometheus.Labels{"model": requestModel, "channel": channel.Name}).Observe(time.Since(startTime).Seconds())
			}
			c.Writer.Write(buf[:n])
			flusher.Flush()
		}
		if err != nil {
			break
		}
	}

	latencyMs := time.Since(startTime).Milliseconds()
	var ftMs int64
	if !firstTokenTime.IsZero() {
		ftMs = firstTokenTime.Sub(startTime).Milliseconds()
	}
	metrics.RequestsTotal.With(prometheus.Labels{"model": requestModel, "channel": channel.Name, "status": "success"}).Inc()
	metrics.RequestDuration.With(prometheus.Labels{"model": requestModel, "channel": channel.Name}).Observe(float64(latencyMs) / 1000.0)
	go g.saveAuditLog(nil, channel, requestModel, latencyMs, ftMs, apiKeyID, attempts, nil, nil)
}

// handleNonStreamPassthrough relays non-streaming response body directly.
func (g *Gateway) handleNonStreamPassthrough(c *gin.Context, resp *http.Response, channel *model.Channel, key *model.ChannelKey, requestModel string, startTime time.Time, apiKeyID uint, attempts int) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read upstream response"})
		return
	}

	latencyMs := time.Since(startTime).Milliseconds()
	metrics.RequestsTotal.With(prometheus.Labels{"model": requestModel, "channel": channel.Name, "status": "success"}).Inc()
	metrics.RequestDuration.With(prometheus.Labels{"model": requestModel, "channel": channel.Name}).Observe(float64(latencyMs) / 1000.0)

	// Copy relevant headers
	for _, h := range []string{"Content-Type", "X-Request-Id"} {
		if v := resp.Header.Get(h); v != "" {
			c.Header(h, v)
		}
	}

	go g.saveAuditLog(nil, channel, requestModel, latencyMs, 0, apiKeyID, attempts, nil, nil)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func (g *Gateway) recordMetrics(resp *types.InternalResponse, channel *model.Channel, requestModel string, latencyMs int64, apiKeyID uint, attempts int, key *model.ChannelKey) {
	metrics.RequestsTotal.With(prometheus.Labels{"model": requestModel, "channel": channel.Name, "status": "success"}).Inc()
	metrics.RequestDuration.With(prometheus.Labels{"model": requestModel, "channel": channel.Name}).Observe(float64(latencyMs) / 1000.0)

	if resp != nil && resp.Usage != nil {
		metrics.TokensTotal.With(prometheus.Labels{"model": requestModel, "direction": "input"}).Add(float64(resp.Usage.PromptTokens))
		metrics.TokensTotal.With(prometheus.Labels{"model": requestModel, "direction": "output"}).Add(float64(resp.Usage.CompletionTokens))
	}

	// Save audit log asynchronously
	go g.saveAuditLog(resp, channel, requestModel, latencyMs, 0, apiKeyID, attempts, nil, nil)
}

func (g *Gateway) recordStreamMetrics(inAdapter types.Inbound, channel *model.Channel, requestModel string, latencyMs, firstTokenMs int64, apiKeyID uint, attempts int, key *model.ChannelKey) {
	metrics.RequestsTotal.With(prometheus.Labels{"model": requestModel, "channel": channel.Name, "status": "success"}).Inc()
	metrics.RequestDuration.With(prometheus.Labels{"model": requestModel, "channel": channel.Name}).Observe(float64(latencyMs) / 1000.0)

	// Get aggregated response for token counting
	resp, _ := inAdapter.GetInternalResponse(context.Background())
	if resp != nil && resp.Usage != nil {
		metrics.TokensTotal.With(prometheus.Labels{"model": requestModel, "direction": "input"}).Add(float64(resp.Usage.PromptTokens))
		metrics.TokensTotal.With(prometheus.Labels{"model": requestModel, "direction": "output"}).Add(float64(resp.Usage.CompletionTokens))
	}

	go g.saveAuditLog(resp, channel, requestModel, latencyMs, firstTokenMs, apiKeyID, attempts, nil, nil)
}

func (g *Gateway) saveAuditLog(resp *types.InternalResponse, channel *model.Channel, requestModel string, latencyMs, firstTokenMs int64, apiKeyID uint, attempts int, lastErr error, reqBody []byte) {
	audit := model.AuditLog{
		APIKeyID:     apiKeyID,
		Model:        requestModel,
		ChannelID:    channel.ID,
		ChannelName:  channel.Name,
		LatencyMs:    latencyMs,
		FirstTokenMs: firstTokenMs,
		Attempts:     attempts,
		Stream:       firstTokenMs > 0,
	}

	if resp != nil && resp.Usage != nil {
		audit.InputTokens = int64(resp.Usage.PromptTokens)
		audit.OutputTokens = int64(resp.Usage.CompletionTokens)
	}
	if lastErr != nil {
		audit.Error = lastErr.Error()
		// Store request body on error for debugging (truncate to 16KB)
		if len(reqBody) > 0 {
			if len(reqBody) > 16384 {
				audit.RequestBody = string(reqBody[:16384]) + "\n...[truncated]"
			} else {
				audit.RequestBody = string(reqBody)
			}
		}
		audit.ResponseBody = lastErr.Error()
	}

	g.db.Create(&audit)

	// Update per-key token window for TPM enforcement
	if audit.APIKeyID > 0 {
		ratelimit.Global.RecordTokens(audit.APIKeyID, audit.InputTokens+audit.OutputTokens)
	}

	// Update aggregated stats tables
	g.updateStats(&audit)

	// Publish to SSE log stream
	if g.logPublisher != nil {
		g.logPublisher(&audit)
	}
}

// updateStats updates all aggregated statistics tables based on an audit log entry.
func (g *Gateway) updateStats(audit *model.AuditLog) {
	now := time.Now()
	today := now.Format("2006-01-02")
	hour := now.Hour()

	var failed int64
	if audit.Error != "" {
		failed = 1
	}

	// Calculate cost from model prices
	var inputCost, outputCost float64
	var price model.ModelPrice
	if err := g.db.Where("model_name = ?", audit.Model).First(&price).Error; err == nil {
		inputCost = float64(audit.InputTokens) * price.InputPrice / 1_000_000
		outputCost = float64(audit.OutputTokens) * price.OutputPrice / 1_000_000
		audit.Cost = inputCost + outputCost
		g.db.Model(audit).Update("cost", audit.Cost)
	}

	// StatsDaily
	g.db.Exec(`INSERT INTO stats_dailies (date, input_tokens, output_tokens, input_cost, output_cost, total_requests, failed_requests, total_latency_ms)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(date) DO UPDATE SET
			input_tokens = input_tokens + ?,
			output_tokens = output_tokens + ?,
			input_cost = input_cost + ?,
			output_cost = output_cost + ?,
			total_requests = total_requests + 1,
			failed_requests = failed_requests + ?,
			total_latency_ms = total_latency_ms + ?`,
		today, audit.InputTokens, audit.OutputTokens, inputCost, outputCost, failed, audit.LatencyMs,
		audit.InputTokens, audit.OutputTokens, inputCost, outputCost, failed, audit.LatencyMs)

	// StatsHourly
	g.db.Exec(`INSERT INTO stats_hourlies (hour, date, input_tokens, output_tokens, input_cost, output_cost, total_requests, failed_requests, total_latency_ms)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(hour) DO UPDATE SET
			date = ?,
			input_tokens = CASE WHEN date = ? THEN input_tokens + ? ELSE ? END,
			output_tokens = CASE WHEN date = ? THEN output_tokens + ? ELSE ? END,
			input_cost = CASE WHEN date = ? THEN input_cost + ? ELSE ? END,
			output_cost = CASE WHEN date = ? THEN output_cost + ? ELSE ? END,
			total_requests = CASE WHEN date = ? THEN total_requests + 1 ELSE 1 END,
			failed_requests = CASE WHEN date = ? THEN failed_requests + ? ELSE ? END,
			total_latency_ms = CASE WHEN date = ? THEN total_latency_ms + ? ELSE ? END`,
		hour, today, audit.InputTokens, audit.OutputTokens, inputCost, outputCost, failed, audit.LatencyMs,
		today,
		today, audit.InputTokens, audit.InputTokens,
		today, audit.OutputTokens, audit.OutputTokens,
		today, inputCost, inputCost,
		today, outputCost, outputCost,
		today,
		today, failed, failed,
		today, audit.LatencyMs, audit.LatencyMs)

	// StatsModel
	g.db.Exec(`INSERT INTO stats_models (model_name, channel_id, input_tokens, output_tokens, input_cost, output_cost, total_requests, failed_requests, total_latency_ms)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(model_name, channel_id) DO UPDATE SET
			input_tokens = input_tokens + ?,
			output_tokens = output_tokens + ?,
			input_cost = input_cost + ?,
			output_cost = output_cost + ?,
			total_requests = total_requests + 1,
			failed_requests = failed_requests + ?,
			total_latency_ms = total_latency_ms + ?`,
		audit.Model, audit.ChannelID, audit.InputTokens, audit.OutputTokens, inputCost, outputCost, failed, audit.LatencyMs,
		audit.InputTokens, audit.OutputTokens, inputCost, outputCost, failed, audit.LatencyMs)

	// StatsChannel
	g.db.Exec(`INSERT INTO stats_channels (channel_id, input_tokens, output_tokens, input_cost, output_cost, total_requests, failed_requests, total_latency_ms)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(channel_id) DO UPDATE SET
			input_tokens = input_tokens + ?,
			output_tokens = output_tokens + ?,
			input_cost = input_cost + ?,
			output_cost = output_cost + ?,
			total_requests = total_requests + 1,
			failed_requests = failed_requests + ?,
			total_latency_ms = total_latency_ms + ?`,
		audit.ChannelID, audit.InputTokens, audit.OutputTokens, inputCost, outputCost, failed, audit.LatencyMs,
		audit.InputTokens, audit.OutputTokens, inputCost, outputCost, failed, audit.LatencyMs)

	// StatsAPIKey
	g.db.Exec(`INSERT INTO stats_api_keys (api_key_id, input_tokens, output_tokens, input_cost, output_cost, total_requests, failed_requests, total_latency_ms)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(api_key_id) DO UPDATE SET
			input_tokens = input_tokens + ?,
			output_tokens = output_tokens + ?,
			input_cost = input_cost + ?,
			output_cost = output_cost + ?,
			total_requests = total_requests + 1,
			failed_requests = failed_requests + ?,
			total_latency_ms = total_latency_ms + ?`,
		audit.APIKeyID, audit.InputTokens, audit.OutputTokens, inputCost, outputCost, failed, audit.LatencyMs,
		audit.InputTokens, audit.OutputTokens, inputCost, outputCost, failed, audit.LatencyMs)
}

func (g *Gateway) findGroup(modelName string) (*model.Group, error) {
	var group model.Group
	if err := g.db.Preload("Items").Where("name = ?", modelName).First(&group).Error; err == nil {
		return &group, nil
	}
	return nil, fmt.Errorf("no group found for model: %s", modelName)
}

// populateRuntimeData fills RuntimeLatencyMs and RuntimeCostTotal on each GroupItem
// by querying ChannelURL latency and StatsChannel cost from the database.
// Only called for LeastLatency and LeastCost modes to avoid unnecessary DB queries.
func (g *Gateway) populateRuntimeData(items []model.GroupItem) {
	for i := range items {
		channelID := items[i].ChannelID

		var url model.ChannelURL
		if err := g.db.Where("channel_id = ? AND latency > 0", channelID).
			Order("latency ASC").First(&url).Error; err == nil {
			items[i].RuntimeLatencyMs = int64(url.Latency)
		}

		var stats model.StatsChannel
		if err := g.db.Where("channel_id = ?", channelID).First(&stats).Error; err == nil {
			items[i].RuntimeCostTotal = stats.InputCost + stats.OutputCost
		}
	}
}

func (g *Gateway) getInbound(t InboundType) types.Inbound {
	switch t {
	case InboundOpenAIChat:
		return &inboundOpenAI.ChatInbound{}
	case InboundAnthropic:
		return &inboundAnthropic.MessagesInbound{}
	case InboundOpenAIEmbedding:
		return &inboundOpenAI.EmbedInbound{}
	default:
		return &inboundOpenAI.ChatInbound{}
	}
}

func (g *Gateway) getOutbound(channelType model.ChannelType) types.Outbound {
	switch channelType {
	case model.ChannelTypeOpenAI:
		return &outboundOpenAI.ChatOutbound{}
	case model.ChannelTypeAnthropic:
		return &outboundAnthropic.MessagesOutbound{}
	default:
		return &outboundOpenAI.ChatOutbound{}
	}
}

// getEmbeddingOutbound returns an outbound adapter for embedding requests.
// Only OpenAI-compatible channels are currently supported.
func (g *Gateway) getEmbeddingOutbound(_ model.ChannelType) types.Outbound {
	return &outboundOpenAI.EmbedOutbound{}
}

// handleEmbeddingPassthrough relays an embedding response body directly to the client,
// extracting usage for audit logging without transforming the vector data.
func (g *Gateway) handleEmbeddingPassthrough(c *gin.Context, resp *http.Response, outAdapter types.Outbound, channel *model.Channel, key *model.ChannelKey, requestModel string, startTime time.Time, apiKeyID uint, attempts int) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read upstream response"})
		return
	}

	latencyMs := time.Since(startTime).Milliseconds()
	metrics.RequestsTotal.With(prometheus.Labels{"model": requestModel, "channel": channel.Name, "status": "success"}).Inc()
	metrics.RequestDuration.With(prometheus.Labels{"model": requestModel, "channel": channel.Name}).Observe(float64(latencyMs) / 1000.0)

	// Parse usage for audit log (best-effort)
	ir, _ := outAdapter.TransformResponse(c.Request.Context(), resp.StatusCode, body)
	go g.saveAuditLog(ir, channel, requestModel, latencyMs, 0, apiKeyID, attempts, nil, nil)

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(http.StatusOK, contentType, body)
}

func (g *Gateway) getHTTPClient(proxy string) *http.Client {
	if proxy == "" {
		return g.client
	}
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		return g.client
	}
	transport := g.client.Transport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(proxyURL)
	return &http.Client{Transport: transport}
}

func selectBestURL(urls []model.ChannelURL) string {
	if len(urls) == 0 {
		return ""
	}
	best := urls[0]
	for _, u := range urls[1:] {
		if u.Latency > 0 && (best.Latency == 0 || u.Latency < best.Latency) {
			best = u
		}
	}
	return best.URL
}

func selectBestKey(keys []model.ChannelKey) *model.ChannelKey {
	now := time.Now().Unix()
	var best *model.ChannelKey
	for i := range keys {
		k := &keys[i]
		if !k.Enabled || k.Key == "" {
			continue
		}
		// Skip keys that got 429 recently (within 5 minutes)
		if k.StatusCode == 429 && k.LastUsedAt > 0 && now-k.LastUsedAt < 300 {
			continue
		}
		if best == nil || k.TotalCost < best.TotalCost {
			best = k
		}
	}
	return best
}

func moveToFront(items []model.GroupItem, channelID uint) []model.GroupItem {
	for i, item := range items {
		if item.ChannelID == channelID {
			result := make([]model.GroupItem, len(items))
			result[0] = item
			copy(result[1:], items[:i])
			copy(result[i+1:], items[i+1:])
			return result
		}
	}
	return items
}

func applyParamOverride(body []byte, override string) []byte {
	if override == "" {
		return body
	}
	var bodyMap map[string]interface{}
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		return body
	}
	var overrideMap map[string]interface{}
	if err := json.Unmarshal([]byte(override), &overrideMap); err != nil {
		return body
	}
	for k, v := range overrideMap {
		bodyMap[k] = v
	}
	result, err := json.Marshal(bodyMap)
	if err != nil {
		return body
	}
	return result
}

// replaceModelInBody replaces only the "model" field in a JSON body,
// preserving all other fields byte-for-byte via generic map round-trip.
func replaceModelInBody(body []byte, newModel string) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	modelBytes, _ := json.Marshal(newModel)
	m["model"] = modelBytes
	result, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return result
}
