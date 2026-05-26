package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slg-serverD/conf"
	"slg-serverD/data"
	"time"

	"github.com/gin-gonic/gin"
)

type CrossService struct {
	serverMap map[int]string // serverId -> internal API base URL
	secret    string
	config    *conf.CrossConfig
	client    *http.Client
}

func NewCrossService(cfg *conf.CrossConfig) *CrossService {
	return &CrossService{
		config: cfg,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// HandleCrossAttack 跨服攻击
func (s *CrossService) HandleCrossAttack(c *gin.Context) {
	// token 解析uid serverId
	uid := c.GetInt64("uid")
	serverId := c.GetInt("serverId")

	var req struct {
		TargetUID int64 `json:"target_uid" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "msg": "invalid request"})
		return
	}

	// 从攻击者原服获取部队数据（通过内部API）
	attackerTroops, err := s.fetchTroopsFromOrigin(serverId, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "msg": "fetch attacker troops failed"})
		return
	}

	// 获取目标玩家信息（假设目标也在跨服场景中，已知目标 serverId）
	// 这里简化：客户端需携带目标serverId，或从跨服匹配服务查询。
	targetServerId := c.GetInt("target_server_id")
	if targetServerId == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "msg": "target_server_id is 0"})
		return
	}
	targetTroops, err := s.fetchTroopsFromOrigin(targetServerId, req.TargetUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "msg": "fetch target troops failed"})
		return
	}

	// 执行简化的战斗计算
	remainingAttacker, remainingDefender := simpleBattle(attackerTroops.Troops, targetTroops.Troops)

	// 回写数据到双方原服
	if err := s.syncToOrigin(serverId, uid, remainingAttacker, nil); err != nil {
		log.Printf("sync attacker failed: %v", err)
		c.JSON(500, "sync A failed")
		return
	}

	if err := s.syncToOrigin(targetServerId, uid, remainingDefender, nil); err != nil {
		// 回滚A todo
		//s.rollback(serverIdA, uidA, deltaA.Troops, deltaA.Resources)

		log.Printf("sync defender failed: %v", err)
		c.JSON(500, "sync A failed")
		return
	}

	// 记录跨服战报（可保存到跨服数据库）
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"attackerTroops": attackerTroops,
			"defenderTroops": remainingDefender,
		},
	})
}

// 通过原服内部api 获取玩家部队数据
func (s *CrossService) fetchTroopsFromOrigin(serverId int, uid int64) (*data.Troops, error) {
	url := s.config.ServerMap[serverId]
	if url == "" {
		return nil, fmt.Errorf("serverId %d not exist", serverId)
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/internal/troops?uid=%d", url, uid), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Auth", "Bearer "+s.config.InternalSecret)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code int          `json:"code"`
		Msg  string       `json:"msg"`
		Data *data.Troops `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Code != 1 {
		return nil, fmt.Errorf("serverId %d get troops failed", serverId)
	}

	return result.Data, nil
}

// 回写部队数据到原服
func (s *CrossService) syncToOrigin(serverId int, uid int64, troops map[string]int, resources map[string]int) error {
	url := s.config.ServerMap[serverId]
	if url == "" {
		return fmt.Errorf("serverId %d not exist", serverId)
	}
	body, _ := json.Marshal(map[string]interface{}{
		"uid":       uid,
		"troops":    troops,
		"resources": resources,
	})
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/internal/sync", url), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Auth", "Bearer "+s.config.InternalSecret)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 解析响应判断成功
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Code != 1 {
		return fmt.Errorf("serverId %d sync troops failed", serverId)
	}

	return nil
}

func simpleBattle(attacker, defender map[string]int) (map[string]int, map[string]int) {
	remA := make(map[string]int)
	remB := make(map[string]int)
	for k, v := range attacker {
		remA[k] = v / 2
	}
	for k, v := range defender {
		remB[k] = v / 2
	}
	return remA, remB
}
