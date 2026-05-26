package ws

import "slg-serverD/game"

type WSPushService struct {
	hub *Hub
}

var _ game.PushService = (*WSPushService)(nil)

func NewWSPushService(hub *Hub) *WSPushService {
	return &WSPushService{hub: hub}
}

func (p *WSPushService) PushBuildingUpgrade(uid int64, buildingId string, level int) {
	p.hub.SendToPlayer(uid, Message{
		Type: "building_upgrade",
		Data: map[string]interface{}{
			"building_id": buildingId,
			"new_level":   level,
		},
	})
}

func (p *WSPushService) PushTroopTrainComplete(uid int64, soldierKey string, total int) {
	p.hub.SendToPlayer(uid, Message{
		Type: "troop_train_complete",
		Data: map[string]interface{}{
			"soldier_key": soldierKey,
			"total":       total,
		},
	})
}
