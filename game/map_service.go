package game

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"slg-serverD/cache"
	"slg-serverD/conf"
	"slg-serverD/data"
	"slg-serverD/db"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

/*
地图核心逻辑（初始化、分片、行军、采集、刷新）
*/

type MapService struct {
	config  *conf.MapConfig
	cache   *cache.MapCache
	repo    *db.MapRepo
	rdb     *redis.Client
	lockSvc *LockService
	userSvc *UserService
	//bagSvc  *BagService
	marchSvc *MarchService

	shards [data.ShardCountX][data.ShardCountY]*data.MapShard // 分片管理器

	// 定时刷新控制
	stopRefresh chan struct{}
	stopSave    chan struct{}
}

func NewMapService(cfg *conf.MapConfig, cache *cache.MapCache, repo *db.MapRepo, rdb *redis.Client, lockSvc *LockService, userSvc *UserService) *MapService {
	ms := &MapService{
		config:  cfg,
		cache:   cache,
		repo:    repo,
		rdb:     rdb,
		lockSvc: lockSvc,
		userSvc: userSvc,
		//bagSvc:      bagSvc,
		stopRefresh: make(chan struct{}),
		stopSave:    make(chan struct{}),
	}

	for i := 0; i < data.ShardCountX; i++ {
		for j := 0; j < data.ShardCountY; j++ {
			ms.shards[i][j] = &data.MapShard{
				ShardX: i,
				ShardY: j,
				Cells:  make(map[int64]*data.MapCell),
			}
		}
	}

	log.Printf("Initialized %d shards (%d x %d)", data.ShardCountX*data.ShardCountY, data.ShardCountX, data.ShardCountY)

	return ms
}

// InitializeMap 服务启动时初始化地图（生成或加载）
func (ms *MapService) InitializeMap() error {
	// 尝试从 DB 加载已有地图（如果表不为空），否则按配置生成新地图
	// 全量生成
	fmt.Println("Initializing Map......")
	ctx := context.Background()

	// 检查地图是否已初始化
	count, err := ms.repo.CountCells(context.Background())
	if err != nil {
		return fmt.Errorf("check map cells: %w", err)
	}
	if count > 0 {
		log.Printf("Map already initialized (%d cells), loading from DB...", count)

		// 从数据库加载所有地块到分片内存（可选择预热热点地块）
		if err := ms.loadAllCells(ctx); err != nil {
			return fmt.Errorf("load map cells: %w", err)
		}
	} else {
		ms.generateMap()
	}

	// 启动定时保存脏地块
	go ms.periodicSave()

	// 启动资源/野怪刷新
	go ms.periodicRefresh()

	return nil
}

// getShard 根据坐标返回所属分片
func (ms *MapService) getShard1(x, y int) *data.MapShard {
	if x == 302 && y == 388 {
		fmt.Println("Getting shard ==========================   ", x, y, x/data.ShardWidth, y/data.ShardHeight)
		fmt.Println(ms.shards[x/data.ShardWidth][y/data.ShardHeight])
	}

	return ms.shards[x/data.ShardWidth][y/data.ShardHeight]
}

func (ms *MapService) getShard(x, y int) *data.MapShard {
	sx := x / data.ShardWidth
	sy := y / data.ShardHeight

	// 防御性检查：避免索引越界（理论上调用方已保证坐标合法）
	if sx < 0 || sx >= data.ShardCountX || sy < 0 || sy >= data.ShardCountY {
		return nil
	}

	shard := ms.shards[sx][sy]
	if shard == nil {
		fmt.Println(4)
		// 惰性初始化：若分片意外为 nil，立即创建
		ms.shards[sx][sy] = &data.MapShard{
			ShardX: sx,
			ShardY: sy,
			Cells:  make(map[int64]*data.MapCell),
		}

		shard = ms.shards[sx][sy]

		if x == 302 && y == 388 {
			log.Printf("Lazy-initialized shard (%d,%d)", sx, sy)
			fmt.Println("Getting shard ==========================   ", x, y, sx, sy)
			fmt.Println(ms.shards[sx][sy])
		}
	}

	return ms.shards[sx][sy]
}

// LoadOrCreateCell 加载地块（优先缓存→Redis→生成模板）
func (ms *MapService) LoadOrCreateCell(ctx context.Context, x, y int) (*data.MapCell, error) {
	fmt.Println("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	if !isValidCoord(x, y) {
		return nil, errors.New("invalid coordinates")
	}

	fmt.Println("Getting cell222222222 =============================  ", x, y)
	shard := ms.getShard(x, y)
	fmt.Println("+++++++++++++")
	fmt.Println(ms.shards)

	id := data.GenCellID(x, y)
	fmt.Println("LoadingLoadingLoadingLoadingLoadingLoadingLoadingLoadingLoadingLoading cell =     ", id)

	shard.Mu.RLock()
	cell, exists := shard.Cells[id]
	shard.Mu.RUnlock()
	if exists {
		return cell, nil
	}

	// 查 Redis
	cell, err := ms.cache.Get(ctx, x, y)
	if err == nil {
		shard.Mu.Lock()
		shard.Cells[id] = cell
		shard.Mu.Unlock()
		return cell, nil
	}

	// 从 DB 查（省略），若无则按模板生成（已在 generateMap 中预生成）
	return nil, errors.New("cell not found")
}

// generateMap 按配置生成全部地块（仅首次调用）
func (ms *MapService) generateMap() {
	var wg sync.WaitGroup

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	// 生成障碍（随机）
	obstacleCount := int(float64(ms.config.Width*ms.config.Height) * ms.config.ObstacleRatio)
	for i := 0; i < obstacleCount; i++ {
		x := rng.Intn(ms.config.Width)
		y := rng.Intn(ms.config.Height)
		cell := &data.MapCell{
			X:     x,
			Y:     y,
			ID:    data.GenCellID(x, y),
			MType: data.CellTypeObstacle,
			Level: 1,
		}
		ms.saveGeneratedCell(cell, &wg)
	}

	// 2. 资源地块（Poisson 采样简化：随机放置，检查最小间距）
	for _, resource := range ms.config.Resources {
		placed := 0
		for placed < resource.Count {
			x := rng.Intn(ms.config.Width)
			y := rng.Intn(ms.config.Height)

			// 检查与已放置资源的最小距离
			//tooClose := false

			// 简化为随机放置，忽略距离检查（生产用 KD-Tree）
			/*if tooClose == true {
				continue
			}*/

			level := resource.MinLevel + rand.Intn(resource.MaxLevel-resource.MinLevel+1)
			cell := &data.MapCell{
				X:     x,
				Y:     y,
				ID:    data.GenCellID(x, y),
				MType: data.CellTypeResource,
				Level: level,
				Data: map[string]interface{}{
					"total":      level * 10000,
					"remaining":  level * 10000,
					"max_gather": 5,
					"refresh_at": 0,
				},
			}

			ms.saveGeneratedCell(cell, &wg)
			placed++
		}
	}

	// 3. 野怪（类似）
	for _, mon := range ms.config.Monsters {
		for i := 0; i < mon.Count; i++ {
			x := rng.Intn(ms.config.Width)
			y := rng.Intn(ms.config.Height)
			level := mon.MinLevel + rand.Intn(mon.MaxLevel-mon.MinLevel+1)
			cell := &data.MapCell{
				X:     x,
				Y:     y,
				ID:    data.GenCellID(x, y),
				MType: data.CellTypeMonster,
				Level: level,
				Data: map[string]interface{}{
					"hp":      level * 500,
					"max_hp":  level * 500,
					"respawn": mon.Respawn,
				},
			}

			ms.saveGeneratedCell(cell, &wg)
		}
	}

	wg.Wait()
}

func (ms *MapService) saveGeneratedCell(cell *data.MapCell, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		// 先存 Redis，再写 DB 异步
		ctx := context.Background()
		_ = ms.cache.Set(ctx, cell)

		// 放入分片内存
		shard := ms.getShard(cell.X, cell.Y)
		shard.Mu.Lock()
		shard.Cells[cell.ID] = cell
		shard.Mu.Unlock()

		if err := ms.repo.SaveCell(ctx, cell); err != nil {
			log.Printf("save cell (%d,%d) error: %v", cell.X, cell.Y, err)
		}
	}()
}

// RegisterMarch 将一个行军注册到目标地块的队列中
func (ms *MapService) RegisterMarch(ctx context.Context, x, y int, item *data.MarchQueueItem) error {
	cell, err := ms.LoadOrCreateCell(ctx, x, y)
	if err != nil {
		return err
	}

	cell.Mu.Lock()
	defer cell.Mu.Unlock()

	cell.Queue = append(cell.Queue, item)
	// 标记脏并更新 Redis
	cell.IsDirty = true
	_ = ms.cache.Set(ctx, cell)
	return nil
}

// OnMarchArrive 处理行军到达，如果到达部队是队列中的第一个，返回 true 表示可立即行动
func (ms *MapService) OnMarchArrive(ctx context.Context, x, y int, marchKey string) (bool, error) {
	cell, err := ms.LoadOrCreateCell(ctx, x, y)
	if err != nil {
		return false, err
	}

	cell.Mu.Lock()
	defer cell.Mu.Unlock()

	// 清理已过时的队列项（到达时间超过当前时间很久但未出队的异常项）
	now := time.Now()
	var validQueue []*data.MarchQueueItem
	for _, item := range cell.Queue {
		// 判断：到达时间 是否在 10 分钟之前（已超时）
		if item.ArriveAt.Before(now.Add(-10 * time.Minute)) {
			continue
		}

		validQueue = append(validQueue, item)
	}

	cell.Queue = validQueue

	// 查找当前行军是否在队列中，并且必须是第一个
	for i, item := range cell.Queue {
		if item.MarchKey == marchKey {
			if i == 0 {
				// 队首，可以行动
				return true, nil
			}
			// 等待
			return false, nil
		}
	}
	return false, errors.New("march not found in queue")
}

// RemoveMarch 从地块队列中移除指定行军
func (ms *MapService) RemoveMarch(ctx context.Context, x, y int, marchKey string) error {
	cell, err := ms.LoadOrCreateCell(ctx, x, y)
	if err != nil {
		return err
	}

	cell.Mu.Lock()
	defer cell.Mu.Unlock()

	for i, item := range cell.Queue {
		if item.MarchKey == marchKey {
			cell.Queue = append(cell.Queue[:i], cell.Queue[i+1:]...)
			cell.IsDirty = false
			_ = ms.cache.Set(ctx, cell)
			return nil
		}
	}

	return errors.New("march not found")
}

// GetGatherElapsed 获取指定行军的已采集时长（秒），用于提前召回计算
func (ms *MapService) GetGatherElapsed(ctx context.Context, x, y int, marchKey string) int64 {
	cell, err := ms.LoadOrCreateCell(ctx, x, y)
	if err != nil {
		return 0
	}

	cell.Mu.Lock()
	defer cell.Mu.Unlock()

	for _, item := range cell.Queue {
		if item.MarchKey == marchKey && item.Type == data.TypeGather {
			elapsed := time.Since(item.ArriveAt)
			if elapsed > 0 {
				return int64(elapsed)
			}
			return 0
		}
	}

	return 0
}

// periodicSave 定期将脏地块写入 DB
func (ms *MapService) periodicSave() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.flushDirtyCells()
		case <-ms.stopSave:
			ms.flushDirtyCells()
			return
		}
	}
}

func (ms *MapService) flushDirtyCells() {
	var cells []*data.MapCell
	for i := 0; i < data.ShardCountX; i++ {
		for j := 0; j < data.ShardCountY; j++ {
			shard := ms.getShard(i, j)
			shard.Mu.Lock()
			for _, cell := range shard.Cells {
				if cell.IsDirty == true {
					cells = append(cells, cell)
				}
			}
			shard.Mu.Unlock()
		}
	}

	if len(cells) > 0 {
		err := ms.repo.BatchSave(context.Background(), cells)
		if err == nil {
			for _, cell := range cells {
				cell.IsDirty = false
			}
		}
	}
}

// periodicRefresh 资源恢复、野怪重生
func (ms *MapService) periodicRefresh() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.refreshResources()
			ms.respawnMonsters()
		case <-ms.stopRefresh:
			return
		}
	}
}

func (ms *MapService) refreshResources() {
	// 遍历 Redis 中标记枯竭的资源 key，检查 refresh_at 是否到期，重置 remaining

}

func (ms *MapService) respawnMonsters() {

}

func (ms *MapService) Stop() {
	close(ms.stopSave)
	close(ms.stopRefresh)
}

// RelocateCity 玩家搬家核心方法
func (ms *MapService) RelocateCity(ctx context.Context, uid int64, toX, toY int) error {
	// 坐标是否合法
	if !isValidCoord(toX, toY) {
		return errors.New("invalid target coordinates")
	}

	// 玩家状态校验（冷却、战斗、行军等）
	if ms.userSvc.CanRelocate(ctx, uid) != nil {
		return errors.New("user not relocated")
	}

	// 检查是否有活跃行军（通过 marchSvc 或 troops 判断）
	// 简单做法：通过 marchSvc 查询该玩家是否有进行中的行军
	// 这里简化：调用 marchSvc.HasActiveMarch(ctx, uid)
	if ms.marchSvc != nil && ms.marchSvc.HasActiveMarch(ctx, uid) {
		return errors.New("cannot relocate while troops are marching")
	}

	// 消耗迁城道具
	/*if err := ms.bagSvc.UseItem(ctx, uid, data.ItemRelocate, 1); err != nil {
		return errors.New("insufficient relocate item")
	}*/

	// 获取玩家当前坐标
	u, err := ms.userSvc.Load(ctx, uid)
	if err != nil {
		return err
	}

	fromX, fromY := u.CityX, u.CityY

	// 同时锁定锁定源坐标、目标坐标、玩家UID
	lockKeys := []string{
		fmt.Sprintf("lock:cell:%d:%d", fromX, fromY),
		fmt.Sprintf("lock:cell:%d:%d", toX, toY),
		UserLockKey(uid),
	}

	unlock, err := ms.lockSvc.LockMulti(ctx, lockKeys...)
	if err != nil {
		return err
	}
	defer unlock()

	// 加载地块最新数据
	oldCell, err := ms.LoadOrCreateCell(ctx, fromX, fromY)
	if err != nil {
		return err
	}

	newCell, err := ms.LoadOrCreateCell(ctx, toX, toY)
	if err != nil {
		return err
	}

	// 二次验证归属和可占领状态
	if oldCell.Owner != uid {
		return errors.New("source cell not owned by you")
	}

	if !newCell.IsOccupiable() {
		return errors.New("target cell is not available")
	}

	// 备份 newCell 原始状态
	backupNew := struct {
		MType    int
		Owner    int64
		Name     string
		Power    int64
		Rank     int
		Alliance int64
		Protect  int64
	}{
		MType:    newCell.MType,
		Owner:    newCell.Owner,
		Name:     newCell.Name,
		Power:    newCell.Power,
		Rank:     newCell.Rank,
		Alliance: newCell.Alliance,
		Protect:  newCell.Protect,
	}

	// 修改地块状态（内存对象）
	oldCell.MType = 0
	oldCell.Owner = 0
	oldCell.Name = ""
	oldCell.Power = 0
	oldCell.Rank = 0
	oldCell.Alliance = 0
	oldCell.Protect = 0
	oldCell.IsDirty = true

	// 设置新地块
	newCell.MType = data.CellTypeUser
	newCell.Owner = uid
	newCell.Name = u.Name
	newCell.Power = u.Power
	newCell.Rank = 0
	newCell.Alliance = u.AllianceId
	newCell.Protect = 0
	newCell.IsDirty = true

	// 执行事务（更新用户坐标、保存两个地块）
	err = ms.repo.RelocateTx(ctx, uid, toX, toY, oldCell, newCell)
	if err != nil {
		// 事务失败，回滚内存状态（旧地块恢复）
		oldCell.Owner = uid
		oldCell.Name = u.Name
		oldCell.Power = u.Power
		oldCell.Alliance = u.AllianceId
		oldCell.Protect = 0
		oldCell.IsDirty = false

		// 注意：newCell 状态无需显式回滚，因为未被其他操作修改
		newCell.MType = backupNew.MType
		newCell.Owner = backupNew.Owner
		newCell.Name = backupNew.Name
		newCell.Power = backupNew.Power
		newCell.Alliance = backupNew.Alliance
		newCell.Protect = backupNew.Protect
		newCell.IsDirty = false

		return err
	}

	// 更新缓存（先删后设，或直接设最新）
	_ = ms.cache.Set(ctx, oldCell)
	_ = ms.cache.Set(ctx, newCell)

	_ = ms.userSvc.cache.Del(ctx, uid)

	return nil
}

// isValidCoord 验证坐标在地图范围内
func isValidCoord(x, y int) bool {
	return x >= 0 && x <= data.MapWidth && y >= 0 && y <= data.MapHeight
}

func (ms *MapService) loadAllCells(ctx context.Context) error {
	cells, err := ms.repo.LoadAllCells(ctx)
	if err != nil {
		return err
	}

	for _, cell := range cells {
		cell.Queue = make([]*data.MarchQueueItem, 0)
		// 放入内存切片
		shard := ms.getShard(cell.X, cell.Y)
		shard.Mu.Lock()
		shard.Cells[cell.ID] = cell
		shard.Mu.Unlock()

		// 热点地块（有主、资源、野怪等）写入 Redis 缓存
		if cell.MType != 0 {
			//fmt.Println("cell =================>>>>>>>>>>>    ", cell.X, " : ", cell.Y)
			_ = ms.cache.Set(ctx, cell)
		}
	}

	shard := ms.shards[7][9] // 需将 shards 字段改为导出或提供 GetShard 方法
	shard.Mu.RLock()
	count := len(shard.Cells)
	shard.Mu.RUnlock()
	log.Printf("Shard (7,9) contains %d cells", count)

	return nil
}
