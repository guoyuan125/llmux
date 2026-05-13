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
	db       *gorm.DB
	circuit  *circuit.Manager
	sessions *session.Store
	client   *http.Client
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

// InboundType identifies the inbound protocol.
type InboundType int

const (
	InboundOpenAIChat InboundType = iota
	InboundOpenAIResponses
	InboundAnthropic
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

	// Find group
	group, err := g.findGroup(requestModel)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("model not found: %s", requestModel)})
		return
	}

	// Get candidates via balancer
	b := balancer.Get(group.Mode)
	candidates := b.Candidates(group.Items)
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
		internalReq.Model = item.ModelName

		// Get outbound adapter
		outAdapter := g.getOutbound(channel.Type)

		// Build outbound request
		outReq, err := outAdapter.TransformRequest(c.Request.Context(), internalReq, baseURL, channelKey.Key)
		if err != nil {
			lastErr = err
			continue
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
		if isStream {
			g.handleStreamResponse(c, resp, inAdapter, outAdapter, &channel, channelKey, requestModel, startTime, apiKeyID, attempts, group.FirstTokenTimeout)
		} else {
			g.handleNonStreamResponse(c, resp, inAdapter, outAdapter, &channel, channelKey, requestModel, startTime, apiKeyID, attempts)
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

func (g *Gateway) recordMetrics(resp *types.InternalResponse, channel *model.Channel, requestModel string, latencyMs int64, apiKeyID uint, attempts int, key *model.ChannelKey) {
	metrics.RequestsTotal.With(prometheus.Labels{"model": requestModel, "channel": channel.Name, "status": "success"}).Inc()
	metrics.RequestDuration.With(prometheus.Labels{"model": requestModel, "channel": channel.Name}).Observe(float64(latencyMs) / 1000.0)

	if resp != nil && resp.Usage != nil {
		metrics.TokensTotal.With(prometheus.Labels{"model": requestModel, "direction": "input"}).Add(float64(resp.Usage.PromptTokens))
		metrics.TokensTotal.With(prometheus.Labels{"model": requestModel, "direction": "output"}).Add(float64(resp.Usage.CompletionTokens))
	}

	// Save audit log asynchronously
	go g.saveAuditLog(resp, channel, requestModel, latencyMs, 0, apiKeyID, attempts, nil)
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

	go g.saveAuditLog(resp, channel, requestModel, latencyMs, firstTokenMs, apiKeyID, attempts, nil)
}

func (g *Gateway) saveAuditLog(resp *types.InternalResponse, channel *model.Channel, requestModel string, latencyMs, firstTokenMs int64, apiKeyID uint, attempts int, lastErr error) {
	audit := model.AuditLog{
		APIKeyID:    apiKeyID,
		Model:       requestModel,
		ChannelID:   channel.ID,
		ChannelName: channel.Name,
		LatencyMs:   latencyMs,
		FirstTokenMs: firstTokenMs,
		Attempts:    attempts,
		Stream:      firstTokenMs > 0,
	}

	if resp != nil && resp.Usage != nil {
		audit.InputTokens = int64(resp.Usage.PromptTokens)
		audit.OutputTokens = int64(resp.Usage.CompletionTokens)
	}
	if lastErr != nil {
		audit.Error = lastErr.Error()
	}

	g.db.Create(&audit)
}

func (g *Gateway) findGroup(modelName string) (*model.Group, error) {
	var group model.Group
	if err := g.db.Preload("Items").Where("name = ?", modelName).First(&group).Error; err == nil {
		return &group, nil
	}

	// Regex match fallback
	var groups []model.Group
	g.db.Preload("Items").Where("match_regex != '' AND match_regex IS NOT NULL").Find(&groups)
	for _, grp := range groups {
		if grp.MatchRegex != "" && matchModel(grp.MatchRegex, modelName) {
			return &grp, nil
		}
	}

	return nil, fmt.Errorf("no group found")
}

func matchModel(pattern, modelName string) bool {
	// Simple glob-style matching: * matches anything
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(modelName, prefix)
	}
	return pattern == modelName
}

func (g *Gateway) getInbound(t InboundType) types.Inbound {
	switch t {
	case InboundOpenAIChat:
		return &inboundOpenAI.ChatInbound{}
	case InboundAnthropic:
		return &inboundAnthropic.MessagesInbound{}
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
