package service

import (
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type UpstreamPollutionDetection struct {
	Matched bool
	Type    string
	Rule    string
	Keyword string
	Reason  string
}

const (
	UpstreamPollutionTypeKeyword    = "keyword"
	UpstreamPollutionTypeSuspicious = "suspicious"
)

// DetectUpstreamPollution 检查响应内容是否命中污染关键词或可疑污染规则。
// 命中则返回命中的关键词/规则字符串，未命中或未启用检测则返回空字符串。
func DetectUpstreamPollution(body []byte) string {
	detection := DetectUpstreamPollutionDetail(body)
	if !detection.Matched {
		return ""
	}
	if detection.Keyword != "" {
		return detection.Keyword
	}
	return detection.Rule
}

func DetectUpstreamPollutionDetail(body []byte) UpstreamPollutionDetection {
	if len(body) == 0 {
		return UpstreamPollutionDetection{}
	}
	text := string(body)
	if hit := detectConfiguredPollutionKeyword(text); hit != "" {
		return UpstreamPollutionDetection{
			Matched: true,
			Type:    UpstreamPollutionTypeKeyword,
			Rule:    "configured_keyword",
			Keyword: hit,
			Reason:  "命中上游响应污染关键词: " + hit,
		}
	}
	if operation_setting.IsUpstreamSuspiciousPollutionDetectionEnabled() {
		return detectSuspiciousUpstreamPollution(text)
	}
	return UpstreamPollutionDetection{}
}

// DetectUpstreamPollutionString 与 DetectUpstreamPollution 等价，接受 string 入参。
func DetectUpstreamPollutionString(text string) string {
	if text == "" {
		return ""
	}
	if hit := detectConfiguredPollutionKeyword(text); hit != "" {
		return hit
	}
	if operation_setting.IsUpstreamSuspiciousPollutionDetectionEnabled() {
		detection := detectSuspiciousUpstreamPollution(text)
		if detection.Matched {
			return detection.Rule
		}
	}
	return ""
}

func detectConfiguredPollutionKeyword(text string) string {
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

func detectSuspiciousUpstreamPollution(text string) UpstreamPollutionDetection {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "chatcmpl_local_") {
		return suspiciousDetection("fake_id_chatcmpl_local", "chatcmpl_local_", "命中伪造响应 ID 特征")
	}
	if containsURLLike(text) && containsTokenKeyClaim(text) {
		return suspiciousDetection("url_token_key_claim", "", "响应同时包含链接和 token/key/公益/领取话术")
	}
	if containsPromotionTerm(text) && containsContactOrLink(text) {
		return suspiciousDetection("promotion_contact_or_link", "", "响应同时包含推广话术和联系方式/链接")
	}
	if containsHTMLProtocolAnomaly(text, lower) {
		return suspiciousDetection("html_protocol_anomaly", "", "响应包含异常 HTML 或协议内容")
	}
	return UpstreamPollutionDetection{}
}

func suspiciousDetection(rule string, keyword string, reason string) UpstreamPollutionDetection {
	return UpstreamPollutionDetection{
		Matched: true,
		Type:    UpstreamPollutionTypeSuspicious,
		Rule:    rule,
		Keyword: keyword,
		Reason:  reason,
	}
}

func containsPromotionTerm(text string) bool {
	terms := []string{"推广", "广告", "福利", "优惠", "免费", "公益", "领取", "赠送", "加群", "通知群", "交流群", "官方群"}
	return containsAny(text, terms)
}

func containsContactOrLink(text string) bool {
	if containsURLLike(text) {
		return true
	}
	terms := []string{"微信", "QQ", "qq", "Telegram", "telegram", "TG", "QQ群", "VX", "vx", "联系", "群号"}
	if containsAny(text, terms) {
		return true
	}
	return hasLongDigitSequence(text, 6)
}

func containsTokenKeyClaim(text string) bool {
	terms := []string{"token", "Token", "TOKEN", "key", "Key", "KEY", "api key", "API key", "公益", "领取"}
	return containsAny(text, terms)
}

func containsURLLike(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "www.") || strings.Contains(lower, "t.me/")
}

func containsHTMLProtocolAnomaly(text string, lower string) bool {
	if strings.Contains(lower, "<!doctype html") || strings.Contains(lower, "<html") || strings.Contains(lower, "<script") || strings.Contains(lower, "<iframe") {
		return true
	}
	if strings.Contains(lower, "data:text/html") || strings.Contains(lower, "javascript:") {
		return true
	}
	return strings.Contains(text, "data: ") && (strings.Contains(lower, "<html") || strings.Contains(lower, "</html>"))
}

func containsAny(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func hasLongDigitSequence(text string, minLength int) bool {
	count := 0
	for _, r := range text {
		if unicode.IsDigit(r) {
			count++
			if count >= minLength {
				return true
			}
			continue
		}
		count = 0
	}
	return false
}
