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

// PollutionRenderResult 渲染结果：rendered 为空时调用方需走 fallback
type PollutionRenderResult struct {
	Rendered string
}

// RenderUpstreamPollutionResponse 根据 isStream 选择模板并渲染。
// - 模板为空 → 返回 nil（调用方走硬编码 fallback）
// - 模板解析/渲染出错 → 写错误日志,返回 nil（调用方走硬编码 fallback）
func RenderUpstreamPollutionResponse(c *gin.Context, isStream bool, keyword string) *PollutionRenderResult {
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
