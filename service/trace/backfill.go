package trace

import (
	"sort"

	"github.com/QuantumNous/new-api/model"
)

// Backfill 按入库的 UA 重算客户端归类，再给仍未挂父的辅助调用重新找主会话。
// 分类规则会变，历史数据不该因此失效。
func Backfill() (int, error) {
	updated := 0
	err := model.ForEachTapSession(500, func(session *model.TapSession) error {
		client := ClientFamily(session.ClientUa)
		if client != session.Client {
			if err := model.UpdateTapSessionClient(session.Id, client); err != nil {
				return err
			}
			updated++
		}
		if session.Kind == "main" || session.ParentId != 0 || session.FirstUser == "" {
			return nil
		}
		candidates, err := model.CandidateTapParents(session.Protocol, session.KeyId, session.StartedAt-600, session.Id)
		if err != nil {
			return err
		}
		if parent := matchParent(session.FirstUser, candidates); parent > 0 {
			if err := model.LinkTapSessionParent(session.Id, parent); err == nil {
				updated++
			}
		}
		return nil
	})
	return updated, err
}

// Describe 单轮请求的结构摘要：详情页头部要显示「多少条消息、几个工具」，
// 不能靠前端解析几百 KB 原文。
func Describe(body map[string]any) map[string]any {
	msgs := bodyMessages(body)
	roles := map[string]int{}
	toolCalls := 0
	for _, msg := range msgs {
		role, _ := msg["role"].(string)
		if role == "" {
			role = "unknown"
		}
		roles[role]++
		toolCalls += len(toolCallNames(msg))
	}
	tools := bodyToolNames(body)
	sort.Strings(tools)
	return map[string]any{
		"kind":         AuxKind(body),
		"messages":     len(msgs),
		"roles":        roles,
		"tools":        len(tools),
		"tool_calls":   toolCalls,
		"tool_names":   tools,
		"system_chars": len(bodySystemText(body)),
	}
}
