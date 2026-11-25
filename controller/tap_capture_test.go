package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/trace"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 抓包链路端到端：一次真实转发（含上游原始 SSE、客户端报文）落库后，
// 管理接口返回的字段必须与前端 /traces 页面读取的一致。
func TestTapCaptureRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDB := model.DB
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, database.AutoMigrate(&model.TapSession{}, &model.TapTurn{}))
	model.DB = database

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	previousOption, hadOption := common.OptionMap[model.TapOptionEnabled]
	common.OptionMap[model.TapOptionEnabled] = "true"
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		model.DB = previousDB
		common.OptionMapRWMutex.Lock()
		if hadOption {
			common.OptionMap[model.TapOptionEnabled] = previousOption
		} else {
			delete(common.OptionMap, model.TapOptionEnabled)
		}
		common.OptionMapRWMutex.Unlock()
		require.NoError(t, sqlDB.Close())
	})

	requestBody := `{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello there"}]}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":3}}\n\n"))
	}))
	defer upstream.Close()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
	ctx.Request.Header.Set("Authorization", "Bearer sk-client-secret-123")
	ctx.Request.Header.Set("User-Agent", "claude-cli/1.2.3")
	ctx.Set("token_id", 7)
	ctx.Set("token_name", "demo-key")

	rec := trace.Start(ctx, "chat", 7, "demo-key")
	require.NotNil(t, rec)

	outReq, _ := http.NewRequest(http.MethodPost, upstream.URL+"/v1/chat/completions", strings.NewReader(requestBody))
	outReq.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(requestBody)), nil
	}
	outReq.Header.Set("Content-Type", "application/json")
	rec.AttachUpstreamRequest(outReq)

	resp, err := http.DefaultClient.Do(outReq)
	require.NoError(t, err)
	rec.AttachUpstreamResponse(resp)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()

	_, _ = ctx.Writer.Write(raw)
	rec.Finish(ctx, "")

	var turn model.TapTurn
	require.Eventually(t, func() bool {
		return database.Order("id ASC").First(&turn).Error == nil
	}, 3*time.Second, 20*time.Millisecond)

	assert.Equal(t, 200, turn.Status)
	assert.Equal(t, "gpt-test", turn.Model)
	assert.True(t, turn.Stream)
	assert.Equal(t, 11, turn.PromptTokens)
	assert.Equal(t, 3, turn.CompletionTokens)
	assert.Contains(t, turn.RequestJson, "sk-cli***")
	assert.NotContains(t, turn.RequestJson, "sk-client-secret-123")
	assert.Contains(t, turn.ResponseText, "data:")

	var session model.TapSession
	require.NoError(t, database.Order("id ASC").First(&session).Error)
	assert.Equal(t, "claude", session.Client)
	assert.Equal(t, "hello there", session.FirstUser)
	assert.Equal(t, 1, session.Turns)

	// 列表与详情接口的字段名就是前端读的那一份，改名会立刻在页面上显示成空白。
	listRec := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRec)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/tap/sessions?p=1&page_size=10", nil)
	ListTapSessions(listCtx)
	assert.Contains(t, listRec.Body.String(), `"actual_turns":1`)
	assert.Contains(t, listRec.Body.String(), `"client":"claude"`)
	assert.NotContains(t, listRec.Body.String(), "sk-client-secret-123")

	detailRec := httptest.NewRecorder()
	detailCtx, _ := gin.CreateTestContext(detailRec)
	detailCtx.Request = httptest.NewRequest(http.MethodGet, "/api/tap/sessions/1", nil)
	detailCtx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(session.Id, 10)}}
	GetTapSession(detailCtx)
	assert.Contains(t, detailRec.Body.String(), `"turns":[`)

	turnRec := httptest.NewRecorder()
	turnCtx, _ := gin.CreateTestContext(turnRec)
	turnCtx.Request = httptest.NewRequest(http.MethodGet, "/api/tap/turns/1", nil)
	turnCtx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(turn.Id, 10)}}
	GetTapTurn(turnCtx)
	assert.Contains(t, turnRec.Body.String(), `"response_text":`)
	assert.Contains(t, turnRec.Body.String(), `"describe":`)
}
