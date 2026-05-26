package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"slg-serverD/data"
	"time"

	"github.com/go-redis/redis/v8"
)

type AllianceCache struct {
	rdb *redis.Client
}

func NewAllianceCache(rdb *redis.Client) *AllianceCache {
	return &AllianceCache{rdb: rdb}
}

func (c *AllianceCache) Get(ctx context.Context, aid int64) (*data.Alliance, error) {
	key := BuildKey("alliance:%d", aid)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var a data.Alliance
	_ = json.Unmarshal([]byte(val), &a)
	return &a, nil
}

func (c *AllianceCache) Set(ctx context.Context, a *data.Alliance) error {
	if a == nil {
		return fmt.Errorf("军团对象不能为空")
	}

	b, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("序列化军团数据失败: %w", err)
	}

	key := BuildKey("alliance:%d", a.Id)
	err = c.rdb.Set(ctx, key, b, 30*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("写入Redis失败 (key=%s): %w", key, err)
	}
	return nil
}

func (c AllianceCache) Del(ctx context.Context, aid int64) error {
	return c.rdb.Del(ctx, BuildKey("lock:alliance:%d", aid)).Err()
}
