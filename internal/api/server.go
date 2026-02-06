package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/terra-clan/sandbox-engine/internal/config"
	"github.com/terra-clan/sandbox-engine/internal/sandbox"
	"github.com/terra-clan/sandbox-engine/internal/storage"
	"github.com/terra-clan/sandbox-engine/internal/templates"
)

// Server represents the HTTP API server
type Server struct {
	config         config.ServerConfig
	router         *chi.Mux
	sandboxManager sandbox.Manager
	templateLoader *templates.Loader
	authMiddleware *AuthMiddleware
}

// NewServer creates a new API server
func NewServer(
	cfg config.ServerConfig,
	manager sandbox.Manager,
	loader *templates.Loader,
	repo storage.Repository,
) *Server {
	s := &Server{
		config:         cfg,
		sandboxManager: manager,
		templateLoader: loader,
		authMiddleware: NewAuthMiddleware(repo),
	}
	s.setupRouter()
	return s
}

// Router returns the configured router
func (s *Server) Router() http.Handler {
	return s.router
}

// setupRouter configures all routes and middleware
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Base middleware stack (no timeout - WebSocket needs long connections)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.loggingMiddleware)
	r.Use(middleware.Recoverer)

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check (outside versioned API - public)
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// --- Public routes (no API key required) ---

		// Join endpoints — session token is the auth
		r.Route("/join/{token}", func(r chi.Router) {
			r.Use(middleware.Timeout(60 * time.Second))
			r.Get("/", s.handleJoinSession)
			r.Post("/activate", s.handleActivateSession)
		})

		// WebSocket terminal with session token auth (public)
		r.Get("/ws/session-terminal/{id}", s.handleSessionTerminalWS)

		// --- Authenticated routes (API key required) ---
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware.Authenticate)

			// WebSocket terminal - NO timeout (needs long-lived connections)
			r.Get("/ws/terminal/{id}", s.handleTerminalWS)

			// REST API routes - with timeout
			r.Group(func(r chi.Router) {
				r.Use(middleware.Timeout(60 * time.Second))

				// Sandboxes
				r.Route("/sandboxes", func(r chi.Router) {
					r.With(s.authMiddleware.RequirePermission("sandboxes:read")).Get("/", s.handleListSandboxes)
					r.With(s.authMiddleware.RequirePermission("sandboxes:write")).Post("/", s.handleCreateSandbox)

					r.Route("/{id}", func(r chi.Router) {
						r.With(s.authMiddleware.RequirePermission("sandboxes:read")).Get("/", s.handleGetSandbox)
						r.With(s.authMiddleware.RequirePermission("sandboxes:write")).Delete("/", s.handleDeleteSandbox)
						r.With(s.authMiddleware.RequirePermission("sandboxes:write")).Post("/extend", s.handleExtendTTL)
						r.With(s.authMiddleware.RequirePermission("sandboxes:write")).Post("/stop", s.handleStopSandbox)
						r.With(s.authMiddleware.RequirePermission("sandboxes:read")).Get("/logs", s.handleGetLogs)
					})
				})

				// Sessions (admin management)
				r.Route("/sessions", func(r chi.Router) {
					r.With(s.authMiddleware.RequirePermission("sessions:read")).Get("/", s.handleListSessions)
					r.With(s.authMiddleware.RequirePermission("sessions:write")).Post("/", s.handleCreateSession)

					r.Route("/{id}", func(r chi.Router) {
						r.With(s.authMiddleware.RequirePermission("sessions:read")).Get("/", s.handleGetSession)
						r.With(s.authMiddleware.RequirePermission("sessions:write")).Delete("/", s.handleDeleteSession)
					})
				})

				// Templates
				r.Route("/templates", func(r chi.Router) {
					r.With(s.authMiddleware.RequirePermission("templates:read")).Get("/", s.handleListTemplates)
					r.With(s.authMiddleware.RequirePermission("templates:read")).Get("/{name}", s.handleGetTemplate)
				})

				// Catalog (hierarchical: domains → projects → tasks)
				r.Route("/catalog", func(r chi.Router) {
					r.With(s.authMiddleware.RequirePermission("templates:read")).Get("/domains", s.handleListDomains)

					r.Route("/domains/{domainId}", func(r chi.Router) {
						r.With(s.authMiddleware.RequirePermission("templates:read")).Get("/", s.handleGetDomain)
						r.With(s.authMiddleware.RequirePermission("templates:read")).Get("/projects", s.handleListProjects)

						r.Route("/projects/{projectName}", func(r chi.Router) {
							r.With(s.authMiddleware.RequirePermission("templates:read")).Get("/", s.handleGetProject)
							r.With(s.authMiddleware.RequirePermission("templates:read")).Get("/tasks", s.handleListTasks)
							r.With(s.authMiddleware.RequirePermission("templates:read")).Get("/tasks/{taskCode}", s.handleGetTask)
						})
					})
				})
			})
		})
	})

	// Serve SPA from web/dist (if exists)
	s.serveSPA(r)

	s.router = r
}

// serveSPA serves the React SPA from web/dist directory.
// For any request that doesn't match an API route and isn't a static file,
// it serves index.html so React Router handles client-side routing.
func (s *Server) serveSPA(r *chi.Mux) {
	// Find web/dist relative to the executable or working directory
	distPath := "web/dist"
	if _, err := os.Stat(distPath); os.IsNotExist(err) {
		// Try relative to executable
		exe, _ := os.Executable()
		distPath = filepath.Join(filepath.Dir(exe), "web", "dist")
	}
	if _, err := os.Stat(distPath); os.IsNotExist(err) {
		slog.Warn("web/dist not found, SPA not served", "path", distPath)
		return
	}

	slog.Info("serving SPA", "path", distPath)
	distFS := os.DirFS(distPath)

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		// Try to serve the exact file first (JS, CSS, images, etc.)
		cleanPath := strings.TrimPrefix(req.URL.Path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		if f, err := distFS.(fs.ReadFileFS).ReadFile(cleanPath); err == nil {
			// Serve the static file with correct content type
			contentType := "application/octet-stream"
			switch {
			case strings.HasSuffix(cleanPath, ".html"):
				contentType = "text/html; charset=utf-8"
			case strings.HasSuffix(cleanPath, ".js"):
				contentType = "application/javascript"
			case strings.HasSuffix(cleanPath, ".css"):
				contentType = "text/css"
			case strings.HasSuffix(cleanPath, ".svg"):
				contentType = "image/svg+xml"
			case strings.HasSuffix(cleanPath, ".png"):
				contentType = "image/png"
			case strings.HasSuffix(cleanPath, ".ico"):
				contentType = "image/x-icon"
			case strings.HasSuffix(cleanPath, ".json"):
				contentType = "application/json"
			case strings.HasSuffix(cleanPath, ".woff2"):
				contentType = "font/woff2"
			case strings.HasSuffix(cleanPath, ".woff"):
				contentType = "font/woff"
			}
			w.Header().Set("Content-Type", contentType)
			w.Write(f)
			return
		}

		// For any non-file path, serve index.html (SPA fallback)
		indexHTML, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
}

// loggingMiddleware logs HTTP requests using slog
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			// Skip noisy logging for WebSocket and health checks
			if strings.Contains(r.URL.Path, "/ws/") || r.URL.Path == "/health" || r.URL.Path == "/ready" {
				return
			}
			slog.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
				"remote_addr", r.RemoteAddr,
			)
		}()

		next.ServeHTTP(ww, r)
	})
}
