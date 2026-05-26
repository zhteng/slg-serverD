package auth

import (
	"errors"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

/*
Token 生成/解析函数
*/

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// Claims 自定义 JWT 声明
type Claims struct {
	Uid       int64     `json:"uid"`
	ServerId  int       `json:"server_id"`
	Jti       string    `json:"jti"`
	TokenType TokenType `json:"type"`
	jwt.RegisteredClaims

	/*secretKey    = []byte("your-secret-key-keep-safe") // 实际应使用配置或环境变量
	tokenExpire  = 24 * time.Hour*/
}

// GenerateAccessToken 生成短期访问令牌
func GenerateAccessToken(uid int64, serverId int) (string, error) {
	return generateToken(uid, serverId, AccessToken, AccessExpire, accessSecret)
}

// GenerateRefreshToken 生成长期刷新令牌
func GenerateRefreshToken(uid int64, serverId int) (string, error) {
	return generateToken(uid, serverId, RefreshToken, RefreshExpire, refreshSecret)
}

func generateToken(uid int64, serverId int, tokenType TokenType, expire time.Duration, secret []byte) (string, error) {
	now := time.Now()
	claims := &Claims{
		Uid:       uid,
		ServerId:  serverId, // 设置区服
		Jti:       uuid.New().String(),
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func ParseAccessToken(tokenString string) (int64, int, error) {
	claims, err := ParseToken(tokenString, accessSecret)
	if err != nil {
		return 0, 0, err
	}
	return claims.Uid, claims.ServerId, nil
}

// ParseToken 解析 token，返回 claims（需自行根据 tokenType 选用对应的 secret）
func ParseToken(tokenString string, secret []byte) (*Claims, error) {
	log.Println("========================>>>>>>>>>>>>>>    ws: missing token", tokenString)
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})

	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// GenerateToken 为指定 uid 生成 JWT token
/*func GenerateToken(uid int64) (string, error) {
	claims := Claims{
		Uid: uid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExpire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}*/

// ParseToken 解析 token，返回 uid
/*func ParseToken(tokenString string) (int64, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})

	if err != nil {
		return 0, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims.Uid, nil
	}
	return 0, errors.New("invalid token")
}*/
