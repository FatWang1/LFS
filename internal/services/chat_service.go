package services

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ChatMessage 表示一条聊天消息。
type ChatMessage struct {
	Type      string `json:"type"`      // 消息类型：message、join、leave
	IP        string `json:"ip"`        // 客户端IP地址
	Nickname  string `json:"nickname"`  // 用户昵称
	Message   string `json:"message"`   // 消息内容
	Timestamp string `json:"timestamp"` // 时间戳
}

// Client 表示一个WebSocket客户端连接。
type Client struct {
	conn     *websocket.Conn
	ip       string
	nickname string
	send     chan ChatMessage
	hub      *ChatHub
}

// ChatHub 管理所有WebSocket客户端连接和消息广播。
// 它是聊天服务的核心组件，负责客户端注册、注销和消息分发。
type ChatHub struct {
	clients    map[*Client]bool
	broadcast  chan ChatMessage
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
}

// NewChatHub 创建并返回一个新的聊天室中心实例。
func NewChatHub() *ChatHub {
	return &ChatHub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan ChatMessage, 256), // 使用缓冲channel避免死锁
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run 启动聊天室中心的消息处理循环。
// 它处理客户端注册、注销和消息广播。
func (h *ChatHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()

			joinMsg := ChatMessage{
				Type:      "join",
				IP:        client.ip,
				Nickname:  client.nickname,
				Message:   client.nickname + " 加入了聊天室",
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
			}
			// 使用 goroutine 发送，避免阻塞
			go func() {
				h.broadcast <- joinMsg
			}()

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()

			leaveMsg := ChatMessage{
				Type:      "leave",
				IP:        client.ip,
				Nickname:  client.nickname,
				Message:   client.nickname + " 离开了聊天室",
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
			}
			// 使用 goroutine 发送，避免阻塞
			go func() {
				h.broadcast <- leaveMsg
			}()

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// ChatService 实现聊天服务的业务逻辑。
// 它管理WebSocket连接、消息广播和客户端状态。
type ChatService struct {
	hub *ChatHub
}

// NewChatService 创建并返回一个新的聊天服务实例。
// 会自动启动hub的消息处理goroutine。
func NewChatService() *ChatService {
	hub := NewChatHub()
	go hub.Run()
	return &ChatService{hub: hub}
}

// HandleWebSocket 处理WebSocket连接升级和客户端注册。
func (s *ChatService) HandleWebSocket(c *gin.Context) error {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return err
	}

	ip := getClientIP(c)
	nickname := getNickname(ip)

	client := &Client{
		conn:     conn,
		ip:       ip,
		nickname: nickname,
		send:     make(chan ChatMessage, 256),
		hub:      s.hub,
	}

	s.hub.register <- client

	go client.writePump()
	go client.readPump()

	return nil
}

// BroadcastMessage 向所有连接的客户端广播消息。
func (s *ChatService) BroadcastMessage(message interface{}) error {
	msg, ok := message.(ChatMessage)
	if !ok {
		return nil
	}
	s.hub.broadcast <- msg
	return nil
}

// GetClientCount 返回当前连接的客户端数量。
func (s *ChatService) GetClientCount() int {
	s.hub.mutex.RLock()
	defer s.hub.mutex.RUnlock()
	return len(s.hub.clients)
}

// readPump 从WebSocket连接读取消息并广播。
// 当连接关闭时自动注销客户端。
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// 意外的关闭错误，静默处理
			}
			break
		}

		var msg ChatMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			continue
		}

		msg.Type = "message"
		msg.IP = c.ip
		msg.Nickname = c.nickname
		msg.Timestamp = time.Now().Format("2006-01-02 15:04:05")

		c.hub.broadcast <- msg
	}
}

// writePump 向WebSocket连接写入消息。
// 定期发送ping消息以保持连接活跃。
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			// 编码并发送消息
			if err := json.NewEncoder(w).Encode(message); err != nil {
				w.Close()
				return
			}

			// 批量发送队列中的其他消息
			n := len(c.send)
			for i := 0; i < n; i++ {
				msg := <-c.send
				if err := json.NewEncoder(w).Encode(msg); err != nil {
					break
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// getClientIP 从请求中提取客户端IP地址。
// 优先检查代理头（X-Forwarded-For、X-Real-IP），然后使用Gin的ClientIP方法。
func getClientIP(c *gin.Context) string {
	ip := c.GetHeader("X-Forwarded-For")
	if ip != "" {
		ips := strings.Split(ip, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	ip = c.GetHeader("X-Real-IP")
	if ip != "" {
		return ip
	}

	return c.ClientIP()
}

// getNickname 根据IP地址生成用户昵称。
// 本机地址返回"本机"，其他地址根据IP段生成昵称。
func getNickname(ip string) string {
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return "本机"
	}

	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		lastOctet := parts[3]
		if lastOctet == "1" {
			return "master"
		}
		return "用户-" + lastOctet
	}

	return "用户-" + ip[len(ip)-4:]
}
