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
	CustomCurrencyExchangeRate       float64 `json:"custom_currency_exchange_rate"`
	UpstreamRateLimitCooldownMessage string  `json:"upstream_rate_limit_cooldown_message"`
	UpstreamErrorMessage             string  `json:"upstream_error_message"`
	// 上游响应污染检测关键词，换行分隔，任意一条匹配即视为命中
	UpstreamPollutionKeywords string `json:"upstream_pollution_keywords"`
	// 命中污染后是否自动禁用渠道
	UpstreamPollutionDisableChannel bool `json:"upstream_pollution_disable_channel"`
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
	UpstreamErrorMessage:             "上游服务返回错误，请稍后重试",
	UpstreamPollutionKeywords: `通▸知◁群
公益 token
chatcmpl_local_`,
	UpstreamPollutionDisableChannel: true,
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

// GetQuotaDisplayType 返回额度展示类型
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
