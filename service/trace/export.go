package trace

import (
	"bytes"
	"html/template"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type exportTurn struct {
	Seq           int
	Time          string
	Status        int
	Stream        bool
	LatencyMs     int
	FirstTokenMs  int
	Model         string
	MappedModel   string
	PromptTokens  int
	Completion    int
	CacheHit      int
	CacheMiss     int
	Error         string
	Truncated     bool
	ClientRequest string
	UpstreamBody  string
	UpstreamResp  string
	ClientResp    string
	Recon         string
}

type exportView struct {
	ID           int64
	Client       string
	ClientUa     string
	Protocol     string
	Model        string
	KeyName      string
	FirstUser    string
	StartedAt    string
	UpdatedAt    string
	PromptTokens int
	Completion   int
	Turns        []exportTurn
	GeneratedAt  string
}

const exportTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>调用抓包 #{{.ID}}</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif;margin:0;padding:24px;color:#1f2328;background:#fff;line-height:1.5}
h1{font-size:20px;margin:0 0 4px}
h2{font-size:16px;margin:24px 0 8px;border-bottom:1px solid #d0d7de;padding-bottom:4px}
h3{font-size:14px;margin:16px 0 4px}
.meta{color:#57606a;font-size:13px;margin-bottom:16px}
.kv{display:grid;grid-template-columns:120px 1fr;gap:4px 12px;font-size:13px;margin-bottom:16px}
.kv dt{color:#57606a}
.kv dd{margin:0;word-break:break-all}
.badge{display:inline-block;padding:1px 6px;border-radius:4px;background:#eaeef2;font-size:12px;margin-right:6px}
.badge.warn{background:#fff8c5}
pre{background:#f6f8fa;border:1px solid #d0d7de;border-radius:6px;padding:10px;font-size:12px;overflow:auto;white-space:pre-wrap;word-break:break-word;max-height:600px}
.turn{margin-bottom:24px}
.note{color:#9a6700;font-size:12px}
@media (prefers-color-scheme: dark){body{background:#0d1117;color:#e6edf3}.meta,.kv dt{color:#8b949e}pre{background:#161b22;border-color:#30363d}h2{border-color:#30363d}.badge{background:#21262d}}
</style>
</head>
<body>
<h1>调用抓包 #{{.ID}}</h1>
<div class="meta">导出时间 {{.GeneratedAt}} · 自包含静态文件，无脚本无外链</div>
<dl class="kv">
<dt>客户端</dt><dd>{{.Client}}</dd>
<dt>User-Agent</dt><dd>{{.ClientUa}}</dd>
<dt>协议</dt><dd>{{.Protocol}}</dd>
<dt>模型</dt><dd>{{.Model}}</dd>
<dt>密钥</dt><dd>{{.KeyName}}</dd>
<dt>开始时间</dt><dd>{{.StartedAt}}</dd>
<dt>最后更新</dt><dd>{{.UpdatedAt}}</dd>
<dt>Token</dt><dd>输入 {{.PromptTokens}} / 输出 {{.Completion}}</dd>
<dt>首条消息</dt><dd>{{.FirstUser}}</dd>
</dl>
{{range .Turns}}
<div class="turn">
<h2>第 {{.Seq}} 轮 · {{.Time}}</h2>
<div>
<span class="badge">HTTP {{.Status}}</span>
{{if .Stream}}<span class="badge">流式</span>{{end}}
<span class="badge">延迟 {{.LatencyMs}}ms</span>
{{if .FirstTokenMs}}<span class="badge">首字 {{.FirstTokenMs}}ms</span>{{end}}
<span class="badge">{{.Model}}</span>
{{if .MappedModel}}<span class="badge">映射 {{.MappedModel}}</span>{{end}}
<span class="badge">入 {{.PromptTokens}} / 出 {{.Completion}}</span>
{{if .CacheHit}}<span class="badge">缓存命中 {{.CacheHit}}</span>{{end}}
{{if .CacheMiss}}<span class="badge">缓存未命中 {{.CacheMiss}}</span>{{end}}
</div>
{{if .Truncated}}<p class="note">本轮已截断，原文不完整。</p>{{end}}
{{if .Error}}<p class="note">错误：{{.Error}}</p>{{end}}
<h3>客户端请求</h3><pre>{{.ClientRequest}}</pre>
{{if .UpstreamBody}}<h3>网关发往上游的请求</h3><pre>{{.UpstreamBody}}</pre>{{end}}
<h3>上游响应（原文）</h3><pre>{{.UpstreamResp}}</pre>
{{if .Recon}}<h3>模型输出（重建）</h3><pre>{{.Recon}}</pre>{{end}}
{{if .ClientResp}}<h3>客户端收到的响应</h3><pre>{{.ClientResp}}</pre>{{end}}
</div>
{{end}}
</body>
</html>
`

var exportTpl = template.Must(template.New("export").Parse(exportTemplate))

// BuildExport 生成自包含导出 HTML。所有外部文本都经 html/template 转义。
func BuildExport(session *model.TapSession, turns []model.TapTurn) (string, error) {
	view := exportView{
		ID:           session.Id,
		Client:       session.Client,
		ClientUa:     session.ClientUa,
		Protocol:     session.Protocol,
		Model:        session.Model,
		KeyName:      session.KeyName,
		FirstUser:    session.FirstUser,
		StartedAt:    formatTime(session.StartedAt),
		UpdatedAt:    formatTime(session.UpdatedAt),
		PromptTokens: session.PromptTokens,
		Completion:   session.CompletionTokens,
		GeneratedAt:  time.Now().Format("2006-01-02 15:04:05"),
	}
	for _, turn := range turns {
		view.Turns = append(view.Turns, exportTurn{
			Seq:           turn.Seq,
			Time:          formatTime(turn.Ts),
			Status:        turn.Status,
			Stream:        turn.Stream,
			LatencyMs:     turn.LatencyMs,
			FirstTokenMs:  turn.FirstTokenMs,
			Model:         turn.Model,
			MappedModel:   turn.MappedModel,
			PromptTokens:  turn.PromptTokens,
			Completion:    turn.CompletionTokens,
			CacheHit:      turn.CacheHitTokens,
			CacheMiss:     turn.CacheMissTokens,
			Error:         turn.Error,
			Truncated:     turn.Truncated,
			ClientRequest: turn.RequestJson,
			UpstreamBody:  upstreamBodyOf(turn.RequestJson),
			UpstreamResp:  turn.ResponseText,
			ClientResp:    turn.ClientResponseText,
			Recon:         turn.ReconJson,
		})
	}
	var buf bytes.Buffer
	if err := exportTpl.Execute(&buf, view); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func formatTime(ts int64) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

func upstreamBodyOf(requestJSON string) string {
	var doc map[string]any
	if err := common.Unmarshal([]byte(requestJSON), &doc); err != nil {
		return ""
	}
	body, ok := doc["upstream_body"]
	if !ok {
		return ""
	}
	raw, err := common.Marshal(body)
	if err != nil {
		return ""
	}
	return string(raw)
}
