package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slg-serverD/cache"
	"slg-serverD/conf"
	"slg-serverD/data"
	"slg-serverD/db"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// 延迟队列key
const troopDelayQueueKey = "delay:queue:troops"

// 训练任务，存入redis中
type TrainTask struct {
	Uid        int64  `json:"uid"`
	SoldierKey string `json:"key"`
	Num        int    `json:"num"`
	FinishAt   int64  `json:"finish_at"`
}

type TroopsService struct {
	repo       *db.TroopsRepo
	cache      *cache.TroopsCache
	lockSvc    *LockService
	rdb        *redis.Client
	pushSvc    PushService
	troopsConf *conf.TroopsConfig
	ticker     *time.Ticker
	stop       chan struct{}
	wakeUp     chan struct{}
}

func NewTroopsService(repo *db.TroopsRepo, cache *cache.TroopsCache, lockSvc *LockService, rdb *redis.Client, pushSvc PushService, troopsConf *conf.TroopsConfig) *TroopsService {
	if pushSvc == nil {
		pushSvc = &defaultPushService{}
	}
	return &TroopsService{repo: repo, cache: cache, lockSvc: lockSvc, rdb: rdb, pushSvc: pushSvc, troopsConf: troopsConf, stop: make(chan struct{}), wakeUp: make(chan struct{})}
}

// StartQueueChecker 启动训练队列消费者（动态休眠版）
func (s *TroopsService) StartQueueChecker() {
	go func() {
		// 启动时从数据库重建一次延迟队列（防止 Redis 数据丢失）
		s.rebuildDelayQueue()

		for {
			fmt.Println("troops 定时任务执行, 时间:", time.Now().Unix())
			select {
			case <-s.stop:
				return
			default:
			}
			// 处理所有已到期的任务
			s.consumeTrainQueue()

			// 休眠直到下一个任务到期
			s.sleepUntilNextTask()
		}
	}()
}

func (s *TroopsService) Stop() {
	close(s.stop)
}

// Load 先缓存，后db（再缓存）
func (s *TroopsService) Load(ctx context.Context, uid int64) (*data.Troops, error) {
	t, err := s.cache.Get(ctx, uid)
	if err == nil {
		return t, nil
	}

	t, err = s.repo.GetTroops(ctx, uid)
	if err != nil {
		return nil, err
	}

	_ = s.cache.Set(ctx, t)
	return t, nil
}

// Save 先写 DB，再删缓存
func (s *TroopsService) Save(ctx context.Context, t *data.Troops) error {
	if t == nil {
		return fmt.Errorf("troops对象不能为空")
	}

	t.UpdateTime = time.Now().Unix()
	t.Version++
	err := s.repo.SaveTroops(ctx, t)
	if err != nil {
		return err
	}

	_ = s.cache.Del(ctx, t.Uid)
	return nil
}

// DeductTroops 扣除士兵（用于出征）
func (s *TroopsService) DeductTroops(ctx context.Context, uid int64, troops map[string]int) error {
	t, err := s.Load(ctx, uid)
	if err != nil {
		return err
	}

	for k, v := range troops {
		if _, ok := t.Troops[k]; !ok {
			return fmt.Errorf("部队异常！")
		}
		if v > t.Troops[k] {
			return fmt.Errorf("部队数量不足！")
		}
		t.Troops[k] -= v
		if t.Troops[k] <= 0 {
			delete(t.Troops, k)
		}
	}

	return s.Save(ctx, t)
}

// UpdateTroops 开始训练士兵（将任务加入队列）
// 需在外部完成资源、建筑等级等校验
func (s *TroopsService) UpdateTroops(ctx context.Context, uid int64, soldierKey string, num int) error {
	// 1. 校验兵种是否存在
	meta, ok := s.troopsConf.Soldiers[soldierKey]
	if !ok {
		return errors.New("unknown soldier type")
	}

	if num <= 0 {
		return errors.New("invalid number")
	}

	// 2. 加玩家锁
	unlock, err := s.lockSvc.Lock(ctx, UserLockKey(uid))
	if err != nil {
		return errors.New("operation conflict")
	}
	defer unlock()

	// 加载最新部队数据
	t, err := s.Load(ctx, uid)
	if err != nil {
		return err
	}

	// 兵种已经在训练中
	for _, v := range t.Queue {
		if v.Key == soldierKey {
			return errors.New("soldier is already in training")
		}
	}

	// 计算生产时间
	totalTime := meta.TrainTime * int64(num)
	finishAt := time.Now().Unix() + totalTime

	t.Queue = append(t.Queue, &data.SoldierQueue{
		Key:      soldierKey,
		Total:    num,
		FinishAt: finishAt,
	})

	// 6. 添加到 Redis 延迟队列
	task := TrainTask{
		Uid:        uid,
		SoldierKey: soldierKey,
		Num:        num,
		FinishAt:   finishAt,
	}
	jsonTask, err := json.Marshal(task)
	if err != nil {
		return err
	}

	err = s.Save(ctx, t)
	if err != nil {
		return fmt.Errorf("save troops err:%v", err)
	}

	s.rdb.ZAdd(ctx, troopDelayQueueKey, &redis.Z{
		Score:  float64(finishAt),
		Member: string(jsonTask),
	})

	// 7. 唤醒休眠的 goroutine（非阻塞发送）
	select {
	case s.wakeUp <- struct{}{}:
	default:
	}

	return err
}

// CancelTrain 取消正在训练的任务（简化：仅移除队列最后一项，实际应支持指定索引）
func (s *TroopsService) CancelTrain(ctx context.Context, uid int64, soldierKey string) error {
	// 加锁
	unlock, err := s.lockSvc.Lock(ctx, UserLockKey(uid))
	if err != nil {
		return errors.New("operation conflict")
	}
	defer unlock()

	t, err := s.Load(ctx, uid)
	if err != nil {
		return err
	}

	targetIndex := -1
	for k, v := range t.Queue {
		if v.Key == soldierKey {
			targetIndex = k
		}
	}

	if targetIndex != -1 {
		return errors.New("no such task")
	}

	t.Queue = append(t.Queue[:targetIndex], t.Queue[targetIndex+1:]...)
	return s.Save(ctx, t)
}

// checkAndCompleteTraining 定时扫描所有有队列的玩家，完成到期训练
func (s *TroopsService) checkAndCompleteTraining() {
	ctx := context.Background()
	troopsList, err := s.repo.GetTroopsWithActiveQueue(ctx)
	if err != nil {
		log.Printf("scan queue error: %v", err)
		return
	}

	now := time.Now().Unix()
	for _, t := range troopsList {
		// 用一个匿名函数包裹，使 defer 作用域为一次迭代
		func() {
			unlock, err := s.lockSvc.Lock(ctx, UserLockKey(t.Uid))
			if err != nil {
				return // 锁获取失败，直接返回
			}
			defer unlock() // 函数退出时自动解锁，无论中间多少个 return/continue

			freshT, err := s.Load(ctx, t.Uid)
			if err != nil {
				return
			}

			var newQueue []*data.SoldierQueue
			updated := false
			for _, q := range freshT.Queue {
				if q.FinishAt <= now {
					freshT.Troops[q.Key] += q.Total
					s.pushSvc.PushTroopTrainComplete(t.Uid, q.Key, q.Total)
					updated = true
				} else {
					newQueue = append(newQueue, q)
				}
			}

			if updated {
				freshT.Queue = newQueue
				if err := s.Save(ctx, freshT); err != nil {
					log.Printf("save troops uid=%d error: %v", t.Uid, err)
				}
			}
		}()
	}
}

func (s *TroopsService) consumeTrainQueue() {
	ctx := context.Background()
	now := time.Now().Unix()

	member, err := s.rdb.ZRangeByScore(ctx, troopDelayQueueKey, &redis.ZRangeBy{
		Min:   "0",
		Max:   strconv.FormatInt(now, 10),
		Count: 100,
	}).Result()
	if err != nil {
		log.Printf("rdb.ZRangeByScore error: %v", err)
		return
	}

	for _, m := range member {
		var task TrainTask
		if err := json.Unmarshal([]byte(m), &task); err != nil {
			s.rdb.ZRem(ctx, troopDelayQueueKey, m)
			log.Printf("unmarshal task error: %v", err)
			continue
		}

		// 使用闭包保证锁及时释放
		func() {
			unlock, err := s.lockSvc.Lock(ctx, UserLockKey(task.Uid))
			if err != nil {
				return // 锁获取失败，下个循环再试
			}
			defer unlock()
			t, err := s.Load(ctx, task.Uid)
			if err != nil {
				s.rdb.ZRem(ctx, troopDelayQueueKey, m)
				return
			}

			var newQueue []*data.SoldierQueue
			completed := false
			for _, q := range t.Queue {
				if q.Key == task.SoldierKey && q.Total == task.Num && q.FinishAt == q.FinishAt {
					// 训练完成
					t.Troops[q.Key] += q.Total
					s.pushSvc.PushTroopTrainComplete(t.Uid, q.Key, q.Total)
					completed = true
				} else {
					newQueue = append(newQueue, q)
				}
			}

			if completed {
				t.Queue = newQueue
				if err := s.Save(ctx, t); err != nil {
					log.Printf("save troops uid=%d error: %v", t.Uid, err)
					return
				}
			}
			s.rdb.ZRem(ctx, troopDelayQueueKey, m)
		}()
	}
}

func (s *TroopsService) sleepUntilNextTask() {
	ctx := context.Background()
	// 获取最小 score 及其成员（只取一个，不需要成员内容）
	result, err := s.rdb.ZRangeWithScores(ctx, troopDelayQueueKey, 0, 0).Result()
	if err != nil || len(result) == 0 {
		// 无任务，休眠 30 秒（可被 stop 中断）
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
}

// 从数据库重建延迟队列（仅启动时调用一次）
func (s *TroopsService) rebuildDelayQueue() {
	ctx := context.Background()
	troopsList, err := s.repo.GetTroopsWithActiveQueue(ctx)
	if err != nil {
		log.Printf("rebuild delay queue error: %v", err)
		return
	}
	for _, t := range troopsList {
		for _, q := range t.Queue {
			task := TrainTask{
				Uid:        t.Uid,
				SoldierKey: q.Key,
				Num:        q.Total,
				FinishAt:   q.FinishAt,
			}

			taskJson, err := json.Marshal(task)
			if err != nil {
				continue
			}

			s.rdb.ZAdd(ctx, troopDelayQueueKey, &redis.Z{
				Score:  float64(task.FinishAt),
				Member: taskJson,
			})
		}
	}

	log.Printf("rebuilt delay queue with %d tasks", len(troopsList))
}

func (s *TroopsService) GetAllActiveMarchs(ctx context.Context) (map[int64]map[string]*data.MarchAttack, error) {
	return s.repo.GetTroopsWithActiveMarch(ctx)
}
