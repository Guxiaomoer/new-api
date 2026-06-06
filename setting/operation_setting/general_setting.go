package operation_setting

import (
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/setting/config"
)

// 额度展示类型
const (
	QuotaDisplayTypeUSD    = "USD"
	QuotaDisplayTypeCNY    = "CNY"
	QuotaDisplayTypeTokens = "TOKENS"
	QuotaDisplayTypeCustom = "CUSTOM"
)

type GeneralSetting struct {
	DocsLink            string `json:"docs_link"`
	PingIntervalEnabled bool   `json:"ping_interval_enabled"`
	PingIntervalSeconds int    `json:"ping_interval_seconds"`
	// 当前站点额度展示类型：USD / CNY / TOKENS
	QuotaDisplayType string `json:"quota_display_type"`
	// 自定义货币符号，用于 CUSTOM 展示类型
	CustomCurrencySymbol string `json:"custom_currency_symbol"`
	// 自定义货币与美元汇率（1 USD = X Custom）
	CustomCurrencyExchangeRate              float64 `json:"custom_currency_exchange_rate"`
	UpstreamRateLimitCooldownMessage        string  `json:"upstream_rate_limit_cooldown_message"`
	UpstreamErrorMessage                    string  `json:"upstream_error_message"`
	// 命中上游污染或上游故障并使用自定义消息体时，是否强制返回 HTTP 200（兼容旧行为）。关闭后返回真实错误状态。
	UpstreamCustomResponseHTTP200Enabled bool `json:"upstream_custom_response_http_200_enabled"`
	// 上游响应污染检测关键词，换行分隔，任意一条匹配即视为命中
	UpstreamPollutionKeywords string `json:"upstream_pollution_keywords"`
	// 是否启用保守的上游可疑污染组合规则检测
	UpstreamSuspiciousPollutionDetectionEnabled bool `json:"upstream_suspicious_pollution_detection_enabled"`
	// 命中污染后是否自动禁用渠道
	UpstreamPollutionDisableChannel bool `json:"upstream_pollution_disable_channel"`
	// 是否保存上游拦截审计明细
	UpstreamInterceptAuditEnabled bool `json:"upstream_intercept_audit_enabled"`
	// 上游拦截审计日志保留天数
	UpstreamInterceptAuditRetentionDays int `json:"upstream_intercept_audit_retention_days"`
	// 命中污染后返回给下游的纯文本自定义内容；后端自动包装为对应协议响应。
	UpstreamPollutionMessage string `json:"upstream_pollution_message"`
	// 命中污染后,非流式响应返回给下游的自定义模板（text/template 语法）。空 = 退回纯文本/硬编码 error 响应。
	// 可用变量: {{.Model}} {{.Keyword}} {{.ChannelId}} {{.ChannelName}} {{.RequestId}} {{.Created}} {{.Timestamp}}
	UpstreamPollutionJSONTemplate string `json:"upstream_pollution_json_template"`
	// 命中污染后,流式响应返回给下游的自定义模板（text/template 语法）。空 = 退回纯文本/硬编码 SSE error 帧。
	// 模板需自行包含完整 SSE 帧格式（含 "data: " 前缀、"\n\n" 分隔符、终止 "[DONE]"）。可用变量同上。
	UpstreamPollutionStreamTemplate string `json:"upstream_pollution_stream_template"`
	// 上游或渠道故障后返回给下游的纯文本自定义内容；后端自动包装为对应协议响应。
	UpstreamFailureMessage string `json:"upstream_failure_message"`
	// 上游或渠道故障后,非流式响应返回给下游的自定义模板（text/template 语法）。空 = 退回纯文本/保持原错误响应。
	// 可用变量: {{.Model}} {{.ErrorCode}} {{.StatusCode}} {{.ChannelId}} {{.ChannelName}} {{.RequestId}} {{.Created}} {{.Timestamp}}
	UpstreamFailureJSONTemplate string `json:"upstream_failure_json_template"`
	// 上游或渠道故障后,流式响应返回给下游的自定义模板（text/template 语法）。空 = 保持原错误响应。
	// 模板需自行包含完整 SSE 帧格式。可用变量同上。
	UpstreamFailureStreamTemplate string `json:"upstream_failure_stream_template"`
	// 全局维护模式开关。开启后 relay 请求不访问上游,直接返回自定义 HTTP 200 响应。
	GlobalMaintenanceEnabled bool `json:"global_maintenance_enabled"`
	// 全局维护模式下返回给下游的简易纯文本消息。优先级高于高级 JSON/SSE 模板,后端会自动包装为对应协议响应。
	// 同时作为渠道维护消息为空、模板为空或模板渲染失败时的默认维护提示。
	GlobalMaintenanceMessage string `json:"global_maintenance_message"`
	// 全局维护模式下,非流式请求返回给下游的自定义模板（text/template 语法）。空 = 使用内置安全响应。
	// 可用变量: {{.Model}} {{.RequestId}} {{.Created}} {{.Timestamp}}
	GlobalMaintenanceJSONTemplate string `json:"global_maintenance_json_template"`
	// 全局维护模式下,流式请求返回给下游的自定义模板（text/template 语法）。空 = 使用内置安全响应。
	// 模板需自行包含完整 SSE 帧格式。可用变量同上。
	GlobalMaintenanceStreamTemplate string `json:"global_maintenance_stream_template"`
}

// 默认配置
var generalSetting = GeneralSetting{
	DocsLink:                         "https://docs.newapi.pro",
	PingIntervalEnabled:              false,
	PingIntervalSeconds:              60,
	QuotaDisplayType:                 QuotaDisplayTypeUSD,
	CustomCurrencySymbol:             "¤",
	CustomCurrencyExchangeRate:       1.0,
	UpstreamRateLimitCooldownMessage: "上游服务触发冷却限制，请稍后重试",
	UpstreamErrorMessage:                    "上游服务返回错误，请稍后重试",
	UpstreamCustomResponseHTTP200Enabled:    true,
	UpstreamPollutionKeywords: `通▸知◁群
公益 token
chatcmpl_local_`,
	UpstreamPollutionDisableChannel:             true,
	UpstreamSuspiciousPollutionDetectionEnabled: false,
	UpstreamInterceptAuditEnabled:               false,
	UpstreamInterceptAuditRetentionDays:         30,
	UpstreamPollutionMessage:                    "上游响应命中安全过滤，请稍后重试",
	UpstreamPollutionJSONTemplate:               "",
	UpstreamPollutionStreamTemplate:             "",
	UpstreamFailureMessage:                      "上游服务暂时不可用，请稍后重试",
	UpstreamFailureJSONTemplate:                 "",
	UpstreamFailureStreamTemplate:               "",
	GlobalMaintenanceEnabled:                    false,
	GlobalMaintenanceMessage:                    "休息一下，号池维护中",
	GlobalMaintenanceJSONTemplate:               "",
	GlobalMaintenanceStreamTemplate:             "",
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("general_setting", &generalSetting)
}

func GetGeneralSetting() *GeneralSetting {
	return &generalSetting
}

func GetUpstreamRateLimitCooldownMessage() string {
	return sanitizeUpstreamErrorMessage(
		generalSetting.UpstreamRateLimitCooldownMessage,
		"上游服务触发冷却限制，请稍后重试",
	)
}

func GetUpstreamErrorMessage() string {
	return sanitizeUpstreamErrorMessage(
		generalSetting.UpstreamErrorMessage,
		"上游服务返回错误，请稍后重试",
	)
}

func IsUpstreamCustomResponseHTTP200Enabled() bool {
	return generalSetting.UpstreamCustomResponseHTTP200Enabled
}

// GetUpstreamPollutionKeywords 返回去重去空后的污染检测关键词切片
func GetUpstreamPollutionKeywords() []string {
	raw := generalSetting.UpstreamPollutionKeywords
	if raw == "" {
		return nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, 8)
	for _, line := range strings.Split(raw, "\n") {
		kw := strings.TrimSpace(line)
		if kw == "" {
			continue
		}
		if _, ok := seen[kw]; ok {
			continue
		}
		seen[kw] = struct{}{}
		result = append(result, kw)
	}
	return result
}

// IsUpstreamPollutionDisableChannel 命中污染后是否自动禁用渠道
func IsUpstreamPollutionDisableChannel() bool {
	return generalSetting.UpstreamPollutionDisableChannel
}

func IsUpstreamSuspiciousPollutionDetectionEnabled() bool {
	return generalSetting.UpstreamSuspiciousPollutionDetectionEnabled
}

func IsUpstreamInterceptAuditEnabled() bool {
	return generalSetting.UpstreamInterceptAuditEnabled
}

func GetUpstreamInterceptAuditRetentionDays() int {
	if generalSetting.UpstreamInterceptAuditRetentionDays <= 0 {
		return 30
	}
	return generalSetting.UpstreamInterceptAuditRetentionDays
}

// GetUpstreamPollutionMessage 返回命中污染后给下游展示的纯文本自定义响应内容
func GetUpstreamPollutionMessage() string {
	return sanitizeCustomResponseMessage(
		generalSetting.UpstreamPollutionMessage,
		"上游响应命中安全过滤，请稍后重试",
	)
}

// GetUpstreamFailureMessage 返回上游故障后给下游展示的纯文本自定义响应内容
func GetUpstreamFailureMessage() string {
	return sanitizeCustomResponseMessage(
		generalSetting.UpstreamFailureMessage,
		"上游服务暂时不可用，请稍后重试",
	)
}

// GetUpstreamPollutionJSONTemplate 返回非流式拦截响应模板（原样，调用方负责渲染和容错）
func GetUpstreamPollutionJSONTemplate() string {
	return strings.TrimSpace(generalSetting.UpstreamPollutionJSONTemplate)
}

// GetUpstreamPollutionStreamTemplate 返回流式拦截响应模板（原样,调用方负责渲染和容错）
// 注意: 此模板不做 TrimSpace,因为 SSE 帧的换行结构是有语义的
func GetUpstreamPollutionStreamTemplate() string {
	return generalSetting.UpstreamPollutionStreamTemplate
}

// GetUpstreamFailureJSONTemplate 返回非流式上游故障安全响应模板（原样，调用方负责渲染和容错）
func GetUpstreamFailureJSONTemplate() string {
	return strings.TrimSpace(generalSetting.UpstreamFailureJSONTemplate)
}

// GetUpstreamFailureStreamTemplate 返回流式上游故障安全响应模板（原样,调用方负责渲染和容错）
// 注意: 此模板不做 TrimSpace,因为 SSE 帧的换行结构是有语义的
func GetUpstreamFailureStreamTemplate() string {
	return generalSetting.UpstreamFailureStreamTemplate
}

// IsGlobalMaintenanceEnabled 全局维护模式是否启用
func IsGlobalMaintenanceEnabled() bool {
	return generalSetting.GlobalMaintenanceEnabled
}

// GetGlobalMaintenanceMessage 返回全局维护模式简易纯文本消息
func GetGlobalMaintenanceMessage() string {
	return strings.TrimSpace(generalSetting.GlobalMaintenanceMessage)
}

// GetGlobalMaintenanceJSONTemplate 返回全局维护模式非流式响应模板（原样，调用方负责渲染和容错）
func GetGlobalMaintenanceJSONTemplate() string {
	return strings.TrimSpace(generalSetting.GlobalMaintenanceJSONTemplate)
}

// GetGlobalMaintenanceStreamTemplate 返回全局维护模式流式响应模板（原样,调用方负责渲染和容错）
// 注意: 此模板不做 TrimSpace,因为 SSE 帧的换行结构是有语义的
func GetGlobalMaintenanceStreamTemplate() string {
	return generalSetting.GlobalMaintenanceStreamTemplate
}

func sanitizeCustomResponseMessage(message string, fallback string) string {
	message = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(message))
	if message == "" {
		return ""
	}
	if len([]rune(message)) > 2000 {
		return fallback
	}
	return message
}

func sanitizeUpstreamErrorMessage(message string, fallback string) string {
	message = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(message))
	if message == "" {
		return fallback
	}
	if len([]rune(message)) > 120 {
		return fallback
	}
	return message
}

// IsCurrencyDisplay 是否以货币形式展示（美元或人民币）
func IsCurrencyDisplay() bool {
	return generalSetting.QuotaDisplayType != QuotaDisplayTypeTokens
}

// IsCNYDisplay 是否以人民币展示
func IsCNYDisplay() bool {
	return generalSetting.QuotaDisplayType == QuotaDisplayTypeCNY
}

// GetQuotaDisplayType 返回额度类型
func GetQuotaDisplayType() string {
	return generalSetting.QuotaDisplayType
}

// GetCurrencySymbol 返回当前展示类型对应符号
func GetCurrencySymbol() string {
	switch generalSetting.QuotaDisplayType {
	case QuotaDisplayTypeUSD:
		return "$"
	case QuotaDisplayTypeCNY:
		return "¥"
	case QuotaDisplayTypeCustom:
		if generalSetting.CustomCurrencySymbol != "" {
			return generalSetting.CustomCurrencySymbol
		}
		return "¤"
	default:
		return ""
	}
}

// GetUsdToCurrencyRate 返回 1 USD = X <currency> 的 X（TOKENS 不适用）
func GetUsdToCurrencyRate(usdToCny float64) float64 {
	switch generalSetting.QuotaDisplayType {
	case QuotaDisplayTypeUSD:
		return 1
	case QuotaDisplayTypeCNY:
		return usdToCny
	case QuotaDisplayTypeCustom:
		if generalSetting.CustomCurrencyExchangeRate > 0 {
			return generalSetting.CustomCurrencyExchangeRate
		}
		return 1
	default:
		return 1
	}
}
