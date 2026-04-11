package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 4096
)

// WebSocket message types
type WSMessage struct {
	Type       string  `json:"type"`
	ChannelID  int64   `json:"channel_id,omitempty"`
	UserID     int64   `json:"user_id,omitempty"`
	Content    string  `json:"content,omitempty"`
	Status     string  `json:"status,omitempty"`
	Message    any     `json:"message,omitempty"`
	ChannelIDs []int64 `json:"channel_ids,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return false
		}
		// Allow connections from the same host
		host := r.Host
		if host == "" {
			return false
		}
		// Strip port from origin for comparison
		originHost := origin
		if idx := strings.LastIndex(origin, ":"); idx > 0 {
			// Check if it's a port (not part of IPv6)
			afterColon := origin[idx+1:]
			allDigits := true
			for _, c := range afterColon {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				originHost = origin[:idx]
			}
		}
		return strings.EqualFold(originHost, strings.Split(host, ":")[0])
	},
}

// Client represents a single WebSocket connection.
type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	userID      int64
	channels    map[int64]bool // subscribed channels
	workspaces  map[int64]bool // subscribed workspaces
	mu          sync.RWMutex   // protects channels and workspaces
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
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
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
			targets := make([]*Client, 0, len(h.clients))
			for client := range h.clients {
				targets = append(targets, client)
			}
			h.mu.RUnlock()
			for _, client := range targets {
				select {
				case client.send <- message:
				default:
				}
			}
		}
	}
}

// BroadcastToChannel sends a message to all clients subscribed to a specific channel.
func (h *Hub) BroadcastToChannel(channelID int64, message []byte) {
	h.mu.RLock()
	targets := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		client.mu.RLock()
		subscribed := client.channels[channelID]
		client.mu.RUnlock()
		if subscribed {
			targets = append(targets, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range targets {
		select {
		case client.send <- message:
		default:
		}
	}
}

// BroadcastToWorkspace sends a message to all clients in a workspace.
func (h *Hub) BroadcastToWorkspace(workspaceID int64, message []byte) {
	h.mu.RLock()
	targets := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		client.mu.RLock()
		subscribed := client.workspaces[workspaceID]
		client.mu.RUnlock()
		if subscribed {
			targets = append(targets, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range targets {
		select {
		case client.send <- message:
		default:
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
		hub:        h,
		conn:       conn,
		send:       make(chan []byte, 256),
		userID:     userID,
		channels:   make(map[int64]bool),
		workspaces: make(map[int64]bool),
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

	c.conn.SetReadLimit(maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPingHandler(func(appData string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

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
			c.mu.Lock()
			for _, chID := range wsMsg.ChannelIDs {
				c.channels[chID] = true
			}
			c.mu.Unlock()

		case "subscribe_workspace":
			c.mu.Lock()
			if wsMsg.ChannelIDs != nil {
				for _, wsID := range wsMsg.ChannelIDs {
					c.workspaces[wsID] = true
				}
			}
			c.mu.Unlock()

		case "typing":
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
			select {
			case c.hub.broadcast <- presenceMsg:
			default:
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		// Don't close conn here — readPump owns the close
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
