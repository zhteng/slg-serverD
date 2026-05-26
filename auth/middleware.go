package auth

/*
所有服务（原服、内部）
*/
import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// AccessTokenAuth 验证 Access Token 的中间件
func AccessTokenAuth() gin.HandlerFunc {
	return tokenAuthMiddleware(AccessToken, accessSecret)
}

// RefreshTokenAuth 验证 Refresh Token 的中间件
func RefreshTokenAuth() gin.HandlerFunc {
	return tokenAuthMiddleware(RefreshToken, refreshSecret)
}

func tokenAuthMiddleware(expectedType TokenType, secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ---------- Debug 模式：跳过 Token 验证 ----------
		if IsDebug {
			uidStr := c.GetHeader("X-Debug-UID")
			if uidStr == "" {
				uidStr = c.Query("uid")
			}

			serverIdStr := c.GetHeader("X-Server-Id")
			if serverIdStr == "" {
				serverIdStr = c.Query("serverId")
			}

			if uidStr == "" || serverIdStr == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code": 0,
					"msg":  "debug mode: missing",
				})
				return
			}

			uid, err := strconv.ParseInt(uidStr, 10, 64)
			serverId, err := strconv.ParseInt(serverIdStr, 10, 64)
			if err != nil || serverId == 0 {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code": 0,
					"msg":  "debug mode: invalid error",
				})
				return
			}

			c.Set("uid", uid)
			c.Set("server_id", serverId)
			c.Next()
			return
		}

		// 提取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 0,
				"msg":  "Authorization header is empty",
			})
			return
		}

		// 格式：Bearer <token>
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 0, "msg": "invalid authorization format"})
			return
		}

		tokenString := parts[1]
		// 解析token
		claims, err := ParseToken(tokenString, secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 0,
				"msg":  "invalid token",
			})
			return
		}

		// 校验 token 类型
		if claims.TokenType != expectedType {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 0,
				"msg":  "wrong token type",
			})
			return
		}

		// 检查黑名单
		if IsBlacklisted(c.Request.Context(), claims.Jti) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 0,
				"msg":  "token is blacklisted",
			})
			return
		}

		// 设置用户信息
		c.Set("uid", claims.Uid)
		c.Set("server_id", claims.ServerId)
		c.Set("jti", claims.Jti)
		c.Next()
	}
}

// JWTAuth 中间件：验证 token，提取 uid 并注入到上下文
/*func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 中获取 Authorization
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 0,
				"msg":  "missing authorization header",
			})
			return
		}

		// 格式：Bearer <token>
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 0, "msg": "invalid auth format"})
			return
		}

		uid, err := ParseToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 0,
				"msg":  "invalid or expired token",
			})
			return
		}

		// 将 uid 存入上下文，供后续 Handler 使用
		c.Set("uid", uid)
		c.Next()
	}
}*/
