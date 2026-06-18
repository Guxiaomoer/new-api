package community_checkin_bot_setting

import (
	"strconv"
	"strings"
	"sync"
)

const (
	DefaultBotUserID        = "amlarbic93"
	DefaultBotName          = "Guxiaomo"
	DefaultIntervalSeconds  = 30
	MinimumIntervalSeconds  = 10
	DefaultMinUSD           = 2
	DefaultMaxUSD           = 5
)

type CommunityCheckinBotSetting struct {
	Enabled         bool
	BotUserID       string
	BotName         string
	IntervalSeconds int
	MinUSD          int
	MaxUSD          int
	LastMessageID   string
}

var (
	setting = CommunityCheckinBotSetting{
		Enabled:         false,
		BotUserID:       DefaultBotUserID,
		BotName:         DefaultBotName,
		IntervalSeconds: DefaultIntervalSeconds,
		MinUSD:          DefaultMinUSD,
		MaxUSD:          DefaultMaxUSD,
		LastMessageID:   "",
	}
	settingMu sync.RWMutex
)

func Get() CommunityCheckinBotSetting {
	settingMu.RLock()
	defer settingMu.RUnlock()
	return setting
}

func Update(key, value string) {
	settingMu.Lock()
	defer settingMu.Unlock()

	switch key {
	case "community_checkin_bot.enabled":
		setting.Enabled = parseBool(value)
	case "community_checkin_bot.bot_user_id":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			setting.BotUserID = DefaultBotUserID
		} else {
			setting.BotUserID = trimmed
		}
	case "community_checkin_bot.bot_name":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			setting.BotName = DefaultBotName
		} else {
			setting.BotName = trimmed
		}
	case "community_checkin_bot.interval_seconds":
		setting.IntervalSeconds = parseIntWithMin(value, DefaultIntervalSeconds, MinimumIntervalSeconds)
	case "community_checkin_bot.min_usd":
		setting.MinUSD = parseIntWithMin(value, DefaultMinUSD, 1)
		if setting.MaxUSD < setting.MinUSD {
			setting.MaxUSD = setting.MinUSD
		}
	case "community_checkin_bot.max_usd":
		setting.MaxUSD = parseIntWithMin(value, DefaultMaxUSD, 1)
		if setting.MinUSD > setting.MaxUSD {
			setting.MinUSD = setting.MaxUSD
		}
	case "community_checkin_bot.last_message_id":
		setting.LastMessageID = strings.TrimSpace(value)
	}
}

func parseIntWithMin(value string, fallback int, min int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < min {
		return fallback
	}
	return n
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
