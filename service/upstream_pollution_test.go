package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// withPollutionKeywords 临时替换全局污染关键词配置，t.Cleanup 自动还原。
func withPollutionKeywords(t *testing.T, kw string) {
	t.Helper()
	general := operation_setting.GetGeneralSetting()
	old := general.UpstreamPollutionKeywords
	general.UpstreamPollutionKeywords = kw
	t.Cleanup(func() {
		general.UpstreamPollutionKeywords = old
	})
}

func withSuspiciousPollutionDetection(t *testing.T, enabled bool) {
	t.Helper()
	general := operation_setting.GetGeneralSetting()
	old := general.UpstreamSuspiciousPollutionDetectionEnabled
	general.UpstreamSuspiciousPollutionDetectionEnabled = enabled
	t.Cleanup(func() {
		general.UpstreamSuspiciousPollutionDetectionEnabled = old
	})
}

func TestDetectUpstreamPollutionHitsKeyword(t *testing.T) {
	withPollutionKeywords(t, "通▸知◁群\n公益 token\nchatcmpl_local_")

	body := []byte(`{"id":"resp_1","choices":[{"message":{"content":"通▸知◁群 175877552 加群送福利"}}]}`)
	hit := DetectUpstreamPollution(body)
	require.Equal(t, "通▸知◁群", hit)
}

func TestDetectUpstreamPollutionHitsFakeId(t *testing.T) {
	withPollutionKeywords(t, "通▸知◁群\n公益 token\nchatcmpl_local_")

	body := []byte(`{"id":"chatcmpl_local_abcdef","choices":[]}`)
	hit := DetectUpstreamPollution(body)
	require.Equal(t, "chatcmpl_local_", hit)
}

func TestDetectUpstreamPollutionMissesCleanResponse(t *testing.T) {
	withPollutionKeywords(t, "通▸知◁群\n公益 token\nchatcmpl_local_")

	body := []byte(`{"id":"chatcmpl-abc","choices":[{"message":{"content":"Hello, how can I help you?"}}]}`)
	hit := DetectUpstreamPollution(body)
	require.Equal(t, "", hit)
}

func TestDetectUpstreamPollutionEmptyConfig(t *testing.T) {
	withPollutionKeywords(t, "")
	withSuspiciousPollutionDetection(t, false)

	body := []byte(`{"content":"通▸知◁群 加群送福利"}`)
	hit := DetectUpstreamPollution(body)
	require.Equal(t, "", hit, "空配置应跳过检测,即使响应体含敏感词")
}

func TestDetectUpstreamPollutionHitsSuspiciousPromotionContact(t *testing.T) {
	withPollutionKeywords(t, "")
	withSuspiciousPollutionDetection(t, true)

	body := []byte(`{"choices":[{"message":{"content":"公益福利领取，请加群 12345678"}}]}`)
	detection := DetectUpstreamPollutionDetail(body)
	require.True(t, detection.Matched)
	require.Equal(t, UpstreamPollutionTypeSuspicious, detection.Type)
	require.Equal(t, "promotion_contact_or_link", detection.Rule)
}

func TestDetectUpstreamPollutionHitsSuspiciousURLTokenClaim(t *testing.T) {
	withPollutionKeywords(t, "")
	withSuspiciousPollutionDetection(t, true)

	hit := DetectUpstreamPollution([]byte(`{"content":"领取 token 请访问 https://example.invalid"}`))
	require.Equal(t, "url_token_key_claim", hit)
}

func TestDetectUpstreamPollutionHitsSuspiciousHTMLProtocolAnomaly(t *testing.T) {
	withPollutionKeywords(t, "")
	withSuspiciousPollutionDetection(t, true)

	hit := DetectUpstreamPollution([]byte(`<!doctype html><html><script>alert(1)</script></html>`))
	require.Equal(t, "html_protocol_anomaly", hit)
}

func TestDetectUpstreamPollutionSuspiciousDisabled(t *testing.T) {
	withPollutionKeywords(t, "")
	withSuspiciousPollutionDetection(t, false)

	hit := DetectUpstreamPollution([]byte(`{"id":"chatcmpl_local_fake"}`))
	require.Equal(t, "", hit)
}

func TestDetectUpstreamPollutionEmptyBody(t *testing.T) {
	withPollutionKeywords(t, "通▸知◁群")

	hit := DetectUpstreamPollution(nil)
	require.Equal(t, "", hit)
	hit = DetectUpstreamPollution([]byte{})
	require.Equal(t, "", hit)
}

func TestDetectUpstreamPollutionStringVariant(t *testing.T) {
	withPollutionKeywords(t, "公益 token")

	hit := DetectUpstreamPollutionString("data: {\"content\":\"公益 token 先休息10分钟\"}\n")
	require.Equal(t, "公益 token", hit)
}

func TestGetUpstreamPollutionKeywordsTrimsAndDedupes(t *testing.T) {
	withPollutionKeywords(t, "  通▸知◁群  \n\n公益 token\n通▸知◁群\n  \n")

	keywords := operation_setting.GetUpstreamPollutionKeywords()
	require.Equal(t, []string{"通▸知◁群", "公益 token"}, keywords,
		"应当去除空白行、首尾空格、重复条目")
}

func TestDetectUpstreamPollutionHandlesLargeBody(t *testing.T) {
	withPollutionKeywords(t, "公益 token")

	// 模拟前面有大段正常内容，末尾才出现污染关键词
	body := strings.Repeat("a", 8*1024) + "公益 token here"
	hit := DetectUpstreamPollution([]byte(body))
	require.Equal(t, "公益 token", hit, "应当扫描完整 body,不限制开头位置")
}

// ============================================================================
// RenderUpstreamPollutionResponse tests
// ============================================================================

func withPollutionTemplates(t *testing.T, message, jsonTmpl, streamTmpl string) {
	t.Helper()
	general := operation_setting.GetGeneralSetting()
	oldMsg := general.UpstreamPollutionMessage
	oldJSON := general.UpstreamPollutionJSONTemplate
	oldStream := general.UpstreamPollutionStreamTemplate
	general.UpstreamPollutionMessage = message
	general.UpstreamPollutionJSONTemplate = jsonTmpl
	general.UpstreamPollutionStreamTemplate = streamTmpl
	t.Cleanup(func() {
		general.UpstreamPollutionMessage = oldMsg
		general.UpstreamPollutionJSONTemplate = oldJSON
		general.UpstreamPollutionStreamTemplate = oldStream
	})
}

func pollutionTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	c.Set(string(constant.ContextKeyOriginalModel), "gpt-test")
	c.Set(string(constant.ContextKeyChannelId), 7)
	c.Set(string(constant.ContextKeyChannelName), "ch-test")
	c.Set(common.RequestIdKey, "req-123")
	return c
}

func TestRenderUpstreamPollutionResponseMessageTakesPriority(t *testing.T) {
	withPollutionTemplates(t, "自定义污染提示", `{"custom":"template"}`, "")

	c := pollutionTestContext()
	result := RenderUpstreamPollutionResponse(c, false, "通▸知◁群")
	require.NotNil(t, result)
	require.Contains(t, result.Rendered, "自定义污染提示")
}

func TestRenderUpstreamPollutionResponseMessageStreamPriority(t *testing.T) {
	withPollutionTemplates(t, "自定义污染提示", "", `data: {"custom":"template"}\n\ndata: [DONE]\n\n`)

	c := pollutionTestContext()
	result := RenderUpstreamPollutionResponse(c, true, "通▸知◁群")
	require.NotNil(t, result)
	require.Contains(t, result.Rendered, "自定义污染提示")
}

func TestRenderUpstreamPollutionResponseJSONTemplate(t *testing.T) {
	withPollutionTemplates(t, "", `{"error":{"message":"blocked","keyword":"{{.Keyword}}","model":"{{.Model}}"}}`, "")

	c := pollutionTestContext()
	result := RenderUpstreamPollutionResponse(c, false, "通▸知◁群")
	require.NotNil(t, result)
	require.Contains(t, result.Rendered, "blocked")
	require.Contains(t, result.Rendered, "通▸知◁群")
	require.Contains(t, result.Rendered, "gpt-test")
}

func TestRenderUpstreamPollutionResponseStreamTemplate(t *testing.T) {
	withPollutionTemplates(t, "", "", "data: {\"content\":\"{{.Keyword}}\"}\n\ndata: [DONE]\n\n")

	c := pollutionTestContext()
	result := RenderUpstreamPollutionResponse(c, true, "通▸知◁群")
	require.NotNil(t, result)
	require.Contains(t, result.Rendered, "通▸知◁群")
	require.Contains(t, result.Rendered, "[DONE]")
}

func TestRenderUpstreamPollutionResponseEmptyTemplateReturnsNil(t *testing.T) {
	withPollutionTemplates(t, "", "", "")

	c := pollutionTestContext()
	result := RenderUpstreamPollutionResponse(c, false, "通▸知◁群")
	require.Nil(t, result, "模板全空应返回nil,走fallback")
}

func TestRenderUpstreamPollutionResponseInvalidJSONReturnsNil(t *testing.T) {
	withPollutionTemplates(t, "", "not-json {{{", "")

	c := pollutionTestContext()
	result := RenderUpstreamPollutionResponse(c, false, "通▸知◁群")
	require.Nil(t, result, "无效JSON模板应返回nil")
}

func TestRenderUpstreamPollutionResponseContextVariables(t *testing.T) {
	withPollutionTemplates(t, "", `{"model":"{{.Model}}","keyword":"{{.Keyword}}","channel":{{.ChannelId}},"name":"{{.ChannelName}}","req":"{{.RequestId}}","ts":{{.Created}}}`, "")

	c := pollutionTestContext()
	result := RenderUpstreamPollutionResponse(c, false, "test-kw")
	require.NotNil(t, result)
	require.Contains(t, result.Rendered, `"model":"gpt-test"`)
	require.Contains(t, result.Rendered, `"keyword":"test-kw"`)
	require.Contains(t, result.Rendered, `"channel":7`)
	require.Contains(t, result.Rendered, `"name":"ch-test"`)
	require.Contains(t, result.Rendered, `"req":"req-123"`)
}

func TestRenderUpstreamPollutionResponseJSONHelper(t *testing.T) {
	withPollutionTemplates(t, "", `{"model":{{json .Model}}}`, "")

	c := pollutionTestContext()
	result := RenderUpstreamPollutionResponse(c, false, "kw")
	require.NotNil(t, result)
	require.Contains(t, result.Rendered, `"gpt-test"`)
}
