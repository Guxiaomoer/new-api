package middleware

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelMatchesExpectedTaskPluginUsesGenericChannelSetting(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeTaskPlugin}
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "generic-alpha"})

	assert.True(t, channelMatchesExpectedTaskPlugin(nil, channel, "generic-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, "generic-beta"))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, ""))
}

func TestChannelMatchesExpectedTaskPluginUsesPinnedLegacyIndex(t *testing.T) {
	registry := jsplugin.NewRegistry()
	alpha, err := registry.Register(distributorTaskPluginSource("legacy-alpha", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)
	pinnedGeneration := registry.Generation()

	require.NoError(t, registry.Unregister("legacy-alpha"))
	_, err = registry.Register(distributorTaskPluginSource("legacy-beta", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{
		Generation: pinnedGeneration,
		Plugin:     alpha,
	})
	channel := &model.Channel{Type: constant.ChannelTypeKling}

	assert.True(t, channelMatchesExpectedTaskPlugin(c, channel, "legacy-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, channel, "legacy-beta"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "legacy-alpha"))
}

func TestChannelMatchesExpectedTaskPluginRejectsUnindexedLegacyChannel(t *testing.T) {
	registry := jsplugin.NewRegistry()
	plugin, err := registry.Register(distributorTaskPluginSource("legacy-alpha", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{
		Generation: registry.Generation(),
		Plugin:     plugin,
	})

	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "legacy-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: 0}, "legacy-alpha"))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, ""))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, &model.Channel{Type: constant.ChannelTypeKling}, "legacy-alpha"))

	c.Set("expected_task_plugin_key", "legacy-alpha")
	setupErr := SetupContextForSelectedChannel(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "task-model")
	require.NotNil(t, setupErr)
	assert.Contains(t, setupErr.Error(), "does not match")
}

func TestSharedEndpointRebindsToSelectedLegacyProvider(t *testing.T) {
	registry := jsplugin.NewRegistry()
	_, err := registry.Register(distributorEndpointPluginSource("gemini-shared", constant.ChannelTypeGemini), jsplugin.Options{})
	require.NoError(t, err)
	_, err = registry.Register(distributorEndpointPluginSource("vertex-shared", constant.ChannelTypeVertexAi), jsplugin.Options{})
	require.NoError(t, err)
	candidates := registry.Generation().LookupEndpointCandidates("POST", "/v1/responses", "task-model")
	require.Len(t, candidates, 2)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: registry.Generation(), Plugin: candidates[0].Plugin})
	c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{
		Generation: registry.Generation(),
		Plugin:     candidates[0].Plugin,
		Protocol:   candidates[0].Protocol,
		Operation:  candidates[0].Operation,
		Model:      "task-model",
		Candidates: candidates,
	})
	c.Set("expected_task_plugin_key", candidates[0].Plugin.Meta.Key)

	geminiChannel := &model.Channel{Id: 1, Type: constant.ChannelTypeGemini}
	vertexChannel := &model.Channel{Id: 2, Type: constant.ChannelTypeVertexAi}
	assert.True(t, channelMatchesExpectedTaskPlugin(c, geminiChannel, candidates[0].Plugin.Meta.Key))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, vertexChannel, candidates[0].Plugin.Meta.Key))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeKling}, candidates[0].Plugin.Meta.Key))

	require.Nil(t, SetupContextForSelectedChannel(c, vertexChannel, "task-model"))
	pinnedValue, exists := c.Get(jsplugin.ContextKeyPinnedEndpoint)
	require.True(t, exists)
	pinned, ok := pinnedValue.(jsplugin.PinnedEndpoint)
	require.True(t, ok)
	assert.Equal(t, "vertex-shared", pinned.Plugin.Meta.Key)
	assert.Equal(t, "vertex-shared", c.GetString("expected_task_plugin_key"))
	assert.Equal(t, "vertex-shared", c.GetString("task_plugin_key"))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, geminiChannel, "vertex-shared"), "a retry may select another declared provider")
}

func distributorTaskPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["task-model"],
  fetchMode: "per_task",
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
`, key, key, channelType)
}

func distributorEndpointPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["task-model"],
  fetchMode: "per_task",
  protocols: [{name: "openai_responses", supports: ["stream", "sync", "background"]}],
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
export const protocols = {openai_responses: {
  decodeRequest: function(ctx) { return {kind: "submit", model: "task-model", requestBody: ctx.body.value}; },
  renderEvents: function() { return {events: [], state: null, done: false}; },
  renderFinal: function() { return {output: []}; },
}};
`, key, key, channelType)
}

func TestNoAvailableChannelMessageNamesClaimingTaskPlugin(t *testing.T) {
	require.NoError(t, i18n.Init())
	registry := jsplugin.NewRegistry()
	plugin, err := registry.Register(distributorTaskPluginSource("claimer", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	pinned, _ := gin.CreateTestContext(nil)
	pinned.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	pinned.Request.Header.Set("Accept-Language", "en")
	pinned.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: registry.Generation(), Plugin: plugin})
	message := noAvailableChannelMessage(pinned, "default", "kling-v1")
	assert.Contains(t, message, `"claimer"`)
	assert.Contains(t, message, "disable or override")
	assert.Contains(t, message, "kling-v1")

	plain, _ := gin.CreateTestContext(nil)
	plain.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	plain.Request.Header.Set("Accept-Language", "en")
	generic := noAvailableChannelMessage(plain, "default", "gpt-4o")
	assert.NotContains(t, generic, "task plugin")
	assert.Contains(t, generic, "gpt-4o")
}

func withGlobalMaintenanceSettings(t *testing.T, enabled bool, jsonTemplate string, streamTemplate string) {
	t.Helper()
	require.NoError(t, i18n.Init())

	oldEnabled := operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled
	oldMessage := operation_setting.GetGeneralSetting().GlobalMaintenanceMessage
	oldJSONTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate
	oldStreamTemplate := operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = enabled
	operation_setting.GetGeneralSetting().GlobalMaintenanceMessage = ""
	operation_setting.GetGeneralSetting().GlobalMaintenanceJSONTemplate = jsonTemplate
	operation_setting.GetGeneralSetting().GlobalMaintenanceStreamTemplate = streamTemplate
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().GlobalMaintenanceEnabled = oldEnabled
		operation_setting.GetGeneralSetting().GlobalMaintenanceMessage = oldMessage
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
