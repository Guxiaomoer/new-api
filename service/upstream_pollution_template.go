package service

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// PollutionTemplateContext 暴露给用户自定义模板的变量集
type PollutionTemplateContext struct {
	Model       string
	Keyword     string
	ChannelId   int
	ChannelName string
	RequestId   string
	Created     int64  // Unix 秒,用于 chat completion 的 created 字段
	Timestamp   string // 人类可读时间字符串
}

// UpstreamFailureTemplateContext 暴露给用户自定义上游故障模板的变量集。
// 注意: 不暴露原始错误正文,避免把上游敏感错误透传给下游客户端。
type UpstreamFailureTemplateContext struct {
	Model       string
	ErrorCode   string
	StatusCode  int
	ChannelId   int
	ChannelName string
	RequestId   string
	Created     int64
	Timestamp   string
}

// GlobalMaintenanceTemplateContext 暴露给全局维护模式自定义模板的变量集。
// 注意: 维护模式在渠道选择前生效,因此不暴露渠道信息。
type GlobalMaintenanceTemplateContext struct {
	Model     string
	ModelJSON string
	RequestId string
	Created   int64
	Timestamp string
}

// PollutionRenderResult 渲染结果：rendered 为空时调用方需走 fallback
type PollutionRenderResult struct {
	Rendered string
}

// RenderUpstreamPollutionResponse 根据 isStream 选择模板并渲染。
// - 模板为空 → 返回 nil（调用方走硬编码 fallback）
// - 模板解析/渲染出错 → 写错误日志,返回 nil（调用方走硬编码 fallback）
func RenderUpstreamPollutionResponse(c *gin.Context, isStream bool, keyword string) *PollutionRenderResult {
	if message := operation_setting.GetUpstreamPollutionMessage(); message != "" {
		channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
		return RenderPlainMaintenanceResponse(c, isStream, message, channelType)
	}

	var tmplText string
	if isStream {
		tmplText = operation_setting.GetUpstreamPollutionStreamTemplate()
	} else {
		tmplText = operation_setting.GetUpstreamPollutionJSONTemplate()
	}
	if tmplText == "" {
		return nil
	}

	ctx := buildPollutionTemplateContext(c, keyword)
	rendered, err := renderPollutionTemplate(tmplText, ctx)
	if err != nil {
		common.SysError(fmt.Sprintf("[upstream_pollution] template render failed (isStream=%v): %s", isStream, err.Error()))
		return nil
	}
	if strings.TrimSpace(rendered) == "" {
		common.SysError(fmt.Sprintf("[upstream_pollution] template render empty (isStream=%v)", isStream))
		return nil
	}
	if !isStream {
		var payload any
		if err := common.Unmarshal([]byte(rendered), &payload); err != nil {
			common.SysError(fmt.Sprintf("[upstream_pollution] template rendered invalid JSON: %s", err.Error()))
			return nil
		}
	}

	return &PollutionRenderResult{
		Rendered: rendered,
	}
}

// RenderUpstreamFailureResponse 根据 isStream 选择模板并渲染。
// - 仅当错误属于上游/渠道故障且模板存在时返回渲染结果
// - 模板解析/渲染出错或非流式 JSON 无效 → 返回 nil（调用方保持原错误响应）
func RenderUpstreamFailureResponse(c *gin.Context, newAPIError *types.NewAPIError, isStream bool) *PollutionRenderResult {
	if !IsUpstreamFailureError(newAPIError) {
		return nil
	}

	if message := operation_setting.GetUpstreamFailureMessage(); message != "" {
		channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
		return RenderPlainMaintenanceResponse(c, isStream, message, channelType)
	}

	var tmplText string
	if isStream {
		tmplText = operation_setting.GetUpstreamFailureStreamTemplate()
	} else {
		tmplText = operation_setting.GetUpstreamFailureJSONTemplate()
	}
	if tmplText == "" {
		return nil
	}

	ctx := buildUpstreamFailureTemplateContext(c, newAPIError)
	rendered, err := renderTemplate("upstream_failure_response", tmplText, ctx)
	if err != nil {
		common.SysError(fmt.Sprintf("[upstream_failure] template render failed (isStream=%v): %s", isStream, err.Error()))
		return nil
	}
	if strings.TrimSpace(rendered) == "" {
		common.SysError(fmt.Sprintf("[upstream_failure] template render empty (isStream=%v)", isStream))
		return nil
	}
	if !isStream {
		var payload any
		if err := common.Unmarshal([]byte(rendered), &payload); err != nil {
			common.SysError(fmt.Sprintf("[upstream_failure] template rendered invalid JSON: %s", err.Error()))
			return nil
		}
	}

	return &PollutionRenderResult{
		Rendered: rendered,
	}
}

// RenderGlobalMaintenanceResponse 根据全局维护模式开关和 isStream 选择模板并渲染。
// - 维护开关关闭 → 返回 nil（调用方继续正常 relay）
// - 配置简易纯文本消息 → 自动包装成当前协议兼容的 JSON/SSE 响应
// - 模板为空、解析/渲染出错或非流式 JSON 无效 → 使用配置中的维护提示兜底
func RenderGlobalMaintenanceResponse(c *gin.Context, isStream bool) *PollutionRenderResult {
	if !operation_setting.IsGlobalMaintenanceEnabled() {
		return nil
	}

	if message := operation_setting.GetGlobalMaintenanceMessage(); message != "" {
		return RenderPlainMaintenanceResponse(c, isStream, message, 0)
	}

	var tmplText string
	if isStream {
		tmplText = operation_setting.GetGlobalMaintenanceStreamTemplate()
	} else {
		tmplText = operation_setting.GetGlobalMaintenanceJSONTemplate()
	}
	if tmplText == "" {
		return defaultGlobalMaintenanceResponse(isStream)
	}

	if isStream && usesRawGlobalMaintenanceStreamModel(tmplText) {
		common.SysError("[global_maintenance] stream template uses raw .Model; use {{json .Model}} or {{.ModelJSON}} for SSE JSON safety")
		return defaultGlobalMaintenanceResponse(isStream)
	}
	ctx := buildGlobalMaintenanceTemplateContext(c)
	if isStream {
		ctx.Model = sanitizeSSETemplateValue(ctx.Model)
		ctx.ModelJSON = templateJSONOrEmpty(ctx.Model)
	}
	rendered, err := renderTemplate("global_maintenance_response", tmplText, ctx)
	if err != nil {
		common.SysError(fmt.Sprintf("[global_maintenance] template render failed (isStream=%v): %s", isStream, err.Error()))
		return defaultGlobalMaintenanceResponse(isStream)
	}
	if strings.TrimSpace(rendered) == "" {
		common.SysError(fmt.Sprintf("[global_maintenance] template render empty (isStream=%v)", isStream))
		return defaultGlobalMaintenanceResponse(isStream)
	}
	if !isStream {
		var payload any
		if err := common.Unmarshal([]byte(rendered), &payload); err != nil {
			common.SysError(fmt.Sprintf("[global_maintenance] template rendered invalid JSON: %s", err.Error()))
			return defaultGlobalMaintenanceResponse(isStream)
		}
	}

	return &PollutionRenderResult{
		Rendered: rendered,
	}
}

func defaultGlobalMaintenanceResponse(isStream bool) *PollutionRenderResult {
	message := operation_setting.GetGlobalMaintenanceMessage()
	if message == "" {
		message = "休息一下，号池维护中"
	}
	return RenderPlainMaintenanceResponse(nil, isStream, message, 0)
}

// RenderPlainMaintenanceResponse 将后台配置的一句纯文本维护提示自动包装为协议兼容响应。
// message 按普通字符串处理，不执行模板，避免 JSON/SSE 注入。
func RenderPlainMaintenanceResponse(c *gin.Context, isStream bool, message string, channelType int) *PollutionRenderResult {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	if isAnthropicMaintenanceRequest(c, channelType) {
		if isStream {
			return renderAnthropicMaintenanceStream(c, message)
		}
		return renderAnthropicMaintenanceJSON(c, message)
	}
	if isGeminiMaintenanceRequest(c, channelType) {
		if isStream {
			return renderGeminiMaintenanceStream(message)
		}
		return renderGeminiMaintenanceJSON(message)
	}
	if isStream {
		return renderOpenAIMaintenanceStream(c, message)
	}
	return renderOpenAIMaintenanceJSON(c, message)
}

func isAnthropicMaintenanceRequest(c *gin.Context, channelType int) bool {
	if channelType == constant.ChannelTypeAnthropic || channelType == constant.ChannelTypeAws {
		return true
	}
	if c == nil || c.Request == nil {
		return false
	}
	return strings.HasPrefix(c.Request.URL.Path, "/v1/messages")
}

func isGeminiMaintenanceRequest(c *gin.Context, channelType int) bool {
	if channelType == constant.ChannelTypeGemini || channelType == constant.ChannelTypeVertexAi {
		return true
	}
	if c == nil || c.Request == nil {
		return false
	}
	if relayMode, ok := c.Get("relay_mode"); ok {
		if mode, ok := relayMode.(int); ok && mode == relayconstant.RelayModeGemini {
			return true
		}
	}
	return strings.HasPrefix(c.Request.URL.Path, "/v1beta/models/") || strings.HasPrefix(c.Request.URL.Path, "/v1/models/")
}

func maintenanceModel(c *gin.Context) string {
	if c == nil {
		return "maintenance"
	}
	model := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	if model == "" {
		return "maintenance"
	}
	return model
}

func maintenanceRequestID(c *gin.Context, fallbackPrefix string) string {
	if c != nil {
		if requestID := c.GetString(common.RequestIdKey); requestID != "" {
			return requestID
		}
	}
	return fallbackPrefix + "_maintenance"
}

func renderOpenAIMaintenanceJSON(c *gin.Context, message string) *PollutionRenderResult {
	now := time.Now().Unix()
	payload := map[string]any{
		"id":      maintenanceRequestID(c, "chatcmpl"),
		"object":  "chat.completion",
		"created": now,
		"model":   maintenanceModel(c),
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": message,
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		},
	}
	return marshalMaintenancePayload(payload)
}

func renderOpenAIMaintenanceStream(c *gin.Context, message string) *PollutionRenderResult {
	now := time.Now().Unix()
	chunk := map[string]any{
		"id":      maintenanceRequestID(c, "chatcmpl"),
		"object":  "chat.completion.chunk",
		"created": now,
		"model":   maintenanceModel(c),
		"choices": []map[string]any{{
			"index": 0,
			"delta": map[string]any{
				"role":    "assistant",
				"content": message,
			},
			"finish_reason": "stop",
		}},
	}
	return marshalMaintenanceSSE([]any{chunk}, true)
}

func renderAnthropicMaintenanceJSON(c *gin.Context, message string) *PollutionRenderResult {
	payload := map[string]any{
		"id":    maintenanceRequestID(c, "msg"),
		"type":  "message",
		"role":  "assistant",
		"model": maintenanceModel(c),
		"content": []map[string]any{{
			"type": "text",
			"text": message,
		}},
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  0,
			"output_tokens": 0,
		},
	}
	return marshalMaintenancePayload(payload)
}

func renderAnthropicMaintenanceStream(c *gin.Context, message string) *PollutionRenderResult {
	messageStart := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            maintenanceRequestID(c, "msg"),
			"type":          "message",
			"role":          "assistant",
			"model":         maintenanceModel(c),
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]any{"input_tokens": 0, "output_tokens": 0},
		},
	}
	events := []struct {
		name string
		data any
	}{
		{"message_start", messageStart},
		{"content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}}},
		{"content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": message}}},
		{"content_block_stop", map[string]any{"type": "content_block_stop", "index": 0}},
		{"message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn", "stop_sequence": nil}, "usage": map[string]any{"output_tokens": 0}}},
		{"message_stop", map[string]any{"type": "message_stop"}},
	}
	var builder strings.Builder
	for _, event := range events {
		data, err := common.Marshal(event.data)
		if err != nil {
			return nil
		}
		builder.WriteString("event: ")
		builder.WriteString(event.name)
		builder.WriteString("\n")
		builder.WriteString("data: ")
		builder.Write(data)
		builder.WriteString("\n\n")
	}
	return &PollutionRenderResult{Rendered: builder.String()}
}

func renderGeminiMaintenanceJSON(message string) *PollutionRenderResult {
	payload := map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"parts": []map[string]any{{"text": message}},
				"role":  "model",
			},
			"finishReason": "STOP",
			"index":        0,
		}},
		"usageMetadata": map[string]any{
			"promptTokenCount":     0,
			"candidatesTokenCount": 0,
			"totalTokenCount":      0,
		},
	}
	return marshalMaintenancePayload(payload)
}

func renderGeminiMaintenanceStream(message string) *PollutionRenderResult {
	chunk := map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"parts": []map[string]any{{"text": message}},
				"role":  "model",
			},
			"finishReason": "STOP",
			"index":        0,
		}},
	}
	return marshalMaintenanceSSE([]any{chunk}, false)
}

func marshalMaintenancePayload(payload any) *PollutionRenderResult {
	data, err := common.Marshal(payload)
	if err != nil {
		return nil
	}
	return &PollutionRenderResult{Rendered: string(data)}
}

func marshalMaintenanceSSE(frames []any, includeDone bool) *PollutionRenderResult {
	var builder strings.Builder
	for _, frame := range frames {
		data, err := common.Marshal(frame)
		if err != nil {
			return nil
		}
		builder.WriteString("data: ")
		builder.Write(data)
		builder.WriteString("\n\n")
	}
	if includeDone {
		builder.WriteString("data: [DONE]\n\n")
	}
	return &PollutionRenderResult{Rendered: builder.String()}
}

// IsUpstreamFailureError 判断错误是否属于可对下游隐藏的上游/渠道故障。
// 本地请求校验、鉴权、余额、敏感词等错误不应被自定义 200 响应掩盖。
func IsUpstreamFailureError(newAPIError *types.NewAPIError) bool {
	if newAPIError == nil {
		return false
	}
	switch newAPIError.GetErrorCode() {
	case types.ErrorCodeDoRequestFailed,
		types.ErrorCodeGetChannelFailed,
		types.ErrorCodeBadResponseStatusCode,
		types.ErrorCodeBadResponse,
		types.ErrorCodeBadResponseBody,
		types.ErrorCodeReadResponseBodyFailed,
		types.ErrorCodeChannelNoAvailableKey,
		types.ErrorCodeChannelAwsClientError,
		types.ErrorCodeChannelInvalidKey,
		types.ErrorCodeChannelResponseTimeExceeded:
		return true
	default:
		if newAPIError.GetErrorType() != types.ErrorTypeNewAPIError {
			code := newAPIError.StatusCode
			return code >= http.StatusInternalServerError && code <= 599
		}
		return false
	}
}

func sanitizeSSETemplateValue(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func usesRawGlobalMaintenanceStreamModel(tmplText string) bool {
	for {
		start := strings.Index(tmplText, "{{")
		if start == -1 {
			return false
		}
		tmplText = tmplText[start+2:]
		end := strings.Index(tmplText, "}}")
		if end == -1 {
			return false
		}
		action := tmplText[:end]
		if streamTemplateActionUsesRawModel(action) {
			return true
		}
		tmplText = tmplText[end+2:]
	}
}

func streamTemplateActionUsesRawModel(action string) bool {
	action = strings.TrimSpace(strings.Trim(action, "-"))
	for index := strings.Index(action, ".Model"); index != -1; index = strings.Index(action, ".Model") {
		after := action[index+len(".Model"):]
		if after == "" || !isTemplateIdentifierChar(after[0]) {
			return !isWholeTemplateActionModelJSONEscaped(action)
		}
		action = after
	}
	return false
}

func isWholeTemplateActionModelJSONEscaped(action string) bool {
	fields := strings.Fields(action)
	if len(fields) == 2 && fields[0] == "json" && fields[1] == ".Model" {
		return true
	}
	normalized := strings.ReplaceAll(action, " ", "")
	normalized = strings.ReplaceAll(normalized, "\t", "")
	return normalized == ".Model|json"
}

func isTemplateIdentifierChar(char byte) bool {
	return char == '_' || char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z'
}

func templateJSONOrEmpty(value any) string {
	encoded, err := templateJSON(value)
	if err != nil {
		return ""
	}
	return encoded
}

func buildUpstreamFailureTemplateContext(c *gin.Context, newAPIError *types.NewAPIError) UpstreamFailureTemplateContext {
	now := time.Now()
	statusCode := 0
	errorCode := ""
	if newAPIError != nil {
		statusCode = newAPIError.StatusCode
		errorCode = string(newAPIError.GetErrorCode())
	}
	return UpstreamFailureTemplateContext{
		Model:       common.GetContextKeyString(c, constant.ContextKeyOriginalModel),
		ErrorCode:   errorCode,
		StatusCode:  statusCode,
		ChannelId:   common.GetContextKeyInt(c, constant.ContextKeyChannelId),
		ChannelName: common.GetContextKeyString(c, constant.ContextKeyChannelName),
		RequestId:   c.GetString(common.RequestIdKey),
		Created:     now.Unix(),
		Timestamp:   now.Format(time.RFC3339),
	}
}

func buildGlobalMaintenanceTemplateContext(c *gin.Context) GlobalMaintenanceTemplateContext {
	now := time.Now()
	model := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	return GlobalMaintenanceTemplateContext{
		Model:     model,
		ModelJSON: templateJSONOrEmpty(model),
		RequestId: c.GetString(common.RequestIdKey),
		Created:   now.Unix(),
		Timestamp: now.Format(time.RFC3339),
	}
}

func buildPollutionTemplateContext(c *gin.Context, keyword string) PollutionTemplateContext {
	now := time.Now()
	return PollutionTemplateContext{
		Model:       common.GetContextKeyString(c, constant.ContextKeyOriginalModel),
		Keyword:     keyword,
		ChannelId:   common.GetContextKeyInt(c, constant.ContextKeyChannelId),
		ChannelName: common.GetContextKeyString(c, constant.ContextKeyChannelName),
		RequestId:   c.GetString(common.RequestIdKey),
		Created:     now.Unix(),
		Timestamp:   now.Format(time.RFC3339),
	}
}

func renderPollutionTemplate(tmplText string, ctx PollutionTemplateContext) (string, error) {
	return renderTemplate("upstream_pollution_response", tmplText, ctx)
}

func renderTemplate(name string, tmplText string, ctx any) (string, error) {
	if tmplText == "" {
		return "", errors.New("empty template")
	}
	funcMap := template.FuncMap{
		"json": templateJSON,
	}
	tmpl, err := template.New(name).Funcs(funcMap).Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute: %w", err)
	}
	return buf.String(), nil
}

func templateJSON(value any) (string, error) {
	data, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
