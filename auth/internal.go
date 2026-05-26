package auth

/*
原服内部 API 鉴权中间件
InternalAuth 中间件，验证 X-Internal-Auth
*/

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const InternalAuthHeader = "X-Internal-Auth"

func InternalAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Request.Header.Get(InternalAuthHeader)
		if token != "Bearer "+secret {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    0,
				"message": "invalid token",
			})
			return
		}
		c.Next()
	}
}
