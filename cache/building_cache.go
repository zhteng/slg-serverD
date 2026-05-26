package cache

import (
	"context"
	"encoding/json"
	"slg-serverD/data"
	"time"

	"github.com/go-redis/redis/v8"
)

type BuildingCache struct {
	rdb *redis.Client
}

func NewBuildingCache(rdb *redis.Client) *BuildingCache {
	return &BuildingCache{rdb: rdb}
}

func (c *BuildingCache) Get(ctx context.Context, uid int64) (*data.Building, error) {
	val, err := c.rdb.Get(ctx, BuildKey("building:%d", uid)).Result()
	if err != nil {
		return nil, err
	}

	var b data.Building
	_ = json.Unmarshal([]byte(val), &b)
	return &b, nil
}

func (c *BuildingCache) Set(ctx context.Context, b *data.Building) error {
	bData, _ := json.Marshal(b)
	return c.rdb.Set(ctx, BuildKey("building:%d", b.Uid), bData, 30*time.Minute).Err()
}

func (c BuildingCache) Del(ctx context.Context, uid int64) error {
	return c.rdb.Del(ctx, BuildKey("building:%d", uid)).Err()
}
