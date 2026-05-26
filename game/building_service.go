package game

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slg-serverD/cache"
	"slg-serverD/conf"
	"slg-serverD/data"
	"slg-serverD/db"
	"time"
)

type BuildingService struct {
	repo         *db.BuildingRepo
	cache        *cache.BuildingCache
	lockSvc      *LockService         // 分布式锁服务
	pushSvc      PushService          // 推送服务
	buildingConf *conf.BuildingConfig // 建筑配置
	ticker       *time.Ticker
	stop         chan struct{}
}

func NewBuildingService1(repo *db.BuildingRepo, cache *cache.BuildingCache, lockSev *LockService, buildingConf *conf.BuildingConfig, pushSvc PushService) *BuildingService {
	if pushSvc == nil {
		pushSvc = &defaultPushService{}
	}
	return &BuildingService{repo: repo, cache: cache, lockSvc: lockSev, buildingConf: buildingConf, stop: make(chan struct{}), pushSvc: pushSvc}
}

func NewBuildingService(repo *db.BuildingRepo, cache *cache.BuildingCache,
	lockSvc *LockService, buildingConf *conf.BuildingConfig, pushSvc PushService) *BuildingService {
	if pushSvc == nil {
		pushSvc = &defaultPushService{} // 默认日志推送
	}
	return &BuildingService{
		repo:         repo,
		cache:        cache,
		lockSvc:      lockSvc,
		pushSvc:      pushSvc,
		buildingConf: buildingConf,
		stop:         make(chan struct{}),
	}
}

// StartQueueChecker 启动定时扫描协程（建议在 main 中调用）
func (s *BuildingService) StartQueueChecker(interval time.Duration) {
	s.ticker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case t := <-s.ticker.C:
				fmt.Println("定时任务执行, 时间:", t)
				s.checkAndCompleteQueues()
			case <-s.stop:
				fmt.Println("收到停止信号")
				s.ticker.Stop()
				return
			}
		}
	}()
}

// 优雅停止定时扫描
func (s *BuildingService) Stop() {
	close(s.stop)
}

// Load 获取玩家建筑数据（缓存优先）
func (s *BuildingService) Load(ctx context.Context, uid int64) (*data.Building, error) {
	b, err := s.cache.Get(ctx, uid)
	if err == nil {
		return b, nil
	}

	b, err = s.repo.GetBuilding(ctx, uid)
	if err != nil {
		// 初始化数据
		ib := &data.Building{
			Uid:       uid,
			Info:      make(map[string]int),
			Queue:     make(map[string]int64),
			UpdatedAt: time.Now().Unix(),
		}
		_ = s.repo.SaveBuilding(ctx, ib)
		return nil, err
	}

	_ = s.cache.Set(ctx, b)
	return b, nil
}

// Save 保存建筑数据（先写DB，后删缓存）
func (s *BuildingService) Save(ctx context.Context, b *data.Building) error {
	if err := s.repo.SaveBuilding(ctx, b); err != nil {
		return err
	}

	_ = s.cache.Del(ctx, b.Uid)
	return nil
}

// Upgrade 开始升级建筑（将建筑加入建造队列）
func (s *BuildingService) Upgrade(ctx context.Context, uid int64, buildingId string) error {
	// 1. 校验建筑是否有效
	meta, ok := s.buildingConf.Buildings[buildingId]
	if !ok {
		return errors.New("unknown building id")
	}

	// 2. 加玩家锁，防止并发修改
	unlock, err := s.lockSvc.Lock(ctx, UserLockKey(uid))
	if err != nil {
		return errors.New("operation conflict, please retry")
	}
	defer unlock()

	// 3. 加载最新数据
	b, err := s.Load(ctx, uid)
	if err != nil {
		return errors.New("build data error")
	}

	log.Printf("Upgrade request: uid=%d, building=%s", uid, buildingId)

	// 4. 检查队列中是否已在升级
	if finishAt, exists := b.Queue[buildingId]; exists {
		if finishAt > time.Now().Unix() {
			return errors.New("building is already upgrading")
		}

		// 已过期但未清理，先移除
		delete(b.Queue, buildingId)
	}

	// 5. 检查等级上限
	if b.Info[buildingId] >= meta.MaxLevel {
		return errors.New("max level reached")
	}

	// 6. 计算升级耗时（可结合等级系数，公式可自由修改）
	upgradeDuration := meta.BaseTime + int64(b.Info[buildingId])*2 // 示例公式
	finishAt := time.Now().Unix() + upgradeDuration

	// 7. 加入建造队列
	b.Queue[buildingId] = finishAt

	log.Printf("Current levels: %+v", b.Info)

	return s.Save(ctx, b)
}

// 定时扫描所有有队列的玩家，完成到期的建筑升级
func (s *BuildingService) checkAndCompleteQueues() {
	ctx := context.Background()
	// 一次性查出所有队列非空的记录（若玩家量巨大可分页或使用游标）
	buildings, err := s.repo.GetBuildingsWithActiveQueue(ctx)
	if err != nil {
		log.Printf("check building queues error: %v", err)
		return
	}

	now := time.Now().Unix()
	for _, b := range buildings {
		// 对每个玩家加锁，避免与在线请求冲突
		unlock, err := s.lockSvc.Lock(ctx, UserLockKey(b.Uid))
		if err != nil {
			// 锁失败，跳过本次，下次重试
			continue
		}

		// 加锁后重新加载最新数据，防止覆盖其它操作
		freshB, err := s.Load(ctx, b.Uid)
		if err != nil {
			unlock()
			continue
		}

		updated := false
		for bid, finishAt := range freshB.Queue {
			// 升级完成，时间结束
			if finishAt <= now {
				freshB.Info[bid]++
				delete(freshB.Queue, bid)

				updated = true

				// 推送客户端
				s.pushSvc.PushBuildingUpgrade(b.Uid, bid, freshB.Info[bid])
			}
		}

		// 跟新数据
		if updated == true {
			if err := s.Save(ctx, freshB); err != nil {
				log.Printf("save building for uid=%d error: %v", b.Uid, err)
			}
		}
		unlock()
	}
}
