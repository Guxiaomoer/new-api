package trace

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const diffArrayCap = 50

func turnBody(turn *model.TapTurn) map[string]any {
	if turn == nil {
		return nil
	}
	var doc map[string]any
	if err := common.Unmarshal([]byte(turn.RequestJson), &doc); err == nil {
		if body, ok := doc["upstream_body"].(map[string]any); ok {
			return body
		}
		if body, ok := doc["body"].(map[string]any); ok {
			return body
		}
	}
	return salvageJSONPrefix(turn.RequestJson)
}

// salvageJSONPrefix 从（可能被截断的）入库原文里切出能解析的请求体前缀。
func salvageJSONPrefix(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	if parsed, ok := jsonPrefix(raw); ok {
		if body, ok := parsed["upstream_body"].(map[string]any); ok {
			return body
		}
		if body, ok := parsed["body"].(map[string]any); ok {
			return body
		}
		if parsed["messages"] != nil || parsed["input"] != nil {
			return parsed
		}
	}
	// 截断点落在 body 之前时外层抢救不出来，单独再抢救 body 段。
	// 不能按 `"body": {` 匹配：序列化后的报文没有空格。
	if at := strings.LastIndex(raw, `"body"`); at >= 0 {
		rest := raw[at:]
		if brace := strings.Index(rest, "{"); brace >= 0 {
			if parsed, ok := jsonPrefix(rest[brace:]); ok {
				return parsed
			}
		}
	}
	return nil
}

// jsonPrefix 切出 JSON 文本里可解析的最长前缀（补齐未闭合的括号）。
// 切点必须落在字符串之外：截断常发生在字符串中间，切出来的前缀带一个未闭合
// 的字符串，整个抢救作废。
func jsonPrefix(text string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil, false
	}
	var stack []byte
	inString := false
	escaped := false
	safeEnds := make([]int, 0, 16)
	lastComma := -1
	for i := 0; i < len(trimmed); i++ {
		ch := trimmed[i]
		if inString {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, ch)
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			safeEnds = append(safeEnds, i+1)
			if len(stack) == 0 {
				i = len(trimmed)
			}
		case ',':
			if len(stack) > 0 {
				lastComma = i
			}
		}
	}
	if lastComma > 0 {
		safeEnds = append(safeEnds, lastComma)
	}
	if len(safeEnds) == 0 {
		return nil, false
	}
	sort.Ints(safeEnds)
	for i := len(safeEnds) - 1; i >= 0; i-- {
		end := safeEnds[i]
		if end <= 0 {
			continue
		}
		prefix := trimmed[:end]
		depth := bracketDepth(trimmed, end)
		for j := len(depth) - 1; j >= 0; j-- {
			if depth[j] == '{' {
				prefix += "}"
			} else {
				prefix += "]"
			}
		}
		prefix = strings.TrimRight(strings.TrimRight(strings.TrimSpace(prefix), ","), " ")
		if prefix == "" {
			continue
		}
		var parsed map[string]any
		if err := common.Unmarshal([]byte(prefix), &parsed); err == nil && parsed != nil {
			return parsed, true
		}
	}
	return nil, false
}

func bracketDepth(text string, upto int) []byte {
	var stack []byte
	inString := false
	escaped := false
	if upto > len(text) {
		upto = len(text)
	}
	for i := 0; i < upto; i++ {
		ch := text[i]
		if inString {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, ch)
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return stack
}

type msgView struct {
	Role    string `json:"role"`
	Preview string `json:"preview"`
	Chars   int    `json:"chars"`
}

func buildMsgView(msg map[string]any) msgView {
	role, _ := msg["role"].(string)
	if role == "" {
		role = "unknown"
	}
	text := contentText(msg["content"])
	return msgView{Role: role, Preview: preview(text, 160), Chars: len(text)}
}

func preview(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}

func paramView(body map[string]any) map[string]string {
	skip := map[string]struct{}{
		"messages": {}, "input": {}, "tools": {}, "system": {}, "instructions": {},
		"stream_options": {},
	}
	out := map[string]string{}
	for key, value := range body {
		if _, ok := skip[key]; ok {
			continue
		}
		raw, err := common.Marshal(value)
		if err != nil {
			continue
		}
		out[key] = clipString(string(raw), 400)
	}
	return out
}

func toolMap(body map[string]any) map[string]string {
	out := map[string]string{}
	raw, ok := body["tools"].([]any)
	if !ok {
		return out
	}
	for _, item := range raw {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fn := tool
		if inner, ok := tool["function"].(map[string]any); ok {
			fn = inner
		}
		name, _ := fn["name"].(string)
		if name == "" {
			continue
		}
		encoded, err := common.Marshal(fn)
		if err != nil {
			continue
		}
		out[name] = string(encoded)
	}
	return out
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// BuildDiff 两轮请求的结构化差异。按「参数 / system / 消息 / 工具」四块比，
// 用户一眼就能看到这一轮到底多了什么。
func BuildDiff(before, after *model.TapTurn) any {
	if before == nil || after == nil {
		return nil
	}
	if before.Truncated || after.Truncated {
		if diff := diffFromDigest(before, after); diff != nil {
			return diff
		}
	}
	bodyA, bodyB := turnBody(before), turnBody(after)

	paramsA, paramsB := paramView(bodyA), paramView(bodyB)
	params := make([]map[string]any, 0)
	for _, key := range unionKeys(paramsA, paramsB) {
		if paramsA[key] != paramsB[key] {
			params = append(params, map[string]any{"key": key, "before": paramsA[key], "after": paramsB[key]})
		}
	}

	sysA, sysB := bodySystemText(bodyA), bodySystemText(bodyB)
	system := map[string]any{
		"changed":      sysA != sysB,
		"before_chars": len(sysA),
		"after_chars":  len(sysB),
		"before":       preview(sysA, 400),
		"after":        preview(sysB, 400),
	}

	msgsA := viewsOf(bodyA)
	msgsB := viewsOf(bodyB)
	commonLen := len(msgsA)
	if len(msgsB) < commonLen {
		commonLen = len(msgsB)
	}
	changed := make([]map[string]any, 0)
	for i := 0; i < commonLen; i++ {
		if msgsA[i] != msgsB[i] {
			changed = append(changed, map[string]any{
				"index": i, "role": msgsB[i].Role,
				"before": msgsA[i].Preview, "after": msgsB[i].Preview,
			})
		}
	}
	added := make([]map[string]any, 0)
	for i := commonLen; i < len(msgsB); i++ {
		added = append(added, map[string]any{
			"index": i, "role": msgsB[i].Role, "preview": msgsB[i].Preview, "chars": msgsB[i].Chars,
		})
	}
	removed := make([]map[string]any, 0)
	for i := commonLen; i < len(msgsA); i++ {
		removed = append(removed, map[string]any{
			"index": i, "role": msgsA[i].Role, "preview": msgsA[i].Preview,
		})
	}

	toolsA, toolsB := toolMap(bodyA), toolMap(bodyB)
	toolsAdded := difference(sortedKeys(toolsB), sortedKeys(toolsA))
	toolsRemoved := difference(sortedKeys(toolsA), sortedKeys(toolsB))
	toolsChanged := make([]map[string]any, 0)
	for _, name := range sortedKeys(toolsA) {
		if valueB, ok := toolsB[name]; ok && valueB != toolsA[name] {
			toolsChanged = append(toolsChanged, map[string]any{
				"name": name, "before": clipString(toolsA[name], 200), "after": clipString(valueB, 200),
			})
		}
	}

	return map[string]any{
		"before": turnHeader(before),
		"after":  turnHeader(after),
		"params": params,
		"system": system,
		"messages": map[string]any{
			"before_total": len(msgsA),
			"after_total":  len(msgsB),
			"common":       commonLen,
			"added":        capAny(added),
			"removed":      capAny(removed),
			"changed":      capAny(changed),
		},
		"tools": map[string]any{
			"added":   capStrings(toolsAdded),
			"removed": capStrings(toolsRemoved),
			"changed": capAny(toolsChanged),
		},
		"summary": map[string]any{
			"params":  len(params),
			"system":  boolToInt(sysA != sysB),
			"added":   len(added),
			"removed": len(removed),
			"changed": len(changed),
			"tools":   len(toolsAdded) + len(toolsRemoved) + len(toolsChanged),
		},
	}
}

func viewsOf(body map[string]any) []msgView {
	msgs := bodyMessages(body)
	out := make([]msgView, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, buildMsgView(msg))
	}
	return out
}

func turnHeader(turn *model.TapTurn) map[string]any {
	return map[string]any{
		"id": turn.Id, "seq": turn.Seq, "ts": turn.Ts, "model": turn.Model,
	}
}

func unionKeys(a, b map[string]string) []string {
	seen := map[string]struct{}{}
	for key := range a {
		seen[key] = struct{}{}
	}
	for key := range b {
		seen[key] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func difference(a, b []string) []string {
	inB := map[string]struct{}{}
	for _, item := range b {
		inB[item] = struct{}{}
	}
	out := make([]string, 0)
	for _, item := range a {
		if _, ok := inB[item]; !ok {
			out = append(out, item)
		}
	}
	return out
}

func capAny(items []map[string]any) []map[string]any {
	if len(items) > diffArrayCap {
		return items[:diffArrayCap]
	}
	return items
}

func capStrings(items []string) []string {
	if len(items) > diffArrayCap {
		return items[:diffArrayCap]
	}
	return items
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// diffFromDigest 截断的两轮用摘要比：摘要的 messages 是逐条哈希，能精确指出
// 「新增了哪几条、改了哪几条」——这正是截断丢掉的部分。
func diffFromDigest(before, after *model.TapTurn) any {
	var digestA, digestB map[string]any
	if err := common.Unmarshal([]byte(before.RequestDigest), &digestA); err != nil || digestA == nil {
		return nil
	}
	if err := common.Unmarshal([]byte(after.RequestDigest), &digestB); err != nil || digestB == nil {
		return nil
	}
	msgsA := digestMessagesOf(digestA)
	msgsB := digestMessagesOf(digestB)
	commonLen := len(msgsA)
	if len(msgsB) < commonLen {
		commonLen = len(msgsB)
	}
	changed := make([]map[string]any, 0)
	for i := 0; i < commonLen; i++ {
		if stringOf(msgsA[i]["h"]) != stringOf(msgsB[i]["h"]) ||
			stringOf(msgsA[i]["role"]) != stringOf(msgsB[i]["role"]) {
			changed = append(changed, map[string]any{
				"index": i, "role": stringOf(msgsB[i]["role"]),
				"before": digestPreview(msgsA[i]), "after": digestPreview(msgsB[i]),
			})
		}
	}
	added := make([]map[string]any, 0)
	for i := commonLen; i < len(msgsB); i++ {
		added = append(added, map[string]any{
			"index": i, "role": stringOf(msgsB[i]["role"]), "preview": digestPreview(msgsB[i]), "chars": 0,
		})
	}
	removed := make([]map[string]any, 0)
	for i := commonLen; i < len(msgsA); i++ {
		removed = append(removed, map[string]any{
			"index": i, "role": stringOf(msgsA[i]["role"]), "preview": digestPreview(msgsA[i]),
		})
	}
	paramsA, _ := digestA["params"].(map[string]any)
	paramsB, _ := digestB["params"].(map[string]any)
	params := make([]map[string]any, 0)
	for _, key := range unionAnyKeys(paramsA, paramsB) {
		if stringOf(paramsA[key]) != stringOf(paramsB[key]) {
			params = append(params, map[string]any{"key": key, "before": stringOf(paramsA[key]), "after": stringOf(paramsB[key])})
		}
	}
	toolsA := stringListOf(digestA["tools"])
	toolsB := stringListOf(digestB["tools"])
	return map[string]any{
		"before": turnHeader(before),
		"after":  turnHeader(after),
		"params": params,
		"system": map[string]any{
			"changed": false, "before_chars": 0, "after_chars": 0, "before": "", "after": "",
		},
		"messages": map[string]any{
			"before_total": len(msgsA), "after_total": len(msgsB), "common": commonLen,
			"added": capAny(added), "removed": capAny(removed), "changed": capAny(changed),
		},
		"tools": map[string]any{
			"added": capStrings(difference(toolsB, toolsA)), "removed": capStrings(difference(toolsA, toolsB)),
			"changed": []any{},
		},
		"summary": map[string]any{
			"params": len(params), "system": 0, "added": len(added), "removed": len(removed),
			"changed": len(changed),
			"tools":   len(difference(toolsB, toolsA)) + len(difference(toolsA, toolsB)),
		},
		// 截断下用的是摘要：界面要说明「比的是摘要」，否则用户会以为看到的是全文。
		"from_digest": true,
	}
}

func digestMessagesOf(digest map[string]any) []map[string]any {
	raw, _ := digest["messages"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if msg, ok := item.(map[string]any); ok {
			out = append(out, msg)
		}
	}
	return out
}

func digestPreview(msg map[string]any) string {
	role := stringOf(msg["role"])
	if role == "" {
		role = "?"
	}
	hash := stringOf(msg["h"])
	if len(hash) > 8 {
		hash = hash[:8]
	}
	if tools := stringListOf(msg["tc"]); len(tools) > 0 {
		if len(tools) > 3 {
			tools = tools[:3]
		}
		return role + " · " + strings.Join(tools, ", ") + " · " + hash
	}
	return role + " · " + hash
}

func stringListOf(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		out = append(out, stringOf(item))
	}
	return out
}

func unionAnyKeys(a, b map[string]any) []string {
	seen := map[string]struct{}{}
	for key := range a {
		seen[key] = struct{}{}
	}
	for key := range b {
		seen[key] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
