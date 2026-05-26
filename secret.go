package main

/*
生成密钥
*/
import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func generateSecret() (string, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func main() {
	access, _ := generateSecret()
	refresh, _ := generateSecret()
	fmt.Println("JWT_ACCESS_SECRET:", access)
	fmt.Println("JWT_REFRESH_SECRET:", refresh)
}
