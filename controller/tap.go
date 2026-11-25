package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/trace"

	"github.com/gin-gonic/gin"
)

func auditTap(c *gin.Context, action, content string) {
	model.RecordAuditLog(c, model.AuditLog{
		UserId:    c.GetInt("id"),
		Username:  c.GetString("username"),
		ActorRole: c.GetInt("role"),
		Category:  model.AuditCategoryOperation,
		Action:    action,
		Content:   content,
		Success:   true,
	})
}

// GetTapConfig 任何登录用户可读：只有开关、容量与计数，不含对话内容。
func GetTapConfig(c *gin.Context) {
	stats, err := model.GetTapStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"config": trace.LoadConfig(),
		"stats":  stats,
	})
}

func UpdateTapConfig(c *gin.Context) {
	var req trace.Config
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的配置"})
		return
	}
	cfg := trace.NormalizeConfig(req)
	values := map[string]string{
		model.TapOptionEnabled:       strconv.FormatBool(cfg.Enabled),
		model.TapOptionMaxBody:       strconv.Itoa(cfg.MaxBody),
		model.TapOptionRetentionDays: strconv.Itoa(cfg.RetentionDays),
		model.TapOptionMaxSessions:   strconv.Itoa(cfg.MaxSessions),
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		common.ApiError(c, err)
		return
	}
	auditTap(c, "tap.config.update", "enabled="+strconv.FormatBool(cfg.Enabled)+
		" max_body="+strconv.Itoa(cfg.MaxBody)+
		" retention_days="+strconv.Itoa(cfg.RetentionDays)+
		" max_sessions="+strconv.Itoa(cfg.MaxSessions))
	common.ApiSuccess(c, cfg)
}

func BackfillTap(c *gin.Context) {
	updated, err := trace.Backfill()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	auditTap(c, "tap.backfill", "updated="+strconv.Itoa(updated))
	common.ApiSuccess(c, gin.H{"updated": updated})
}

func PruneTap(c *gin.Context) {
	cfg := trace.LoadConfig()
	deleted, err := model.PruneTapData(common.GetTimestamp(), cfg.RetentionDays, cfg.MaxSessions)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	auditTap(c, "tap.prune", "deleted="+strconv.FormatInt(deleted, 10))
	common.ApiSuccess(c, gin.H{"deleted": deleted})
}

func ListTapSessions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	days, _ := strconv.Atoi(c.Query("days"))
	query := model.TapSessionQuery{
		Page:       pageInfo.GetPage(),
		PageSize:   pageInfo.GetPageSize(),
		Days:       days,
		Client:     c.Query("client"),
		Search:     c.Query("q"),
		IncludeAux: c.Query("include_aux") == "true",
	}
	items, total, err := model.ListTapSessions(query, common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// GetTapSession 会话头 + 每轮摘要（不含正文），另附挂载的辅助调用列表。
func GetTapSession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	session, err := model.GetTapSessionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if session == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "会话不存在"})
		return
	}
	turns, err := model.ListTapTurnsBySession(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	aux, err := model.ListTapAuxSessions(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"session": session, "turns": turns, "aux": aux})
}

func DeleteTapSession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	found, err := model.DeleteTapSession(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !found {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "会话不存在"})
		return
	}
	auditTap(c, "tap.session.delete", "session_id="+strconv.FormatInt(id, 10))
	common.ApiSuccess(c, nil)
}

func ClearTap(c *gin.Context) {
	if err := model.ClearTapData(); err != nil {
		common.ApiError(c, err)
		return
	}
	auditTap(c, "tap.clear", "")
	common.ApiSuccess(c, nil)
}

// GetTapTurn 单轮完整内容 + 结构摘要。
func GetTapTurn(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	turn, err := model.GetTapTurnById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if turn == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "轮次不存在"})
		return
	}
	body := turnBodyOf(turn)
	common.ApiSuccess(c, gin.H{
		"turn":         turn.Detail(),
		"describe":     trace.Describe(body),
		"client_recon": trace.ReconstructClient(turn.ClientResponseText, turn.Protocol),
	})
}

func DiffTapTurns(c *gin.Context) {
	beforeID, _ := strconv.ParseInt(c.Query("before"), 10, 64)
	afterID, _ := strconv.ParseInt(c.Query("after"), 10, 64)
	if beforeID == 0 || afterID == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "before 和 after 必填"})
		return
	}
	before, err := model.GetTapTurnById(beforeID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	after, err := model.GetTapTurnById(afterID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if before == nil || after == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "轮次不存在"})
		return
	}
	common.ApiSuccess(c, trace.BuildDiff(before, after))
}

func ExportTapSession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	session, err := model.GetTapSessionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if session == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "会话不存在"})
		return
	}
	turns, err := model.ListTapTurnsFull(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	html, err := trace.BuildExport(session, turns)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	auditTap(c, "tap.session.export", "session_id="+strconv.FormatInt(id, 10))
	c.Header("Content-Disposition", `attachment; filename="tap-trace-`+strconv.FormatInt(id, 10)+`.html"`)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func turnBodyOf(turn *model.TapTurn) map[string]any {
	var doc map[string]any
	if err := common.Unmarshal([]byte(turn.RequestJson), &doc); err != nil {
		return nil
	}
	if body, ok := doc["upstream_body"].(map[string]any); ok {
		return body
	}
	body, _ := doc["body"].(map[string]any)
	return body
}
