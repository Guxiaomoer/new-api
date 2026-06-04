package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayErrorHandlerMasksRateLimitCooldownBody(t *testing.T) {
	t.Parallel()

	responseBody := `{"error":{"message":"通▸知◁群 １７５８７７５５２ 公益 token 先休息10分钟","type":"invalid_request_error","code":"rate_limit_cooldown"},"message":"通▸知◁群 １７５８７７５５２ 公益 token 先休息10分钟","code":"rate_limit_cooldown","limit_type":"cooldown"}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, true)

	require.Equal(t, types.ErrorCode("rate_limit_cooldown"), newAPIError.GetErrorCode())
	require.Equal(t, "上游服务触发冷却限制，请稍后重试", newAPIError.Error())
	require.NotContains(t, newAPIError.Error(), "通▸知◁")
	require.NotContains(t, newAPIError.Error(), "通知")
	require.NotContains(t, newAPIError.Error(), "公益")
	require.NotContains(t, newAPIError.Error(), "body:")
}

func TestRelayErrorHandlerMasksNestedRateLimitCooldownBody(t *testing.T) {
	t.Parallel()

	responseBody := `{"error":{"message":"通▸知◁群 １７５８７７５５２ 公益 token 先休息10分钟","type":"invalid_request_error","code":"rate_limit_cooldown"}}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, true)

	require.Equal(t, types.ErrorCode("rate_limit_cooldown"), newAPIError.GetErrorCode())
	require.Equal(t, "上游服务触发冷却限制，请稍后重试", newAPIError.Error())
	require.NotContains(t, newAPIError.Error(), "通▸知◁")
	require.NotContains(t, newAPIError.Error(), "通知")
	require.NotContains(t, newAPIError.Error(), "公益")
	require.NotContains(t, newAPIError.Error(), "body:")
}

func TestRelayErrorHandlerMasksShowBodyWhenFailUpstreamError(t *testing.T) {
	t.Parallel()

	responseBody := `{"error":{"message":"通▸知◁群 １７５８７７５５２ 公益 token","type":"invalid_request_error","code":"bad_upstream"}}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, true)

	require.Equal(t, types.ErrorCode("bad_upstream"), newAPIError.GetErrorCode())
	require.Equal(t, "上游服务返回错误，请稍后重试", newAPIError.Error())
	require.NotContains(t, newAPIError.Error(), "通▸知◁")
	require.NotContains(t, newAPIError.Error(), "公益")
	require.NotContains(t, newAPIError.Error(), "body:")
}

func TestRelayErrorHandlerUsesConfiguredRateLimitCooldownMessage(t *testing.T) {
	oldMessage := operation_setting.GetGeneralSetting().UpstreamRateLimitCooldownMessage
	operation_setting.GetGeneralSetting().UpstreamRateLimitCooldownMessage = "自定义冷却提示"
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().UpstreamRateLimitCooldownMessage = oldMessage
	})

	responseBody := `{"code":"rate_limit_cooldown"}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, true)

	require.Equal(t, types.ErrorCode("rate_limit_cooldown"), newAPIError.GetErrorCode())
	require.Equal(t, "自定义冷却提示", newAPIError.Error())
}

func TestRelayErrorHandlerFallsBackForUnsafeConfiguredMessage(t *testing.T) {
	oldMessage := operation_setting.GetGeneralSetting().UpstreamRateLimitCooldownMessage
	operation_setting.GetGeneralSetting().UpstreamRateLimitCooldownMessage = strings.Repeat("很长", 80)
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().UpstreamRateLimitCooldownMessage = oldMessage
	})

	responseBody := `{"code":"rate_limit_cooldown"}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, true)

	require.Equal(t, types.ErrorCode("rate_limit_cooldown"), newAPIError.GetErrorCode())
	require.Equal(t, "上游服务触发冷却限制，请稍后重试", newAPIError.Error())
}

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestRelayErrorHandlerTruncatesInvalidJSONBodyInLog(t *testing.T) {
	withDebugEnabled(t, false)

	body := strings.Repeat("b", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, "bad response status code 500", newAPIError.Error())
	require.Contains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), fmt.Sprintf("original_length=%d", len(body)))
	require.NotContains(t, logBuffer.String(), strings.Repeat("b", common.LocalLogContentLimit+1))
}

func TestRelayErrorHandlerKeepsStructuredErrorMessage(t *testing.T) {
	message := strings.Repeat("c", common.LocalLogContentLimit+256)
	body := `{"message":"` + message + `"}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsOpenAIErrorMessage(t *testing.T) {
	message := strings.Repeat("d", common.LocalLogContentLimit+256)
	body := `{"error":{"message":"` + message + `","type":"server_error","code":"server_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsInvalidJSONBodyInDebugLog(t *testing.T) {
	withDebugEnabled(t, true)

	body := strings.Repeat("e", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.NotContains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), body)
}

func TestRenderUpstreamFailureResponse(t *testing.T) {
	oldJSONTemplate := operation_setting.GetGeneralSetting().UpstreamFailureJSONTemplate
	oldStreamTemplate := operation_setting.GetGeneralSetting().UpstreamFailureStreamTemplate
	operation_setting.GetGeneralSetting().UpstreamFailureJSONTemplate = `{"error":{"message":"休息一下，号池维护中","type":"{{.ErrorCode}}","code":"upstream_maintenance"},"model":{{json .Model}}}`
	operation_setting.GetGeneralSetting().UpstreamFailureStreamTemplate = "data: {\"choices\":[{\"delta\":{\"content\":\"休息一下，号池维护中\"}}]}\n\ndata: [DONE]\n\n"
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().UpstreamFailureJSONTemplate = oldJSONTemplate
		operation_setting.GetGeneralSetting().UpstreamFailureStreamTemplate = oldStreamTemplate
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set(string(constant.ContextKeyOriginalModel), "gpt-test")
	c.Set(string(constant.ContextKeyChannelId), 123)
	c.Set(string(constant.ContextKeyChannelName), "upstream-a")
	c.Set(common.RequestIdKey, "req-test")

	newAPIError := types.NewErrorWithStatusCode(
		errors.New("upstream unavailable"),
		types.ErrorCodeDoRequestFailed,
		http.StatusInternalServerError,
	)

	jsonResult := RenderUpstreamFailureResponse(c, newAPIError, false)
	require.NotNil(t, jsonResult)
	require.Contains(t, jsonResult.Rendered, "休息一下，号池维护中")
	require.Contains(t, jsonResult.Rendered, "do_request_failed")
	require.Contains(t, jsonResult.Rendered, "gpt-test")

	streamResult := RenderUpstreamFailureResponse(c, newAPIError, true)
	require.NotNil(t, streamResult)
	require.Equal(t, operation_setting.GetGeneralSetting().UpstreamFailureStreamTemplate, streamResult.Rendered)
}

func TestRenderUpstreamFailureResponseInvalidJSONFallback(t *testing.T) {
	oldJSONTemplate := operation_setting.GetGeneralSetting().UpstreamFailureJSONTemplate
	operation_setting.GetGeneralSetting().UpstreamFailureJSONTemplate = `not-json`
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().UpstreamFailureJSONTemplate = oldJSONTemplate
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	newAPIError := types.NewErrorWithStatusCode(
		errors.New("upstream bad response"),
		types.ErrorCodeBadResponse,
		http.StatusBadGateway,
	)

	result := RenderUpstreamFailureResponse(c, newAPIError, false)
	require.Nil(t, result)
}

func TestRenderGlobalMaintenanceResponse(t *testing.T) {
	oldEnabled := operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled
	oldJSONTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate
	oldStreamTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = true
	operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = `{"error":{"message":"休息一下，号池维护中","type":"maintenance","code":"maintenance"},"model":{{json .Model}}}`
	operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = "data: {\"choices\":[{\"delta\":{\"content\":\"休息一下，号池维护中\"}}]}\n\ndata: [DONE]\n\n"
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = oldEnabled
		operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = oldJSONTemplate
		operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = oldStreamTemplate
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set(string(constant.ContextKeyOriginalModel), "gpt-test")
	c.Set(common.RequestIdKey, "req-test")

	jsonResult := RenderGlobalMaintenanceResponse(c, false)
	require.NotNil(t, jsonResult)
	require.Contains(t, jsonResult.Rendered, "休息一下，号池维护中")
	require.Contains(t, jsonResult.Rendered, "maintenance")
	require.Contains(t, jsonResult.Rendered, "gpt-test")

	streamResult := RenderGlobalMaintenanceResponse(c, true)
	require.NotNil(t, streamResult)
	require.Equal(t, operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate, streamResult.Rendered)
}

func TestRenderGlobalMaintenanceResponseDisabled(t *testing.T) {
	oldEnabled := operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled
	oldJSONTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = false
	operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = `{"message":"maintenance"}`
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = oldEnabled
		operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = oldJSONTemplate
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	result := RenderGlobalMaintenanceResponse(c, false)
	require.Nil(t, result)
}

func TestRenderGlobalMaintenanceResponseInvalidJSONFallback(t *testing.T) {
	oldEnabled := operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled
	oldJSONTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = true
	operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = `not-json`
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = oldEnabled
		operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = oldJSONTemplate
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	result := RenderGlobalMaintenanceResponse(c, false)
	require.NotNil(t, result)
	require.Contains(t, result.Rendered, "maintenance")
	require.Contains(t, result.Rendered, "休息一下，号池维护中")
}

func TestRenderGlobalMaintenanceResponseEmptyTemplateFallback(t *testing.T) {
	oldEnabled := operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled
	oldJSONTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = true
	operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = ``
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = oldEnabled
		operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = oldJSONTemplate
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	result := RenderGlobalMaintenanceResponse(c, false)
	require.NotNil(t, result)
	require.Contains(t, result.Rendered, "maintenance")
}

func TestRenderGlobalMaintenanceResponseEmptyStreamTemplateFallback(t *testing.T) {
	oldEnabled := operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled
	oldStreamTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = true
	operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = ``
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = oldEnabled
		operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = oldStreamTemplate
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	result := RenderGlobalMaintenanceResponse(c, true)
	require.NotNil(t, result)
	require.Contains(t, result.Rendered, "data:")
	require.Contains(t, result.Rendered, "[DONE]")
}

func TestRenderGlobalMaintenanceResponseRejectsRawStreamModel(t *testing.T) {
	oldEnabled := operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled
	oldStreamTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = true
	operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = "data: {{.Model}}\n\ndata: [DONE]\n\n"
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = oldEnabled
		operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = oldStreamTemplate
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set(string(constant.ContextKeyOriginalModel), "safe\n\ndata: injected")

	result := RenderGlobalMaintenanceResponse(c, true)

	require.NotNil(t, result)
	require.Contains(t, result.Rendered, "休息一下，号池维护中")
	require.NotContains(t, result.Rendered, "safe")
}

func TestRenderGlobalMaintenanceResponseRejectsRawStreamModelWithModelJSON(t *testing.T) {
	oldEnabled := operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled
	oldStreamTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = true
	operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = `data: {{printf "%s%s" .ModelJSON .Model}}

data: [DONE]

`
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = oldEnabled
		operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = oldStreamTemplate
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set(string(constant.ContextKeyOriginalModel), `safe"model`)

	result := RenderGlobalMaintenanceResponse(c, true)

	require.NotNil(t, result)
	require.Contains(t, result.Rendered, "休息一下，号池维护中")
	require.NotContains(t, result.Rendered, "safe")
}

func TestRenderGlobalMaintenanceResponseAllowsModelJSONInStream(t *testing.T) {
	oldEnabled := operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled
	oldStreamTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = true
	operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = "data: {\"model\":{{.ModelJSON}}}\n\ndata: [DONE]\n\n"
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = oldEnabled
		operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = oldStreamTemplate
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set(string(constant.ContextKeyOriginalModel), `safe"model`)

	result := RenderGlobalMaintenanceResponse(c, true)

	require.NotNil(t, result)
	require.Contains(t, result.Rendered, `"safe\"model"`)
}

func TestIsUpstreamFailureError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		err      *types.NewAPIError
		expected bool
	}{
		{
			name:     "do request failed",
			err:      types.NewError(errors.New("connect failed"), types.ErrorCodeDoRequestFailed),
			expected: true,
		},
		{
			name:     "get channel failed",
			err:      types.NewError(errors.New("no channel"), types.ErrorCodeGetChannelFailed),
			expected: true,
		},
		{
			name:     "bad upstream status code",
			err:      types.NewErrorWithStatusCode(errors.New("bad status"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway),
			expected: true,
		},
		{
			name:     "bad upstream response",
			err:      types.NewError(errors.New("bad response"), types.ErrorCodeBadResponse),
			expected: true,
		},
		{
			name:     "bad upstream body",
			err:      types.NewError(errors.New("bad body"), types.ErrorCodeBadResponseBody),
			expected: true,
		},
		{
			name:     "read upstream body failed",
			err:      types.NewError(errors.New("read body failed"), types.ErrorCodeReadResponseBodyFailed),
			expected: true,
		},
		{
			name:     "generic 5xx upstream-like OpenAI error",
			err:      types.NewOpenAIError(errors.New("server error"), types.ErrorCode("server_error"), http.StatusInternalServerError),
			expected: true,
		},
		{
			name:     "channel operational no available key",
			err:      types.NewError(errors.New("no available key"), types.ErrorCodeChannelNoAvailableKey),
			expected: true,
		},
		{
			name:     "channel operational invalid key",
			err:      types.NewError(errors.New("invalid key"), types.ErrorCodeChannelInvalidKey),
			expected: true,
		},
		{
			name:     "channel operational response timeout",
			err:      types.NewOpenAIError(errors.New("timeout"), types.ErrorCodeChannelResponseTimeExceeded, http.StatusRequestTimeout),
			expected: true,
		},
		{
			name:     "channel config param override invalid",
			err:      types.NewError(errors.New("bad param override"), types.ErrorCodeChannelParamOverrideInvalid),
			expected: false,
		},
		{
			name:     "channel config header override invalid",
			err:      types.NewError(errors.New("bad header override"), types.ErrorCodeChannelHeaderOverrideInvalid),
			expected: false,
		},
		{
			name:     "channel config model mapped error",
			err:      types.NewError(errors.New("model mapped error"), types.ErrorCodeChannelModelMappedError),
			expected: false,
		},
		{
			name:     "generic channel error code is not auto allowed",
			err:      types.NewError(errors.New("channel disabled"), types.ErrorCode("channel:disabled")),
			expected: false,
		},
		{
			name:     "invalid local request",
			err:      types.NewErrorWithStatusCode(errors.New("invalid request"), types.ErrorCodeInvalidRequest, http.StatusBadRequest),
			expected: false,
		},
		{
			name:     "sensitive words local error",
			err:      types.NewErrorWithStatusCode(errors.New("sensitive"), types.ErrorCodeSensitiveWordsDetected, http.StatusBadRequest),
			expected: false,
		},
		{
			name:     "read request body local error",
			err:      types.NewErrorWithStatusCode(errors.New("too large"), types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge),
			expected: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.expected, IsUpstreamFailureError(tc.err))
		})
	}
}

func withDebugEnabled(t *testing.T, enabled bool) {
	t.Helper()

	oldDebug := common.DebugEnabled
	common.DebugEnabled = enabled
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
	})
}
