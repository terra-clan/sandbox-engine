package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/terra-clan/sandbox-engine/internal/config"
	"github.com/terra-clan/sandbox-engine/internal/sandbox"
	"github.com/terra-clan/sandbox-engine/internal/templates"
)

// Server represents the HTTP API server
type Server struct {
	config         config.ServerConfig
	router         *chi.Mux
	sandboxManager sandbox.Manager
	templateLoader *templates.Loader
}

// NewServer creates a new API server
func NewServer(
	cfg config.ServerConfig,
	manager sandbox.Manager,
	loader *templates.Loader,
) *Server {
	s := &Server{
		config:         cfg,
		sandboxManager: manager,
		templateLoader: loader,
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

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.loggingMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check (outside versioned API)
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Sandboxes
		r.Route("/sandboxes", func(r chi.Router) {
			r.Get("/", s.handleListSandboxes)
			r.Post("/", s.handleCreateSandbox)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.handleGetSandbox)
				r.Delete("/", s.handleDeleteSandbox)
				r.Post("/extend", s.handleExtendTTL)
				r.Post("/stop", s.handleStopSandbox)
				r.Get("/logs", s.handleGetLogs)
			})
		})

		// Templates
		r.Route("/templates", func(r chi.Router) {
			r.Get("/", s.handleListTemplates)
			r.Get("/{name}", s.handleGetTemplate)
		})
	})

	s.router = r
}

// loggingMiddleware logs HTTP requests using slog
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
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
