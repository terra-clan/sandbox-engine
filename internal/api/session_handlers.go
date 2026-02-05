package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/terra-clan/sandbox-engine/internal/models"
	"github.com/terra-clan/sandbox-engine/internal/sandbox"
)

// --- Admin handlers (API key auth) ---

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req models.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.TemplateID == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "template_id is required")
		return
	}

	if req.TTL <= 0 {
		respondError(w, http.StatusBadRequest, "validation_error", "ttl must be positive (seconds)")
		return
	}

	// Identify who created the session
	createdBy := ""
	if client := ClientFromContext(r.Context()); client != nil {
		createdBy = client.Name
	}

	session, err := s.sandboxManager.CreateSession(r.Context(), req, createdBy)
	if err != nil {
		if errors.Is(err, sandbox.ErrTemplateNotFound) {
			respondError(w, http.StatusNotFound, "template_not_found", "template not found")
			return
		}
		slog.Error("failed to create session", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to create session")
		return
	}

	// Build join URL
	domain := s.config.Host
	if domain == "0.0.0.0" {
		domain = "localhost"
	}
	joinURL := fmt.Sprintf("http://%s:%d/join/%s", domain, s.config.Port, session.Token)

	respondJSON(w, http.StatusCreated, models.CreateSessionResponse{
		ID:         session.ID,
		Token:      session.Token,
		TemplateID: session.TemplateID,
		Status:     session.Status,
		JoinURL:    joinURL,
		CreatedAt:  session.CreatedAt,
	})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := 50
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	sessions, err := s.sandboxManager.ListSessions(r.Context(), status, limit, offset)
	if err != nil {
		slog.Error("failed to list sessions", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to list sessions")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "session id is required")
		return
	}

	session, err := s.sandboxManager.GetSessionByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sandbox.ErrSessionNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "session not found")
			return
		}
		slog.Error("failed to get session", "error", err, "id", id)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to get session")
		return
	}

	respondJSON(w, http.StatusOK, session)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "session id is required")
		return
	}

	if err := s.sandboxManager.DeleteSession(r.Context(), id); err != nil {
		if errors.Is(err, sandbox.ErrSessionNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "session not found")
			return
		}
		slog.Error("failed to delete session", "error", err, "id", id)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to delete session")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "session deleted",
	})
}

// --- Public handlers (session token = auth) ---

func (s *Server) handleJoinSession(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "session token is required")
		return
	}

	session, err := s.sandboxManager.GetSessionByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, sandbox.ErrSessionNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "session not found")
			return
		}
		slog.Error("failed to get session by token", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to get session")
		return
	}

	resp := models.JoinSessionResponse{
		Status:   session.Status,
		Metadata: session.Metadata,
	}

	// Populate template info
	tmpl := s.templateLoader.Get(session.TemplateID)
	if tmpl != nil {
		resp.Template = &models.TemplateInfo{
			Name:        tmpl.Name,
			Description: tmpl.Description,
		}
	}

	// If active, include sandbox info
	if session.Status == models.SessionActive && session.SandboxID != "" {
		sb, err := s.sandboxManager.Get(r.Context(), session.SandboxID)
		if err == nil {
			resp.Sandbox = &models.SandboxInfo{
				ID:        sb.ID,
				Status:    string(sb.Status),
				Endpoints: sb.Endpoints,
				ExpiresAt: session.ExpiresAt,
			}
		}
	}

	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleActivateSession(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "session token is required")
		return
	}

	session, err := s.sandboxManager.ActivateSession(r.Context(), token)
	if err != nil {
		if errors.Is(err, sandbox.ErrSessionNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "session not found")
			return
		}
		if errors.Is(err, sandbox.ErrSessionNotReady) {
			respondError(w, http.StatusConflict, "not_ready", "session is not in ready state")
			return
		}
		slog.Error("failed to activate session", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to activate session")
		return
	}

	respondJSON(w, http.StatusOK, models.ActivateSessionResponse{
		Status:    session.Status,
		SandboxID: session.SandboxID,
	})
}

// handleSessionTerminalWS handles WebSocket terminal with session token auth
func (s *Server) handleSessionTerminalWS(w http.ResponseWriter, r *http.Request) {
	sandboxID := chi.URLParam(r, "id")
	sessionToken := r.URL.Query().Get("session_token")

	if sandboxID == "" {
		http.Error(w, "sandbox id required", http.StatusBadRequest)
		return
	}
	if sessionToken == "" {
		http.Error(w, "session_token required", http.StatusUnauthorized)
		return
	}

	// Validate session token and match sandbox
	session, err := s.sandboxManager.GetSessionByToken(r.Context(), sessionToken)
	if err != nil {
		http.Error(w, "invalid session token", http.StatusUnauthorized)
		return
	}

	if session.Status != models.SessionActive {
		http.Error(w, "session is not active", http.StatusBadRequest)
		return
	}

	if session.SandboxID != sandboxID {
		http.Error(w, "sandbox does not belong to this session", http.StatusForbidden)
		return
	}

	// Delegate to the existing terminal handler
	s.handleTerminalWS(w, r)
}
