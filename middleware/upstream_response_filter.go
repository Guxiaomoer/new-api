package middleware

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	pollutionFilterThreshold = 4 * 1024 // 缓冲首部 4KB 即触发检测
)

const (
	pollutionStateBuffering = 0
	pollutionStatePassing   = 1
	pollutionStateBlocked   = 2
)

// UpstreamResponseFilter 拦截上游污染响应。
// 缓冲首部最多 4KB 进行关键词检测，命中则改写为错误响应并自动禁用渠道。
// 未命中则正常透传，后续 chunk 直接写出不再扫描。
func UpstreamResponseFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(operation_setting.GetUpstreamPollutionKeywords()) == 0 && !operation_setting.IsUpstreamSuspiciousPollutionDetectionEnabled() {
			c.Next()
			return
		}

		original := c.Writer
		filter := &pollutionFilterWriter{
			ResponseWriter: original,
			ctx:            c,
			buffer:         &bytes.Buffer{},
			state:          pollutionStateBuffering,
		}
		c.Writer = filter

		c.Next()

		// 如果 handler 写入未触达阈值，兜底做一次最终决策
		filter.finalize()
	}
}

// pollutionFilterWriter 包装 gin.ResponseWriter，实现首段缓冲+延迟决策。
type pollutionFilterWriter struct {
	gin.ResponseWriter
	ctx        *gin.Context
	buffer     *bytes.Buffer
	state      int
	statusCode int
	headerSet  bool
}

// WriteHeader 在缓冲阶段暂存状态码，决策后再实际写入。
func (w *pollutionFilterWriter) WriteHeader(code int) {
	if w.state == pollutionStateBlocked {
		return
	}
	if w.state == pollutionStatePassing {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	w.statusCode = code
	w.headerSet = true
}

// Write 在缓冲阶段累积数据，达到阈值时触发决策。
func (w *pollutionFilterWriter) Write(data []byte) (int, error) {
	if w.state == pollutionStateBlocked {
		// 已改写错误响应，丢弃后续数据但伪装写入成功
		return len(data), nil
	}
	if w.state == pollutionStatePassing {
		return w.ResponseWriter.Write(data)
	}

	// 缓冲态：先攒数据，到阈值触发决策
	w.buffer.Write(data)
	if w.buffer.Len() >= pollutionFilterThreshold {
		return len(data), w.decide()
	}
	return len(data), nil
}

// WriteString 等价于 Write([]byte(s))。
func (w *pollutionFilterWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Flush 缓冲态吞掉 flush（避免提前推 chunk），其他状态透传。
func (w *pollutionFilterWriter) Flush() {
	if w.state == pollutionStateBuffering {
		return
	}
	w.ResponseWriter.Flush()
}

// Status 优先返回我们记录的状态码（处理头未真正写出的情况）。
func (w *pollutionFilterWriter) Status() int {
	if w.state == pollutionStateBlocked && w.statusCode != 0 {
		return w.statusCode
	}
	if w.state == pollutionStateBuffering && w.headerSet {
		return w.statusCode
	}
	return w.ResponseWriter.Status()
}

// decide 在缓冲状态下根据已缓冲数据做最终决策。
func (w *pollutionFilterWriter) decide() error {
	if w.state != pollutionStateBuffering {
		return nil
	}

	hit := service.DetectUpstreamPollutionDetail(w.buffer.Bytes())
	if !hit.Matched {
		// 干净：先冲 header，再 flush 缓冲数据
		w.state = pollutionStatePassing
		if w.headerSet {
			w.ResponseWriter.WriteHeader(w.statusCode)
		}
		if w.buffer.Len() > 0 {
			if _, err := w.ResponseWriter.Write(w.buffer.Bytes()); err != nil {
				logger.LogError(w.ctx, fmt.Sprintf("[upstream_pollution] flush buffer failed: %s", err.Error()))
				return err
			}
		}
		w.buffer.Reset()
		return nil
	}

	upstreamBody := w.buffer.String()
	w.buffer.Reset()
	w.state = pollutionStateBlocked
	w.handlePollution(hit, upstreamBody)
	return nil
}

// finalize 在 c.Next() 结束后兜底，如仍在缓冲态则强制决策。
func (w *pollutionFilterWriter) finalize() {
	if w.state == pollutionStateBuffering {
		_ = w.decide()
	}
}

// handlePollution 命中污染时的处理：日志 + 禁用渠道 + 改写错误响应。
func (w *pollutionFilterWriter) handlePollution(hit service.UpstreamPollutionDetection, upstreamBody string) {
	channelId := common.GetContextKeyInt(w.ctx, constant.ContextKeyChannelId)
	channelName := common.GetContextKeyString(w.ctx, constant.ContextKeyChannelName)
	channelType := common.GetContextKeyInt(w.ctx, constant.ContextKeyChannelType)
	channelKey := common.GetContextKeyString(w.ctx, constant.ContextKeyChannelKey)
	isMultiKey := common.GetContextKeyBool(w.ctx, constant.ContextKeyChannelIsMultiKey)
	autoBan := common.GetContextKeyBool(w.ctx, constant.ContextKeyChannelAutoBan)
	modelName := common.GetContextKeyString(w.ctx, constant.ContextKeyOriginalModel)

	logger.LogError(w.ctx, fmt.Sprintf(
		"[upstream_pollution] HIT channel=#%d(%s) model=%s type=%s rule=%s keyword=%q",
		channelId, channelName, modelName, hit.Type, hit.Rule, hit.Keyword,
	))

	if operation_setting.IsUpstreamPollutionDisableChannel() && channelId > 0 {
		chErr := types.NewChannelError(channelId, channelType, channelName, isMultiKey, channelKey, autoBan)
		go service.DisableChannel(*chErr, hit.Reason)
	}

	originalContentType := w.ResponseWriter.Header().Get("Content-Type")
	originalStatusCode := w.statusCode
	if originalStatusCode == 0 {
		originalStatusCode = w.ResponseWriter.Status()
	}
	if originalStatusCode == 0 {
		originalStatusCode = http.StatusOK
	}
	safeBody, isStream := w.writeReplacementResponse(hit)
	w.recordPollutionLog(hit, channelId, channelName, channelType, modelName, upstreamBody, safeBody, isStream, originalStatusCode, originalContentType)
}

func (w *pollutionFilterWriter) recordPollutionLog(hit service.UpstreamPollutionDetection, channelId int, channelName string, channelType int, modelName string, upstreamBody string, safeBody string, isStream bool, upstreamStatusCode int, upstreamContentType string) {
	userId := common.GetContextKeyInt(w.ctx, constant.ContextKeyUserId)
	tokenId := common.GetContextKeyInt(w.ctx, constant.ContextKeyTokenId)
	group := common.GetContextKeyString(w.ctx, constant.ContextKeyTokenGroup)
	content := hit.Reason
	metadata := map[string]any{
		"channel_name":                channelName,
		"token_group":                 group,
		"auto_disable_configured":     operation_setting.IsUpstreamPollutionDisableChannel(),
		"auto_disable_attempted":      operation_setting.IsUpstreamPollutionDisableChannel() && channelId > 0,
		"captured_upstream_max_bytes": model.InterceptMaxBodySize,
	}
	metadataBytes, _ := common.Marshal(metadata)
	contentHashes := []string{fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(upstreamBody)))}
	contentHashBytes, _ := common.Marshal(contentHashes)
	clientBody := w.getClientRequestBody()
	if operation_setting.IsUpstreamInterceptAuditEnabled() {
		log := &model.InterceptLog{
			CreatedAt:                time.Now().Unix(),
			RequestId:                w.ctx.GetString(common.RequestIdKey),
			UserId:                   userId,
			TokenId:                  tokenId,
			ChannelId:                channelId,
			ChannelType:              channelType,
			ModelName:                modelName,
			RequestPath:              requestPath(w.ctx),
			IsStream:                 isStream,
			InterceptType:            hit.Type,
			Rule:                     hit.Rule,
			Reason:                   hit.Reason,
			Keyword:                  hit.Keyword,
			Severity:                 interceptSeverity(hit),
			AutoDisabledChannel:      operation_setting.IsUpstreamPollutionDisableChannel() && channelId > 0,
			UpstreamStatusCode:       upstreamStatusCode,
			UpstreamContentType:      upstreamContentType,
			ContentHashes:            string(contentHashBytes),
			Metadata:                 string(metadataBytes),
			FullClientRequestBody:    clientBody,
			FullUpstreamResponseBody: upstreamBody,
			FullSafeResponseBody:     safeBody,
			ExcerptClientRequest:     excerpt(clientBody),
			ExcerptUpstreamResponse:  excerpt(upstreamBody),
			ExcerptSafeResponse:      excerpt(safeBody),
		}
		if err := model.CreateInterceptLog(log); err != nil {
			logger.LogError(w.ctx, fmt.Sprintf("[upstream_pollution] create intercept log failed: %s", err.Error()))
		}
	}
	other := map[string]interface{}{
		"admin_info": map[string]interface{}{
			"upstream_pollution": map[string]interface{}{
				"type":                    hit.Type,
				"rule":                    hit.Rule,
				"keyword":                 hit.Keyword,
				"channel_id":              channelId,
				"channel_name":            channelName,
				"model":                   modelName,
				"auto_disable_configured": operation_setting.IsUpstreamPollutionDisableChannel(),
			},
		},
	}
	if model.LOG_DB != nil {
		model.RecordErrorLog(w.ctx, userId, channelId, modelName, "", content, tokenId, 0, isStream, group, other)
	}
}

// writeReplacementResponse 改写命中污染的响应。
// 优先尝试用户自定义模板（service.RenderUpstreamPollutionResponse）;
// 模板为空或渲染失败则回退到硬编码的安全错误响应。
func (w *pollutionFilterWriter) writeReplacementResponse(hit service.UpstreamPollutionDetection) (string, bool) {
	originalCT := strings.ToLower(w.ResponseWriter.Header().Get("Content-Type"))
	isStream := common.GetContextKeyBool(w.ctx, constant.ContextKeyIsStream) || strings.Contains(originalCT, "text/event-stream")

	headers := w.ResponseWriter.Header()
	headers.Del("Content-Length")
	headers.Del("Content-Encoding")

	keyword := hit.Keyword
	if keyword == "" {
		keyword = hit.Rule
	}
	if rendered := service.RenderUpstreamPollutionResponse(w.ctx, isStream, keyword); rendered != nil {
		w.writeTemplatedResponse(rendered, isStream)
		return rendered.Rendered, isStream
	}

	return w.writeFallbackErrorResponse(isStream), isStream
}

// writeTemplatedResponse 写用户自定义模板渲染后的响应（HTTP 200,假装正常应答）
func (w *pollutionFilterWriter) writeTemplatedResponse(result *service.PollutionRenderResult, isStream bool) {
	headers := w.ResponseWriter.Header()
	if isStream {
		headers.Set("Content-Type", "text/event-stream; charset=utf-8")
		headers.Set("Cache-Control", "no-cache")
		headers.Set("Connection", "keep-alive")
	} else {
		headers.Set("Content-Type", "application/json; charset=utf-8")
	}
	w.ResponseWriter.WriteHeader(http.StatusOK)
	if _, err := w.ResponseWriter.Write([]byte(result.Rendered)); err != nil {
		logger.LogError(w.ctx, fmt.Sprintf("[upstream_pollution] write templated response failed: %s", err.Error()))
	}
	if isStream {
		w.ResponseWriter.Flush()
	}
}

// writeFallbackErrorResponse 保留原有硬编码 error 响应（模板为空/出错时使用）
func (w *pollutionFilterWriter) writeFallbackErrorResponse(isStream bool) string {
	message := operation_setting.GetUpstreamErrorMessage()
	headers := w.ResponseWriter.Header()

	errorPayload := map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "new_api_error",
			"code":    "upstream_pollution",
		},
	}
	payloadBytes, err := common.Marshal(errorPayload)
	if err != nil {
		payloadBytes = []byte(`{"error":{"message":"upstream response blocked","type":"new_api_error","code":"upstream_pollution"}}`)
	}

	if isStream {
		headers.Set("Content-Type", "text/event-stream; charset=utf-8")
		headers.Set("Cache-Control", "no-cache")
		headers.Set("Connection", "keep-alive")
		w.ResponseWriter.WriteHeader(http.StatusOK)
		body := fmt.Sprintf("data: %s\n\ndata: [DONE]\n\n", string(payloadBytes))
		if _, err := w.ResponseWriter.Write([]byte(body)); err != nil {
			logger.LogError(w.ctx, fmt.Sprintf("[upstream_pollution] write replacement failed: %s", err.Error()))
		}
		w.ResponseWriter.Flush()
		return body
	}

	headers.Set("Content-Type", "application/json; charset=utf-8")
	w.ResponseWriter.WriteHeader(http.StatusBadGateway)
	if _, writeErr := w.ResponseWriter.Write(payloadBytes); writeErr != nil {
		logger.LogError(w.ctx, fmt.Sprintf("[upstream_pollution] write replacement failed: %s", writeErr.Error()))
	}
	return string(payloadBytes)
}

func (w *pollutionFilterWriter) getClientRequestBody() string {
	storage, err := common.GetBodyStorage(w.ctx)
	if err != nil {
		return ""
	}
	if _, err = storage.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	body, err := storage.Bytes()
	if err != nil {
		return ""
	}
	if _, err = storage.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	w.ctx.Request.Body = io.NopCloser(storage)
	return string(body)
}

func requestPath(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return ""
	}
	return c.Request.URL.Path
}

func interceptSeverity(hit service.UpstreamPollutionDetection) string {
	if hit.Type == service.UpstreamPollutionTypeKeyword {
		return "high"
	}
	return "medium"
}

func excerpt(body string) string {
	const limit = 512
	if len(body) <= limit {
		return body
	}
	return body[:limit]
}
