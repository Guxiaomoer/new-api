package controller

import (
	"bufio"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const serverMonitorCacheTTL = 10 * time.Second

var serverMonitorCache = struct {
	sync.Mutex
	data      *ServerMonitorOverview
	expiresAt time.Time
	cpu       cpuSample
}{}

type ServerMonitorOverview struct {
	CollectedAt int64                   `json:"collected_at"`
	Host        ServerMonitorHost       `json:"host"`
	App         ServerMonitorApp        `json:"app"`
	Database    ServerMonitorDatabase   `json:"database"`
	Capacity    ServerMonitorCapacity   `json:"capacity"`
	Warnings    []string                `json:"warnings"`
	Partial     bool                    `json:"partial"`
}

type ServerMonitorHost struct {
	UptimeSeconds     float64            `json:"uptime_seconds"`
	CPUCores          int                `json:"cpu_cores"`
	CPUUsagePercent   float64            `json:"cpu_usage_percent"`
	LoadAverage       ServerMonitorLoad  `json:"load_average"`
	Memory            ServerMonitorUsage `json:"memory"`
	Swap              ServerMonitorUsage `json:"swap"`
	RootDisk          ServerMonitorUsage `json:"root_disk"`
}

type ServerMonitorLoad struct {
	OneMinute     float64 `json:"one_minute"`
	FiveMinutes   float64 `json:"five_minutes"`
	FifteenMinutes float64 `json:"fifteen_minutes"`
}

type ServerMonitorUsage struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Available   uint64  `json:"available"`
	UsedPercent float64 `json:"used_percent"`
}

type ServerMonitorApp struct {
	GoVersion      string `json:"go_version"`
	Goroutines     int    `json:"goroutines"`
	HeapAlloc      uint64 `json:"heap_alloc"`
	HeapSys        uint64 `json:"heap_sys"`
	Sys            uint64 `json:"sys"`
	NumGC          uint32 `json:"num_gc"`
}

type ServerMonitorDatabase struct {
	TotalUsers        int64 `json:"total_users"`
	EnabledUsers      int64 `json:"enabled_users"`
	OAuthBindings     int64 `json:"oauth_bindings"`
	TotalTokens       int64 `json:"total_tokens"`
	EnabledTokens     int64 `json:"enabled_tokens"`
	RecentUsers24h    int64 `json:"recent_users_24h"`
	RecentLogins24h   int64 `json:"recent_logins_24h"`
	RecentLogins7d    int64 `json:"recent_logins_7d"`
}

type ServerMonitorCapacity struct {
	Level                         string   `json:"level"`
	ConservativeConcurrentRange   string   `json:"conservative_concurrent_range"`
	RegisteredUsersSuggestion     string   `json:"registered_users_suggestion"`
	Hints                         []string `json:"hints"`
}

type cpuSample struct {
	idle  uint64
	total uint64
	valid bool
}

// GetServerMonitorOverview returns a sanitized, root-only server overview.
func GetServerMonitorOverview(c *gin.Context) {
	overview := getCachedServerMonitorOverview()
	common.ApiSuccess(c, overview)
}

func getCachedServerMonitorOverview() *ServerMonitorOverview {
	now := time.Now()
	serverMonitorCache.Lock()
	defer serverMonitorCache.Unlock()

	if serverMonitorCache.data != nil && now.Before(serverMonitorCache.expiresAt) {
		return serverMonitorCache.data
	}

	overview := collectServerMonitorOverview(now)
	serverMonitorCache.data = overview
	serverMonitorCache.expiresAt = now.Add(serverMonitorCacheTTL)
	return overview
}

func collectServerMonitorOverview(now time.Time) *ServerMonitorOverview {
	warnings := make([]string, 0)
	partial := false

	host := ServerMonitorHost{CPUCores: runtime.NumCPU()}
	if uptime, err := readProcUptime(); err == nil {
		host.UptimeSeconds = uptime
	} else {
		partial = true
		warnings = append(warnings, "无法读取服务器运行时间")
	}
	if load, err := readProcLoadAverage(); err == nil {
		host.LoadAverage = load
	} else {
		partial = true
		warnings = append(warnings, "无法读取系统负载")
	}
	if memory, swap, err := readProcMemInfo(); err == nil {
		host.Memory = memory
		host.Swap = swap
	} else {
		partial = true
		warnings = append(warnings, "无法读取内存信息")
	}
	if disk, err := readRootDiskUsage(); err == nil {
		host.RootDisk = disk
	} else {
		partial = true
		warnings = append(warnings, "无法读取根分区磁盘信息")
	}
	if usage, err := readCPUUsagePercent(&serverMonitorCache.cpu); err == nil {
		host.CPUUsagePercent = usage
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	app := ServerMonitorApp{
		GoVersion:  runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
		HeapAlloc:  memStats.HeapAlloc,
		HeapSys:    memStats.HeapSys,
		Sys:        memStats.Sys,
		NumGC:      memStats.NumGC,
	}

	database, dbWarnings := collectServerMonitorDatabase(now)
	if len(dbWarnings) > 0 {
		partial = true
	}
	warnings = append(warnings, dbWarnings...)
	capacity := buildServerMonitorCapacity(host, database)
	if capacity.Level != "ok" {
		warnings = append(warnings, capacity.Hints...)
	}

	return &ServerMonitorOverview{
		CollectedAt: now.Unix(),
		Host:        host,
		App:         app,
		Database:    database,
		Capacity:    capacity,
		Warnings:    dedupeStrings(warnings),
		Partial:     partial,
	}
}

func collectServerMonitorDatabase(now time.Time) (ServerMonitorDatabase, []string) {
	stats := ServerMonitorDatabase{}
	warnings := make([]string, 0)
	count := func(target any, query *int64, label string) {
		if err := model.DB.Model(target).Count(query).Error; err != nil {
			warnings = append(warnings, label+"统计失败")
		}
	}
	countWhere := func(target any, query *int64, label string, where string, args ...any) {
		if err := model.DB.Model(target).Where(where, args...).Count(query).Error; err != nil {
			warnings = append(warnings, label+"统计失败")
		}
	}

	count(&model.User{}, &stats.TotalUsers, "用户总数")
	countWhere(&model.User{}, &stats.EnabledUsers, "启用用户", "status = ?", common.UserStatusEnabled)
	count(&model.UserOAuthBinding{}, &stats.OAuthBindings, "OAuth 绑定")
	count(&model.Token{}, &stats.TotalTokens, "令牌总数")
	countWhere(&model.Token{}, &stats.EnabledTokens, "启用令牌", "status = ?", common.TokenStatusEnabled)

	recent24h := now.Add(-24 * time.Hour).Unix()
	recent7d := now.Add(-7 * 24 * time.Hour).Unix()
	countWhere(&model.User{}, &stats.RecentUsers24h, "24 小时新用户", "created_at >= ?", recent24h)
	countWhere(&model.User{}, &stats.RecentLogins24h, "24 小时登录用户", "last_login_at >= ?", recent24h)
	countWhere(&model.User{}, &stats.RecentLogins7d, "7 天登录用户", "last_login_at >= ?", recent7d)

	return stats, warnings
}

func buildServerMonitorCapacity(host ServerMonitorHost, database ServerMonitorDatabase) ServerMonitorCapacity {
	level := "ok"
	hints := make([]string, 0)
	escalate := func(next string) {
		if level == "critical" || next == "ok" {
			return
		}
		if next == "critical" || level == "ok" {
			level = next
		}
	}

	if host.RootDisk.Total > 0 {
		switch {
		case host.RootDisk.UsedPercent > 85:
			escalate("critical")
			hints = append(hints, "磁盘使用率超过 85%，建议尽快清理 Docker 镜像、日志或扩容")
		case host.RootDisk.UsedPercent >= 75:
			escalate("warning")
			hints = append(hints, "磁盘使用率超过 75%，建议关注日志和镜像占用")
		}
	}

	if host.Memory.Total > 0 {
		availablePercent := percent(host.Memory.Available, host.Memory.Total)
		if availablePercent < 20 {
			escalate("warning")
			hints = append(hints, "可用内存低于 20%，高峰期可能触发 Swap 或响应变慢")
		}
	}

	if host.Swap.Total > 0 && host.Swap.UsedPercent > 35 {
		escalate("warning")
		hints = append(hints, "Swap 使用偏高，说明内存压力已经出现")
	}

	cores := host.CPUCores
	if cores <= 0 {
		cores = 1
	}
	if host.LoadAverage.OneMinute > float64(cores*2) {
		escalate("critical")
		hints = append(hints, "1 分钟负载超过 CPU 核心数 2 倍，请降低并发或排查慢请求")
	} else if host.LoadAverage.OneMinute > float64(cores) {
		escalate("warning")
		hints = append(hints, "1 分钟负载超过 CPU 核心数，建议观察高峰请求量")
	}

	if database.EnabledUsers > 800 {
		escalate("warning")
		hints = append(hints, "启用用户接近 1000，当前 2C/4G 机器建议开始准备扩容或迁移")
	}
	if len(hints) == 0 {
		hints = append(hints, "当前资源状态正常，继续观察高峰时段负载和公网延迟")
	}

	return ServerMonitorCapacity{
		Level:                       level,
		ConservativeConcurrentRange: "30-50 个同时活跃请求较稳，80+ 高峰请求风险较高",
		RegisteredUsersSuggestion:   "轻量公益站可先按几百到约 1000 注册用户观察，真实上限取决于高峰并发和上游响应",
		Hints:                       dedupeStrings(hints),
	}
}

func readProcUptime() (float64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, errors.New("invalid /proc/uptime")
	}
	return strconv.ParseFloat(fields[0], 64)
}

func readProcLoadAverage() (ServerMonitorLoad, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return ServerMonitorLoad{}, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return ServerMonitorLoad{}, errors.New("invalid /proc/loadavg")
	}
	one, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return ServerMonitorLoad{}, err
	}
	five, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return ServerMonitorLoad{}, err
	}
	fifteen, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return ServerMonitorLoad{}, err
	}
	return ServerMonitorLoad{OneMinute: one, FiveMinutes: five, FifteenMinutes: fifteen}, nil
}

func readProcMemInfo() (ServerMonitorUsage, ServerMonitorUsage, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return ServerMonitorUsage{}, ServerMonitorUsage{}, err
	}
	defer file.Close()

	values := map[string]uint64{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[key] = value * 1024
	}
	if err := scanner.Err(); err != nil {
		return ServerMonitorUsage{}, ServerMonitorUsage{}, err
	}
	memTotal := values["MemTotal"]
	memAvailable := values["MemAvailable"]
	if memAvailable == 0 {
		memAvailable = values["MemFree"] + values["Buffers"] + values["Cached"]
	}
	memUsed := memTotal - minUint64(memAvailable, memTotal)
	memory := ServerMonitorUsage{Total: memTotal, Used: memUsed, Available: memTotal - memUsed, UsedPercent: percent(memUsed, memTotal)}

	swapTotal := values["SwapTotal"]
	swapFree := values["SwapFree"]
	swapUsed := swapTotal - minUint64(swapFree, swapTotal)
	swap := ServerMonitorUsage{Total: swapTotal, Used: swapUsed, Available: swapTotal - swapUsed, UsedPercent: percent(swapUsed, swapTotal)}
	return memory, swap, nil
}

func readRootDiskUsage() (ServerMonitorUsage, error) {
	disk := common.GetDiskSpaceInfo()
	if disk.Total == 0 {
		return ServerMonitorUsage{}, errors.New("invalid root disk usage")
	}
	return ServerMonitorUsage{Total: disk.Total, Used: disk.Used, Available: disk.Free, UsedPercent: disk.UsedPercent}, nil
}

func readCPUUsagePercent(previous *cpuSample) (float64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	lines := strings.SplitN(string(data), "\n", 2)
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, errors.New("invalid /proc/stat")
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, err
		}
		values = append(values, value)
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	current := cpuSample{idle: idle, total: total, valid: true}
	if !previous.valid {
		*previous = current
		return 0, errors.New("first cpu sample")
	}
	totalDelta := total - previous.total
	idleDelta := idle - previous.idle
	*previous = current
	if totalDelta == 0 || idleDelta > totalDelta {
		return 0, errors.New("invalid cpu delta")
	}
	return float64(totalDelta-idleDelta) * 100 / float64(totalDelta), nil
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) * 100 / float64(total)
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
