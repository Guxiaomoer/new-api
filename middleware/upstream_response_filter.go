package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
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
		// 关键词为空 → 检测关闭，零开销直接放行
		if len(operation_setting.GetUpstreamPollutionKeywords()) == 0 {
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

	hit := service.DetectUpstreamPollution(w.buffer.Bytes())
	if hit == "" {
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

	// 命中：阻断并改写响应
	pollutedBody := append([]byte(nil), w.buffer.Bytes()...)
	w.buffer.Reset()
	w.state = pollutionStateBlocked
	w.handlePollution(hit, pollutedBody)
	return nil
}

// finalize 在 c.Next() 结束后兜底，如仍在缓冲态则强制决策。
func (w *pollutionFilterWriter) finalize() {
	if w.state == pollutionStateBuffering {
		_ = w.decide()
	}
}

// handlePollution 命中污染时的处理：日志 + 禁用渠道 + 改写错误响应。
func (w *pollutionFilterWriter) handlePollution(hit string, body []byte) {
	channelId := common.GetContextKeyInt(w.ctx, constant.ContextKeyChannelId)
	channelName := common.GetContextKeyString(w.ctx, constant.ContextKeyChannelName)
	channelType := common.GetContextKeyInt(w.ctx, constant.ContextKeyChannelType)
	channelKey := common.GetContextKeyString(w.ctx, constant.ContextKeyChannelKey)
	isMultiKey := common.GetContextKeyBool(w.ctx, constant.ContextKeyChannelIsMultiKey)
	autoBan := common.GetContextKeyBool(w.ctx, constant.ContextKeyChannelAutoBan)
	modelName := common.GetContextKeyString(w.ctx, constant.ContextKeyOriginalModel)

	bodyPreview := common.LocalLogPreview(string(body))

	logger.LogError(w.ctx, fmt.Sprintf(
		"[upstream_pollution] HIT channel=#%d(%s) model=%s keyword=%q body=%s",
		channelId, channelName, modelName, hit, bodyPreview,
	))

	// 自动禁用渠道（异步，不阻塞响应）
	if operation_setting.IsUpstreamPollutionDisableChannel() && channelId > 0 {
		chErr := types.NewChannelError(channelId, channelType, channelName, isMultiKey, channelKey, autoBan)
		reason := fmt.Sprintf("命中上游响应污染关键词: %s", hit)
		go service.DisableChannel(*chErr, reason)
	}

	w.writeReplacementResponse()
}

// writeReplacementResponse 根据原响应类型改写为统一错误。
// SSE 类型 → 写错误数据帧 + [DONE]；其他 → JSON 错误。
func (w *pollutionFilterWriter) writeReplacementResponse() {
	originalCT := strings.ToLower(w.ResponseWriter.Header().Get("Content-Type"))
	message := operation_setting.GetUpstreamErrorMessage()

	headers := w.ResponseWriter.Header()
	headers.Del("Content-Length")
	headers.Del("Content-Encoding")

	errorPayload := map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "new_api_error",
			"code":    "upstream_pollution",
		},
	}
	payloadBytes, err := json.Marshal(errorPayload)
	if err != nil {
		// 极端兜底
		payloadBytes = []byte(`{"error":{"message":"upstream response blocked","type":"new_api_error","code":"upstream_pollution"}}`)
	}

	if strings.Contains(originalCT, "text/event-stream") {
		// 保留 SSE Content-Type，状态码 200（流式协议要求）
		headers.Set("Content-Type", "text/event-stream; charset=utf-8")
		headers.Set("Cache-Control", "no-cache")
		headers.Set("Connection", "keep-alive")
		w.ResponseWriter.WriteHeader(http.StatusOK)
		fmt.Fprintf(w.ResponseWriter, "data: %s\n\ndata: [DONE]\n\n", string(payloadBytes))
		w.ResponseWriter.Flush()
		return
	}

	// 非流式：JSON + 502
	headers.Set("Content-Type", "application/json; charset=utf-8")
	w.ResponseWriter.WriteHeader(http.StatusBadGateway)
	if _, writeErr := w.ResponseWriter.Write(payloadBytes); writeErr != nil {
		logger.LogError(w.ctx, fmt.Sprintf("[upstream_pollution] write replacement failed: %s", writeErr.Error()))
	}
}
