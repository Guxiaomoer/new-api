package trace

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type ToolCall struct {
	Id        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Recon struct {
	Text      string     `json:"text"`
	Reasoning string     `json:"reasoning"`
	ToolCalls []ToolCall `json:"tool_calls"`
}

func (r *Recon) empty() bool {
	return r.Text == "" && r.Reasoning == "" && len(r.ToolCalls) == 0
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CacheHitTokens   int `json:"cache_hit_tokens"`
	CacheMissTokens  int `json:"cache_miss_tokens"`
}

type toolCallState struct {
	order []int
	calls map[int]*ToolCall
}

func newToolCallState() *toolCallState {
	return &toolCallState{calls: map[int]*ToolCall{}}
}

func (s *toolCallState) get(index int) *ToolCall {
	call, ok := s.calls[index]
	if !ok {
		call = &ToolCall{}
		s.calls[index] = call
		s.order = append(s.order, index)
	}
	return call
}

func (s *toolCallState) list() []ToolCall {
	sort.Ints(s.order)
	out := make([]ToolCall, 0, len(s.order))
	for _, index := range s.order {
		out = append(out, *s.calls[index])
	}
	return out
}

func sseDataLines(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		out = append(out, payload)
	}
	return out
}

func looksLikeSSE(raw string) bool {
	return strings.Contains(raw, "data:")
}

// Reconstruct 从上游原始响应重建「模型说了什么」：正文 / 思考 / 工具调用。
// 先按 OpenAI chat 形态解析；解析不出内容时再按目标协议解析（上游可能是原生
// Anthropic / Responses 形态）。
func Reconstruct(raw string, protocol string) Recon {
	if raw == "" {
		return Recon{}
	}
	if !looksLikeSSE(raw) {
		var payload map[string]any
		if err := common.Unmarshal([]byte(raw), &payload); err == nil {
			if recon := reconstructChatPayload(payload); !recon.empty() {
				return recon
			}
			switch protocol {
			case "anthropic":
				if recon := reconstructAnthropicPayload(payload); !recon.empty() {
					return recon
				}
			case "responses":
				if recon := reconstructResponsesPayload(payload); !recon.empty() {
					return recon
				}
			}
			return Recon{}
		}
		return Recon{}
	}
	recon := reconstructSSE(raw, protocol)
	if !recon.empty() {
		return recon
	}
	return Recon{}
}

func reconstructSSE(raw string, protocol string) Recon {
	var text, reasoning strings.Builder
	tools := newToolCallState()
	for _, payload := range sseDataLines(raw) {
		var event map[string]any
		if err := common.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		if choices, ok := event["choices"].([]any); ok && len(choices) > 0 {
			choice, _ := choices[0].(map[string]any)
			delta, _ := choice["delta"].(map[string]any)
			if delta == nil {
				delta, _ = choice["message"].(map[string]any)
			}
			mergeChatDelta(delta, &text, &reasoning, tools)
			continue
		}
		if eventType, _ := event["type"].(string); eventType != "" {
			switch eventType {
			case "content_block_start":
				index := intOf(event["index"])
				block, _ := event["content_block"].(map[string]any)
				if block != nil && block["type"] == "tool_use" {
					call := tools.get(index)
					call.Id, _ = block["id"].(string)
					call.Name, _ = block["name"].(string)
				}
			case "content_block_delta":
				index := intOf(event["index"])
				delta, _ := event["delta"].(map[string]any)
				if delta == nil {
					continue
				}
				switch delta["type"] {
				case "text_delta":
					text.WriteString(stringOf(delta["text"]))
				case "thinking_delta":
					reasoning.WriteString(stringOf(delta["thinking"]))
				case "input_json_delta":
					tools.get(index).Arguments += stringOf(delta["partial_json"])
				}
			case "response.output_text.delta":
				text.WriteString(stringOf(event["delta"]))
			case "response.reasoning_text.delta", "response.reasoning_summary_text.delta":
				reasoning.WriteString(stringOf(event["delta"]))
			case "response.output_item.added":
				item, _ := event["item"].(map[string]any)
				if item != nil && item["type"] == "function_call" {
					call := tools.get(intOf(event["output_index"]))
					call.Id, _ = item["call_id"].(string)
					call.Name, _ = item["name"].(string)
				}
			case "response.function_call_arguments.delta":
				tools.get(intOf(event["output_index"])).Arguments += stringOf(event["delta"])
			}
		}
	}
	return Recon{Text: text.String(), Reasoning: reasoning.String(), ToolCalls: tools.list()}
}

func mergeChatDelta(delta map[string]any, text, reasoning *strings.Builder, tools *toolCallState) {
	if delta == nil {
		return
	}
	text.WriteString(stringOf(delta["content"]))
	reasoning.WriteString(stringOf(delta["reasoning_content"]))
	calls, _ := delta["tool_calls"].([]any)
	for _, item := range calls {
		call, ok := item.(map[string]any)
		if !ok {
			continue
		}
		state := tools.get(intOf(call["index"]))
		if id, ok := call["id"].(string); ok && id != "" {
			state.Id = id
		}
		fn, _ := call["function"].(map[string]any)
		if fn != nil {
			if name, ok := fn["name"].(string); ok && name != "" {
				state.Name = name
			}
			state.Arguments += stringOf(fn["arguments"])
		}
	}
}

func reconstructChatPayload(payload map[string]any) Recon {
	var text, reasoning strings.Builder
	tools := newToolCallState()
	if choices, ok := payload["choices"].([]any); ok && len(choices) > 0 {
		choice, _ := choices[0].(map[string]any)
		message, _ := choice["message"].(map[string]any)
		mergeChatDelta(message, &text, &reasoning, tools)
	}
	return Recon{Text: text.String(), Reasoning: reasoning.String(), ToolCalls: tools.list()}
}

func reconstructAnthropicPayload(payload map[string]any) Recon {
	var text, reasoning strings.Builder
	var toolCalls []ToolCall
	blocks, _ := payload["content"].([]any)
	for _, item := range blocks {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch block["type"] {
		case "text":
			text.WriteString(stringOf(block["text"]))
		case "thinking":
			reasoning.WriteString(stringOf(block["thinking"]))
		case "tool_use":
			args, _ := common.Marshal(block["input"])
			toolCalls = append(toolCalls, ToolCall{
				Id:        stringOf(block["id"]),
				Name:      stringOf(block["name"]),
				Arguments: string(args),
			})
		}
	}
	return Recon{Text: text.String(), Reasoning: reasoning.String(), ToolCalls: toolCalls}
}

func reconstructResponsesPayload(payload map[string]any) Recon {
	var text, reasoning strings.Builder
	var toolCalls []ToolCall
	items, _ := payload["output"].([]any)
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch entry["type"] {
		case "message":
			text.WriteString(contentText(entry["content"]))
		case "reasoning":
			reasoning.WriteString(contentText(entry["summary"]))
			reasoning.WriteString(contentText(entry["content"]))
		case "function_call":
			toolCalls = append(toolCalls, ToolCall{
				Id:        stringOf(entry["call_id"]),
				Name:      stringOf(entry["name"]),
				Arguments: stringOf(entry["arguments"]),
			})
		}
	}
	return Recon{Text: text.String(), Reasoning: reasoning.String(), ToolCalls: toolCalls}
}

// ReconstructClient 客户端视角：翻译层（anthropic / responses）回给客户端的报文
// 与上游不同，必须按目标协议解析。
func ReconstructClient(raw string, protocol string) *Recon {
	if raw == "" {
		return nil
	}
	var recon Recon
	switch protocol {
	case "anthropic":
		recon = Reconstruct(raw, "anthropic")
		if recon.empty() {
			recon = reconstructSSE(raw, "anthropic")
		}
	case "responses":
		recon = Reconstruct(raw, "responses")
	default:
		recon = Reconstruct(raw, protocol)
	}
	if recon.empty() {
		return nil
	}
	return &recon
}

// ParseUsage 从响应原文里取用量。取不到返回零值，不影响抓包本身。
func ParseUsage(raw string, protocol string) Usage {
	var usage Usage
	if raw == "" {
		return usage
	}
	if !looksLikeSSE(raw) {
		var payload map[string]any
		if err := common.Unmarshal([]byte(raw), &payload); err != nil {
			return usage
		}
		applyUsageObject(payload["usage"], &usage)
		if protocol == "anthropic" {
			if message, ok := payload["message"].(map[string]any); ok {
				applyAnthropicUsage(message["usage"], &usage)
			}
		}
		return usage
	}
	for _, payload := range sseDataLines(raw) {
		var event map[string]any
		if err := common.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		if u, ok := event["usage"]; ok {
			applyUsageObject(u, &usage)
		}
		if event["type"] == "message_start" {
			if message, ok := event["message"].(map[string]any); ok {
				applyAnthropicUsage(message["usage"], &usage)
			}
		}
		if event["type"] == "message_delta" {
			applyAnthropicUsage(event["usage"], &usage)
		}
		if event["type"] == "response.completed" {
			if response, ok := event["response"].(map[string]any); ok {
				applyUsageObject(response["usage"], &usage)
			}
		}
	}
	return usage
}

func applyUsageObject(raw any, usage *Usage) {
	obj, ok := raw.(map[string]any)
	if !ok {
		return
	}
	if v := intOf(obj["prompt_tokens"]); v > 0 {
		usage.PromptTokens = v
	}
	if v := intOf(obj["completion_tokens"]); v > 0 {
		usage.CompletionTokens = v
	}
	if v := intOf(obj["input_tokens"]); v > 0 {
		usage.PromptTokens = v
	}
	if v := intOf(obj["output_tokens"]); v > 0 {
		usage.CompletionTokens = v
	}
	if details, ok := obj["prompt_tokens_details"].(map[string]any); ok {
		if v := intOf(details["cached_tokens"]); v > 0 {
			usage.CacheHitTokens = v
		}
	}
	if details, ok := obj["input_tokens_details"].(map[string]any); ok {
		if v := intOf(details["cached_tokens"]); v > 0 {
			usage.CacheHitTokens = v
		}
	}
	if details, ok := obj["prompt_cache_hit_tokens"]; ok {
		if v := intOf(details); v > 0 {
			usage.CacheHitTokens = v
		}
	}
	if miss := intOf(obj["prompt_cache_miss_tokens"]); miss > 0 {
		usage.CacheMissTokens = miss
	}
}

func applyAnthropicUsage(raw any, usage *Usage) {
	obj, ok := raw.(map[string]any)
	if !ok {
		return
	}
	if v := intOf(obj["input_tokens"]); v > 0 {
		usage.PromptTokens = v
	}
	if v := intOf(obj["output_tokens"]); v > 0 {
		usage.CompletionTokens = v
	}
	if v := intOf(obj["cache_read_input_tokens"]); v > 0 {
		usage.CacheHitTokens = v
	}
	if v := intOf(obj["cache_creation_input_tokens"]); v > 0 {
		usage.CacheMissTokens = v
	}
}

func stringOf(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	if value == nil {
		return ""
	}
	raw, err := common.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func intOf(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		n := 0
		for _, ch := range v {
			if ch < '0' || ch > '9' {
				return 0
			}
			n = n*10 + int(ch-'0')
		}
		return n
	}
	return 0
}
