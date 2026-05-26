package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"slg-serverD/data"
	"time"

	"github.com/go-redis/redis/v8"
)

type MapCache struct {
	rdb *redis.Client
}

func NewMapCache(rdb *redis.Client) *MapCache {
	return &MapCache{rdb: rdb}
}

func (c *MapCache) Get(ctx context.Context, x, y int) (*data.MapCell, error) {
	key := c.cellKey(x, y)
	value, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	cell := &data.MapCell{}
	if err := json.Unmarshal([]byte(value), cell); err != nil {
		return nil, err
	}
	return cell, nil
}

func (c *MapCache) Set(ctx context.Context, cell *data.MapCell) error {
	key := c.cellKey(cell.X, cell.Y)
	value, err := json.Marshal(cell)
	if err != nil {
		return err
	}

	// 热点常驻，较长过期时间
	return c.rdb.Set(ctx, key, value, 30*time.Minute).Err()
}

func (c *MapCache) Del(ctx context.Context, x, y int) error {
	key := c.cellKey(x, y)
	return c.rdb.Del(ctx, key).Err()
}

func (c *MapCache) cellKey(x, y int) string {
	return fmt.Sprintf("map:%d:%d", x, y)
}
