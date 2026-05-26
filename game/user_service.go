package game

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"slg-serverD/cache"
	"slg-serverD/data"
	"slg-serverD/db"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repo     *db.UserRepo
	cache    *cache.UserCache
	lockSvc  *LockService
	troopSvc *TroopsService
}

// 玩家状态常量
const (
	UserStatusIdle      = 0
	UserStatusMarching  = 1 // 有行军队列
	UserStatusGathering = 2 // 有采集部队
	UserStatusInBattle  = 3 // 战斗中（可扩展）
)

func NewUserService(repo *db.UserRepo, cache *cache.UserCache, lockSvc *LockService) *UserService {
	return &UserService{repo: repo, cache: cache, lockSvc: lockSvc}
}

// Load 先查缓存，miss 则回源 DB 并写缓存
func (s *UserService) Load(ctx context.Context, uid int64) (*data.User, error) {
	u, err := s.cache.Get(ctx, uid)
	if err == nil {
		return u, nil
	}

	// 缓存未命中
	u, err = s.repo.GetUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, u)
	return u, nil
}

/*func Load(ctx context.Context, uid int64, s *UserService) (*data.User, error) {
	u, err := s.repo.GetUser(ctx, uid)
	return u, err
}*/

// Save 先写 DB，成功后删除缓存，下次读取自动重建。
// 遵循 Cache-Aside 模式，避免并发写导致缓存不一致。
func (s *UserService) Save(ctx context.Context, u *data.User) error {
	if err := s.repo.SaveUser(ctx, u); err != nil {
		return err
	}

	_ = s.cache.Set(ctx, u)
	return nil
}

// AddGold 增加金币（负数则扣除），业务上会配合锁使用
func (s *UserService) AddGold(ctx context.Context, uid int64, amount int) error {
	u, err := s.Load(ctx, uid)
	if err != nil {
		return err
	}

	if u.Gold+amount < 0 {
		return errors.New("insufficient gold")
	}

	u.Gold += amount
	return s.Save(ctx, u)
}

func (s *UserService) AddGoldWithLock(ctx context.Context, uid int64, amount int) error {
	// 1. 获取玩家锁，锁的 key 为 "lock:user:{uid}"
	lockKey := fmt.Sprintf("lock:user:%d", uid)
	unlock, err := s.lockSvc.Lock(ctx, lockKey)
	if err != nil {
		return errors.New("operation conflict, please retry")
	}

	defer unlock()

	// 2. 在锁保护下执行读取 -> 修改 -> 写入
	u, err := s.Load(ctx, uid)
	if err != nil {
		return err
	}

	if u.Gold+amount < 0 {
		return errors.New("insufficient gold")
	}

	u.Gold += amount
	return s.Save(ctx, u)
}

// Register 用户注册，返回创建的用户信息
func (s *UserService) Register(ctx context.Context, name, password string) (*data.User, error) {
	// 参数校验
	if len(name) <= 2 || len(name) > 20 {
		return nil, errors.New("name length must be between 2 and 20")
	}

	if len(password) < 6 || len(password) > 32 {
		return nil, errors.New("password length must be between 6 and 32")
	}

	fmt.Println(name, password)
	// 检查用户名是否已存在（通过 repo）
	existUser, _ := s.repo.GetUserByName(ctx, name)
	if existUser != nil {
		return nil, errors.New("username already exists")
	}

	// 生成密码哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("password hash failed")
	}

	// 分配 UID（可用雪花算法或数据库自增，这里简化用时间戳+随机数）
	// 生产环境建议使用雪花算法（如 github.com/bwmarrin/snowflake）
	uid := s.generateUID()

	// 自动分配不重复的出生坐标
	cityX, cityY, err := s.assignCoordinate(ctx)

	// 创建用户结构
	user := &data.User{
		Uid:          uid,
		Name:         name,
		PasswordHash: string(hashedPassword),
		Level:        1,
		Gold:         1000,
		CityX:        cityX,
		CityY:        cityY,
		Resources:    make(map[string]int),
		//ServerId:     serverId,
	}

	// 背包
	bag := &data.Bag{
		Uid:  uid,
		Info: make(map[string]int),
	}

	building := &data.Building{
		Uid:   uid,
		Info:  map[string]int{"b1": 0, "b2": 0, "b3": 0, "b4": 0, "b5": 0, "b6": 0},
		Queue: make(map[string]int64),
	}

	troops := &data.Troops{
		Uid:        uid,
		Troops:     map[string]int{"a10001": 10, "a10002": 10, "a10003": 10, "a10004": 10},
		Damaged:    make(map[string]int),
		Attack:     make(map[string]*data.MarchAttack),
		Queue:      make([]*data.SoldierQueue, 0),
		UpdateTime: time.Now().Unix(),
		Version:    1,
	}
	fmt.Println(building)

	// 事务写入数据库
	if err := s.repo.CreateUser(ctx, user, bag, building, troops); err != nil {
		return nil, errors.New("create user failed")
	}

	// 写入缓存（可异步）
	_ = s.cache.Set(ctx, user)
	return user, nil
}

func (s *UserService) Authenticate(ctx context.Context, username, password string) (*data.User, error) {
	// 从数据库通过用户名查询用户
	user, err := s.repo.GetUserByName(ctx, username)
	if err != nil {
		return nil, err
	}

	// 验证 bcrypt 密码哈希
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) assignCoordinate(ctx context.Context) (int, int, error) {
	const maxAttempts = 100
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	// 假设地图范围 0-1000
	minX, maxX := 0, 1000
	minY, maxY := 0, 1000

	for i := 0; i < maxAttempts; i++ {
		x := rng.Intn(maxX-minX+1) + minX
		y := rng.Intn(maxY-minY+1) + minY

		occupied, err := s.repo.IsCoordinateOccupied(ctx, x, y)
		if err != nil {
			return 0, 0, err
		}
		if !occupied {
			return x, y, nil
		}
	}
	return 0, 0, errors.New("no available coordinate")
}

// generateUID 生成唯一UID（简化版，生产应使用雪花算法）
func (s *UserService) generateUID() int64 {
	// 推荐 github.com/bwmarrin/snowflake
	return time.Now().UnixNano()/1000 + rand.Int63n(10000)
}

func (s *UserService) SyncCrossData(ctx context.Context, uid int64, troops map[string]int, resources map[string]int) error {
	unlock, err := s.lockSvc.Lock(ctx, UserLockKey(uid))
	if err != nil {
		return err
	}
	defer unlock()

	u, err := s.Load(ctx, uid)
	if err != nil {
		return err
	}

	// 更新部队
	if troops != nil {

	}

	if resources != nil {
		for k, v := range resources {
			u.Resources[k] += v
		}
	}

	return s.Save(ctx, u)
}

// CanRelocate 检查玩家是否可以搬迁
func (s *UserService) CanRelocate(ctx context.Context, uid int64) error {
	u, err := s.Load(ctx, uid)
	if err != nil {
		return err
	}

	// 冷却检查
	if u.LastRelocateTime > time.Now().Unix() {
		return errors.New("relocate cooldown (h)")
	}

	t, err := s.troopSvc.Load(ctx, u.Uid)
	if err != nil {
		return err
	}

	// 如果有任何行军（attack 非空），则不能搬迁
	if t.Attack != nil {
		return errors.New("attack is not empty")
	}

	return nil
}
