package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/terra-clan/sandbox-engine/internal/models"
	"github.com/terra-clan/sandbox-engine/internal/sandbox"
)

// Response helpers

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *apiError   `json:"error,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := apiResponse{
		Success: status >= 200 && status < 300,
		Data:    data,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := apiResponse{
		Success: false,
		Error: &apiError{
			Code:    code,
			Message: message,
		},
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

// Health handlers

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	// Check if sandbox manager is ready
	if err := s.sandboxManager.Ping(r.Context()); err != nil {
		respondError(w, http.StatusServiceUnavailable, "not_ready", "service not ready")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}

// Sandbox handlers

func (s *Server) handleCreateSandbox(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.TemplateID == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "template_id is required")
		return
	}

	if req.UserID == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "user_id is required")
		return
	}

	sb, err := s.sandboxManager.Create(r.Context(), req.TemplateID, req.UserID, sandbox.CreateOptions{
		TTL:      req.TTL,
		Env:      req.Env,
		Metadata: req.Metadata,
	})
	if err != nil {
		if errors.Is(err, sandbox.ErrTemplateNotFound) {
			respondError(w, http.StatusNotFound, "template_not_found", "template not found")
			return
		}
		slog.Error("failed to create sandbox", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to create sandbox")
		return
	}

	respondJSON(w, http.StatusCreated, sb)
}

func (s *Server) handleGetSandbox(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "sandbox id is required")
		return
	}

	sb, err := s.sandboxManager.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, sandbox.ErrSandboxNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "sandbox not found")
			return
		}
		slog.Error("failed to get sandbox", "error", err, "id", id)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to get sandbox")
		return
	}

	respondJSON(w, http.StatusOK, sb)
}

func (s *Server) handleDeleteSandbox(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "sandbox id is required")
		return
	}

	if err := s.sandboxManager.Delete(r.Context(), id); err != nil {
		if errors.Is(err, sandbox.ErrSandboxNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "sandbox not found")
			return
		}
		slog.Error("failed to delete sandbox", "error", err, "id", id)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to delete sandbox")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "sandbox deleted",
	})
}

func (s *Server) handleStopSandbox(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "sandbox id is required")
		return
	}

	if err := s.sandboxManager.Stop(r.Context(), id); err != nil {
		if errors.Is(err, sandbox.ErrSandboxNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "sandbox not found")
			return
		}
		slog.Error("failed to stop sandbox", "error", err, "id", id)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to stop sandbox")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "sandbox stopped",
	})
}

func (s *Server) handleListSandboxes(w http.ResponseWriter, r *http.Request) {
	filters := models.ListFilters{
		UserID:     r.URL.Query().Get("user_id"),
		TemplateID: r.URL.Query().Get("template_id"),
		Status:     models.SandboxStatus(r.URL.Query().Get("status")),
		Limit:      50, // default
		Offset:     0,
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filters.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filters.Offset = offset
		}
	}

	sandboxes, err := s.sandboxManager.List(r.Context(), filters)
	if err != nil {
		slog.Error("failed to list sandboxes", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to list sandboxes")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sandboxes": sandboxes,
		"total":     len(sandboxes),
	})
}

func (s *Server) handleExtendTTL(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "sandbox id is required")
		return
	}

	var req models.ExtendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.Duration <= 0 {
		respondError(w, http.StatusBadRequest, "validation_error", "duration must be positive")
		return
	}

	if err := s.sandboxManager.ExtendTTL(r.Context(), id, req.Duration); err != nil {
		if errors.Is(err, sandbox.ErrSandboxNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "sandbox not found")
			return
		}
		slog.Error("failed to extend TTL", "error", err, "id", id)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to extend TTL")
		return
	}

	// Get updated sandbox
	sb, _ := s.sandboxManager.Get(r.Context(), id)
	respondJSON(w, http.StatusOK, sb)
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "sandbox id is required")
		return
	}

	tail := 100 // default
	if tailStr := r.URL.Query().Get("tail"); tailStr != "" {
		if t, err := strconv.Atoi(tailStr); err == nil && t > 0 {
			tail = t
		}
	}

	logs, err := s.sandboxManager.GetLogs(r.Context(), id, tail)
	if err != nil {
		if errors.Is(err, sandbox.ErrSandboxNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "sandbox not found")
			return
		}
		slog.Error("failed to get logs", "error", err, "id", id)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to get logs")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"logs": logs,
	})
}

// Template handlers

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates := s.templateLoader.List()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "validation_error", "template name is required")
		return
	}

	template := s.templateLoader.Get(name)
	if template == nil {
		respondError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}

	respondJSON(w, http.StatusOK, template)
}
