package service

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"

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
	if tmplText == "" {
		return "", errors.New("empty template")
	}
	funcMap := template.FuncMap{
		"json": templateJSON,
	}
	tmpl, err := template.New("upstream_pollution_response").Funcs(funcMap).Parse(tmplText)
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
