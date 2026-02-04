package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type TerminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

func (s *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	sandboxID := chi.URLParam(r, "id")
	if sandboxID == "" {
		http.Error(w, "sandbox id required", http.StatusBadRequest)
		return
	}

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

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("failed to upgrade to websocket", "error", err)
		return
	}
	defer conn.Close()

	slog.Info("terminal websocket connected", "sandbox_id", sandboxID)

	execCtx := context.Background()

	execID, execConn, err := s.sandboxManager.ExecAttach(execCtx, sb.ContainerID)
	if err != nil {
		slog.Error("failed to create exec session", "error", err)
		s.sendTerminalError(conn, "failed to connect to container")
		return
	}
	defer execConn.Close()

	slog.Info("exec session created", "sandbox_id", sandboxID, "exec_id", execID)

	// Set initial terminal size (80x24 default)
	if err := s.sandboxManager.ExecResize(execCtx, execID, 24, 80); err != nil {
		slog.Warn("failed to set initial terminal size", "error", err)
	}

	s.sendTerminalMessage(conn, TerminalMessage{
		Type: "connected",
		Data: "Connected to sandbox terminal",
	})

	ctx, cancel := context.WithCancel(context.Background())
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
					if err := s.sendTerminalMessage(conn, TerminalMessage{
						Type: "output",
						Data: string(buf[:n]),
					}); err != nil {
						return
					}
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
					execConn.Write([]byte(msg.Data))
				case "resize":
					if msg.Cols > 0 && msg.Rows > 0 {
						if err := s.sandboxManager.ExecResize(execCtx, execID, uint(msg.Rows), uint(msg.Cols)); err != nil {
							slog.Debug("failed to resize terminal", "error", err, "cols", msg.Cols, "rows", msg.Rows)
						} else {
							slog.Debug("terminal resized", "cols", msg.Cols, "rows", msg.Rows)
						}
					}
				}
			}
		}
	}()

	wg.Wait()
	slog.Info("terminal websocket disconnected", "sandbox_id", sandboxID)
}

func (s *Server) sendTerminalMessage(conn *websocket.Conn, msg TerminalMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal terminal message", "error", err)
		return err
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		slog.Debug("failed to send terminal message", "error", err)
		return err
	}
	return nil
}

func (s *Server) sendTerminalError(conn *websocket.Conn, message string) {
	s.sendTerminalMessage(conn, TerminalMessage{
		Type: "error",
		Data: message,
	})
}
