package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slg-serverD/data"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const marchDelayQueueKey = "delay:queue:march"

// MarchTask 存入 ZSET 的任务摘要
type MarchTask struct {
	Uid      int64  `json:"uid"`
	MarchKey string `json:"march_key"`
	FinishAt int64  `json:"finish_at"`
}

type MarchService struct {
	troopSvc *TroopsService
	userSvc  *UserService
	mapSvc   *MapService
	lockSvc  *LockService
	rdb      *redis.Client
	pushSvc  PushService
	stop     chan struct{}
	wakeUp   chan struct{}
}

func NewMarchService(troopSvc *TroopsService, userSvc *UserService, mapSvc *MapService, lockSvc *LockService, rdb *redis.Client, pushSvc PushService) *MarchService {
	if pushSvc == nil {
		pushSvc = NewDefaultPushService()
	}
	return &MarchService{troopSvc: troopSvc, userSvc: userSvc, mapSvc: mapSvc, lockSvc: lockSvc, rdb: rdb, pushSvc: pushSvc, stop: make(chan struct{}), wakeUp: make(chan struct{}, 1)}
}

// StartQueueChecker 启动行军到达消费者（动态休眠版）
func (s *MarchService) StartQueueChecker() {
	go func() {
		s.rebuildDelayQueue()
		for {
			fmt.Println("march 定时任务执行, 时间:", time.Now().Unix())
			select {
			case <-s.stop:
				return
			default:
			}
			s.consumeMarchQueue()
			s.sleepUntilNextMarch()
		}
	}()
}

// 关闭
func (s *MarchService) Stop() { close(s.stop) }

// 发起行军，扣兵并生成行军记录   gatherDuration: 采集时长（秒），仅采集类型有效；<=0 则默认 3600 秒
func (s *MarchService) LaunchMarch(ctx context.Context, uid int64, toX, toY, mType int, troops map[string]int, gatherDuration int64, speedBonus float64) (*data.MarchAttack, error) {
	// 边界检查
	if toX < 0 || toX >= data.MapWidth || toY < 0 || toY >= data.MapHeight {
		return nil, errors.New("invalid coordinates")
	}

	if s.mapSvc == nil {
		return nil, errors.New("map service not available")
	}

	// 获取目标地块
	target, err := s.mapSvc.LoadOrCreateCell(ctx, toX, toY)
	fmt.Println("cccccccccccccccccccccccccccccccccccccccccccccccc")
	if err != nil {
		return nil, err
	}

	// 校验目标类型是否可行动
	if target.MType != data.CellTypeResource && target.MType != data.CellTypeMonster {
		return nil, errors.New("cannot to here")
	}

	// 从玩家服务获取出发坐标
	u, err := s.userSvc.Load(ctx, uid)
	if err != nil {
		return nil, err
	}
	fromX, fromY := u.CityX, u.CityY

	// 加玩家锁，防止并发扣减部队
	unlock, err := s.lockSvc.Lock(ctx, UserLockKey(uid))
	if err != nil {
		return nil, errors.New("operation conflict")
	}
	defer unlock()

	// 扣除部队
	/*if err := s.troopSvc.DeductTroops(ctx, uid, troops); err != nil {
		return nil, err
	}*/
	t, err := s.troopSvc.Load(ctx, uid)
	if err != nil {
		return nil, err
	}
	for k, v := range troops {
		if _, ok := t.Troops[k]; !ok {
			return nil, fmt.Errorf("troop key %s not exist", k)
		}

		if v > t.Troops[k] {
			return nil, errors.New("insufficient troops")
		}

		t.Troops[k] -= v
		if t.Troops[k] <= 0 {
			delete(t.Troops, k)
		}
	}

	// 计算行军时间
	marchSeconds := CalcMarchDuration(fromX, fromY, toX, toY, speedBonus)
	now := time.Now().Unix()
	arriveTime := now + int64(marchSeconds)

	// 生成行军标识
	marchKey := generateMarchKey(uid, now)
	// 将行军记录存入玩家部队数据
	march := &data.MarchAttack{
		FromX:      fromX,
		FromY:      fromY,
		ToX:        toX,
		ToY:        toY,
		Type:       mType,
		SkinId:     1,
		Status:     data.StatusGoing,
		StartTime:  now,
		ArriveTime: arriveTime,
		RealArrive: arriveTime,
		Troops:     troops,
	}

	// 是否为采集
	if mType == data.TypeGather {
		if gatherDuration <= 0 {
			gatherDuration = 3600 // 默认采集1小时
		}
		march.GatherDuration = gatherDuration
	}

	// 保存行军到玩家部队数据（Attack 字段）
	t.Attack[marchKey] = march
	if err := s.troopSvc.Save(ctx, t); err != nil {
		return nil, err
	}

	// 在地图服务中登记行军（加入目标地块队列）
	err = s.mapSvc.RegisterMarch(ctx, toX, toY, &data.MarchQueueItem{
		MarchKey:  marchKey,
		Uid:       uid,
		Troops:    troops,
		ArriveAt:  time.Unix(arriveTime, 0),
		Type:      mType,
		ActionEnd: time.Unix(arriveTime+int64(gatherDuration), 0),
	})
	if err != nil {
		// 地图注册失败，需要回滚
		for k, v := range troops {
			t.Troops[k] += v
		}
		s.troopSvc.Save(ctx, t)
		return nil, errors.New("map register failed")
	}

	// 加入 Redis 延迟队列
	task := MarchTask{
		Uid:      uid,
		MarchKey: marchKey,
		FinishAt: arriveTime,
	}
	taskJson, _ := json.Marshal(task)
	s.rdb.ZAdd(ctx, marchDelayQueueKey, &redis.Z{
		Score:  float64(arriveTime),
		Member: taskJson,
	})

	// 唤醒休眠的消费者
	select {
	case s.wakeUp <- struct{}{}:
	default:
	}

	return march, nil
}

// 玩家主动中断采集，立刻结算已采集资源并返程
func (s *MarchService) CancelGather(ctx context.Context, uid int64, marchKey string) error {
	unlock, err := s.lockSvc.Lock(ctx, UserLockKey(uid))
	if err != nil {
		return errors.New("operation conflict")
	}
	defer unlock()

	t, err := s.troopSvc.Load(ctx, uid)
	if err != nil {
		return err
	}

	march, ok := t.Attack[marchKey]
	if !ok || march.Type != data.TypeGather || march.Status != data.StatusGathering {
		return errors.New("no gathering march found")
	}

	// 从地图服务获取已采集时间并计算资源
	elapsed := s.mapSvc.GetGatherElapsed(ctx, march.ToX, march.ToY, marchKey)
	if elapsed < 0 {
		elapsed = march.GatherDuration // 默认走完时长
	}

	resourceAmount := s.calcGatherAmount(march.ToX, march.ToY, elapsed)

	// 通知地图服务采集完成，移除队列
	_ = s.mapSvc.RemoveMarch(ctx, march.ToX, march.ToY, marchKey)
	return s.createReturnMarch(ctx, uid, marchKey, t, resourceAmount)
}

// 模拟延迟到达
func (s *MarchService) scheduleArrival(uid int64, marchKey string, arriveTime int64) {
	delay := time.Until(time.Unix(arriveTime, 0))
	if delay > 0 {
		time.Sleep(delay)
	}
	s.OnMarchArrive(uid, marchKey, arriveTime)
}

func (s *MarchService) OnMarchArrive(uid int64, marchKey string, arriveTime int64) {
	ctx := context.Background()
	t, err := s.troopSvc.Load(ctx, uid)
	if err != nil {
		return
	}

	march, ok := t.Attack[marchKey]
	if !ok || march.Status == data.StatusGoing {
		return
	}

	switch march.Type {
	case data.TypeAttack:
		// 战斗结算（简化：损失一半，转为伤兵）
		for k, v := range march.Troops {
			t.Damaged[k] += v / 2
		}

	case data.TypeGather:
		// 采集资源奖励
		user, _ := s.userSvc.Load(ctx, uid)
		user.Gold += 100
	}

	march.Status = data.StatusFinished
	delete(t.Attack, marchKey)
	_ = s.troopSvc.Save(ctx, t)
}

// sleepUntilNextMarch 动态休眠：根据下一个任务的到期时间，或 30 秒，可被 stop 和 wakeUp 打断
func (s *MarchService) sleepUntilNextMarch() {
	ctx := context.Background()

	result, err := s.rdb.ZRangeWithScores(ctx, marchDelayQueueKey, 0, 0).Result()
	if err != nil || len(result) == 0 {
		// 无任务：休眠 30 秒，但可被 stop/wakeUp 中断
		select {
		case <-time.After(30 * time.Second):
		case <-s.stop:
		case <-s.wakeUp:
		}
		return
	}
	nextTime := int64(result[0].Score)
	now := time.Now().Unix()
	if nextTime > now {
		sleepDuration := time.Duration(nextTime-now) * time.Second
		if sleepDuration > 30*time.Second {
			sleepDuration = 30 * time.Second
		}
		select {
		case <-time.After(sleepDuration):
		case <-s.stop:
		case <-s.wakeUp:
		}
	}
	// 如果 nextTime <= now，不阻塞，直接返回
}

// 从数据库加载所有进行中的行军，重新加入 Redis ZSET
func (s *MarchService) rebuildDelayQueue() {
	ctx := context.Background()
	attacksMap, err := s.troopSvc.GetAllActiveMarchs(ctx)
	if err != nil {
		log.Printf("rebuild march delay queue error: %v", err)
		return
	}

	count := 0
	for uid, attacks := range attacksMap {
		for key, march := range attacks {
			// 只处理需要定时触发的状态：去程、采集中、返程
			switch march.Status {
			case data.StatusGoing, data.StatusGathering, data.StatusReturning:
				finishAt := march.ArriveTime
				task := MarchTask{
					Uid:      uid,
					MarchKey: key,
					FinishAt: finishAt,
				}
				taskJSON, _ := json.Marshal(task)

				s.rdb.ZAdd(ctx, marchDelayQueueKey, &redis.Z{
					Score:  float64(finishAt),
					Member: taskJSON,
				})

				count++
			}
			//if march.Status == data.StatusGoing || march.Status == data.StatusGathering || march.Status == data.StatusReturning {}
		}
	}
	fmt.Printf("Rebuilt march delay queue with %d tasks\n", count)
	//os.Exit(1)
}

// 消费所有到期的行军任务
func (s *MarchService) consumeMarchQueue() {
	ctx := context.Background()
	now := time.Now().Unix()

	member, err := s.rdb.ZRangeByScore(ctx, marchDelayQueueKey, &redis.ZRangeBy{
		Min:   "0",
		Max:   strconv.FormatInt(now, 10),
		Count: 100,
	}).Result()

	if err != nil {
		log.Printf("consume march queue error: %v", err)
		return
	}

	for _, member := range member {
		var task MarchTask
		if err := json.Unmarshal([]byte(member), &task); err != nil {
			log.Printf("consume march queue error: %v", err)
			continue
		}

		func() {
			unlock, err := s.lockSvc.Lock(ctx, UserLockKey(task.Uid))
			if err != nil {
				log.Printf("lock error: %d", task.Uid)
				return
			}
			defer unlock()

			t, err := s.troopSvc.Load(ctx, task.Uid)
			if err != nil {
				log.Printf("load error: %d", task.Uid)
				s.rdb.ZRem(ctx, marchDelayQueueKey, member)
				return
			}

			march, ok := t.Attack[task.MarchKey]
			if !ok {
				log.Printf("consume march queue error: %d", task.Uid)
				s.rdb.ZRem(ctx, marchDelayQueueKey, member)
				return
			}

			//if march.Status == data.StatusGoing || march.Status == data.StatusGathering || march.Status == data.StatusReturning {}
			switch march.Status {
			case data.StatusGoing:
				s.handleArrival(ctx, task.Uid, task.MarchKey, t)
			case data.StatusGathering:
				s.handleGatherComplete(ctx, task.Uid, task.MarchKey, t)
			case data.StatusReturning:
				s.handleReturn(ctx, task.Uid, task.MarchKey, t)
			default:
				delete(t.Attack, task.MarchKey)
				_ = s.troopSvc.Save(ctx, t)
			}

			s.rdb.ZRem(ctx, marchDelayQueueKey, member)
		}()
	}
}

// 去程到达，根据类型进入不同分支
// handleArrival 到达处理：通知地图服务，获取是否可立即行动
func (s *MarchService) handleArrival(ctx context.Context, uid int64, marchKey string, t *data.Troops) {
	march := t.Attack[marchKey]
	if march == nil || march.Status != data.StatusGoing {
		return
	}

	// 通知地图服务行军到达，若需要排队则等待；若为队首则执行操作
	canAct, err := s.mapSvc.OnMarchArrive(ctx, march.ToX, march.ToY, marchKey)
	if err != nil || !canAct {
		// 需要排队，保持 Status=Going，不处理，等待下次定时触发（或由地图服务通知）
		return
	}

	// 可以行动，根据类型进入分支
	switch march.Type {
	case data.TypeAttack:
		remaining := s.handleAttack(march)                              // 发生战斗
		s.createReturnMarchWithTroops(ctx, uid, marchKey, t, remaining) // 行军返回
		// 战斗后通知地图服务移除队列
		_ = s.mapSvc.RemoveMarch(ctx, march.ToX, march.ToY, marchKey)
	case data.TypeGather:
		// 进入采集状态，记录开始时间，更新状态并设置采集完成定时
		now := time.Now().Unix()
		march.Status = data.StatusGathering
		march.GatherStartTime = now
		march.ArriveTime = now + march.GatherDuration

		if err := s.troopSvc.Save(ctx, t); err != nil {
			log.Printf("save gather start error: %v", err)
			return
		}

		// 加入采集完成延迟任务
		task := MarchTask{
			Uid:      uid,
			MarchKey: marchKey,
			FinishAt: march.ArriveTime,
		}
		taskJSON, _ := json.Marshal(task)
		s.rdb.ZAdd(ctx, marchDelayQueueKey, &redis.Z{
			Score:  float64(now),
			Member: taskJSON,
		})

		select {
		case s.wakeUp <- struct{}{}:
		default:
		}
		log.Printf("User %d start gathering at (%d,%d), will finish in %ds", uid, march.ToX, march.ToY, march.GatherDuration)
	case data.TypeStation:
		// 驻扎需玩家手动无损返程
		s.createReturnMarchWithTroops(ctx, uid, marchKey, t, march.Troops)
		_ = s.mapSvc.RemoveMarch(ctx, march.ToX, march.ToY, marchKey)
	}
}

// handleGatherComplete 采集完成，结算资源并创建返程
func (s *MarchService) handleGatherComplete(ctx context.Context, uid int64, marchKey string, t *data.Troops) {
	march := t.Attack[marchKey]
	if march == nil || march.Status != data.StatusGathering {
		return
	}

	// 获取实际采集时长
	elapsed := s.mapSvc.GetGatherElapsed(ctx, march.ToX, march.ToY, marchKey)
	if elapsed == 0 {
		elapsed = march.GatherDuration
	}

	resourceAmount := s.calcGatherAmount(march.ToX, march.ToY, elapsed)

	// 通知地图服务采集完成，移除队列
	_ = s.mapSvc.RemoveMarch(ctx, march.ToX, march.ToY, marchKey)
	_ = s.createReturnMarch(ctx, uid, marchKey, t, resourceAmount)
}

// 返程到达，部队回归，如果有携带资源则加给玩家
func (s *MarchService) handleReturn(ctx context.Context, uid int64, marchKey string, t *data.Troops) {
	march, ok := t.Attack[marchKey]
	if !ok || march == nil || march.Status != data.StatusReturning {
		return
	}

	// 部队加回
	for k, v := range march.Troops {
		t.Troops[k] += v
	}

	//发放携带资源
	if march.CarryResources > 0 {
		user, err := s.userSvc.Load(ctx, uid)
		if err == nil {
			// 返程的 FromX,FromY 为资源点坐标，ToX,ToY 为主城坐标，获取资源类型
			resourceType := getResourceType(march.FromX, march.FromY)
			if user.Resources == nil {
				user.Resources = make(map[string]int)
			}

			user.Resources[resourceType] += march.CarryResources
			_ = s.userSvc.Save(ctx, user)
			log.Printf("User %d received %d %s from return march", uid, march.CarryResources, resourceType)
		}
	}

	// 移除行军记录
	delete(t.Attack, marchKey)
	if err := s.troopSvc.Save(ctx, t); err != nil {
		log.Printf("save gather start error: %v", err)
	}
}

// 战斗/驻扎后创建返程（无资源携带）
func (s *MarchService) createReturnMarchWithTroops(ctx context.Context, uid int64, marchKey string, t *data.Troops, troops map[string]int) {
	original := t.Attack[marchKey]
	if original == nil {
		return
	}

	// 临时修改剩余部队以生成正确的返程
	original.Troops = troops
	_ = s.createReturnMarch(ctx, uid, marchKey, t, 0)
}

// 战斗结算：损失一半（向下取整），返回剩余部队
func (s *MarchService) handleAttack(march *data.MarchAttack) map[string]int {
	remaining := make(map[string]int)
	for k, v := range march.Troops {
		lost := v / 2
		if v-lost > 0 {
			remaining[k] = v - lost
		}
	}
	return remaining
}

// 创建返程行军，resourceAmount 为携带的资源量
func (s *MarchService) createReturnMarch(ctx context.Context, uid int64, originalKey string, t *data.Troops, resourceAmount int) error {
	march := t.Attack[originalKey]
	if march == nil {
		return errors.New("original march not found")
	}

	// 返程时间同去程
	duration := CalcMarchDuration(march.ToX, march.ToY, march.FromX, march.FromY, 0)
	now := time.Now().Unix()
	returnArriveTime := now + int64(duration)

	returnKey := generateReturnKey(uid, originalKey)
	returnMarch := &data.MarchAttack{
		FromX:          march.ToX, // 资源点
		FromY:          march.ToY,
		ToX:            march.FromX, // 主城
		ToY:            march.FromY,
		Type:           march.Type,
		Status:         data.StatusReturning,
		StartTime:      now,
		ArriveTime:     returnArriveTime,
		RealArrive:     returnArriveTime,
		Troops:         march.Troops,
		CarryResources: resourceAmount,
	}

	// 移除去程/采集行军，写入返程
	delete(t.Attack, originalKey)
	t.Attack[returnKey] = returnMarch
	if err := s.troopSvc.Save(ctx, t); err != nil {
		return err
	}

	// 加入协程延迟任务
	task := MarchTask{
		Uid:      uid,
		MarchKey: returnKey,
		FinishAt: returnArriveTime,
	}
	taskJSON, _ := json.Marshal(task)
	s.rdb.ZAdd(ctx, marchDelayQueueKey, &redis.Z{
		Score:  float64(returnArriveTime),
		Member: taskJSON,
	})

	// 唤醒协程
	select {
	case s.wakeUp <- struct{}{}:
	default:
	}
	return nil
}

// 根据资源点坐标和已采集时长计算资源量（每秒 1 单位）
func (s *MarchService) calcGatherAmount(x, y int, elapsed int64) int {
	rate := 1 // 每秒采集 1 单位，可配置
	return rate
}

// HasActiveMarch 检查玩家是否有进行中的行军
func (s *MarchService) HasActiveMarch(ctx context.Context, uid int64) bool {
	t, err := s.troopSvc.Load(ctx, uid)
	if err != nil {
		return false
	}

	return len(t.Attack) > 0
}

func getResourceType(x, y int) string {
	switch (x + y) % 3 {
	case 0:
		return "r1"
	case 1:
		return "r2"
	default:
		return "r3"
	}
}

// 生成行军唯一标识
func generateMarchKey(uid int64, startTime int64) string {
	return fmt.Sprintf("march_%d_%d", uid, startTime)
}

func generateReturnKey(uid int64, originalKey string) string {
	return fmt.Sprintf("return_%d_%s_%d", uid, originalKey, time.Now().UnixNano())
}
