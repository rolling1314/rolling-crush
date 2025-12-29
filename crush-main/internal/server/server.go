package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

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
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade error", "error", err)
		return
	}

	s.mutex.Lock()
	s.clients[ws] = true
	s.mutex.Unlock()
	slog.Info("New WebSocket connection established")

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
			fmt.Println(msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					slog.Error("WebSocket read error", "error", err)
				}
				break
			}

			// Handle incoming message via callback
			if s.handler != nil {
				s.handler(msg)
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

// Start starts the WebSocket server on the specified port
func (s *Server) Start(port string) {
	slog.Info("Starting WebSocket server", "port", port)

	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/ws", s.HandleConnections)

	if err := http.ListenAndServe(":"+port, wsMux); err != nil {
		slog.Error("WebSocket server error", "error", err)
	}
}
