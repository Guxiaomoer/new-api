package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/community_checkin_bot_setting"
	"github.com/QuantumNous/new-api/setting/community_sync_setting"
	"gorm.io/gorm"
)

const (
	communityCheckinTimelineEndpoint = "https://dc.hhhl.cc/api/chat/messages/room-timeline"
	communityCheckinSendEndpoint     = "https://dc.hhhl.cc/api/chat/messages/create-to-room"
	communityCheckinPageLimit        = 30
)

type CommunityCheckinBotStatus struct {
	Enabled              bool   `json:"enabled"`
	RoomID               string `json:"room_id"`
	BotUserID            string `json:"bot_user_id"`
	BotName              string `json:"bot_name"`
	IntervalSeconds      int    `json:"interval_seconds"`
	MinUSD               int    `json:"min_usd"`
	MaxUSD               int    `json:"max_usd"`
	LastMessageID        string `json:"last_message_id"`
	AuthorizationSet     bool   `json:"authorization_set"`
	FingerprintSet       bool   `json:"fingerprint_set"`
	LastRunAt            int64  `json:"last_run_at"`
	LastProcessedCount   int    `json:"last_processed_count"`
	LastTriggeredCount   int    `json:"last_triggered_count"`
	LastRewardedCount    int    `json:"last_rewarded_count"`
	LastError            string `json:"last_error"`
}

type CommunityCheckinBotRunResult struct {
	ProcessedCount int    `json:"processed_count"`
	TriggeredCount int    `json:"triggered_count"`
	RewardedCount  int    `json:"rewarded_count"`
	LastMessageID  string `json:"last_message_id"`
	Error          string `json:"error,omitempty"`
}

type communityCheckinMessage struct {
	ID               string   `json:"id"`
	Text             string   `json:"text"`
	UserID           string   `json:"userId"`
	FromUserID       string   `json:"fromUserId"`
	MentionedUserIDs []string `json:"mentionedUserIds"`
	User             struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"user"`
	FromUser struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"fromUser"`
}

type communityCheckinTimelineRequest struct {
	RoomID  string `json:"roomId"`
	Limit   int    `json:"limit"`
	SinceID string `json:"sinceId,omitempty"`
}

type communityCheckinSendRequest struct {
	ToRoomID string `json:"toRoomId"`
	Text     string `json:"text"`
}

var (
	communityCheckinLoopOnce sync.Once
	communityCheckinStateMu  sync.RWMutex
	communityCheckinState    CommunityCheckinBotStatus
)

func GetCommunityCheckinBotStatus() CommunityCheckinBotStatus {
	botCfg := community_checkin_bot_setting.Get()
	syncCfg := community_sync_setting.Get()

	communityCheckinStateMu.RLock()
	state := communityCheckinState
	communityCheckinStateMu.RUnlock()

	state.Enabled = botCfg.Enabled
	state.RoomID = syncCfg.RoomID
	state.BotUserID = botCfg.BotUserID
	state.BotName = botCfg.BotName
	state.IntervalSeconds = botCfg.IntervalSeconds
	state.MinUSD = botCfg.MinUSD
	state.MaxUSD = botCfg.MaxUSD
	state.LastMessageID = botCfg.LastMessageID
	state.AuthorizationSet = strings.TrimSpace(syncCfg.Authorization) != ""
	state.FingerprintSet = strings.TrimSpace(syncCfg.Fingerprint) != ""
	return state
}

func RunCommunityCheckinBotOnce(ctx context.Context) (*CommunityCheckinBotRunResult, error) {
	botCfg := community_checkin_bot_setting.Get()
	syncCfg := community_sync_setting.Get()
	result, err := runCommunityCheckinBotOnce(ctx, botCfg, syncCfg)
	updateCommunityCheckinState(result, err)
	return result, err
}

func runCommunityCheckinBotOnce(ctx context.Context, botCfg community_checkin_bot_setting.CommunityCheckinBotSetting, syncCfg community_sync_setting.CommunitySyncSetting) (*CommunityCheckinBotRunResult, error) {
	result := &CommunityCheckinBotRunResult{LastMessageID: botCfg.LastMessageID}
	if strings.TrimSpace(syncCfg.RoomID) == "" {
		return result, errors.New("社区群 roomId 不能为空")
	}
	if strings.TrimSpace(syncCfg.Authorization) == "" {
		return result, errors.New("社区群接口 Authorization token 不能为空")
	}
	if strings.TrimSpace(botCfg.BotUserID) == "" {
		return result, errors.New("机器人用户 ID 不能为空")
	}

	messages, err := fetchCommunityCheckinMessages(ctx, syncCfg, botCfg.LastMessageID)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(botCfg.LastMessageID) == "" {
		for _, message := range messages {
			if strings.TrimSpace(message.ID) != "" {
				result.LastMessageID = message.ID
				if err := model.UpdateOption("community_checkin_bot.last_message_id", message.ID); err != nil {
					return result, err
				}
				break
			}
		}
		return result, nil
	}

	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if strings.TrimSpace(message.ID) == "" {
			continue
		}
		result.ProcessedCount++
		result.LastMessageID = message.ID

		if isCommunityCheckinTrigger(message, botCfg) {
			result.TriggeredCount++
			if processErr := processCommunityCheckinMessage(ctx, syncCfg, botCfg, message, result); processErr != nil {
				common.SysLog("community checkin bot: " + processErr.Error())
			}
		}

		if err := model.UpdateOption("community_checkin_bot.last_message_id", message.ID); err != nil {
			return result, err
		}
	}

	return result, nil
}

func processCommunityCheckinMessage(ctx context.Context, syncCfg community_sync_setting.CommunitySyncSetting, botCfg community_checkin_bot_setting.CommunityCheckinBotSetting, message communityCheckinMessage, result *CommunityCheckinBotRunResult) error {
	username := senderUsername(message)
	if username == "" {
		return sendCommunityCheckinReply(ctx, syncCfg, fmt.Sprintf("@%s 未找到发送者用户名，请联系管理员。", botCfg.BotName))
	}

	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sendCommunityCheckinReply(ctx, syncCfg, fmt.Sprintf("@%s 未找到绑定的站内账号，请先使用相同用户名注册或联系管理员。", username))
		}
		return err
	}

	usd := botCfg.MinUSD
	if botCfg.MaxUSD > botCfg.MinUSD {
		usd = botCfg.MinUSD + rand.Intn(botCfg.MaxUSD-botCfg.MinUSD+1)
	}
	quotaAwarded := int(float64(usd) * common.QuotaPerUnit)
	checkin, err := model.UserCheckinWithQuota(user.Id, quotaAwarded)
	if err != nil {
		if strings.Contains(err.Error(), "今日已签到") {
			return sendCommunityCheckinReply(ctx, syncCfg, fmt.Sprintf("@%s 今天已经签到过啦，明天再来吧。", username))
		}
		return err
	}

	result.RewardedCount++
	model.RecordLog(user.Id, model.LogTypeSystem, fmt.Sprintf("社区群签到，获得额度 %s", logger.LogQuota(checkin.QuotaAwarded)))
	return sendCommunityCheckinReply(ctx, syncCfg, fmt.Sprintf("@%s 签到成功，获得 $%d 额度。", username, usd))
}

func fetchCommunityCheckinMessages(ctx context.Context, cfg community_sync_setting.CommunitySyncSetting, sinceID string) ([]communityCheckinMessage, error) {
	payload := communityCheckinTimelineRequest{RoomID: cfg.RoomID, Limit: communityCheckinPageLimit, SinceID: sinceID}
	var messages []communityCheckinMessage
	if err := doCommunityCheckinRequest(ctx, http.MethodPost, communityCheckinTimelineEndpoint, cfg, payload, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func sendCommunityCheckinReply(ctx context.Context, cfg community_sync_setting.CommunitySyncSetting, text string) error {
	payload := communityCheckinSendRequest{ToRoomID: cfg.RoomID, Text: text}
	return doCommunityCheckinRequest(ctx, http.MethodPost, communityCheckinSendEndpoint, cfg, payload, nil)
}

func doCommunityCheckinRequest(ctx context.Context, method string, endpoint string, cfg community_sync_setting.CommunitySyncSetting, payload any, out any) error {
	body, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.Authorization))
	if strings.TrimSpace(cfg.Fingerprint) != "" {
		req.Header.Set("x-client-fingerprint", strings.TrimSpace(cfg.Fingerprint))
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	respBody, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("社区签到机器人接口返回 HTTP %d", resp.StatusCode)
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	return common.Unmarshal(respBody, out)
}

func isCommunityCheckinTrigger(message communityCheckinMessage, cfg community_checkin_bot_setting.CommunityCheckinBotSetting) bool {
	botUserID := strings.TrimSpace(cfg.BotUserID)
	if botUserID == "" || senderID(message) == botUserID {
		return false
	}
	botName := strings.TrimSpace(cfg.BotName)
	mentioned := false
	for _, id := range message.MentionedUserIDs {
		if strings.TrimSpace(id) == botUserID {
			mentioned = true
			break
		}
	}
	if !mentioned && botName != "" {
		text := strings.TrimSpace(message.Text)
		mentioned = strings.Contains(text, "@"+botName) || strings.Contains(text, "＠"+botName)
	}
	if !mentioned {
		return false
	}
	return normalizeCommunityCheckinText(message.Text, cfg.BotName) == "签到"
}

func normalizeCommunityCheckinText(text string, botName string) string {
	cleaned := strings.TrimSpace(text)
	name := strings.TrimSpace(botName)
	if name != "" {
		cleaned = strings.ReplaceAll(cleaned, "@"+name, "")
		cleaned = strings.ReplaceAll(cleaned, "＠"+name, "")
	}
	return strings.TrimSpace(cleaned)
}

func senderID(message communityCheckinMessage) string {
	if strings.TrimSpace(message.FromUserID) != "" {
		return strings.TrimSpace(message.FromUserID)
	}
	if strings.TrimSpace(message.UserID) != "" {
		return strings.TrimSpace(message.UserID)
	}
	if strings.TrimSpace(message.FromUser.ID) != "" {
		return strings.TrimSpace(message.FromUser.ID)
	}
	return strings.TrimSpace(message.User.ID)
}

func senderUsername(message communityCheckinMessage) string {
	if strings.TrimSpace(message.FromUser.Username) != "" {
		return strings.TrimSpace(message.FromUser.Username)
	}
	return strings.TrimSpace(message.User.Username)
}

func updateCommunityCheckinState(result *CommunityCheckinBotRunResult, err error) {
	communityCheckinStateMu.Lock()
	defer communityCheckinStateMu.Unlock()
	communityCheckinState.LastRunAt = time.Now().Unix()
	if result != nil {
		communityCheckinState.LastProcessedCount = result.ProcessedCount
		communityCheckinState.LastTriggeredCount = result.TriggeredCount
		communityCheckinState.LastRewardedCount = result.RewardedCount
		communityCheckinState.LastMessageID = result.LastMessageID
	}
	if err != nil {
		communityCheckinState.LastError = err.Error()
	} else {
		communityCheckinState.LastError = ""
	}
}

func StartCommunityCheckinBotLoop() {
	if !common.IsMasterNode {
		return
	}
	communityCheckinLoopOnce.Do(func() {
		go func() {
			ticker := time.NewTimer(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				cfg := community_checkin_bot_setting.Get()
				if cfg.Enabled {
					result, err := RunCommunityCheckinBotOnce(context.Background())
					if err != nil {
						common.SysLog("community checkin bot failed: " + err.Error())
					} else {
						common.SysLog(fmt.Sprintf("community checkin bot finished: processed=%d triggered=%d rewarded=%d", result.ProcessedCount, result.TriggeredCount, result.RewardedCount))
					}
				}

				interval := cfg.IntervalSeconds
				if interval < community_checkin_bot_setting.MinimumIntervalSeconds {
					interval = community_checkin_bot_setting.DefaultIntervalSeconds
				}
				ticker.Reset(time.Duration(interval) * time.Second)
			}
		}()
	})
}
