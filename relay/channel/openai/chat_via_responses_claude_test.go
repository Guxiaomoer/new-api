package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newResponsesToClaudeTestContext mirrors newResponsesChatTestContext but sets
// RelayFormat=Claude, reproducing the production path Claude Code takes when
// /v1/messages is routed through the Responses upstream (Grok) and converted
// back to Anthropic Messages SSE.
func newResponsesToClaudeTestContext(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set(common.RequestIdKey, "responses-claude-test")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta:        &relaycommon.ChannelMeta{UpstreamModelName: "grok-test"},
		IsStream:           true,
		RelayFormat:        types.RelayFormatClaude,
		ShouldIncludeUsage: true,
		DisablePing:        true,
	}
	return c, recorder, resp, info
}

func setupClaudeStreamTest(t *testing.T) {
	t.Helper()
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
}

// Case 1: pure text stream. Claude Code needs the full Anthropic terminal
// sequence (message_delta + message_stop) or it hangs waiting.
func TestResponsesToClaudeStreamPureText(t *testing.T) {
	setupClaudeStreamTest(t)

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"grok-test","created_at":1710000000}}`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		`data: {"type":"response.output_text.delta","delta":" world"}`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5}}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	c, recorder, resp, info := newResponsesToClaudeTestContext(t, body)

	usage, apiErr := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	got := recorder.Body.String()
	t.Logf("=== PURE TEXT Anthropic SSE output ===\n%s", got)

	require.Contains(t, got, "event: message_start", "missing message_start")
	require.Contains(t, got, "event: content_block_start", "missing content_block_start")
	require.Contains(t, got, "event: content_block_delta", "missing content_block_delta")
	require.Contains(t, got, `"text":"hello"`)
	require.Contains(t, got, "event: content_block_stop", "missing content_block_stop")
	require.Contains(t, got, "event: message_delta", "missing message_delta (stop_reason carrier)")
	require.Contains(t, got, `"stop_reason":"end_turn"`)
	require.Contains(t, got, "event: message_stop", "missing message_stop => Claude Code hangs")

	requireOrderedSubstrings(t, got,
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		"event: content_block_stop",
		"event: message_delta",
		"event: message_stop",
	)
}

// Case 2: single tool_call streamed incrementally, then response.completed.
func TestResponsesToClaudeStreamSingleToolCall(t *testing.T) {
	setupClaudeStreamTest(t)

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"grok-test","created_at":1710000000}}`,
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup"}}`,
		`data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"q\":"}`,
		`data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"\"x\"}"}`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":4,"output_tokens":6,"total_tokens":10}}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	c, recorder, resp, info := newResponsesToClaudeTestContext(t, body)

	usage, apiErr := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	got := recorder.Body.String()
	t.Logf("=== SINGLE TOOL_CALL Anthropic SSE output ===\n%s", got)

	require.Contains(t, got, "event: message_start")
	require.Contains(t, got, `"type":"tool_use"`, "missing tool_use content block")
	require.Contains(t, got, `"name":"lookup"`)
	require.Contains(t, got, "event: content_block_stop")
	require.Contains(t, got, `"stop_reason":"tool_use"`, "tool call must map to stop_reason tool_use")
	require.Contains(t, got, "event: message_stop", "missing message_stop => Claude Code hangs")
}

// Case 3: two consecutive tool_calls, with a trailing usage-only chunk pattern
// (finish_reason arrives via response.completed carrying usage). This is the
// exact shape that stresses breakpoint B (finish_reason/usage separation).
func TestResponsesToClaudeStreamConsecutiveToolCalls(t *testing.T) {
	setupClaudeStreamTest(t)

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"grok-test","created_at":1710000000}}`,
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"first"}}`,
		`data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"a\":1}"}`,
		`data: {"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"fc_2","call_id":"call_2","name":"second"}}`,
		`data: {"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"b\":2}"}`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":8,"output_tokens":9,"total_tokens":17}}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	c, recorder, resp, info := newResponsesToClaudeTestContext(t, body)

	usage, apiErr := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	got := recorder.Body.String()
	t.Logf("=== CONSECUTIVE TOOL_CALLS Anthropic SSE output ===\n%s", got)

	require.Contains(t, got, "event: message_start")
	require.Contains(t, got, `"name":"first"`)
	require.Contains(t, got, `"name":"second"`)
	require.Contains(t, got, `"stop_reason":"tool_use"`)
	require.Contains(t, got, "event: message_delta")
	require.Contains(t, got, "event: message_stop", "missing message_stop => Claude Code hangs")
}
