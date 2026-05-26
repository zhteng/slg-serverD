package game

import "log"

// 推送接口
type PushService interface {
	PushBuildingUpgrade(uid int64, buildingId string, newLevel int)
	PushTroopTrainComplete(uid int64, soldierKey string, total int)
}

// 默认推送实现（生产环境应替换为 WebSocket 等）
type defaultPushService struct{}

func (d *defaultPushService) PushBuildingUpgrade(uid int64, buildingId string, newLevel int) {
	log.Printf("[PUSH] Player %d: building %s upgraded to level %d", uid, buildingId, newLevel)
}

func (s *defaultPushService) PushTroopTrainComplete(uid int64, soldierKey string, total int) {
	log.Printf("[PUSH] Player %d: training of %d %s(s) completed", uid, total, soldierKey)
}

// 返回默认推送服务实例
func NewDefaultPushService() PushService {
	return &defaultPushService{}
}
