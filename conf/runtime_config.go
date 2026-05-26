package conf

import "sync"

type RuntimeConfig struct {
	mu             sync.RWMutex
	ServerID       int    `json:"server_id"`   // 当前区服 ID（0 表示全局默认）
	ServerName     string `json:"server_name"` // 区服名称
	DBDSN          string `json:"-"`           // 数据库连接串 // 不暴露
	RedisAddr      string `json:"-"`           // Redis 地址
	RedisPass      string `json:"-"`           // Redis 密码
	RedisPrefix    string `json:"-"`           // Redis key 前缀
	HTTPPort       int    `json:"http_port"`   // 监听端口
	Status         int    `json:"status"`
	MaxPlayers     int    `json:"max_players"`
	CurrentOnline  int    `json:"current_online"`
	InternalSecret string `json:"internal_secret"`
}

// SetCurrentOnline 线程安全更新在线人数
func (r *RuntimeConfig) SetCurrentOnline(count int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.CurrentOnline = count
}

// GetCurrentOnline 获取当前在线人数
func (r *RuntimeConfig) GetCurrentOnline() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.CurrentOnline
}

// SafeInfo 返回脱敏后的配置信息（用于 HTTP 响应）
func (r *RuntimeConfig) SafeInfo() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"server_id":      r.ServerID,
		"server_name":    r.ServerName,
		"http_port":      r.HTTPPort,
		"status":         r.Status,
		"max_players":    r.MaxPlayers,
		"current_online": r.CurrentOnline,
	}
}

// 全局运行时配置单例
var runtimeConfig *RuntimeConfig

func SetRuntimeConfig(cfg *RuntimeConfig) {
	runtimeConfig = cfg
}

func GetRuntimeConfig() *RuntimeConfig {
	return runtimeConfig
}
