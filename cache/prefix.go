package cache

import "fmt"

var globalPrefix string

func SetGlobalPrefix(prefix string) {
	globalPrefix = prefix
}

// BuildKey 生成带前缀的 Redis key
func BuildKey(format string, args ...interface{}) string {
	key := globalPrefix + fmt.Sprintf(format, args...)
	return key
}
