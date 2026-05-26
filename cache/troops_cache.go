package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"slg-serverD/data"
	"time"

	"github.com/go-redis/redis/v8"
)

type TroopsCache struct {
	rdb *redis.Client
}

func NewTroopsCache(rdb *redis.Client) *TroopsCache {
	return &TroopsCache{rdb: rdb}
}

func (c *TroopsCache) Get(ctx context.Context, uid int64) (*data.Troops, error) {
	key := BuildKey("troops:%d", uid)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var t data.Troops
	_ = json.Unmarshal([]byte(val), &t)
	return &t, nil
}

func (c *TroopsCache) Set(ctx context.Context, t *data.Troops) error {
	if t == nil {
		return fmt.Errorf("部队对象不能为空")
	}

	b, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}
	key := BuildKey("troops:%d", t.Uid)
	err = c.rdb.Set(ctx, key, b, 30*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("写入Redis失败 (key=%s): %w", key, err)
	}
	return nil
}

func (c *TroopsCache) Del(ctx context.Context, uid int64) error {
	key := BuildKey("troops:%d", uid)
	err := c.rdb.Del(ctx, key).Err()
	if err != nil {
		return err
	}
	return nil
}
