package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
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

func withDebugEnabled(t *testing.T, enabled bool) {
	t.Helper()

	oldDebug := common.DebugEnabled
	common.DebugEnabled = enabled
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
	})
}
