package trace

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	// 上游原始响应的累积上限：异常流（没有换行、死循环重发）不能先把面板内存
	// 吃掉；超限只停止累积，不打断转发。
	rawStreamLimit = 4 * 1024 * 1024
	// 请求体快照上限，防止单条请求把内存与库撑爆。
	captureBodyLimit = 8 * 1024 * 1024
	pruneEvery       = 32
	sessionIdleSecs  = 12 * 3600
)

const recorderContextKey = "tap_trace_recorder"

type rawBuffer struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	full bool
}

func (b *rawBuffer) feed(piece []byte) {
	if len(piece) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.full {
		return
	}
	remain := rawStreamLimit - b.buf.Len()
	if remain <= 0 {
		b.full = true
		return
	}
	if len(piece) > remain {
		b.buf.Write(piece[:remain])
		b.full = true
		return
	}
	b.buf.Write(piece)
}

func (b *rawBuffer) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
	b.full = false
}

func (b *rawBuffer) text() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type Recorder struct {
	traceID  string
	protocol string
	path     string
	ua       string
	keyID    int
	keyName  string
	started  time.Time

	clientHeaders http.Header
	clientBody    []byte

	mu               sync.Mutex
	upstreamHeaders  http.Header
	upstreamBody     []byte
	upstreamStatus   int
	firstClientWrite time.Time

	upstreamRaw *rawBuffer
	clientRaw   *rawBuffer
}

// Start 在转发路径的最前面创建记录器。关闭时返回 nil，不做任何拷贝。
func Start(c *gin.Context, protocol string, keyID int, keyName string) *Recorder {
	if !Enabled() {
		return nil
	}
	rec := &Recorder{
		traceID:       common.GetRandomString(16),
		protocol:      protocol,
		path:          c.Request.URL.Path,
		ua:            c.Request.UserAgent(),
		keyID:         keyID,
		keyName:       CleanExternal(keyName, 128),
		started:       time.Now(),
		clientHeaders: c.Request.Header.Clone(),
		upstreamRaw:   &rawBuffer{},
		clientRaw:     &rawBuffer{},
	}
	if storage, err := common.GetBodyStorage(c); err == nil {
		if data, err := storage.Bytes(); err == nil {
			if len(data) > captureBodyLimit {
				data = data[:captureBodyLimit]
			}
			rec.clientBody = data
		}
	}
	c.Set(recorderContextKey, rec)
	c.Writer = &teeWriter{ResponseWriter: c.Writer, rec: rec}
	return rec
}

func FromGin(c *gin.Context) *Recorder {
	value, ok := c.Get(recorderContextKey)
	if !ok {
		return nil
	}
	rec, _ := value.(*Recorder)
	return rec
}

// AttachUpstreamRequest 在出站请求最终定稿后调用：补 X-Trace-ID，留一份完整
// 出站头与改写后的请求体。重试时以最后一次尝试为准。
func (r *Recorder) AttachUpstreamRequest(req *http.Request) {
	if r == nil || req == nil {
		return
	}
	if req.Header.Get("X-Trace-ID") == "" {
		req.Header.Set("X-Trace-ID", r.traceID)
	}
	var bodyCopy []byte
	if req.GetBody != nil {
		if reader, err := req.GetBody(); err == nil {
			if data, err := io.ReadAll(io.LimitReader(reader, captureBodyLimit)); err == nil {
				bodyCopy = data
			}
			_ = reader.Close()
		}
	}
	r.mu.Lock()
	r.upstreamHeaders = req.Header.Clone()
	r.upstreamBody = bodyCopy
	r.upstreamStatus = 0
	r.mu.Unlock()
	r.upstreamRaw.reset()
}

// AttachUpstreamResponse 记下状态码并旁路累积上游原文。
func (r *Recorder) AttachUpstreamResponse(resp *http.Response) {
	if r == nil || resp == nil {
		return
	}
	r.mu.Lock()
	r.upstreamStatus = resp.StatusCode
	r.mu.Unlock()
	if resp.Body != nil {
		resp.Body = &teeReadCloser{ReadCloser: resp.Body, rec: r}
	}
}

type teeReadCloser struct {
	io.ReadCloser
	rec *Recorder
}

func (t *teeReadCloser) Read(p []byte) (int, error) {
	n, err := t.ReadCloser.Read(p)
	if n > 0 {
		t.rec.upstreamRaw.feed(p[:n])
	}
	return n, err
}

type teeWriter struct {
	gin.ResponseWriter
	rec *Recorder
}

func (w *teeWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if n > 0 {
		w.rec.feedClient(p[:n])
	}
	return n, err
}

func (w *teeWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	if n > 0 {
		w.rec.feedClient([]byte(s[:n]))
	}
	return n, err
}

func (r *Recorder) feedClient(piece []byte) {
	if hasSSEPayload(piece) {
		r.mu.Lock()
		if r.firstClientWrite.IsZero() {
			r.firstClientWrite = time.Now()
		}
		r.mu.Unlock()
	}
	r.clientRaw.feed(piece)
}

// hasSSEPayload 跳过保活注释。网关自己的 ping 也经过同一个 writer，若把它
// 当成首字，首字延迟会永远是保活间隔。
func hasSSEPayload(piece []byte) bool {
	for _, line := range bytes.Split(piece, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 || trimmed[0] == ':' {
			continue
		}
		return true
	}
	return false
}

// stripSSEComments 入库的客户端报文去掉保活注释，否则详情页全是 `: ping`。
func stripSSEComments(raw string) string {
	if !strings.Contains(raw, "data:") {
		return raw
	}
	lines := strings.Split(raw, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); len(trimmed) > 0 && trimmed[0] == ':' {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// Finish 在响应已发给客户端之后调用，只做内存整理，落库交给后台 worker。
func (r *Recorder) Finish(c *gin.Context, errText string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	status := r.upstreamStatus
	upstreamHeaders := r.upstreamHeaders
	upstreamBody := r.upstreamBody
	firstWrite := r.firstClientWrite
	r.mu.Unlock()
	if status == 0 {
		status = c.Writer.Status()
	}
	if status == 0 {
		status = http.StatusOK
	}
	finished := time.Now()
	latencyMs := int(finished.Sub(r.started).Milliseconds())
	firstTokenMs := 0
	if !firstWrite.IsZero() {
		firstTokenMs = int(firstWrite.Sub(r.started).Milliseconds())
	}
	submit(&pendingTurn{
		rec:             r,
		now:             finished.Unix(),
		status:          status,
		latencyMs:       latencyMs,
		firstTokenMs:    firstTokenMs,
		errorText:       errText,
		upstreamHeaders: upstreamHeaders,
		upstreamBody:    upstreamBody,
		upstreamRaw:     r.upstreamRaw.text(),
		clientRaw:       r.clientRaw.text(),
	})
}

var (
	writeQueue   = make(chan *pendingTurn, 16)
	workerOnce   sync.Once
	pruneCounter atomic.Int64
)

func submit(p *pendingTurn) {
	workerOnce.Do(func() { go traceWorker() })
	select {
	case writeQueue <- p:
	default:
		// 队列满时宁可丢一条证据，也不能让请求收尾卡在抓包上。
	}
}

func traceWorker() {
	for p := range writeQueue {
		func() {
			defer func() {
				_ = recover() // 抓包是旁路，任何异常都不能影响主流程
			}()
			p.persist()
		}()
	}
}

type pendingTurn struct {
	rec             *Recorder
	now             int64
	status          int
	latencyMs       int
	firstTokenMs    int
	errorText       string
	upstreamHeaders http.Header
	upstreamBody    []byte
	upstreamRaw     string
	clientRaw       string
}

func (p *pendingTurn) persist() {
	cfg := LoadConfig()
	rec := p.rec

	clientBody := ParseObject(rec.clientBody)
	upstreamBody := ParseObject(p.upstreamBody)
	viewBody := upstreamBody
	if viewBody == nil {
		viewBody = clientBody
	}

	fingerprint := Fingerprint(viewBody, rec.protocol)
	clientFamily := ClientFamily(rec.ua)
	kind := AuxKind(viewBody)
	firstUser := bodyFirstUserText(viewBody)

	session, err := findOrCreateSession(rec, fingerprint, clientFamily, kind, firstUser, p.now)
	if err != nil {
		return
	}

	requestDoc := map[string]any{
		"headers": RedactHeaders(rec.clientHeaders),
		"body":    bodyOf(clientBody, rec.clientBody),
	}
	if len(p.upstreamHeaders) > 0 {
		requestDoc["upstream_headers"] = RedactHeaders(p.upstreamHeaders)
	}
	if upstreamBody != nil {
		requestDoc["upstream_body"] = upstreamBody
	}
	requestRaw, err := common.Marshal(requestDoc)
	if err != nil {
		return
	}
	requestText, cutRequest := Clip(Scrub(string(requestRaw)), cfg.MaxBody)

	responseText, cutResponse := Clip(Scrub(p.upstreamRaw), cfg.MaxBody)

	clientText := ""
	cutClient := false
	if p.clientRaw != "" && p.clientRaw != p.upstreamRaw {
		clientText, cutClient = Clip(Scrub(stripSSEComments(p.clientRaw)), cfg.MaxBody)
	}

	recon := Reconstruct(p.upstreamRaw, rec.protocol)
	reconRaw, err := common.Marshal(recon)
	if err != nil {
		reconRaw = []byte("{}")
	}
	reconText, cutRecon := Clip(string(reconRaw), cfg.MaxBody)

	usage := ParseUsage(p.upstreamRaw, rec.protocol)
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && clientText != "" {
		alt := ParseUsage(p.clientRaw, rec.protocol)
		if alt.PromptTokens > 0 || alt.CompletionTokens > 0 {
			usage = alt
		}
	}
	usageRaw, _ := common.Marshal(usage)

	digestRaw, err := common.Marshal(RequestDigest(viewBody))
	if err != nil {
		digestRaw = []byte("{}")
	}

	modelName := stringOf(viewBody["model"])
	mappedModel := ""
	if upstreamBody != nil {
		mappedModel = stringOf(upstreamBody["model"])
	}

	turn := &model.TapTurn{
		SessionId:          session.Id,
		Seq:                session.Turns + 1,
		Ts:                 p.now,
		Path:               CleanExternal(rec.path, 128),
		Protocol:           rec.protocol,
		Model:              CleanExternal(modelName, 128),
		MappedModel:        CleanExternal(mappedModel, 128),
		Status:             p.status,
		Stream:             looksLikeSSE(p.upstreamRaw),
		LatencyMs:          p.latencyMs,
		FirstTokenMs:       p.firstTokenMs,
		PromptTokens:       usage.PromptTokens,
		CompletionTokens:   usage.CompletionTokens,
		CacheHitTokens:     usage.CacheHitTokens,
		CacheMissTokens:    usage.CacheMissTokens,
		RequestJson:        requestText,
		RequestDigest:      string(digestRaw),
		ResponseText:       responseText,
		ClientResponseText: clientText,
		TraceId:            rec.traceID,
		ReconJson:          reconText,
		UsageJson:          clipString(string(usageRaw), 8192),
		Error:              CleanExternal(p.errorText, 500),
		Truncated:          cutRequest || cutResponse || cutRecon || cutClient,
	}
	if err := model.CreateTapTurn(turn); err != nil {
		return
	}
	_ = model.UpdateTapSessionTurn(session.Id, p.now, session.Turns+1, usage.PromptTokens, usage.CompletionTokens, CleanExternal(modelName, 128))

	if pruneCounter.Add(1)%pruneEvery == 0 {
		_, _ = model.PruneTapData(p.now, cfg.RetentionDays, cfg.MaxSessions)
	}
}

func bodyOf(parsed map[string]any, raw []byte) any {
	if parsed != nil {
		return parsed
	}
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

func findOrCreateSession(rec *Recorder, fingerprint, clientFamily, kind, firstUser string, now int64) (*model.TapSession, error) {
	session, err := model.FindTapSession(fingerprint, rec.keyID, rec.protocol, now-sessionIdleSecs)
	if err != nil {
		return nil, err
	}
	if session != nil {
		_ = model.FillTapSessionFirstTurn(session.Id, CleanExternal(rec.ua, 512), CleanExternal(firstUser, 2000))
		return session, nil
	}
	session = &model.TapSession{
		Fingerprint: fingerprint,
		KeyId:       rec.keyID,
		KeyName:     rec.keyName,
		Client:      clientFamily,
		ClientUa:    CleanExternal(rec.ua, 512),
		FirstUser:   CleanExternal(firstUser, 2000),
		Kind:        kind,
		Protocol:    rec.protocol,
		Model:       CleanExternal(stringOf(ParseObject(rec.clientBody)["model"]), 128),
		StartedAt:   now,
		UpdatedAt:   now,
	}
	if err := model.CreateTapSession(session); err != nil {
		return nil, err
	}
	if kind != "main" {
		linkAuxParent(session, firstUser, rec, now)
	}
	return session, nil
}

// linkAuxParent 标题/压缩请求把人类消息原文塞进自己的 user 内容里，所以主会话
// 的首条 user 消息必然是这段文本的子串。判据是内容包含，不是时间窗。
func linkAuxParent(session *model.TapSession, firstUser string, rec *Recorder, now int64) {
	candidates, err := model.CandidateTapParents(rec.protocol, rec.keyID, now-600, session.Id)
	if err != nil {
		return
	}
	if parent := matchParent(firstUser, candidates); parent > 0 {
		_ = model.LinkTapSessionParent(session.Id, parent)
	}
}

var auxPrefixes = []int{120, 60, 30, 16, 8, 6}

func matchParent(firstUser string, candidates []model.TapSession) int64 {
	needle := NormText(firstUser)
	if len(needle) < 6 {
		return 0
	}
	for _, candidate := range candidates {
		haystack := NormText(candidate.FirstUser)
		for _, size := range auxPrefixes {
			if len(haystack) < size {
				continue
			}
			if bytes.Contains([]byte(needle), []byte(haystack[:size])) {
				return candidate.Id
			}
		}
	}
	return 0
}
