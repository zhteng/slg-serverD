package api

import (
	"net/http"
	"net/url"
	"slg-serverD/auth"
	"slg-serverD/ws"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// websocketHandler 处理 WebSocket 连接请求
// 支持从 Header 或 URL query 参数中获取 JWT token
func (s *Server) websocketHandler(c *gin.Context) {
	if auth.IsDebug == true {
		uidStr := c.GetHeader("X-Debug-UID")
		if uidStr == "" {
			uidStr = c.Query("uid")
		}

		serverIdStr := c.GetHeader("X-Server-Id")
		if serverIdStr == "" {
			serverIdStr = c.Query("serverId")
		}

		if uidStr == "" || serverIdStr == "" {
			return
		}

		uid, err := strconv.ParseInt(uidStr, 10, 64)
		serverId, err := strconv.ParseInt(serverIdStr, 10, 64)
		if err != nil || serverId == 0 {
			return
		}

		ws.HandleWebSocketWithAuth(s.hub, uid)(c.Writer, c.Request)
		return
	}

	// 1. 提取 token
	tokenString := ""
	// 优先从 Authorization Header 获取
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			tokenString = strings.TrimSpace(parts[1])
			tokenString = strings.Trim(tokenString, "<>")
		}
	}

	// 从URL query 参数 token=? 获取
	if tokenString == "" {
		tokenString = strings.TrimSpace(c.Query("token"))
	}

	if tokenString != "" {
		// 可能的 URL 解码
		if decoded, err := url.QueryUnescape(tokenString); err == nil {
			tokenString = decoded
		}
	}

	if tokenString == "" {
		c.String(http.StatusUnauthorized, "missing token")
		return
	}

	// 解析token  获得uid serverId(待定)
	uid, _, err := auth.ParseAccessToken(tokenString)

	//log.Println("========================>>>>>>>>>>>>>>    ws: missing token", uid)
	if err != nil {
		c.String(http.StatusUnauthorized, "invalid or expired token: "+err.Error())
		return
	}

	// 升级连接并启动 WebSocket
	ws.HandleWebSocketWithAuth(s.hub, uid)(c.Writer, c.Request)
}
