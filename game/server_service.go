package game

import "slg-serverD/conf"

type ServerService struct {
	cfg     *conf.Config
	runtime *conf.RuntimeConfig
}

func NewServerService(cfg *conf.Config, runtime *conf.RuntimeConfig) *ServerService {
	return &ServerService{cfg: cfg, runtime: runtime}
}

// GetAllServers 返回所有区服配置（可加入在线人数等动态信息）
func (s *ServerService) GetAllServers() []conf.ServerInfo {
	// 简单返回配置文件中的 servers，实际可结合 Redis 状态动态更新
	return s.cfg.Servers
}

// GetCurrentServerID 返回当前服务实例对应的区服 ID
func (s *ServerService) GetCurrentServerID() int {
	return s.runtime.ServerID
}
