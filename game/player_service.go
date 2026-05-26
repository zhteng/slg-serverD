// -----------------------------------------------------------
// 模块:
// 功能:
// 作者: zteng
// 创建: 2026/5/26 18:37
// 文件: player_service.go
// 版权: 仅限内部项目使用
// -----------------------------------------------------------
package game

import (
	"context"
	"slg-serverD/data"
)

type PlayerService struct {
	userSvc  *UserService
	troopSvc *TroopsService
	buildSvc *BuildingService
}

func NewPlayerService(userSvc *UserService, troopSvc *TroopsService, buildSvc *BuildingService) *PlayerService {
	return &PlayerService{userSvc: userSvc, troopSvc: troopSvc, buildSvc: buildSvc}
}

func (s *PlayerService) GetUserInfo(ctx context.Context, uid int64) (*data.PlayerInfo, error) {
	u, err := s.userSvc.Load(ctx, uid)
	if err != nil {
		return nil, err
	}

	t, err := s.troopSvc.Load(ctx, uid)
	if err != nil {
		return nil, err
	}

	var info = &data.PlayerInfo{
		User:   u,
		Troops: t,
	}

	return info, nil
}
