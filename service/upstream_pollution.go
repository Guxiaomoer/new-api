package service

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// DetectUpstreamPollution 检查响应内容是否命中污染关键词。
// 命中则返回命中的关键词字符串，未命中或未配置任何关键词则返回空字符串。
// 检测策略：从 operation_setting 读取换行分隔的关键词列表，任意子串包含即视为命中。
func DetectUpstreamPollution(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	keywords := operation_setting.GetUpstreamPollutionKeywords()
	if len(keywords) == 0 {
		return ""
	}
	text := string(body)
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return kw
		}
	}
	return ""
}

// DetectUpstreamPollutionString 与 DetectUpstreamPollution 等价，接受 string 入参，
// 便于在流式累积场景下直接传字符串而不重复转换。
func DetectUpstreamPollutionString(text string) string {
	if text == "" {
		return ""
	}
	keywords := operation_setting.GetUpstreamPollutionKeywords()
	if len(keywords) == 0 {
		return ""
	}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return kw
		}
	}
	return ""
}
