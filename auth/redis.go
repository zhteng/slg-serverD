package auth

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

var rdb *redis.Client

// InitRedis 注入 Redis 客户端（在 main 或 di 中调用一次）
func InitRedis(client *redis.Client) {
	rdb = client
}

// AddToBlacklist 将 jti 加入黑名单，自动过期
func AddToBlacklist(ctx context.Context, jti string, expire time.Duration) error {
	return rdb.Set(ctx, "jwt:blacklist:"+jti, "1", expire).Err()
}

func RemoveFromBlacklist(ctx context.Context, jti string) error {
	return rdb.Del(ctx, "jwt:blacklist:"+jti).Err()
}

// IsBlacklisted 检查 jti 是否在黑名单中
func IsBlacklisted(ctx context.Context, jti string) bool {
	exists, err := rdb.Exists(ctx, "jwt:blacklist:"+jti).Result()
	return err == nil && exists > 0
}
