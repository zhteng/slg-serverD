package ws

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	// 读取消息的超时时间：5 秒内未收到任何消息（包括心跳），判定连接断开
	readTimeout = 5 * time.Minute
)

// HandleWebSocket 升级连接，并从 url 参数中获取 uid 注册到 Hub
func HandleWebSocket(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uidStr := r.URL.Query().Get("uid")
		uid, err := strconv.ParseInt(uidStr, 10, 64)
		if err != nil || uid <= 0 {
			http.Error(w, "invalid uid", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade error: %v", err)
			return
		}

		hub.Register(uid, conn)
		defer func() {
			hub.Unregister(uid)
			_ = conn.Close()
		}()

		// 设置初始读取超时
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))

		// 设置 Pong 处理器（可选，用于响应客户端的 Ping 帧，延长超时）
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
			return nil
		})

		// 保持连接，读取客户端消息（可处理心跳或命令，这里仅保持连接）
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("ws player %d connection error: %v", uid, err)
				}
				break // 连接关闭或超时
			}

			// 收到任意消息（包括心跳文本 "ping"）都重置超时
			_ = conn.SetReadDeadline(time.Now().Add(readTimeout))

			// 可以处理客户端发来的业务消息（目前忽略心跳，只做超时重置）
			log.Printf("ws received from %d: %s", uid, string(message))
		}
	}
}

// HandleWebSocketWithAuth 升级 WebSocket 连接并注册到 Hub
// uid 由调用方从 JWT 解析后传入，不再从 URL 获取
func HandleWebSocketWithAuth(hub *Hub, uid int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade error: %v", err)
			return
		}

		hub.Register(uid, conn)
		defer func() {
			hub.Unregister(uid)
			_ = conn.Close()
		}()

		// 初始设置读取超时
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
			return nil
		})

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("ws player %d connection error: %v", uid, err)
				}
				break
			}
			// 收到任意消息（包括心跳 ping）都刷新超时
			_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		}
	}
}
