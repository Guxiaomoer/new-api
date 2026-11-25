package model

import (
	"gorm.io/gorm"
)

// 抓包配置项在 options 表里的键名。热读：网关每次请求读一次 OptionMap。
const (
	TapOptionEnabled       = "TapEnabled"
	TapOptionMaxBody       = "TapMaxBody"
	TapOptionRetentionDays = "TapRetentionDays"
	TapOptionMaxSessions   = "TapMaxSessions"
)

// TapSession 一次调用链（一个 agent 任务）的会话头。
// 抓包数据与 logs 刻意分表：logs 是计量核心，查询频繁；抓包单行可达数百 KB，
// 需要单独的容量与清理策略。
type TapSession struct {
	Id               int64  `json:"id" gorm:"primaryKey"`
	Fingerprint      string `json:"fingerprint" gorm:"type:varchar(64);not null;index:idx_tap_sessions_fp,priority:1"`
	KeyId            int    `json:"key_id" gorm:"index:idx_tap_sessions_fp,priority:2"`
	KeyName          string `json:"key_name" gorm:"type:varchar(128)"`
	Client           string `json:"client" gorm:"type:varchar(64)"`
	ClientUa         string `json:"client_ua" gorm:"type:varchar(512)"`
	FirstUser        string `json:"first_user" gorm:"type:text"`
	Kind             string `json:"kind" gorm:"type:varchar(16);not null;index"`
	ParentId         int64  `json:"parent_id" gorm:"index"`
	Protocol         string `json:"protocol" gorm:"type:varchar(16);not null;index:idx_tap_sessions_fp,priority:3"`
	Model            string `json:"model" gorm:"type:varchar(128)"`
	StartedAt        int64  `json:"started_at" gorm:"not null"`
	UpdatedAt        int64  `json:"updated_at" gorm:"not null;index:idx_tap_sessions_updated"`
	Turns            int    `json:"turns" gorm:"not null"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"not null"`
	CompletionTokens int    `json:"completion_tokens" gorm:"not null"`
}

func (TapSession) TableName() string {
	return "tap_sessions"
}

// TapTurn 一轮调用的完整证据。正文列必须是 TEXT：抓包会按 max_body 截断，
// 截断后的 JSON 是非法 JSON，写进 JSONB 会直接失败。
type TapTurn struct {
	Id                 int64   `json:"id" gorm:"primaryKey"`
	SessionId          int64   `json:"session_id" gorm:"not null;index:idx_tap_turns_session,priority:1"`
	Seq                int     `json:"seq" gorm:"not null;index:idx_tap_turns_session,priority:2"`
	Ts                 int64   `json:"ts" gorm:"not null;index:idx_tap_turns_ts"`
	Path               string  `json:"path" gorm:"type:varchar(128)"`
	Protocol           string  `json:"protocol" gorm:"type:varchar(16)"`
	Model              string  `json:"model" gorm:"type:varchar(128)"`
	MappedModel        string  `json:"mapped_model" gorm:"type:varchar(128)"`
	Status             int     `json:"status"`
	Stream             bool    `json:"stream"`
	LatencyMs          int     `json:"latency_ms"`
	FirstTokenMs       int     `json:"first_token_ms"`
	PromptTokens       int     `json:"prompt_tokens"`
	CompletionTokens   int     `json:"completion_tokens"`
	CacheHitTokens     int     `json:"cache_hit_tokens"`
	CacheMissTokens    int     `json:"cache_miss_tokens"`
	Cost               float64 `json:"cost"`
	RequestJson        string  `json:"-" gorm:"type:text"`
	RequestDigest      string  `json:"-" gorm:"type:text"`
	ResponseText       string  `json:"-" gorm:"type:text"`
	ClientResponseText string  `json:"-" gorm:"type:text"`
	TraceId            string  `json:"trace_id" gorm:"type:varchar(64)"`
	ReconJson          string  `json:"-" gorm:"type:text"`
	UsageJson          string  `json:"-" gorm:"type:text"`
	Error              string  `json:"error" gorm:"type:varchar(500)"`
	Truncated          bool    `json:"truncated" gorm:"not null"`
}

func (TapTurn) TableName() string {
	return "tap_turns"
}

// TapTurnDetail 单轮详情。正文列在列表与普通查询里是 json:"-"，只有管理员
// 点开某一轮时才随详情一起下发。
type TapTurnDetail struct {
	TapTurn
	RequestJson        string `json:"request_json"`
	RequestDigest      string `json:"request_digest"`
	ResponseText       string `json:"response_text"`
	ClientResponseText string `json:"client_response_text"`
	ReconJson          string `json:"recon_json"`
	UsageJson          string `json:"usage_json"`
}

func (t *TapTurn) Detail() TapTurnDetail {
	return TapTurnDetail{
		TapTurn:            *t,
		RequestJson:        t.RequestJson,
		RequestDigest:      t.RequestDigest,
		ResponseText:       t.ResponseText,
		ClientResponseText: t.ClientResponseText,
		ReconJson:          t.ReconJson,
		UsageJson:          t.UsageJson,
	}
}

func CreateTapTurn(turn *TapTurn) error {
	return DB.Create(turn).Error
}

func UpdateTapSessionTurn(sessionId int64, now int64, seq, promptTokens, completionTokens int, modelName string) error {
	updates := map[string]any{
		"updated_at":        now,
		"turns":             seq,
		"prompt_tokens":     gorm.Expr("prompt_tokens + ?", promptTokens),
		"completion_tokens": gorm.Expr("completion_tokens + ?", completionTokens),
	}
	if modelName != "" {
		updates["model"] = modelName
	}
	return DB.Model(&TapSession{}).Where("id = ?", sessionId).Updates(updates).Error
}

// FindTapSession 在合并窗口内找同指纹会话。
func FindTapSession(fingerprint string, keyId int, protocol string, since int64) (*TapSession, error) {
	var session TapSession
	err := DB.Where("fingerprint = ? AND key_id = ? AND protocol = ? AND updated_at >= ?",
		fingerprint, keyId, protocol, since).Order("updated_at DESC").First(&session).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func CreateTapSession(session *TapSession) error {
	return DB.Create(session).Error
}

// FillTapSessionFirstTurn 补齐首轮可能缺失的字段（UA / 首条用户消息）。
func FillTapSessionFirstTurn(sessionId int64, ua, firstUser string) error {
	updates := map[string]any{}
	if ua != "" {
		updates["client_ua"] = ua
	}
	if firstUser != "" {
		updates["first_user"] = firstUser
	}
	if len(updates) == 0 {
		return nil
	}
	return DB.Model(&TapSession{}).Where("id = ?", sessionId).Updates(updates).Error
}

// LinkTapSessionParent 把辅助调用挂回主会话。
func LinkTapSessionParent(sessionId, parentId int64) error {
	return DB.Model(&TapSession{}).Where("id = ? AND parent_id = 0", sessionId).
		Update("parent_id", parentId).Error
}

// CandidateTapParents 辅助调用挂父时的扫描范围：同协议、同密钥、主会话、最近 10 分钟。
func CandidateTapParents(protocol string, keyId int, since int64, excludeId int64) ([]TapSession, error) {
	var sessions []TapSession
	err := DB.Where("COALESCE(parent_id, 0) = 0 AND id != ? AND protocol = ? AND key_id = ? AND updated_at >= ?",
		excludeId, protocol, keyId, since).
		Order("updated_at DESC").Limit(50).Find(&sessions).Error
	return sessions, err
}

type TapStats struct {
	Sessions         int64 `json:"sessions"`
	Turns            int64 `json:"turns"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
}

func GetTapStats() (TapStats, error) {
	var stats TapStats
	if err := DB.Model(&TapSession{}).Count(&stats.Sessions).Error; err != nil {
		return stats, err
	}
	if err := DB.Model(&TapTurn{}).Count(&stats.Turns).Error; err != nil {
		return stats, err
	}
	var sums []struct {
		PromptTokens     int64
		CompletionTokens int64
	}
	if err := DB.Model(&TapSession{}).
		Select("COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens").
		Scan(&sums).Error; err != nil {
		return stats, err
	}
	if len(sums) > 0 {
		stats.PromptTokens = sums[0].PromptTokens
		stats.CompletionTokens = sums[0].CompletionTokens
	}
	return stats, nil
}

type TapSessionListItem struct {
	TapSession
	ActualTurns int64 `json:"actual_turns"`
	AuxCount    int64 `json:"aux_count"`
}

type TapSessionQuery struct {
	Page       int
	PageSize   int
	Days       int
	Client     string
	Search     string
	IncludeAux bool
}

func ListTapSessions(query TapSessionQuery, now int64) ([]TapSessionListItem, int64, error) {
	q := DB.Model(&TapSession{})
	if !query.IncludeAux {
		q = q.Where("kind = ?", "main")
	}
	if query.Days > 0 {
		q = q.Where("updated_at >= ?", now-int64(query.Days)*86400)
	}
	if query.Client != "" {
		q = q.Where("client LIKE ?", "%"+query.Client+"%")
	}
	if query.Search != "" {
		like := "%" + query.Search + "%"
		q = q.Where("model LIKE ? OR fingerprint LIKE ? OR first_user LIKE ?", like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var sessions []TapSession
	if err := q.Order("updated_at DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).Find(&sessions).Error; err != nil {
		return nil, 0, err
	}
	if len(sessions) == 0 {
		return []TapSessionListItem{}, total, nil
	}
	ids := make([]int64, 0, len(sessions))
	for _, s := range sessions {
		ids = append(ids, s.Id)
	}
	// turns 必须实算：会话表的汇总列在库损坏恢复后可能与明细不一致，
	// 列表上显示「33 轮」却每轮都点不开，比没有数字更糟。
	turnCounts := map[int64]int64{}
	var turnRows []struct {
		SessionId int64
		N         int64
	}
	if err := DB.Model(&TapTurn{}).Select("session_id, COUNT(*) AS n").
		Where("session_id IN ?", ids).Group("session_id").Scan(&turnRows).Error; err != nil {
		return nil, 0, err
	}
	for _, row := range turnRows {
		turnCounts[row.SessionId] = row.N
	}
	auxCounts := map[int64]int64{}
	var auxRows []struct {
		ParentId int64
		N        int64
	}
	if err := DB.Model(&TapSession{}).Select("parent_id, COUNT(*) AS n").
		Where("parent_id IN ?", ids).Group("parent_id").Scan(&auxRows).Error; err != nil {
		return nil, 0, err
	}
	for _, row := range auxRows {
		auxCounts[row.ParentId] = row.N
	}
	items := make([]TapSessionListItem, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, TapSessionListItem{
			TapSession:  s,
			ActualTurns: turnCounts[s.Id],
			AuxCount:    auxCounts[s.Id],
		})
	}
	return items, total, nil
}

func GetTapSessionById(id int64) (*TapSession, error) {
	var session TapSession
	err := DB.First(&session, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

type TapTurnListItem struct {
	Id               int64   `json:"id"`
	SessionId        int64   `json:"session_id"`
	Seq              int     `json:"seq"`
	Ts               int64   `json:"ts"`
	Path             string  `json:"path"`
	Protocol         string  `json:"protocol"`
	Model            string  `json:"model"`
	MappedModel      string  `json:"mapped_model"`
	Status           int     `json:"status"`
	Stream           bool    `json:"stream"`
	LatencyMs        int     `json:"latency_ms"`
	FirstTokenMs     int     `json:"first_token_ms"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CacheHitTokens   int     `json:"cache_hit_tokens"`
	CacheMissTokens  int     `json:"cache_miss_tokens"`
	Cost             float64 `json:"cost"`
	TraceId          string  `json:"trace_id"`
	Error            string  `json:"error"`
	Truncated        bool    `json:"truncated"`
}

func ListTapTurnsBySession(sessionId int64) ([]TapTurnListItem, error) {
	var rows []TapTurnListItem
	err := DB.Model(&TapTurn{}).
		Select("id, session_id, seq, ts, path, protocol, model, mapped_model, status, stream, latency_ms, first_token_ms, prompt_tokens, completion_tokens, cache_hit_tokens, cache_miss_tokens, cost, trace_id, error, truncated").
		Where("session_id = ?", sessionId).Order("seq ASC").Scan(&rows).Error
	return rows, err
}

// ListTapTurnsFull 导出时一次取全正文；只在管理员主动导出时调用。
func ListTapTurnsFull(sessionId int64) ([]TapTurn, error) {
	var turns []TapTurn
	err := DB.Where("session_id = ?", sessionId).Order("seq ASC").Find(&turns).Error
	return turns, err
}

func ListTapAuxSessions(parentId int64) ([]TapSession, error) {
	var sessions []TapSession
	err := DB.Where("parent_id = ?", parentId).Order("id ASC").Find(&sessions).Error
	return sessions, err
}

func GetTapTurnById(id int64) (*TapTurn, error) {
	var turn TapTurn
	err := DB.First(&turn, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &turn, nil
}

// ForEachTapSession 分批遍历会话，供按入库 UA 重算归类（backfill）使用。
func ForEachTapSession(batch int, fn func(*TapSession) error) error {
	var sessions []TapSession
	return DB.FindInBatches(&sessions, batch, func(_ *gorm.DB, _ int) error {
		for i := range sessions {
			if err := fn(&sessions[i]); err != nil {
				return err
			}
		}
		return nil
	}).Error
}

func UpdateTapSessionClient(id int64, client string) error {
	return DB.Model(&TapSession{}).Where("id = ?", id).Update("client", client).Error
}

// DeleteTapSession 删一条会话连同它的轮次。
func DeleteTapSession(id int64) (bool, error) {
	result := DB.Where("id = ?", id).Delete(&TapSession{})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	if err := DB.Where("session_id = ?", id).Delete(&TapTurn{}).Error; err != nil {
		return true, err
	}
	return true, nil
}

func ClearTapData() error {
	if err := DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&TapTurn{}).Error; err != nil {
		return err
	}
	return DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&TapSession{}).Error
}

// PruneTapData 按保留期与会话数上限清理，幂等。
func PruneTapData(now int64, retentionDays, maxSessions int) (int64, error) {
	var deleted int64
	cutoff := now - int64(retentionDays)*86400
	result := DB.Where("ts < ?", cutoff).Delete(&TapTurn{})
	if result.Error != nil {
		return deleted, result.Error
	}
	deleted += result.RowsAffected

	var ids []int64
	if err := DB.Model(&TapSession{}).Order("updated_at DESC").
		Offset(maxSessions).Limit(1000).Pluck("id", &ids).Error; err != nil {
		return deleted, err
	}
	for len(ids) > 0 {
		result = DB.Where("session_id IN ?", ids).Delete(&TapTurn{})
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
		if err := DB.Where("id IN ?", ids).Delete(&TapSession{}).Error; err != nil {
			return deleted, err
		}
		ids = ids[:0]
		if err := DB.Model(&TapSession{}).Order("updated_at DESC").
			Offset(maxSessions).Limit(1000).Pluck("id", &ids).Error; err != nil {
			return deleted, err
		}
	}
	// 空壳会话在列表里是一行什么都点不开的条目。
	if err := DB.Exec("DELETE FROM tap_sessions WHERE id NOT IN (SELECT session_id FROM tap_turns)").Error; err != nil {
		return deleted, err
	}
	return deleted, nil
}
