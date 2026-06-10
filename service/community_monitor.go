package service

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	communityMonitorDirName      = "community-monitor"
	communityMonitorConfigFile   = "config.json"
	communityMonitorStateFile    = "collector-state.json"
	communityMonitorDefaultQuery = "sk-"
	communityMonitorDefaultRegex = `sk-[A-Za-z0-9_-]{8,}`
	communityMonitorMaxBodyBytes = 2 << 20
	communityMonitorMaxScanLimit = 1000
	communityMonitorMaxPageSize  = 100
	communityMonitorMinInterval  = time.Minute
	communityMonitorDetectGap    = 10 * time.Minute
)

type CommunityMonitorConfig struct {
	SourceURL                string            `json:"source_url"`
	RoomID                   string            `json:"room_id"`
	UserID                   string            `json:"user_id"`
	RoomURL                  string            `json:"room_url"`
	StartTime                string            `json:"start_time"`
	EndTime                  string            `json:"end_time"`
	Query                    string            `json:"query"`
	ExtractRegex             string            `json:"extract_regex"`
	ScanLimit                int               `json:"scan_limit"`
	PageSize                 int               `json:"page_size"`
	CollectorIntervalMinutes int               `json:"collector_interval_minutes"`
	AccessToken              string            `json:"access_token,omitempty"`
	Headers                  map[string]string `json:"headers,omitempty"`
	DetectionBaseURL         string            `json:"detection_base_url"`
}

type CommunityMonitorPublicConfig struct {
	SourceURL                string            `json:"source_url"`
	RoomID                   string            `json:"room_id"`
	UserID                   string            `json:"user_id"`
	RoomURL                  string            `json:"room_url"`
	StartTime                string            `json:"start_time"`
	EndTime                  string            `json:"end_time"`
	Query                    string            `json:"query"`
	ExtractRegex             string            `json:"extract_regex"`
	ScanLimit                int               `json:"scan_limit"`
	PageSize                 int               `json:"page_size"`
	CollectorIntervalMinutes int               `json:"collector_interval_minutes"`
	AccessTokenConfigured    bool              `json:"access_token_configured"`
	Headers                  map[string]string `json:"headers"`
	DetectionBaseURL         string            `json:"detection_base_url"`
	ConfigPath               string            `json:"config_path"`
	StatePath                string            `json:"state_path"`
}

type CommunityMonitorProgress struct {
	Checked       int     `json:"checked"`
	Read          int     `json:"read"`
	Pages         int     `json:"pages"`
	Hits          int     `json:"hits"`
	Duplicates    int     `json:"duplicates"`
	Percent       float64 `json:"percent"`
	ScanStartedAt string  `json:"scan_started_at"`
	ScanEndedAt   string  `json:"scan_ended_at"`
}

type sharkeyChatMessage struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"createdAt"`
	Text       string `json:"text"`
	FromUserID string `json:"fromUserId"`
	FromUser   struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"fromUser"`
	ToRoomID string `json:"toRoomId"`
}

type CommunityMonitorResult struct {
	Fingerprint string `json:"fingerprint"`
	MaskedValue string `json:"masked_value"`
	Value       string `json:"value,omitempty"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	Source      string `json:"source"`
	DetectedAt  string `json:"detected_at"`
	CreatedAt   string `json:"created_at"`
}

type CommunityMonitorState struct {
	Progress         CommunityMonitorProgress `json:"progress"`
	Results          []CommunityMonitorResult `json:"results"`
	MessageCount     int                      `json:"message_count"`
	CandidateCount   int                      `json:"candidate_count"`
	DetectedCount    int                      `json:"detected_count"`
	ValidCount       int                      `json:"valid_count"`
	FailureCache     int                      `json:"failure_cache"`
	LastRunAt        string                   `json:"last_run_at"`
	NextRunAt        string                   `json:"next_run_at"`
	LastError        string                   `json:"last_error"`
	CollectorRunning bool                     `json:"collector_running"`
	LastMessageID    string                   `json:"last_message_id"`
}

type CommunityMonitorStatus struct {
	Config   CommunityMonitorPublicConfig `json:"config"`
	State    CommunityMonitorState        `json:"state"`
	Rules    []CommunityMonitorRule       `json:"rules"`
	Running  bool                         `json:"running"`
}

type CommunityMonitorRule struct {
	Name  string `json:"name"`
	Query string `json:"query"`
	Regex string `json:"regex"`
}

type communityMonitorStore struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

var communityMonitor = &communityMonitorStore{}

func GetCommunityMonitorConfig() (CommunityMonitorPublicConfig, error) {
	communityMonitor.mu.Lock()
	defer communityMonitor.mu.Unlock()
	config, err := loadCommunityMonitorConfigLocked()
	if err != nil {
		return CommunityMonitorPublicConfig{}, err
	}
	return publicCommunityMonitorConfig(config), nil
}

func SaveCommunityMonitorConfig(config CommunityMonitorConfig) (CommunityMonitorPublicConfig, error) {
	communityMonitor.mu.Lock()
	defer communityMonitor.mu.Unlock()
	current, err := loadCommunityMonitorConfigLocked()
	if err != nil {
		return CommunityMonitorPublicConfig{}, err
	}
	if config.AccessToken == "" {
		config.AccessToken = current.AccessToken
	}
	normalizeCommunityMonitorConfig(&config)
	preserveCommunityMonitorSecretHeaders(&config, current)
	if err := validateCommunityMonitorConfig(config); err != nil {
		return CommunityMonitorPublicConfig{}, err
	}
	if err := writeCommunityMonitorJSONLocked(communityMonitorConfigPath(), config); err != nil {
		return CommunityMonitorPublicConfig{}, err
	}
	return publicCommunityMonitorConfig(config), nil
}

func GetCommunityMonitorStatus() (CommunityMonitorStatus, error) {
	communityMonitor.mu.Lock()
	defer communityMonitor.mu.Unlock()
	config, err := loadCommunityMonitorConfigLocked()
	if err != nil {
		return CommunityMonitorStatus{}, err
	}
	state, err := loadCommunityMonitorStateLocked()
	if err != nil {
		return CommunityMonitorStatus{}, err
	}
	state.CollectorRunning = communityMonitor.cancel != nil
	return CommunityMonitorStatus{
		Config:  publicCommunityMonitorConfig(config),
		State:   publicCommunityMonitorState(state),
		Rules:   GetCommunityMonitorRules(),
		Running: communityMonitor.cancel != nil,
	}, nil
}

func GetCommunityMonitorResults() ([]CommunityMonitorResult, error) {
	communityMonitor.mu.Lock()
	defer communityMonitor.mu.Unlock()
	state, err := loadCommunityMonitorStateLocked()
	if err != nil {
		return nil, err
	}
	return publicCommunityMonitorResults(state.Results), nil
}

func ScanCommunityMonitor() (CommunityMonitorStatus, error) {
	communityMonitor.mu.Lock()
	defer communityMonitor.mu.Unlock()
	config, err := loadCommunityMonitorConfigLocked()
	if err != nil {
		return CommunityMonitorStatus{}, err
	}
	state, err := loadCommunityMonitorStateLocked()
	if err != nil {
		return CommunityMonitorStatus{}, err
	}
	if err := scanCommunityMonitorLocked(config, &state); err != nil {
		state.LastError = err.Error()
		_ = writeCommunityMonitorJSONLocked(communityMonitorStatePath(), state)
		return CommunityMonitorStatus{}, err
	}
	if err := writeCommunityMonitorJSONLocked(communityMonitorStatePath(), state); err != nil {
		return CommunityMonitorStatus{}, err
	}
	state.CollectorRunning = communityMonitor.cancel != nil
	return CommunityMonitorStatus{Config: publicCommunityMonitorConfig(config), State: publicCommunityMonitorState(state), Rules: GetCommunityMonitorRules(), Running: communityMonitor.cancel != nil}, nil
}

func DetectCommunityMonitorCandidates() (CommunityMonitorStatus, error) {
	communityMonitor.mu.Lock()
	defer communityMonitor.mu.Unlock()
	config, err := loadCommunityMonitorConfigLocked()
	if err != nil {
		return CommunityMonitorStatus{}, err
	}
	state, err := loadCommunityMonitorStateLocked()
	if err != nil {
		return CommunityMonitorStatus{}, err
	}
	detectCommunityMonitorLocked(config, &state)
	recalculateCommunityMonitorState(&state)
	if err := writeCommunityMonitorJSONLocked(communityMonitorStatePath(), state); err != nil {
		return CommunityMonitorStatus{}, err
	}
	state.CollectorRunning = communityMonitor.cancel != nil
	return CommunityMonitorStatus{Config: publicCommunityMonitorConfig(config), State: publicCommunityMonitorState(state), Rules: GetCommunityMonitorRules(), Running: communityMonitor.cancel != nil}, nil
}

func StartCommunityMonitorCollector() (CommunityMonitorStatus, error) {
	communityMonitor.mu.Lock()
	if communityMonitor.cancel != nil {
		communityMonitor.mu.Unlock()
		return GetCommunityMonitorStatus()
	}
	ctx, cancel := context.WithCancel(context.Background())
	communityMonitor.cancel = cancel
	communityMonitor.mu.Unlock()

	go runCommunityMonitorCollector(ctx)
	return GetCommunityMonitorStatus()
}

func StopCommunityMonitorCollector() (CommunityMonitorStatus, error) {
	communityMonitor.mu.Lock()
	if communityMonitor.cancel != nil {
		communityMonitor.cancel()
		communityMonitor.cancel = nil
	}
	communityMonitor.mu.Unlock()
	return GetCommunityMonitorStatus()
}

func GetCommunityMonitorRules() []CommunityMonitorRule {
	return []CommunityMonitorRule{
		{Name: "OpenAI Key", Query: "sk-", Regex: `sk-[A-Za-z0-9_-]{8,}`},
		{Name: "GitHub Token", Query: "gh", Regex: `gh[pousr]_[A-Za-z0-9_]{20,}`},
		{Name: "JWT", Query: "eyJ", Regex: `eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`},
		{Name: "Generic Secret", Query: "key", Regex: `[A-Za-z0-9_-]{24,}`},
	}
}

func runCommunityMonitorCollector(ctx context.Context) {
	for {
		config, err := loadCommunityMonitorConfig()
		if err == nil {
			_, _ = ScanCommunityMonitor()
		}
		interval := time.Duration(config.CollectorIntervalMinutes) * time.Minute
		if interval < communityMonitorMinInterval {
			interval = 10 * time.Minute
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func loadCommunityMonitorConfig() (CommunityMonitorConfig, error) {
	communityMonitor.mu.Lock()
	defer communityMonitor.mu.Unlock()
	return loadCommunityMonitorConfigLocked()
}

func loadCommunityMonitorConfigLocked() (CommunityMonitorConfig, error) {
	config := defaultCommunityMonitorConfig()
	data, err := os.ReadFile(communityMonitorConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, err
	}
	if err := common.Unmarshal(data, &config); err != nil {
		return config, err
	}
	normalizeCommunityMonitorConfig(&config)
	return config, nil
}

func loadCommunityMonitorStateLocked() (CommunityMonitorState, error) {
	state := CommunityMonitorState{}
	data, err := os.ReadFile(communityMonitorStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}
	if err := common.Unmarshal(data, &state); err != nil {
		return CommunityMonitorState{}, err
	}
	recalculateCommunityMonitorState(&state)
	return state, nil
}

func scanCommunityMonitorLocked(config CommunityMonitorConfig, state *CommunityMonitorState) error {
	if err := validateCommunityMonitorConfig(config); err != nil {
		return err
	}
	// Check if we should use Sharkey chat API
	if config.RoomID != "" && config.SourceURL != "" && config.AccessToken != "" {
		return scanSharkeyChatRoom(config, state)
	}
	// Fallback to HTML scraping
	scanURL := config.RoomURL
	if scanURL == "" {
		scanURL = config.SourceURL
	}
	body, err := fetchCommunityMonitorBody(scanURL, config)
	if err != nil {
		return err
	}
	pattern, err := regexp.Compile(config.ExtractRegex)
	if err != nil {
		return err
	}
	now := time.Now()
	matches := pattern.FindAllString(string(body), config.ScanLimit)
	existing := map[string]struct{}{}
	for _, result := range state.Results {
		existing[result.Fingerprint] = struct{}{}
	}
	progress := CommunityMonitorProgress{Read: len(body), Pages: 1, ScanStartedAt: now.Format(time.RFC3339)}
	for _, match := range matches {
		progress.Checked++
		if config.Query != "" && !strings.Contains(match, config.Query) {
			continue
		}
		progress.Hits++
		fingerprint := fingerprintSecret(match)
		if _, ok := existing[fingerprint]; ok {
			progress.Duplicates++
			continue
		}
		existing[fingerprint] = struct{}{}
		state.Results = append(state.Results, CommunityMonitorResult{
			Fingerprint: fingerprint,
			MaskedValue: maskSecret(match),
			Value:       match,
			Kind:        classifyCommunitySecret(match),
			Status:      "candidate",
			Reason:      "matched regex",
			Source:      scanURL,
			CreatedAt:   now.Format(time.RFC3339),
		})
	}
	progress.ScanEndedAt = time.Now().Format(time.RFC3339)
	progress.Percent = 100
	state.Progress = progress
	state.LastRunAt = progress.ScanEndedAt
	state.LastError = ""
	state.NextRunAt = now.Add(time.Duration(config.CollectorIntervalMinutes) * time.Minute).Format(time.RFC3339)
	recalculateCommunityMonitorState(state)
	return nil
}

func scanSharkeyChatRoom(config CommunityMonitorConfig, state *CommunityMonitorState) error {
	sourceURL := strings.TrimRight(config.SourceURL, "/")
	apiURL := sourceURL + "/api/chat/messages/room-timeline"
	if err := validateOutboundURL(apiURL); err != nil {
		return err
	}
	pattern, err := regexp.Compile(config.ExtractRegex)
	if err != nil {
		return err
	}
	now := time.Now()
	existing := map[string]struct{}{}
	for _, result := range state.Results {
		existing[result.Fingerprint] = struct{}{}
	}
	progress := CommunityMonitorProgress{ScanStartedAt: now.Format(time.RFC3339)}
	totalMessages := 0
	totalHits := 0
	totalDuplicates := 0
	// Fetch messages with pagination using sinceId for incremental updates
	limit := config.PageSize
	if limit <= 0 {
		limit = 50
	}
	if limit > communityMonitorMaxPageSize {
		limit = communityMonitorMaxPageSize
	}
	sinceID := state.LastMessageID
	for {
		messages, err := fetchSharkeyChatMessages(apiURL, config.AccessToken, config.RoomID, limit, sinceID)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			break
		}
		for _, msg := range messages {
			totalMessages++
			text := msg.Text
			if text == "" {
				continue
			}
			matches := pattern.FindAllString(text, -1)
			for _, match := range matches {
				totalHits++
				if config.Query != "" && !strings.Contains(match, config.Query) {
					continue
				}
				fingerprint := fingerprintSecret(match)
				if _, ok := existing[fingerprint]; ok {
					totalDuplicates++
					continue
				}
				existing[fingerprint] = struct{}{}
				state.Results = append(state.Results, CommunityMonitorResult{
					Fingerprint: fingerprint,
					MaskedValue: maskSecret(match),
					Value:       match,
					Kind:        classifyCommunitySecret(match),
					Status:      "candidate",
					Reason:      "matched from chat message",
					Source:      fmt.Sprintf("chat:%s@%s", msg.FromUser.Username, msg.ToRoomID),
					CreatedAt:   msg.CreatedAt,
				})
			}
		}
		// Update lastMessageID to the newest message
		if messages[0].ID != "" {
			sinceID = messages[0].ID
		}
		if len(messages) < limit {
			break
		}
		if totalMessages >= config.ScanLimit {
			break
		}
	}
	state.LastMessageID = sinceID
	progress.Read = totalMessages
	progress.Pages = 1
	progress.Checked = totalMessages
	progress.Hits = totalHits
	progress.Duplicates = totalDuplicates
	progress.ScanEndedAt = time.Now().Format(time.RFC3339)
	progress.Percent = 100
	state.Progress = progress
	state.MessageCount = totalMessages
	state.LastRunAt = progress.ScanEndedAt
	state.LastError = ""
	state.NextRunAt = now.Add(time.Duration(config.CollectorIntervalMinutes) * time.Minute).Format(time.RFC3339)
	recalculateCommunityMonitorState(state)
	return nil
}

func fetchSharkeyChatMessages(apiURL, accessToken, roomID string, limit int, sinceID string) ([]sharkeyChatMessage, error) {
	return fetchSharkeyChatMessagesImpl(apiURL, accessToken, roomID, limit, sinceID)
}

var fetchSharkeyChatMessagesImpl = func(apiURL, accessToken, roomID string, limit int, sinceID string) ([]sharkeyChatMessage, error) {
	client := newCommunityMonitorHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	reqBody := map[string]interface{}{
		"roomId": roomID,
		"limit":  limit,
	}
	if sinceID != "" {
		reqBody["sinceId"] = sinceID
	}
	bodyBytes, err := common.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Sharkey API returned status %d", res.StatusCode)
	}
	var messages []sharkeyChatMessage
	if err := common.Unmarshal(io.LimitReader(res.Body, communityMonitorMaxBodyBytes), &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func detectCommunityMonitorLocked(config CommunityMonitorConfig, state *CommunityMonitorState) {
	for i := range state.Results {
		result := &state.Results[i]
		if result.Value == "" || result.Status == "reachable" {
			continue
		}
		if result.DetectedAt != "" {
			if detectedAt, err := time.Parse(time.RFC3339, result.DetectedAt); err == nil && time.Since(detectedAt) < communityMonitorDetectGap {
				continue
			}
		}
		status, reason := detectCommunitySecret(config, result.Value)
		result.Status = status
		result.Reason = reason
		result.DetectedAt = time.Now().Format(time.RFC3339)
	}
}

func detectCommunitySecret(config CommunityMonitorConfig, secret string) (string, string) {
	if !strings.HasPrefix(secret, "sk-") {
		return "not_openai_like", "only OpenAI-like keys support metadata detection"
	}
	baseURL := strings.TrimRight(config.DetectionBaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	endpoint := baseURL + "/v1/models"
	if err := validateOutboundURL(endpoint); err != nil {
		return "server_or_proxy_error", "invalid detection endpoint"
	}
	client := newCommunityMonitorHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "server_or_proxy_error", "failed to build detection request"
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	res, err := client.Do(req)
	if err != nil {
		return "server_or_proxy_error", "request failed"
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	switch {
	case res.StatusCode >= 200 && res.StatusCode < 300:
		return "reachable", "metadata endpoint accepted the key"
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return "auth_failed", fmt.Sprintf("metadata endpoint returned %d", res.StatusCode)
	case res.StatusCode == http.StatusTooManyRequests:
		return "rate_limited", "metadata endpoint returned 429"
	case res.StatusCode == http.StatusNotFound:
		return "not_openai_like", "metadata endpoint returned 404"
	case res.StatusCode >= 500:
		return "server_or_proxy_error", fmt.Sprintf("metadata endpoint returned %d", res.StatusCode)
	default:
		return "not_openai_like", fmt.Sprintf("metadata endpoint returned %d", res.StatusCode)
	}
}

var fetchCommunityMonitorBody = func(rawURL string, config CommunityMonitorConfig) ([]byte, error) {
	if err := validateOutboundURL(rawURL); err != nil {
		return nil, err
	}
	client := newCommunityMonitorHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if config.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	}
	for key, value := range config.Headers {
		if isUnsafeHeaderName(key) {
			continue
		}
		req.Header.Set(key, value)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("source returned status %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, communityMonitorMaxBodyBytes))
}

func validateCommunityMonitorConfig(config CommunityMonitorConfig) error {
	if config.RoomURL == "" && config.SourceURL == "" {
		return errors.New("room_url or source_url is required")
	}
	if config.ExtractRegex == "" {
		return errors.New("extract_regex is required")
	}
	if len(config.ExtractRegex) > 512 {
		return errors.New("extract_regex is too long")
	}
	if _, err := regexp.Compile(config.ExtractRegex); err != nil {
		return err
	}
	if config.RoomURL != "" {
		if err := validateOutboundURL(config.RoomURL); err != nil {
			return err
		}
	}
	if config.SourceURL != "" {
		if err := validateOutboundURL(config.SourceURL); err != nil {
			return err
		}
	}
	if config.DetectionBaseURL != "" {
		if err := validateOutboundURL(config.DetectionBaseURL); err != nil {
			return err
		}
	}
	return nil
}

func validateOutboundURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("only http and https URLs are allowed")
	}
	if parsed.Hostname() == "" {
		return errors.New("url host is required")
	}
	ips, err := net.LookupIP(parsed.Hostname())
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return errors.New("url host has no address")
	}
	for _, ip := range ips {
		if isBlockedOutboundIP(ip) {
			return errors.New("private or local addresses are not allowed")
		}
	}
	return nil
}

func newCommunityMonitorHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.LookupIP(host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if isBlockedOutboundIP(ip) {
					continue
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			}
			return nil, errors.New("private or local addresses are not allowed")
		},
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{Timeout: 12 * time.Second, Transport: transport}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return http.ErrUseLastResponse
		}
		if err := validateOutboundURL(req.URL.String()); err != nil {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return client
}

func isBlockedOutboundIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsMulticast()
}

func normalizeCommunityMonitorConfig(config *CommunityMonitorConfig) {
	config.SourceURL = strings.TrimSpace(config.SourceURL)
	config.RoomID = strings.TrimSpace(config.RoomID)
	config.UserID = strings.TrimSpace(config.UserID)
	config.RoomURL = strings.TrimSpace(config.RoomURL)
	config.Query = strings.TrimSpace(config.Query)
	config.ExtractRegex = strings.TrimSpace(config.ExtractRegex)
	config.DetectionBaseURL = strings.TrimRight(strings.TrimSpace(config.DetectionBaseURL), "/")
	if config.Query == "" {
		config.Query = communityMonitorDefaultQuery
	}
	if config.ExtractRegex == "" {
		config.ExtractRegex = communityMonitorDefaultRegex
	}
	if config.ScanLimit <= 0 || config.ScanLimit > communityMonitorMaxScanLimit {
		config.ScanLimit = 100
	}
	if config.PageSize <= 0 || config.PageSize > communityMonitorMaxPageSize {
		config.PageSize = 30
	}
	if config.CollectorIntervalMinutes <= 0 {
		config.CollectorIntervalMinutes = 10
	}
	if config.Headers == nil {
		config.Headers = map[string]string{}
	}
}

func preserveCommunityMonitorSecretHeaders(config *CommunityMonitorConfig, current CommunityMonitorConfig) {
	for key, value := range config.Headers {
		if isSecretHeaderName(key) && strings.Contains(value, "***") {
			if currentValue, ok := current.Headers[key]; ok {
				config.Headers[key] = currentValue
			}
		}
	}
}

func defaultCommunityMonitorConfig() CommunityMonitorConfig {
	config := CommunityMonitorConfig{
		Query:                    communityMonitorDefaultQuery,
		ExtractRegex:             communityMonitorDefaultRegex,
		ScanLimit:                100,
		PageSize:                 30,
		CollectorIntervalMinutes: 10,
		Headers:                  map[string]string{},
	}
	return config
}

func communityMonitorDataDir() string {
	return filepath.Join("data", communityMonitorDirName)
}

func communityMonitorConfigPath() string {
	return filepath.Join(communityMonitorDataDir(), communityMonitorConfigFile)
}

func communityMonitorStatePath() string {
	return filepath.Join(communityMonitorDataDir(), communityMonitorStateFile)
}

func writeCommunityMonitorJSONLocked(path string, value any) error {
	data, err := common.Marshal(value)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func publicCommunityMonitorConfig(config CommunityMonitorConfig) CommunityMonitorPublicConfig {
	headers := make(map[string]string, len(config.Headers))
	for key, value := range config.Headers {
		if isSecretHeaderName(key) {
			headers[key] = maskSecret(value)
		} else {
			headers[key] = value
		}
	}
	return CommunityMonitorPublicConfig{
		SourceURL:                config.SourceURL,
		RoomID:                   config.RoomID,
		UserID:                   config.UserID,
		RoomURL:                  config.RoomURL,
		StartTime:                config.StartTime,
		EndTime:                  config.EndTime,
		Query:                    config.Query,
		ExtractRegex:             config.ExtractRegex,
		ScanLimit:                config.ScanLimit,
		PageSize:                 config.PageSize,
		CollectorIntervalMinutes: config.CollectorIntervalMinutes,
		AccessTokenConfigured:    config.AccessToken != "",
		Headers:                  headers,
		DetectionBaseURL:         config.DetectionBaseURL,
		ConfigPath:               communityMonitorConfigPath(),
		StatePath:                communityMonitorStatePath(),
	}
}

func publicCommunityMonitorState(state CommunityMonitorState) CommunityMonitorState {
	state.Results = publicCommunityMonitorResults(state.Results)
	return state
}

func publicCommunityMonitorResults(results []CommunityMonitorResult) []CommunityMonitorResult {
	publicResults := make([]CommunityMonitorResult, 0, len(results))
	for _, result := range results {
		if result.MaskedValue == "" {
			result.MaskedValue = maskSecret(result.Value)
		}
		result.Value = ""
		publicResults = append(publicResults, result)
	}
	sort.SliceStable(publicResults, func(i, j int) bool {
		return publicResults[i].CreatedAt > publicResults[j].CreatedAt
	})
	return publicResults
}

func recalculateCommunityMonitorState(state *CommunityMonitorState) {
	state.CandidateCount = len(state.Results)
	state.DetectedCount = 0
	state.ValidCount = 0
	for _, result := range state.Results {
		if result.DetectedAt != "" {
			state.DetectedCount++
		}
		if result.Status == "reachable" {
			state.ValidCount++
		}
	}
	state.MessageCount = state.Progress.Checked
}

func fingerprintSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])[:16]
}

func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 12 {
		return secret[:2] + "***"
	}
	return secret[:6] + "***" + secret[len(secret)-4:]
}

func classifyCommunitySecret(secret string) string {
	switch {
	case strings.HasPrefix(secret, "sk-"):
		return "openai_key"
	case strings.HasPrefix(secret, "ghp_") || strings.HasPrefix(secret, "gho_") || strings.HasPrefix(secret, "ghu_") || strings.HasPrefix(secret, "ghs_") || strings.HasPrefix(secret, "ghr_"):
		return "github_token"
	case strings.Count(secret, ".") == 2 && strings.HasPrefix(secret, "eyJ"):
		return "jwt"
	default:
		return "generic_secret"
	}
}

func isSecretHeaderName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "authorization") || strings.Contains(lower, "cookie") || strings.Contains(lower, "token") || strings.Contains(lower, "key") || strings.Contains(lower, "secret")
}

func isUnsafeHeaderName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return lower == "host" || lower == "content-length" || lower == "connection" || lower == "transfer-encoding" || lower == "upgrade"
}
