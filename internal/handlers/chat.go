package handlers

import (
	"lfs/internal/interfaces"

	"github.com/gin-gonic/gin"
)

// ChatHandlers handles chat-related HTTP requests.
// It depends on ChatService to handle WebSocket connections and message broadcasting.
type ChatHandlers struct {
	chatService interfaces.ChatService
}

// NewChatHandlers creates and returns a new chat handlers instance.
func NewChatHandlers(chatService interfaces.ChatService) *ChatHandlers {
	return &ChatHandlers{
		chatService: chatService,
	}
}

// Register registers chat-related routes.
func (h *ChatHandlers) Register(r *gin.Engine) {
	r.GET("/ws/chat", h.HandleWebSocket)
}

// HandleWebSocket handles WebSocket connection upgrade requests.
func (h *ChatHandlers) HandleWebSocket(c *gin.Context) {
	if err := h.chatService.HandleWebSocket(c); err != nil {
		if !c.Writer.Written() {
			c.JSON(500, gin.H{"error": err.Error()})
		}
	}
}
