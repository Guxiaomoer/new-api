package trace

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

func ParseObject(raw []byte) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]any
	if err := common.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	return obj
}

func sha1Hex(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func shortHash(value any) string {
	raw, err := common.Marshal(value)
	if err != nil {
		return sha1Hex("")[:16]
	}
	return sha1Hex(string(raw))[:16]
}

// contentText 把 content 拉平成纯文本。四种形态都要认：字符串、OpenAI 的
// [{type:text,text}]、Anthropic 的 [{type:text,text}]、Responses 的 [{type:input_text}]。
func contentText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			parts = append(parts, contentText(item))
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"text", "content", "input_text", "output_text"} {
			if s, ok := v[key].(string); ok && s != "" {
				return s
			}
		}
		return contentText(v["content"])
	case nil:
		return ""
	default:
		raw, err := common.Marshal(v)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}

func bodyMessages(body map[string]any) []map[string]any {
	if body == nil {
		return nil
	}
	if raw, ok := body["messages"].([]any); ok {
		out := make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			if msg, ok := item.(map[string]any); ok {
				out = append(out, msg)
			}
		}
		return out
	}
	if raw, ok := body["input"].([]any); ok {
		out := make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			msg, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if _, hasRole := msg["role"]; hasRole {
				out = append(out, msg)
				continue
			}
			// Responses 的 function_call / function_call_output / reasoning 等条目
			// 没有 role，但要计入消息总量，否则「这一轮多了什么」会漏。
			out = append(out, map[string]any{
				"role":    "tool",
				"content": contentText(msg),
				"_type":   msg["type"],
			})
		}
		return out
	}
	return nil
}

func bodySystemText(body map[string]any) string {
	if body == nil {
		return ""
	}
	if system, ok := body["system"]; ok {
		return contentText(system)
	}
	if instructions, ok := body["instructions"].(string); ok {
		return instructions
	}
	for _, msg := range bodyMessages(body) {
		role, _ := msg["role"].(string)
		if role == "system" || role == "developer" {
			return contentText(msg["content"])
		}
	}
	return ""
}

func bodyFirstUserText(body map[string]any) string {
	for _, msg := range bodyMessages(body) {
		if role, _ := msg["role"].(string); role == "user" {
			return contentText(msg["content"])
		}
	}
	return ""
}

func bodyToolNames(body map[string]any) []string {
	if body == nil {
		return nil
	}
	raw, ok := body["tools"].([]any)
	if !ok {
		return nil
	}
	var names []string
	for _, item := range raw {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fn := tool
		if inner, ok := tool["function"].(map[string]any); ok {
			fn = inner
		}
		if name, ok := fn["name"].(string); ok && name != "" {
			names = append(names, name)
		}
	}
	return names
}

// Fingerprint 同一 agent 任务里 system prompt 和「用户最初那句话」不变，
// 中间的工具往返只是往后追加，所以这两段足以稳定归组。
func Fingerprint(body map[string]any, protocol string) string {
	raw := protocol + "\x00" + bodySystemText(body) + "\x00" + bodyFirstUserText(body)
	return sha1Hex(raw)[:20]
}

var clientRules = []struct {
	family  string
	markers []string
}{
	{"claude", []string{"claude-code", "claude_cli", "claude-cli", "claude", "anthropic"}},
	{"codex", []string{"codex"}},
	{"dsh", []string{"deepseek-harness", "deepseek_harness"}},
	{"cursor", []string{"cursor"}},
	{"vscode", []string{"vscode", "visual studio code", "github-copilot", "copilot-chat"}},
	{"cli", []string{"curl", "wget", "httpie", "python-httpx", "python-requests", "go-http-client", "node", "undici", "postman", "insomnia"}},
}

var clientVersionRe = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_.\-]*)/(\d+(?:\.\d+)+)`)

// ClientFamily 认不出来时取 UA 的第一个产品 token，不能一律记成 other ——
// 那样列表里所有未知客户端会挤成一行，永远分不清是谁。
func ClientFamily(ua string) string {
	lower := strings.ToLower(ua)
	for _, rule := range clientRules {
		for _, marker := range rule.markers {
			if strings.Contains(lower, marker) {
				return rule.family
			}
		}
	}
	if ua == "" {
		return "unknown"
	}
	token := strings.Fields(ua)
	if len(token) == 0 {
		return "unknown"
	}
	name := strings.SplitN(token[0], "/", 2)[0]
	if name == "" {
		return "unknown"
	}
	return clipString(name, 32)
}

func ClientVersion(ua string) string {
	if match := clientVersionRe.FindStringSubmatch(ua); len(match) == 3 {
		return match[2]
	}
	return ""
}

type digestMessage struct {
	Role      string   `json:"role"`
	Hash      string   `json:"h"`
	ToolCalls []string `json:"tc"`
}

type requestDigest struct {
	Kind     string            `json:"kind"`
	Messages []digestMessage   `json:"messages"`
	Tools    []string          `json:"tools"`
	Params   map[string]string `json:"params"`
	Model    string            `json:"model"`
	N        int               `json:"n"`
}

func digestMessages(body map[string]any) []digestMessage {
	msgs := bodyMessages(body)
	out := make([]digestMessage, 0, len(msgs))
	for _, msg := range msgs {
		role, _ := msg["role"].(string)
		out = append(out, digestMessage{
			Role:      role,
			Hash:      shortHash(msg["content"]),
			ToolCalls: toolCallNames(msg),
		})
	}
	return out
}

func toolCallNames(msg map[string]any) []string {
	var names []string
	if calls, ok := msg["tool_calls"].([]any); ok {
		for _, item := range calls {
			call, ok := item.(map[string]any)
			if !ok {
				continue
			}
			fn, _ := call["function"].(map[string]any)
			if fn == nil {
				fn = call
			}
			if name, ok := fn["name"].(string); ok && name != "" {
				names = append(names, name)
			}
		}
	}
	if blocks, ok := msg["content"].([]any); ok {
		for _, item := range blocks {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if block["type"] == "tool_use" {
				if name, ok := block["name"].(string); ok && name != "" {
					names = append(names, name)
				}
			}
		}
	}
	return names
}

func digestParams(body map[string]any) map[string]string {
	skip := map[string]struct{}{
		"messages": {}, "input": {}, "tools": {}, "system": {}, "instructions": {},
		"stream_options": {},
	}
	out := map[string]string{}
	for key, value := range body {
		if _, ok := skip[key]; ok {
			continue
		}
		out[key] = shortHash(value)
	}
	return out
}

// RequestDigest 在截断之前算，只保留结构不保留全文，体积几 KB，永远可比。
func RequestDigest(body map[string]any) any {
	if body == nil {
		return nil
	}
	tools := bodyToolNames(body)
	sort.Strings(tools)
	model, _ := body["model"].(string)
	digest := requestDigest{
		Kind:     "object",
		Messages: digestMessages(body),
		Tools:    tools,
		Params:   digestParams(body),
		Model:    model,
		N:        len(bodyMessages(body)),
	}
	return digest
}

var (
	auxTitlePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)create a concise title`),
		regexp.MustCompile(`(?i)generate the session title`),
		regexp.MustCompile(`(?i)\bsession title\b`),
	}
	auxCompactPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bcompaction\b`),
		regexp.MustCompile(`(?i)compact (the|this) conversation`),
		regexp.MustCompile(`(?i)summar(y|ize) (of )?(the )?conversation so far`),
		regexp.MustCompile(`(?i)conversation summary`),
	}
)

// AuxKind 识别标题生成 / 上下文压缩这类辅助调用：system 特征 + 没有工具，双保险。
func AuxKind(body map[string]any) string {
	system := bodySystemText(body)
	if system == "" || len(bodyToolNames(body)) > 0 {
		return "main"
	}
	for _, pattern := range auxTitlePatterns {
		if pattern.MatchString(system) {
			return "title"
		}
	}
	for _, pattern := range auxCompactPatterns {
		if pattern.MatchString(system) {
			return "compact"
		}
	}
	return "main"
}

var whitespaceRe = regexp.MustCompile(`\s+`)

// NormText 把字面量 \n \r \t 换成空格再折叠空白：客户端序列化进 JSON 的换行
// 是字面量 `\\n`，主会话入库时已压成空格，两边形态完全不同。
func NormText(text string) string {
	text = strings.NewReplacer(`\n`, " ", `\r`, " ", `\t`, " ", "\n", " ", "\r", " ", "\t", " ").Replace(text)
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(text, " "))
}
