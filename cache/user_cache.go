package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"slg-serverD/data"
	"time"

	"github.com/go-redis/redis/v8"
)

type UserCache struct {
	rdb *redis.Client
}

func NewUserCache(rdb *redis.Client) *UserCache {
	return &UserCache{rdb: rdb}
}

func (c UserCache) Get(ctx context.Context, uid int64) (*data.User, error) {
	key := BuildKey("user:%d", uid)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var u data.User
	_ = json.Unmarshal([]byte(val), &u)
	return &u, nil
}

// Set 缓存玩家数据，TTL 设为 30 分钟（可根据活跃情况调整）
func (c *UserCache) Set(ctx context.Context, u *data.User) error {
	// 1. 防御性编程：检查空指针
	if u == nil {
		return fmt.Errorf("用户对象不能为空")
	}

	// 2. 处理 Marshal 错误
	// 建议使用更高效的序列化库，如 msgpack 或 protobuf
	// b, err := msgpack.Marshal(u)
	b, err := json.Marshal(u)
	if err != nil {
		// 记录错误日志，方便排查
		return fmt.Errorf("序列化用户数据失败: %w", err)
	}

	key := BuildKey("user:%d", u.Uid)
	err = c.rdb.Set(ctx, key, b, 30*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("写入Redis失败 (key=%s): %w", key, err)
	}

	return nil
}

// Del 删除玩家缓存（写库后调用，保证下次读取从 DB 取最新）
func (c *UserCache) Del(ctx context.Context, uid int64) error {
	return c.rdb.Del(ctx, BuildKey("user:%d", uid)).Err()
}

// MDel 批量删除
func (c UserCache) MDel(ctx context.Context, uids []int64) error {
	key := make([]string, 0, len(uids))
	for _, uid := range uids {
		key = append(key, BuildKey("user:%d", uid))
	}

	/*key1 := make([]string, len(uids))
	for i, uid := range uids {
		key1[i] = fmt.Sprintf("user:%d", uid)
	}*/

	if _, err := c.rdb.Del(ctx, key...).Result(); err != nil {
		return fmt.Errorf("批量删除缓存失败: %w", err)
	}

	return nil
}
