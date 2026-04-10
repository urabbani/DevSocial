package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// WebSocket message types
type WSMessage struct {
	Type      string `json:"type"`
	ChannelID int64  `json:"channel_id,omitempty"`
	UserID    int64  `json:"user_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Status    string `json:"status,omitempty"`
	Message   any    `json:"message,omitempty"`
	ChannelIDs []int64 `json:"channel_ids,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Client represents a single WebSocket connection.
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	userID  int64
	channels map[int64]bool // subscribed channels
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client send buffer full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToChannel sends a message to all clients subscribed to a specific channel.
func (h *Hub) BroadcastToChannel(channelID int64, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.channels[channelID] {
			select {
			case client.send <- message:
			default:
			}
		}
	}
}

// ServeWS upgrades an HTTP connection to WebSocket and registers the client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, userID int64) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256),
		userID:   userID,
		channels: make(map[int64]bool),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			continue
		}

		switch wsMsg.Type {
		case "subscribe":
			for _, chID := range wsMsg.ChannelIDs {
				c.channels[chID] = true
			}

		case "typing":
			// Broadcast typing indicator to other clients in the channel
			typingMsg, _ := json.Marshal(WSMessage{
				Type:      "typing",
				ChannelID: wsMsg.ChannelID,
				UserID:    c.userID,
			})
			c.hub.BroadcastToChannel(wsMsg.ChannelID, typingMsg)

		case "presence":
			presenceMsg, _ := json.Marshal(WSMessage{
				Type:   "presence",
				UserID: c.userID,
				Status: wsMsg.Status,
			})
			c.hub.broadcast <- presenceMsg
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		message, ok := <-c.send
		if !ok {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
}
