package auth

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

var (
	accessSecret  []byte
	refreshSecret []byte
	AccessExpire  time.Duration
	RefreshExpire time.Duration
)

func init() {
	accessSecret = []byte(getEnv("JWT_ACCESS_SECRET", "l6GuPuPmomAGg/VeRTJFOqn+J8xmdnq3jFQhvFdTTmg="))
	refreshSecret = []byte(getEnv("JWT_REFRESH_SECRET", "uj3hubGhxMPi3cbw8W0ZKkNDnOfPbIGNMreg69/hGvY="))

	var err error
	AccessExpire, err = time.ParseDuration(os.Getenv("JWT_ACCESS_TOKEN_EXPIRE"))
	if err != nil {
		AccessExpire = 15 * time.Minute
	}
	RefreshExpire, err = time.ParseDuration(os.Getenv("JWT_REFRESH_TOKEN_EXPIRE"))
	if err != nil {
		RefreshExpire = 7 * 24 * time.Hour
	}
}

// 生成 secret: openssl rand -base64 32
// 加载 .env 文件，如果没有则获取环境变量，否则返回默认值
func getEnv(key, fallback string) string {
	_ = godotenv.Load()
	//fmt.Println(key, fallback)
	if value, ok := os.LookupEnv(key); ok {
		//fmt.Println("================   ", value)
		return value
	}
	return fallback
}
