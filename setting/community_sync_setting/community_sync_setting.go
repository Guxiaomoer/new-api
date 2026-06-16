package community_sync_setting

import (
	"strconv"
	"strings"
	"sync"
)

const (
	DefaultEndpoint       = "https://dc.hhhl.cc/api/chat/rooms/members"
	DefaultRoomID         = "ani5zrxyl7"
	DefaultIntervalMinute = 5
	DefaultProtectedUsers = "1456671048@qq.com\nlufeng2820@163.com"
)

type CommunitySyncSetting struct {
	Enabled         bool
	Endpoint        string
	RoomID          string
	Authorization   string
	Fingerprint     string
	IntervalMinutes int
	ProtectedUsers  []string
}

var (
	setting = CommunitySyncSetting{
		Enabled:         false,
		Endpoint:        DefaultEndpoint,
		RoomID:          DefaultRoomID,
		Authorization:   "",
		Fingerprint:     "",
		IntervalMinutes: DefaultIntervalMinute,
		ProtectedUsers:  SplitProtectedUsers(DefaultProtectedUsers),
	}
	settingMu sync.RWMutex
)

func Get() CommunitySyncSetting {
	settingMu.RLock()
	defer settingMu.RUnlock()
	copySetting := setting
	copySetting.ProtectedUsers = append([]string(nil), setting.ProtectedUsers...)
	return copySetting
}

func Update(key, value string) {
	settingMu.Lock()
	defer settingMu.Unlock()

	switch key {
	case "community_sync.enabled":
		setting.Enabled = parseBool(value)
	case "community_sync.endpoint":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			setting.Endpoint = DefaultEndpoint
		} else {
			setting.Endpoint = trimmed
		}
	case "community_sync.room_id":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			setting.RoomID = DefaultRoomID
		} else {
			setting.RoomID = trimmed
		}
	case "community_sync.authorization":
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			setting.Authorization = trimmed
		}
	case "community_sync.fingerprint":
		setting.Fingerprint = strings.TrimSpace(value)
	case "community_sync.interval_minutes":
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n >= 1 {
			setting.IntervalMinutes = n
		} else {
			setting.IntervalMinutes = DefaultIntervalMinute
		}
	case "community_sync.protected_users":
		users := SplitProtectedUsers(value)
		if len(users) == 0 {
			users = SplitProtectedUsers(DefaultProtectedUsers)
		}
		setting.ProtectedUsers = users
	}
}

func SplitProtectedUsers(value string) []string {
	lines := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';'
	})
	out := make([]string, 0, len(lines))
	seen := map[string]bool{}
	for _, line := range lines {
		normalized := strings.ToLower(strings.TrimSpace(line))
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

func ProtectedUsersString() string {
	return strings.Join(Get().ProtectedUsers, "\n")
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
