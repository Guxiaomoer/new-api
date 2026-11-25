package trace

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustObject(t *testing.T, raw string) map[string]any {
	t.Helper()
	var obj map[string]any
	require.NoError(t, common.Unmarshal([]byte(raw), &obj))
	return obj
}

func TestScrubKeepsShapeAndMasksSecrets(t *testing.T) {
	raw := `{"Authorization":"Bearer sk-abcdef1234567890","api_key":"sk-live-9876543210","note":"AKIAABCDEFGHIJKLMNOP","n":1}`
	scrubbed := Scrub(raw)

	assert.NotContains(t, scrubbed, "sk-abcdef1234567890")
	assert.NotContains(t, scrubbed, "sk-live-9876543210")
	assert.NotContains(t, scrubbed, "AKIAABCDEFGHIJKLMNOP")

	var parsed map[string]any
	require.NoError(t, common.Unmarshal([]byte(scrubbed), &parsed), "脱敏后必须仍是合法 JSON")
	assert.Equal(t, true, parsed["n"] == float64(1) || parsed["n"] == 1)
	assert.True(t, strings.Contains(scrubbed, "sk-abc***") || strings.HasPrefix(scrubbed, `{"Authorization"`))
}

func TestFingerprintStableAcrossToolRound(t *testing.T) {
	first := mustObject(t, `{"model":"m","system":"sys","messages":[{"role":"user","content":"do the thing"}]}`)
	later := mustObject(t, `{"model":"m","system":"sys","messages":[
		{"role":"user","content":"do the thing"},
		{"role":"assistant","content":"working"},
		{"role":"tool","content":"result"}
	]}`)

	assert.Equal(t, Fingerprint(first, "chat"), Fingerprint(later, "chat"))
	assert.NotEqual(t, Fingerprint(first, "chat"), Fingerprint(first, "anthropic"))
}

func TestBuildDiffReportsAddedMessages(t *testing.T) {
	before := &model.TapTurn{
		Id: 1, Seq: 1,
		RequestJson: `{"body":{"model":"m","messages":[{"role":"user","content":"a"}]}}`,
	}
	after := &model.TapTurn{
		Id: 2, Seq: 2,
		RequestJson: `{"body":{"model":"m","messages":[{"role":"user","content":"a"},{"role":"assistant","content":"b"}]}}`,
	}

	diff := BuildDiff(before, after)
	require.NotNil(t, diff)
	doc, ok := diff.(map[string]any)
	require.True(t, ok)
	messages, ok := doc["messages"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, messages["before_total"])
	assert.Equal(t, 2, messages["after_total"])
	assert.Len(t, messages["added"].([]map[string]any), 1)
}

func TestSalvageJSONPrefixRecoversTruncatedBody(t *testing.T) {
	full := `{"headers":{},"body":{"model":"m","messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"wor`
	recovered := salvageJSONPrefix(full)
	require.NotNil(t, recovered)
	msgs := bodyMessages(recovered)
	require.Len(t, msgs, 2, "截断处的那条消息也要尽量保留")
	assert.Equal(t, "assistant", msgs[1]["role"])
}

func TestAuxKindRequiresNoTools(t *testing.T) {
	titleWithTools := mustObject(t, `{"system":"Create a concise title for this conversation","tools":[{"type":"function","function":{"name":"x"}}]}`)
	titleWithoutTools := mustObject(t, `{"system":"Create a concise title for this conversation"}`)

	assert.Equal(t, "main", AuxKind(titleWithTools))
	assert.Equal(t, "title", AuxKind(titleWithoutTools))
}

func TestReconstructSSEToolCallAndText(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"hi"}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"read","arguments":"{\"p\":"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]}}]}`,
		`data: [DONE]`,
	}, "\n\n")

	recon := Reconstruct(raw, "chat")
	assert.Equal(t, "hi", recon.Text)
	require.Len(t, recon.ToolCalls, 1)
	assert.Equal(t, "read", recon.ToolCalls[0].Name)
	assert.JSONEq(t, `{"p":1}`, recon.ToolCalls[0].Arguments)
}
