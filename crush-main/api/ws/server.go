package ws

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/auth"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// HandlerFunc defines the callback for processing incoming messages
type HandlerFunc func(message []byte)

// DisconnectFunc defines the callback for WebSocket disconnection
type DisconnectFunc func()

type Server struct {
	clients           map[*websocket.Conn]string // conn -> sessionID
	broadcast         chan []byte
	mutex             sync.Mutex
	handler           HandlerFunc
	disconnectHandler DisconnectFunc
}

func New() *Server {
	return &Server{
		clients:   make(map[*websocket.Conn]string),
		broadcast: make(chan []byte),
	}
}

// SetMessageHandler sets the callback for incoming messages
func (s *Server) SetMessageHandler(handler HandlerFunc) {
	s.handler = handler
}

// SetDisconnectHandler sets the callback for WebSocket disconnection
func (s *Server) SetDisconnectHandler(handler DisconnectFunc) {
	s.disconnectHandler = handler
}

func (s *Server) HandleConnections(w http.ResponseWriter, r *http.Request) {
	// Validate JWT token before upgrading connection
	token := extractToken(r)
	if token == "" {
		slog.Warn("WebSocket connection rejected: no token provided")
		http.Error(w, "Unauthorized: token required", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ValidateToken(token)
	if err != nil {
		slog.Warn("WebSocket connection rejected: invalid token", "error", err)
		http.Error(w, "Unauthorized: invalid or expired token", http.StatusUnauthorized)
		return
	}

	slog.Info("WebSocket authentication successful", "user_id", claims.UserID, "username", claims.Username)

	// Get sessionID from query parameter
	sessionID := r.URL.Query().Get("session_id")
	slog.Info("WebSocket connection with session", "session_id", sessionID)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade error", "error", err)
		return
	}

	s.mutex.Lock()
	s.clients[ws] = sessionID
	s.mutex.Unlock()
	slog.Info("New WebSocket connection established", "username", claims.Username, "session_id", sessionID)

	// Keep connection alive and handle disconnects
	go func() {
		defer func() {
			s.mutex.Lock()
			delete(s.clients, ws)
			s.mutex.Unlock()
			ws.Close()
			slog.Info("WebSocket connection closed")
			
			// Call disconnect handler to clean up agent state
			if s.disconnectHandler != nil {
				slog.Info("Calling disconnect handler to clean up agent state")
				s.disconnectHandler()
			}
		}()

		for {
			_, msg, err := ws.ReadMessage()
			fmt.Println("=== WebSocket message received ===")
			fmt.Println("Message bytes:", msg)
			fmt.Println("Message string:", string(msg))
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					slog.Error("WebSocket read error", "error", err)
				}
				break
			}

			// Handle incoming message via callback
			fmt.Println("Handler exists:", s.handler != nil)
			if s.handler != nil {
				fmt.Println("Calling handler with message")
				s.handler(msg)
				fmt.Println("Handler returned")
			} else {
				fmt.Println("WARNING: No handler set!")
			}
		}
	}()
}

func (s *Server) Broadcast(msg interface{}) {
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		slog.Error("JSON marshal error", "error", err)
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	for client := range s.clients {
		err := client.WriteMessage(websocket.TextMessage, jsonMsg)
		if err != nil {
			slog.Error("WebSocket write error", "error", err)
			client.Close()
			delete(s.clients, client)
		}
	}
}

// SendToSession sends a message only to clients connected to a specific session
func (s *Server) SendToSession(sessionID string, msg interface{}) {
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		slog.Error("JSON marshal error", "error", err)
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	sentCount := 0
	for client, clientSessionID := range s.clients {
		if clientSessionID == sessionID {
			err := client.WriteMessage(websocket.TextMessage, jsonMsg)
			if err != nil {
				slog.Error("WebSocket write error", "error", err)
				client.Close()
				delete(s.clients, client)
			} else {
				sentCount++
			}
		}
	}
	slog.Debug("SendToSession completed", "session_id", sessionID, "sent_to_clients", sentCount)
}

// UpdateClientSession updates the session ID for a specific client connection
func (s *Server) UpdateClientSession(ws *websocket.Conn, sessionID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if _, exists := s.clients[ws]; exists {
		s.clients[ws] = sessionID
		slog.Info("Updated client session", "session_id", sessionID)
	}
}

// extractToken extracts the JWT token from the request
// It checks Authorization header first, then query parameters
func extractToken(r *http.Request) string {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// Try query parameter (for WebSocket connections that can't set headers easily)
	token := r.URL.Query().Get("token")
	if token != "" {
		// Decode URL-encoded token
		decoded, err := url.QueryUnescape(token)
		if err == nil {
			return decoded
		}
		return token
	}

	return ""
}

// Start starts the WebSocket server on the specified port
func (s *Server) Start(port string) {
	slog.Info("Starting WebSocket server", "port", port)

	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/ws", s.HandleConnections)

	if err := http.ListenAndServe(":"+port, wsMux); err != nil {
		slog.Error("WebSocket server error", "error", err)
	}
}
