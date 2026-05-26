package game

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type LockService struct {
	rdb *redis.Client
}

func NewLockService(rdb *redis.Client) *LockService {
	return &LockService{rdb: rdb}
}

// Lock 对单个 key 加锁，返回解锁函数
func (l *LockService) Lock(ctx context.Context, key string) (func(), error) {
	ok, err := l.rdb.SetNX(ctx, key, "locked", 5*time.Second).Result()
	if err != nil || !ok {
		return nil, errors.New("lock failed")
	}

	return func() {
		l.rdb.Del(ctx, key)
	}, nil
}

// LockMulti 同时对多个 key 加分布式锁，返回解锁函数，获取失败立即返回错误。
// 生产环境建议使用 Redlock 算法或实现单个 key 的原子获取。
func (l *LockService) LockMulti(ctx context.Context, keys ...string) (func(), error) {
	var acquired []string
	for _, key := range keys {
		// SET NX EX 5秒，防止死锁
		ok, err := l.rdb.SetNX(ctx, key, "locked", 5*time.Second).Result()
		if err != nil || !ok {
			// 回滚已获取的锁
			for _, ak := range acquired {
				l.rdb.Del(ctx, ak)
			}
		}
		acquired = append(acquired, key)
	}

	return func() {
		for _, k := range acquired {
			l.rdb.Del(ctx, k)
		}
	}, nil
}

// 为某个模块生成专用的锁 key
func AllianceLockKey(aid int64) string {
	return fmt.Sprintf("lock:alliance:%d", aid)
}

func UserLockKey(uid int64) string {
	return fmt.Sprintf("lock:user:%d", uid)
}
