package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/auth"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// HandlerFunc defines the callback for processing incoming messages
type HandlerFunc func(message []byte)

type Server struct {
	clients   map[*websocket.Conn]bool
	broadcast chan []byte
	mutex     sync.Mutex
	handler   HandlerFunc
}

func New() *Server {
	return &Server{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan []byte),
	}
}

// SetMessageHandler sets the callback for incoming messages
func (s *Server) SetMessageHandler(handler HandlerFunc) {
	s.handler = handler
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

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade error", "error", err)
		return
	}

	s.mutex.Lock()
	s.clients[ws] = true
	s.mutex.Unlock()
	slog.Info("New WebSocket connection established", "username", claims.Username)

	// Keep connection alive and handle disconnects
	go func() {
		defer func() {
			s.mutex.Lock()
			delete(s.clients, ws)
			s.mutex.Unlock()
			ws.Close()
			slog.Info("WebSocket connection closed")
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
