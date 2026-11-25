package trace

import (
	"net/http"
	"regexp"
	"strings"
)

var secretHeaders = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"x-api-key":           {},
	"api-key":             {},
	"apikey":              {},
	"x-goog-api-key":      {},
	"cookie":              {},
	"set-cookie":          {},
	"x-auth-token":        {},
	"x-access-token":      {},
	"anthropic-api-key":   {},
	"openai-api-key":      {},
	"x-stainless-api-key": {},
}

// 顺序有意义：先打码带前缀的完整形态，再收拾 `"api_key": "…"` 这种键值对，
// 避免前者被后者切碎后留下半个令牌。
var (
	tokenRuleSK     = regexp.MustCompile(`sk-[A-Za-z0-9_\-]{6,}`)
	tokenRuleBearer = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._\-]{8,}`)
	tokenRuleValue  = regexp.MustCompile(`(?i)\b(x-api-key|api[_-]?key|secret|password|token)(\s*[:=]\s*"?)([A-Za-z0-9._\-]{8,})`)
	tokenRuleAKIA   = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
)

// maskValue 打码但保留可辨认前缀：抓包页要看的是「换没换凭证 / 用的是哪一把」，
// 全打码等于把要对照的东西一起抹掉。
func maskValue(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.SplitN(value, " ", 2)
	if len(parts) == 2 {
		switch strings.ToLower(parts[0]) {
		case "bearer", "basic", "token", "digest":
			cred := parts[1]
			if len(cred) > 10 {
				return parts[0] + " " + cred[:6] + "***"
			}
			return parts[0] + " ***"
		}
	}
	if len(value) > 10 {
		return value[:6] + "***"
	}
	return "***"
}

func RedactHeaders(headers http.Header) map[string]string {
	out := make(map[string]string, len(headers))
	for name, values := range headers {
		value := strings.Join(values, ", ")
		if _, ok := secretHeaders[strings.ToLower(name)]; ok {
			out[name] = maskValue(value)
			continue
		}
		out[name] = clipString(value, 512)
	}
	return out
}

func RedactHeaderMap(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for name, value := range headers {
		if _, ok := secretHeaders[strings.ToLower(name)]; ok {
			out[name] = maskValue(value)
			continue
		}
		out[name] = clipString(value, 512)
	}
	return out
}

// Scrub 正文脱敏。只替换值、不动结构，保持 JSON 可解析。
func Scrub(text string) string {
	if text == "" {
		return text
	}
	text = tokenRuleSK.ReplaceAllStringFunc(text, maskValue)
	text = tokenRuleBearer.ReplaceAllStringFunc(text, maskValue)
	text = tokenRuleValue.ReplaceAllStringFunc(text, func(match string) string {
		sub := tokenRuleValue.FindStringSubmatch(match)
		if len(sub) != 4 {
			return maskValue(match)
		}
		return sub[1] + sub[2] + maskValue(sub[3])
	})
	text = tokenRuleAKIA.ReplaceAllStringFunc(text, maskValue)
	return text
}

// Clip 截断正文并告知是否截断。
func Clip(text string, limit int) (string, bool) {
	if len(text) <= limit {
		return text, false
	}
	return text[:limit], true
}

func clipString(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}

// CleanExternal 外部文本入库前去掉换行与控制字符，避免详情页被伪造出额外行。
func CleanExternal(value string, limit int) string {
	value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
	var b strings.Builder
	b.Grow(len(value))
	for _, ch := range value {
		if ch >= ' ' || ch == '\t' {
			b.WriteRune(ch)
		}
	}
	return clipString(b.String(), limit)
}
