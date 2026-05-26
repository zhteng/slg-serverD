package ws

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Hub 维护在线玩家 uid -> WebSocket 连接
type Hub struct {
	mu      sync.RWMutex
	clients map[int64]*websocket.Conn
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[int64]*websocket.Conn),
	}
}

// Register 玩家上线时注册连接
func (h *Hub) Register(uid int64, conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[uid] = conn
	h.mu.Unlock()
	log.Printf("ws: player %d connected", uid)
}

// Unregister 玩家下线时移除连接
func (h *Hub) Unregister(uid int64) {
	h.mu.Lock()
	delete(h.clients, uid)
	h.mu.Unlock()
	log.Printf("ws: player %d disconnected", uid)
}

// SendToPlayer 向指定玩家推送消息，失败则移除连接
func (h *Hub) SendToPlayer(uid int64, msg Message) {
	h.mu.RLock()
	conn, ok := h.clients[uid]
	h.mu.RUnlock()
	if !ok {
		return
	}
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("ws: send to %d error: %v", uid, err)
		h.Unregister(uid)
		conn.Close()
	}
}
