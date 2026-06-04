package middleware

import (
	"bytes"
	"io"
	"mime/multipart"
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

func withGlobalMaintenanceSettings(t *testing.T, enabled bool, jsonTemplate string, streamTemplate string) {
	t.Helper()
	require.NoError(t, i18n.Init())

	oldEnabled := operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled
	oldJSONTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate
	oldStreamTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = enabled
	operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = jsonTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = streamTemplate
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = oldEnabled
		operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = oldJSONTemplate
		operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = oldStreamTemplate
	})
}

func TestDistributeGlobalMaintenanceAbortsHandlerChain(t *testing.T) {
	withGlobalMaintenanceSettings(t, true, `{"message":"maintenance","model":{{json .Model}}}`, "")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	downstreamCalled := false
	router.POST("/v1/chat/completions", Distribute(), func(c *gin.Context) {
		downstreamCalled = true
		c.JSON(http.StatusAccepted, gin.H{"called": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, downstreamCalled)
	require.Contains(t, recorder.Body.String(), "maintenance")
	require.Contains(t, recorder.Body.String(), "gpt-test")
}

func TestDistributeGlobalMaintenanceHandlesInvalidJSONBeforeValidation(t *testing.T) {
	withGlobalMaintenanceSettings(t, true, `{"message":"maintenance"}`, "")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/chat/completions", Distribute(), func(c *gin.Context) {
		c.JSON(http.StatusAccepted, gin.H{"called": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "maintenance")
}

func TestDistributeGlobalMaintenanceRespectsTokenModelLimit(t *testing.T) {
	withGlobalMaintenanceSettings(t, true, `{"message":"maintenance","model":{{json .Model}}}`, "")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	downstreamCalled := false
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
		common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"allowed-model": true})
	}, Distribute(), func(c *gin.Context) {
		downstreamCalled = true
		c.JSON(http.StatusAccepted, gin.H{"called": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.False(t, downstreamCalled)
	require.NotContains(t, recorder.Body.String(), "maintenance")
}

func TestDistributeGlobalMaintenanceRespectsFormTokenModelLimit(t *testing.T) {
	withGlobalMaintenanceSettings(t, true, `{"message":"maintenance","model":{{json .Model}}}`, "")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	downstreamCalled := false
	router.POST("/v1/audio/transcriptions", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
		common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"allowed-model": true})
	}, Distribute(), func(c *gin.Context) {
		downstreamCalled = true
		c.JSON(http.StatusAccepted, gin.H{"called": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", strings.NewReader(`model=gpt-test`))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.False(t, downstreamCalled)
	require.NotContains(t, recorder.Body.String(), "maintenance")
}

func TestDistributeGlobalMaintenanceRejectsMultipartWhenModelLimitEnabled(t *testing.T) {
	withGlobalMaintenanceSettings(t, true, `{"message":"maintenance","model":{{json .Model}}}`, "")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	downstreamCalled := false
	router.POST("/v1/audio/transcriptions", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
		common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"allowed-model": true})
	}, Distribute(), func(c *gin.Context) {
		downstreamCalled = true
		c.JSON(http.StatusAccepted, gin.H{"called": true})
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-test"))
	fileWriter, err := writer.CreateFormFile("file", "audio.wav")
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte(strings.Repeat("a", 1024)))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.False(t, downstreamCalled)
	require.NotContains(t, recorder.Body.String(), "maintenance")
}

func TestBestEffortGlobalMaintenanceModelRequestRestoresBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := `{"model":"gpt-test","stream":true}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	modelRequest := bestEffortGlobalMaintenanceModelRequest(c)
	requestBody, err := io.ReadAll(c.Request.Body)

	require.NoError(t, err)
	require.Equal(t, "gpt-test", modelRequest.Model)
	require.Equal(t, body, string(requestBody))
}
