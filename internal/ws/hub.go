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
	maxMsgSize = 65536 // Increased for document content
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
	// Document fields
	DocumentID int64  `json:"document_id,omitempty"`
	Filename   string `json:"filename,omitempty"`
	Language   string `json:"language,omitempty"`
	Cursor     *WSCursor `json:"cursor,omitempty"`
}

// WSCursor represents cursor position in a document
type WSCursor struct {
	Line int `json:"line"`
	Col  int `json:"col"`
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
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	userID     int64
	channels   map[int64]bool // subscribed channels
	workspaces map[int64]bool // subscribed workspaces
	documents  map[int64]bool // subscribed documents (for collaborative editing)
	mu         sync.RWMutex   // protects channels, workspaces, and documents
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	clients         map[*Client]bool
	broadcast       chan []byte
	register        chan *Client
	unregister      chan *Client
	documentClients map[int64]map[*Client]bool // document rooms
	mu              sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:         make(map[*Client]bool),
		broadcast:       make(chan []byte, 256),
		register:        make(chan *Client, 16),
		unregister:      make(chan *Client, 16),
		documentClients: make(map[int64]map[*Client]bool),
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
				// Remove from all document rooms
				client.mu.RLock()
				for docID := range client.documents {
					if room, ok := h.documentClients[docID]; ok {
						delete(room, client)
						if len(room) == 0 {
							delete(h.documentClients, docID)
						}
					}
				}
				client.mu.RUnlock()
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

// SendToUser sends a message to a specific user's WebSocket connections.
func (h *Hub) SendToUser(userID int64, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.userID == userID {
			select {
			case client.send <- message:
			default:
				// Client channel is full, skip
			}
		}
	}
}

// BroadcastDocumentEdit broadcasts a document edit event to all clients viewing the document.
func (h *Hub) BroadcastDocumentEdit(documentID, userID int64, username string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.documentClients[documentID]
	if !ok {
		return
	}

	msg, _ := json.Marshal(WSMessage{
		Type:       "doc_edit",
		DocumentID: documentID,
		UserID:     userID,
		Content:    username, // Using content field to pass username
	})

	for client := range clients {
		if client.userID != userID { // Don't send to the editor
			select {
			case client.send <- msg:
			default:
			}
		}
	}
}

// BroadcastToDocument sends a message to all clients viewing a specific document.
func (h *Hub) BroadcastToDocument(documentID int64, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.documentClients[documentID]
	if !ok {
		return
	}

	for client := range clients {
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
		documents:  make(map[int64]bool),
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

			case "doc_open":
				// Subscribe to document room
				if wsMsg.DocumentID > 0 {
					c.mu.Lock()
					c.documents[wsMsg.DocumentID] = true
					c.mu.Unlock()

					c.hub.mu.Lock()
					if c.hub.documentClients[wsMsg.DocumentID] == nil {
						c.hub.documentClients[wsMsg.DocumentID] = make(map[*Client]bool)
					}
					c.hub.documentClients[wsMsg.DocumentID][c] = true
					c.hub.mu.Unlock()

					// Broadcast to other viewers
					openMsg, _ := json.Marshal(WSMessage{
						Type:       "doc_open",
						DocumentID: wsMsg.DocumentID,
						UserID:     c.userID,
						Content:    wsMsg.Content, // username
					})
					c.hub.BroadcastToDocument(wsMsg.DocumentID, openMsg)
				}

			case "doc_close":
				// Unsubscribe from document room
				if wsMsg.DocumentID > 0 {
					c.mu.Lock()
					delete(c.documents, wsMsg.DocumentID)
					c.mu.Unlock()

					c.hub.mu.Lock()
					if room, ok := c.hub.documentClients[wsMsg.DocumentID]; ok {
						delete(room, c)
						if len(room) == 0 {
							delete(c.hub.documentClients, wsMsg.DocumentID)
						}
					}
					c.hub.mu.Unlock()

					// Broadcast to other viewers
					closeMsg, _ := json.Marshal(WSMessage{
						Type:       "doc_close",
						DocumentID: wsMsg.DocumentID,
						UserID:     c.userID,
					})
					c.hub.BroadcastToDocument(wsMsg.DocumentID, closeMsg)
				}

			case "doc_cursor":
				// Broadcast cursor position to other document viewers
				if wsMsg.DocumentID > 0 && wsMsg.Cursor != nil {
					cursorMsg, _ := json.Marshal(WSMessage{
						Type:       "doc_cursor",
						DocumentID: wsMsg.DocumentID,
						UserID:     c.userID,
						Cursor:     wsMsg.Cursor,
					})
					c.hub.BroadcastToDocument(wsMsg.DocumentID, cursorMsg)
				}
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
