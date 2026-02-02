package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/terra-clan/sandbox-engine/internal/storage"
)

// AuthMiddleware handles API key authentication
type AuthMiddleware struct {
	repo storage.Repository
}

// NewAuthMiddleware creates new auth middleware
func NewAuthMiddleware(repo storage.Repository) *AuthMiddleware {
	return &AuthMiddleware{repo: repo}
}

// Authenticate verifies API key from Authorization header
// Supports formats: "Bearer sk_xxx" or "sk_xxx" in Authorization header
// Also supports X-API-Key header
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := extractAPIKey(r)
		if apiKey == "" {
			writeAuthError(w, http.StatusUnauthorized, "missing api key", "provide Authorization header with Bearer token or X-API-Key header")
			return
		}

		// Lookup client by API key
		client, err := m.repo.GetClientByApiKey(r.Context(), apiKey)
		if err != nil {
			slog.Error("failed to lookup api client", "error", err, "key_prefix", maskKey(apiKey))
			writeAuthError(w, http.StatusInternalServerError, "authentication error", "internal server error")
			return
		}

		if client == nil {
			slog.Warn("invalid api key attempt", "key_prefix", maskKey(apiKey), "remote_addr", r.RemoteAddr)
			writeAuthError(w, http.StatusUnauthorized, "invalid api key", "the provided api key is not valid")
			return
		}

		if !client.IsActive {
			slog.Warn("inactive client attempt", "client", client.Name, "key_prefix", maskKey(apiKey))
			writeAuthError(w, http.StatusUnauthorized, "client inactive", "this api key has been deactivated")
			return
		}

		// Update last_used_at asynchronously (don't block request)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5000000000) // 5 seconds
			defer cancel()
			if err := m.repo.UpdateClientLastUsed(ctx, apiKey); err != nil {
				slog.Error("failed to update client last_used_at", "error", err, "client", client.Name)
			}
		}()

		slog.Debug("authenticated request", "client", client.Name, "key_prefix", client.MaskedApiKey())

		// Add client to context and continue
		ctx := ContextWithClient(r.Context(), client)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission returns middleware that checks for specific permission
func (m *AuthMiddleware) RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client := ClientFromContext(r.Context())
			if client == nil {
				writeAuthError(w, http.StatusUnauthorized, "not authenticated", "authentication required")
				return
			}

			if !client.HasPermission(permission) {
				slog.Warn("permission denied",
					"client", client.Name,
					"required", permission,
					"has", client.Permissions,
				)
				writeAuthError(w, http.StatusForbidden, "permission denied",
					"client does not have required permission: "+permission)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractAPIKey extracts API key from request headers
func extractAPIKey(r *http.Request) string {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Handle "Bearer sk_xxx" format
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
		// Handle raw key in Authorization header
		return authHeader
	}

	// Fallback to X-API-Key header
	return r.Header.Get("X-API-Key")
}

// maskKey returns first 8 chars of key for safe logging
func maskKey(key string) string {
	if len(key) < 8 {
		return "***"
	}
	return key[:8] + "..."
}

// AuthError represents an authentication error response
type AuthError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// writeAuthError writes JSON error response
func writeAuthError(w http.ResponseWriter, status int, error, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(AuthError{
		Error:   error,
		Message: message,
	})
}
