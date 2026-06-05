package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func withPollutionFilterSettings(t *testing.T, keywords string, disableChannel bool, message string) {
	t.Helper()
	require.NoError(t, i18n.Init())

	general := operation_setting.GetGeneralSetting()
	oldKeywords := general.UpstreamPollutionKeywords
	oldDisable := general.UpstreamPollutionDisableChannel
	oldMessage := general.UpstreamPollutionMessage
	oldJSON := general.UpstreamPollutionJSONTemplate
	oldStream := general.UpstreamPollutionStreamTemplate
	oldAuditEnabled := general.UpstreamInterceptAuditEnabled
	oldRedisEnabled := common.RedisEnabled
	general.UpstreamPollutionKeywords = keywords
	general.UpstreamPollutionDisableChannel = disableChannel
	general.UpstreamPollutionMessage = message
	general.UpstreamPollutionJSONTemplate = ""
	general.UpstreamPollutionStreamTemplate = ""
	general.UpstreamInterceptAuditEnabled = false
	common.RedisEnabled = false
	t.Cleanup(func() {
		general.UpstreamPollutionKeywords = oldKeywords
		general.UpstreamPollutionDisableChannel = oldDisable
		general.UpstreamPollutionMessage = oldMessage
		general.UpstreamPollutionJSONTemplate = oldJSON
		general.UpstreamPollutionStreamTemplate = oldStream
		general.UpstreamInterceptAuditEnabled = oldAuditEnabled
		common.RedisEnabled = oldRedisEnabled
	})
}

func setupFilterRouter(t *testing.T, handler gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(UpstreamResponseFilter())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyChannelId, 42)
		common.SetContextKey(c, constant.ContextKeyChannelName, "test-channel")
		common.SetContextKey(c, constant.ContextKeyChannelType, 1)
		common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")
	}, handler)
	return router
}

func TestUpstreamResponseFilterPassthroughWhenNoKeywords(t *testing.T) {
	withPollutionFilterSettings(t, "", false, "")

	router := setupFilterRouter(t, func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, `{"choices":[{"message":{"content":"通▸知◁群 175877552"}}]}`)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "通▸知◁群")
}

func TestUpstreamResponseFilterBlocksPollutedNonStream(t *testing.T) {
	withPollutionFilterSettings(t, "通▸知◁群\n公益 token", false, "")

	router := setupFilterRouter(t, func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		body := strings.Repeat("x", pollutionFilterThreshold-10) + "通▸知◁群 rest of body"
		c.String(http.StatusOK, body)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.Contains(t, recorder.Body.String(), "upstream_pollution")
	require.NotContains(t, recorder.Body.String(), "通▸知◁群")
}

func TestUpstreamResponseFilterBlocksPollutedStream(t *testing.T) {
	withPollutionFilterSettings(t, "通▸知◁群", false, "")

	router := setupFilterRouter(t, func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		body := strings.Repeat("x", pollutionFilterThreshold-10) + "通▸知◁群 data"
		c.String(http.StatusOK, body)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, recorder.Body.String(), "upstream_pollution")
	require.Contains(t, recorder.Body.String(), "[DONE]")
	require.NotContains(t, recorder.Body.String(), "通▸知◁群")
}

func TestUpstreamResponseFilterPassesCleanResponse(t *testing.T) {
	withPollutionFilterSettings(t, "通▸知◁群", false, "")

	router := setupFilterRouter(t, func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		body := strings.Repeat("a", pollutionFilterThreshold+100)
		c.String(http.StatusOK, body)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "upstream_pollution")
}

func TestUpstreamResponseFilterFinalizeSmallBody(t *testing.T) {
	withPollutionFilterSettings(t, "通▸知◁群", false, "")

	router := setupFilterRouter(t, func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, "通▸知◁群 small body")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.Contains(t, recorder.Body.String(), "upstream_pollution")
}

func TestUpstreamResponseFilterFinalizeSmallCleanBody(t *testing.T) {
	withPollutionFilterSettings(t, "通▸知◁群", false, "")

	router := setupFilterRouter(t, func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, `{"ok":true}`)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `{"ok":true}`)
}

func TestUpstreamResponseFilterUsesCustomMessage(t *testing.T) {
	withPollutionFilterSettings(t, "通▸知◁群", false, "自定义污染提示")

	router := setupFilterRouter(t, func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, "通▸知◁群 data "+strings.Repeat("x", pollutionFilterThreshold))
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "自定义污染提示")
}

func TestUpstreamResponseFilterDisablesChannelWhenConfigured(t *testing.T) {
	withPollutionFilterSettings(t, "通▸知◁群", true, "")

	router := setupFilterRouter(t, func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, "通▸知◁群 "+strings.Repeat("x", pollutionFilterThreshold))
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
}

func TestUpstreamResponseFilterSmallBodyBelowThresholdPasses(t *testing.T) {
	withPollutionFilterSettings(t, "通▸知◁群", false, "")

	router := setupFilterRouter(t, func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		body := strings.Repeat("a", pollutionFilterThreshold-100)
		c.String(http.StatusOK, body)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "upstream_pollution")
}
