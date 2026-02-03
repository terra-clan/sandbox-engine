package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// TerminalMessage represents a message sent over WebSocket
type TerminalMessage struct {
	Type string `json:"type"` // input, output, resize, connected, error
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// handleTerminalWS handles WebSocket connections for terminal access
func (s *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	sandboxID := chi.URLParam(r, "id")
	if sandboxID == "" {
		http.Error(w, "sandbox id required", http.StatusBadRequest)
		return
	}

	// Get sandbox to verify it exists and is running
	sb, err := s.sandboxManager.Get(r.Context(), sandboxID)
	if err != nil {
		slog.Error("failed to get sandbox", "id", sandboxID, "error", err)
		http.Error(w, "sandbox not found", http.StatusNotFound)
		return
	}

	if sb.Status != "running" {
		http.Error(w, "sandbox is not running", http.StatusBadRequest)
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("failed to upgrade to websocket", "error", err)
		return
	}
	defer conn.Close()

	slog.Info("terminal websocket connected", "sandbox_id", sandboxID)

	// Create docker exec session
	execID, execConn, err := s.sandboxManager.ExecAttach(r.Context(), sb.ContainerID)
	if err != nil {
		slog.Error("failed to create exec session", "error", err)
		s.sendTerminalError(conn, "failed to connect to container")
		return
	}
	defer execConn.Close()

	slog.Info("exec session created", "sandbox_id", sandboxID, "exec_id", execID)

	// Send connected message
	s.sendTerminalMessage(conn, TerminalMessage{
		Type: "connected",
		Data: "Connected to sandbox terminal",
	})

	// Start Claude Code automatically
	time.AfterFunc(500*time.Millisecond, func() {
		execConn.Write([]byte("claude\n"))
	})

	// Create context for cleanup
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var wg sync.WaitGroup

	// Read from container -> send to WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		buf := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := execConn.Read(buf)
				if err != nil {
					if err != io.EOF {
						slog.Debug("exec read error", "error", err)
					}
					return
				}
				if n > 0 {
					s.sendTerminalMessage(conn, TerminalMessage{
						Type: "output",
						Data: string(buf[:n]),
					})
				}
			}
		}
	}()

	// Read from WebSocket -> send to container
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						slog.Debug("websocket read error", "error", err)
					}
					return
				}

				var msg TerminalMessage
				if err := json.Unmarshal(message, &msg); err != nil {
					slog.Debug("invalid message format", "error", err)
					continue
				}

				switch msg.Type {
				case "input":
					// Block dangerous signals (Ctrl+C, Ctrl+D, Ctrl+Z)
					// These could interrupt Claude Code
					data := msg.Data
					blocked := false
					for _, b := range []byte{0x03, 0x04, 0x1a} { // Ctrl+C, Ctrl+D, Ctrl+Z
						if len(data) == 1 && data[0] == b {
							blocked = true
							break
						}
					}
					if !blocked {
						execConn.Write([]byte(data))
					}
				case "resize":
					// Resize is handled by docker exec, but we can log it
					slog.Debug("terminal resize", "cols", msg.Cols, "rows", msg.Rows)
					// Note: Resizing docker exec requires recreating the session
					// For now we ignore resize events
				}
			}
		}
	}()

	wg.Wait()
	slog.Info("terminal websocket disconnected", "sandbox_id", sandboxID)
}

func (s *Server) sendTerminalMessage(conn *websocket.Conn, msg TerminalMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal terminal message", "error", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		slog.Debug("failed to send terminal message", "error", err)
	}
}

func (s *Server) sendTerminalError(conn *websocket.Conn, message string) {
	s.sendTerminalMessage(conn, TerminalMessage{
		Type: "error",
		Data: message,
	})
}
