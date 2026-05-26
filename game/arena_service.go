// -----------------------------------------------------------
// 模块:
// 功能:
// 作者: zteng
// 创建: 2026/5/26 20:31
// 文件: arena_service.go
// 版权: 仅限内部项目使用
// -----------------------------------------------------------
package game

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"slg-serverD/data"
	"slg-serverD/db"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

type ArenaService struct {
	rdb      *redis.Client
	userSvc  *UserService
	lockSvc  *LockService
	userRepo *db.UserRepo
}

func NewArenaService(rdb *redis.Client, userSvc *UserService, lockSvc *LockService) *ArenaService {
	return &ArenaService{
		rdb:     rdb,
		userSvc: userSvc,
		lockSvc: lockSvc,
	}
}

// 战力变化时更新或登录注册时更新
func (s *ArenaService) UpdatePower(ctx context.Context, uid int64, newPower int64) {
	// 直接更新该成员的 score（战力），排名自动调整
	s.rdb.ZAdd(ctx, data.ArenaRankKey, &redis.Z{
		Score:  float64(newPower),
		Member: uid,
	})
}

// InitRank 初始化排名（仅首次启动时调用，根据战力排序）
func (s *ArenaService) InitRank(ctx context.Context) error {
	exists, _ := s.rdb.Exists(ctx, data.ArenaRankKey).Result()
	if exists > 0 {
		return nil
	}

	const pageSize = 1000 // 每批处理1000条
	rank := 1
	for offset := 0; ; offset += pageSize {
		users, err := s.userRepo.GetUsersByPower(ctx, offset, pageSize)
		if err != nil {
			return err
		}
		if len(users) == 0 {
			break
		}
		// 批量写入本批次的排名
		pipe := s.rdb.Pipeline()
		for _, u := range users {
			pipe.ZAdd(ctx, data.ArenaRankKey, &redis.Z{
				Score:  float64(rank),
				Member: u.Uid,
			})
			rank++
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// GetPanel 获取竞技场面板
func (s *ArenaService) GetPanel(ctx context.Context, uid int64) (*data.ArenaPanel, error) {
	// 获取玩家排名和战力
	rank, err := s.getRank(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get rank11: %w", err)
	}

	user, err := s.userSvc.Load(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}

	merit := user.ArenaMerit
	targets, err := s.getTargets(ctx, uid, rank)
	return &data.ArenaPanel{
		MyRank:  rank,
		MyPower: user.Power,
		Merit:   merit,
		Targets: targets,
	}, nil
}

// Challenge 执行挑战
func (s *ArenaService) Challenge(ctx context.Context, uid int64, targetUid int64) (*data.ArenaChallengeResult, error) {
	// 加锁
	unlock, err := s.lockSvc.Lock(ctx, UserLockKey(uid))
	if err != nil {
		return nil, fmt.Errorf("lock user: %w", err)
	}
	defer unlock()

	// 检查挑战次数
	chances, err := s.getDailyChances(ctx, uid)
	if err != nil {
		return nil, err
	}

	if chances <= 0 {
		return nil, fmt.Errorf("get chances: %w", err)
	}

	// 检查目标是否合法（排名比自己高，且在可挑战范围内）
	myRank, err := s.getRank(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get rank: %w", err)
	}
	targetRank, err := s.getRank(ctx, targetUid)
	if err != nil {
		return nil, fmt.Errorf("get rank: %w", err)
	}
	if targetRank >= myRank {
		//return nil, errors.New("can only challenge higher rank")
	}

	// 消耗挑战次数
	if err := s.consumeChance(ctx, uid); err != nil {
		return nil, err
	}

	// 获取双方战力
	user, err := s.userSvc.Load(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	target, err := s.userSvc.Load(ctx, targetUid)
	if err != nil {
		return nil, fmt.Errorf("load target: %w", err)
	}

	// 胜利失败
	// 战力判断
	victory := s.calcVictory(user.Power, target.Power)

	// 结果
	result := &data.ArenaChallengeResult{
		Victory: victory,
		NewRank: myRank,
		Merit:   user.ArenaMerit,
	}

	if victory {
		// 交换排名
		pipe := s.rdb.Pipeline()
		pipe.ZAdd(ctx, data.ArenaRankKey, &redis.Z{
			Score:  float64(targetRank),
			Member: targetUid,
		})
		pipe.ZAdd(ctx, data.ArenaRankKey, &redis.Z{
			Score:  float64(myRank),
			Member: uid,
		})
		if _, err := pipe.Exec(ctx); err != nil {
			return nil, fmt.Errorf("pipe exec: %w", err)
		}

		result.NewRank = targetRank

		// 增加军工
		user.ArenaMerit += data.ArenaMeritReward
		if err := s.userSvc.Save(ctx, user); err != nil {
			return nil, fmt.Errorf("save user: %w", err)
		}

		result.Merit = user.ArenaMerit
		result.Reward = data.ArenaMeritReward
	} else {
		// 失败排名不变
		result.NewRank = myRank
	}
	return result, nil
}

// BuyChance 金币购买挑战次数
func (s *ArenaService) BuyChance(ctx context.Context, uid int64) error {
	unlock, err := s.lockSvc.Lock(ctx, UserLockKey(uid))
	if err != nil {
		return fmt.Errorf("lock user: %w", err)
	}
	defer unlock()

	// 检查今天已购买的次数
	buyCnt, err := s.getDailyBuyCount(ctx, uid)
	if err != nil {
		return fmt.Errorf("get chances: %w", err)
	}

	if buyCnt >= data.ArenaBuyChancesMax {
		return fmt.Errorf("buy chances max %d", data.ArenaBuyChancesMax)
	}

	// 扣除金币
	if err := s.userSvc.AddGoldWithLock(ctx, uid, -data.ArenaBuyChancesCost); err != nil {
		return err
	}

	// 增加购买次数记录
	dataStr := time.Now().Format("20060102")
	buyKey := data.ArenaBuyCntKeyPrefix + strconv.FormatInt(uid, 10) + ":" + dataStr
	if _, err = s.rdb.Incr(ctx, buyKey).Result(); err != nil {
		return fmt.Errorf("incr buy count: %w", err)
	}
	s.rdb.Expire(ctx, buyKey, time.Hour*48)

	// 加挑战次数
	chancesKey := data.ArenaChancesKeyPrefix + strconv.FormatInt(uid, 10) + ":" + dataStr
	if _, err = s.rdb.Incr(ctx, chancesKey).Result(); err != nil {
		return fmt.Errorf("incr chances: %w", err)
	}
	s.rdb.Expire(ctx, chancesKey, time.Hour*48)
	return nil
}

// getRank 获取玩家排名
func (s *ArenaService) getRank(ctx context.Context, uid int64) (int, error) {
	fmt.Println(uid)
	rank, err := s.rdb.ZRank(ctx, data.ArenaRankKey, strconv.FormatInt(uid, 10)).Result()
	fmt.Println(err)
	if err != nil {
		return 0, err
	}
	return int(rank) + 1, nil
}

// getTargets 获取可挑战的5个目标
func (s *ArenaService) getTargets(ctx context.Context, uid int64, myRank int) ([]*data.ArenaTarget, error) {
	// 根据排名区间决定拉取范围
	var start, end int64
	if myRank <= 5 {
		start, end = 0, 5 // 排名1-6 (索引0~5)
	} else {
		start, end = int64(myRank-6), int64(myRank-1) // 前面5名
	}

	members, err := s.rdb.ZRangeWithScores(ctx, data.ArenaRankKey, start, end).Result()
	if err != nil {
		return nil, err
	}

	targets := make([]*data.ArenaTarget, 0, 5)
	for _, z := range members {
		targetUid, _ := strconv.ParseInt(z.Member.(string), 10, 64)
		if targetUid == uid {
			continue // 排除自己
		}

		u, err := s.userSvc.Load(ctx, targetUid)
		if err != nil {
			continue
		}

		targets = append(targets, &data.ArenaTarget{
			Rank:  int(z.Score),
			Uid:   targetUid,
			Name:  u.Name,
			Level: u.Level,
			Power: u.Power,
		})

		if len(targets) >= 5 {
			break
		}
	}

	return targets, nil
}

// getDailyChances 返回剩余挑战次数（每日重置）
func (s *ArenaService) getDailyChances(ctx context.Context, uid int64) (int, error) {
	dateStr := time.Now().Format("20060102")
	key := data.ArenaChancesKeyPrefix + strconv.FormatInt(uid, 10) + ":" + dateStr
	usedStr, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return data.ArenaDailyChancesMax, nil
	}

	if err != nil {
		return 0, err
	}

	used, _ := strconv.Atoi(usedStr)
	left := data.ArenaDailyChancesMax - used
	if left < 0 {
		left = 0
	}
	return left, nil
}

// consumeChance 消耗一次挑战次数
func (s *ArenaService) consumeChance(ctx context.Context, uid int64) error {
	dateStr := time.Now().Format("20060102")
	key := data.ArenaChancesKeyPrefix + strconv.FormatInt(uid, 10) + ":" + dateStr
	val, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	if val > data.ArenaDailyChancesMax {
		// 回滚
		s.rdb.Decr(ctx, key)
		return errors.New("exceed daily chances")
	}
	s.rdb.Expire(ctx, key, time.Hour*48)
	return nil
}

// calcVictory 简化快速战斗，战力越高胜率越大
func (s *ArenaService) calcVictory(myPower, targetPower int64) bool {
	if myPower <= 0 {
		return false
	}
	ratio := float64(myPower) / float64(targetPower+1)
	winProb := ratio / (1 + ratio)
	return rand.Float64() < winProb
}

// getDailyBuyCount 获取今日已购买次数
func (s *ArenaService) getDailyBuyCount(ctx context.Context, uid int64) (int, error) {
	dateStr := time.Now().Format("20060102")
	key := data.ArenaBuyCntKeyPrefix + strconv.FormatInt(uid, 10) + ":" + dateStr
	val, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	cnt, _ := strconv.Atoi(val)
	return cnt, nil
}
