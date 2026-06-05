package model

import (
	"strings"
)

const (
	// InterceptMaxBodySize is the maximum bytes stored per body field.
	// MySQL TEXT is limited to 65,535 bytes; we cap at 50 KB to stay safe
	// across all three supported databases (SQLite, MySQL, PostgreSQL).
	InterceptMaxBodySize = 50 * 1024

	interceptTruncationSuffix = "\n...[TRUNCATED]"
)

// InterceptLog stores full audit records when an upstream response is intercepted.
// Full body fields use TEXT for cross-DB compatibility (SQLite/MySQL/PostgreSQL).
type InterceptLog struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index:idx_intercept_created_at"`
	RequestId string `json:"request_id" gorm:"size:128;index:idx_intercept_request_id"`

	// Queryable columns
	UserId              int    `json:"user_id" gorm:"index:idx_intercept_user_id"`
	TokenId             int    `json:"token_id" gorm:"index:idx_intercept_token_id"`
	ChannelId           int    `json:"channel_id" gorm:"index:idx_intercept_channel_id"`
	ChannelType         int    `json:"channel_type"`
	ModelName           string `json:"model_name" gorm:"size:256;index:idx_intercept_model_name"`
	RequestPath         string `json:"request_path" gorm:"size:512"`
	IsStream            bool   `json:"is_stream"`
	InterceptType       string `json:"intercept_type" gorm:"size:64;index:idx_intercept_type"`
	Rule                string `json:"rule" gorm:"size:128"`
	Reason              string `json:"reason" gorm:"size:256"`
	Keyword             string `json:"keyword" gorm:"size:256"`
	Severity            string `json:"severity" gorm:"size:32"`
	AutoDisabledChannel bool   `json:"auto_disabled_channel"`

	// Upstream response metadata
	UpstreamStatusCode  int    `json:"upstream_status_code"`
	UpstreamContentType string `json:"upstream_content_type" gorm:"size:256"`

	// Content hashes for dedup (JSON array stored as TEXT)
	ContentHashes string `json:"content_hashes" gorm:"type:text"`

	// Arbitrary extra metadata (JSON object stored as TEXT)
	Metadata string `json:"metadata" gorm:"type:text"`

	// Full body fields (TEXT for cross-DB compat)
	// Full client request body sent to us
	FullClientRequestBody string `json:"full_client_request_body" gorm:"type:text"`
	// Full upstream response body (the body returned by upstream when intercepted)
	FullUpstreamResponseBody string `json:"full_upstream_response_body" gorm:"type:text"`
	// Full safe response body we sent back to the client
	FullSafeResponseBody string `json:"full_safe_response_body" gorm:"type:text"`

	// Excerpt (short preview) fields
	ExcerptClientRequest    string `json:"excerpt_client_request" gorm:"type:text"`
	ExcerptUpstreamResponse string `json:"excerpt_upstream_response" gorm:"type:text"`
	ExcerptSafeResponse     string `json:"excerpt_safe_response" gorm:"type:text"`
}

func (InterceptLog) TableName() string {
	return "intercept_logs"
}

// InterceptLogSummary is the list-safe view of InterceptLog.
// It omits the three full body fields that are only available via detail endpoint.
type InterceptLogSummary struct {
	Id        int    `json:"id"`
	CreatedAt int64  `json:"created_at"`
	RequestId string `json:"request_id"`

	UserId              int    `json:"user_id"`
	TokenId             int    `json:"token_id"`
	ChannelId           int    `json:"channel_id"`
	ChannelType         int    `json:"channel_type"`
	ModelName           string `json:"model_name"`
	RequestPath         string `json:"request_path"`
	IsStream            bool   `json:"is_stream"`
	InterceptType       string `json:"intercept_type"`
	Rule                string `json:"rule"`
	Reason              string `json:"reason"`
	Keyword             string `json:"keyword"`
	Severity            string `json:"severity"`
	AutoDisabledChannel bool   `json:"auto_disabled_channel"`

	UpstreamStatusCode  int    `json:"upstream_status_code"`
	UpstreamContentType string `json:"upstream_content_type"`

	ContentHashes string `json:"content_hashes"`
	Metadata      string `json:"metadata"`

	// Excerpts only — full bodies are excluded from list responses.
	ExcerptClientRequest    string `json:"excerpt_client_request"`
	ExcerptUpstreamResponse string `json:"excerpt_upstream_response"`
	ExcerptSafeResponse     string `json:"excerpt_safe_response"`
}

// SanitizeBodyForStorage truncates a body string if it exceeds InterceptMaxBodySize.
// Returns the (possibly truncated) body and true if truncation occurred.
func SanitizeBodyForStorage(body string) (string, bool) {
	if len(body) <= InterceptMaxBodySize {
		return body, false
	}
	truncated := body[:InterceptMaxBodySize-len(interceptTruncationSuffix)] + interceptTruncationSuffix
	return truncated, true
}

// CreateInterceptLog inserts a new audit record.
// Body fields are automatically truncated to InterceptMaxBodySize before storage.
func CreateInterceptLog(log *InterceptLog) error {
	log.FullClientRequestBody, _ = SanitizeBodyForStorage(log.FullClientRequestBody)
	log.FullUpstreamResponseBody, _ = SanitizeBodyForStorage(log.FullUpstreamResponseBody)
	log.FullSafeResponseBody, _ = SanitizeBodyForStorage(log.FullSafeResponseBody)
	return LOG_DB.Create(log).Error
}

// InterceptLogQueryParams holds all optional filters for listing intercept logs.
type InterceptLogQueryParams struct {
	RequestId           string
	UserId              int
	TokenId             int
	ChannelId           int
	ChannelType         int
	ModelName           string
	RequestPath         string
	InterceptType       string
	Rule                string
	Keyword             string
	Severity            string
	AutoDisabledChannel *bool
	UpstreamStatusCode  int
	StartTimestamp      int64
	EndTimestamp        int64
}

// GetAllInterceptLogs returns paginated intercept log summaries (no full bodies) with optional filters.
func GetAllInterceptLogs(params InterceptLogQueryParams, startIdx int, pageSize int) ([]*InterceptLogSummary, int64, error) {
	tx := LOG_DB.Model(&InterceptLog{})
	var total int64

	if params.RequestId != "" {
		tx = tx.Where("request_id = ?", params.RequestId)
	}
	if params.UserId != 0 {
		tx = tx.Where("user_id = ?", params.UserId)
	}
	if params.TokenId != 0 {
		tx = tx.Where("token_id = ?", params.TokenId)
	}
	if params.ChannelId != 0 {
		tx = tx.Where("channel_id = ?", params.ChannelId)
	}
	if params.ChannelType != 0 {
		tx = tx.Where("channel_type = ?", params.ChannelType)
	}
	if params.ModelName != "" {
		if strings.Contains(params.ModelName, "%") {
			pattern, err := sanitizeLikePattern(params.ModelName)
			if err == nil {
				tx = tx.Where("model_name LIKE ? ESCAPE '!'", pattern)
			}
		} else {
			tx = tx.Where("model_name = ?", params.ModelName)
		}
	}
	if params.RequestPath != "" {
		pattern, err := sanitizeLikePattern("%" + params.RequestPath + "%")
		if err == nil {
			tx = tx.Where("request_path LIKE ? ESCAPE '!'", pattern)
		}
	}
	if params.InterceptType != "" {
		tx = tx.Where("intercept_type = ?", params.InterceptType)
	}
	if params.Rule != "" {
		pattern, err := sanitizeLikePattern("%" + params.Rule + "%")
		if err == nil {
			tx = tx.Where("rule LIKE ? ESCAPE '!'", pattern)
		}
	}
	if params.Keyword != "" {
		pattern, err := sanitizeLikePattern("%" + params.Keyword + "%")
		if err == nil {
			tx = tx.Where("keyword LIKE ? ESCAPE '!'", pattern)
		}
	}
	if params.Severity != "" {
		tx = tx.Where("severity = ?", params.Severity)
	}
	if params.AutoDisabledChannel != nil {
		tx = tx.Where("auto_disabled_channel = ?", *params.AutoDisabledChannel)
	}
	if params.UpstreamStatusCode != 0 {
		tx = tx.Where("upstream_status_code = ?", params.UpstreamStatusCode)
	}
	if params.StartTimestamp != 0 {
		tx = tx.Where("created_at >= ?", params.StartTimestamp)
	}
	if params.EndTimestamp != 0 {
		tx = tx.Where("created_at <= ?", params.EndTimestamp)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Exclude full body columns from list query for security and performance.
	var logs []*InterceptLogSummary
	err := tx.Omit(
		"full_client_request_body",
		"full_upstream_response_body",
		"full_safe_response_body",
	).Order("created_at desc, id desc").
		Limit(pageSize).Offset(startIdx).
		Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

// GetInterceptLogById returns a single intercept log by primary key.
func GetInterceptLogById(id int) (*InterceptLog, error) {
	var log InterceptLog
	err := LOG_DB.Where("id = ?", id).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// DeleteOldInterceptLogs removes intercept logs older than the given timestamp in batches.
func DeleteOldInterceptLogs(targetTimestamp int64, limit int, maxTotal int64) (int64, error) {
	var total int64
	for {
		if maxTotal > 0 && total >= maxTotal {
			break
		}
		batchLimit := limit
		if maxTotal > 0 && total+int64(batchLimit) > maxTotal {
			batchLimit = int(maxTotal - total)
		}
		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(batchLimit).Delete(&InterceptLog{})
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected
		if result.RowsAffected < int64(batchLimit) {
			break
		}
	}
	return total, nil
}
