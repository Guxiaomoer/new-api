package trace

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Config 抓包开关与容量。热读：网关每个请求读一次 OptionMap，改完立即生效。
type Config struct {
	Enabled       bool `json:"enabled"`
	MaxBody       int  `json:"max_body"`
	RetentionDays int  `json:"retention_days"`
	MaxSessions   int  `json:"max_sessions"`
}

func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		MaxBody:       512 * 1024,
		RetentionDays: 7,
		MaxSessions:   300,
	}
}

const (
	maxBodyFloor   = 4 * 1024
	maxBodyCeiling = 8 * 1024 * 1024
	retentionFloor = 1
	retentionCeil  = 90
	sessionsFloor  = 10
	sessionsCeil   = 5000
)

func optionValue(key string) string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap[key]
}

func LoadConfig() Config {
	cfg := DefaultConfig()
	if v := optionValue(model.TapOptionEnabled); v != "" {
		cfg.Enabled = v == "true"
	}
	if v, err := strconv.Atoi(optionValue(model.TapOptionMaxBody)); err == nil && v > 0 {
		cfg.MaxBody = v
	}
	if v, err := strconv.Atoi(optionValue(model.TapOptionRetentionDays)); err == nil && v > 0 {
		cfg.RetentionDays = v
	}
	if v, err := strconv.Atoi(optionValue(model.TapOptionMaxSessions)); err == nil && v > 0 {
		cfg.MaxSessions = v
	}
	if cfg.MaxBody < maxBodyFloor {
		cfg.MaxBody = maxBodyFloor
	}
	if cfg.MaxBody > maxBodyCeiling {
		cfg.MaxBody = maxBodyCeiling
	}
	if cfg.RetentionDays < retentionFloor {
		cfg.RetentionDays = retentionFloor
	}
	if cfg.RetentionDays > retentionCeil {
		cfg.RetentionDays = retentionCeil
	}
	if cfg.MaxSessions < sessionsFloor {
		cfg.MaxSessions = sessionsFloor
	}
	if cfg.MaxSessions > sessionsCeil {
		cfg.MaxSessions = sessionsCeil
	}
	return cfg
}

// NormalizeConfig 把界面传上来的值夹到允许区间，越界不报错、直接取边界值。
func NormalizeConfig(cfg Config) Config {
	if cfg.MaxBody < maxBodyFloor {
		cfg.MaxBody = maxBodyFloor
	}
	if cfg.MaxBody > maxBodyCeiling {
		cfg.MaxBody = maxBodyCeiling
	}
	if cfg.RetentionDays < retentionFloor {
		cfg.RetentionDays = retentionFloor
	}
	if cfg.RetentionDays > retentionCeil {
		cfg.RetentionDays = retentionCeil
	}
	if cfg.MaxSessions < sessionsFloor {
		cfg.MaxSessions = sessionsFloor
	}
	if cfg.MaxSessions > sessionsCeil {
		cfg.MaxSessions = sessionsCeil
	}
	return cfg
}

// Enabled 热路径每请求调一次。读配置失败按关闭处理，不能因此拦住请求。
func Enabled() bool {
	return LoadConfig().Enabled
}
