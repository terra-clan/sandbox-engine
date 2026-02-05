package storage

import (
	"context"

	"github.com/terra-clan/sandbox-engine/internal/models"
)

// Repository defines the interface for sandbox persistence
type Repository interface {
	// Sandboxes
	CreateSandbox(ctx context.Context, sb *models.Sandbox) error
	GetSandbox(ctx context.Context, id string) (*models.Sandbox, error)
	UpdateSandbox(ctx context.Context, sb *models.Sandbox) error
	DeleteSandbox(ctx context.Context, id string) error
	ListSandboxes(ctx context.Context, filters models.ListFilters) ([]*models.Sandbox, error)
	GetExpiredSandboxes(ctx context.Context) ([]*models.Sandbox, error)

	// Services
	CreateService(ctx context.Context, sandboxID string, svc *models.ServiceInstance) error
	GetServices(ctx context.Context, sandboxID string) ([]*models.ServiceInstance, error)
	UpdateService(ctx context.Context, sandboxID string, svc *models.ServiceInstance) error
	DeleteServices(ctx context.Context, sandboxID string) error

	// Sessions
	CreateSession(ctx context.Context, s *models.Session) error
	GetSessionByToken(ctx context.Context, token string) (*models.Session, error)
	GetSessionByID(ctx context.Context, id string) (*models.Session, error)
	UpdateSession(ctx context.Context, s *models.Session) error
	DeleteSession(ctx context.Context, id string) error
	ListSessions(ctx context.Context, status string, limit, offset int) ([]*models.Session, error)
	GetExpiredSessions(ctx context.Context) ([]*models.Session, error)

	// API Clients
	GetClientByApiKey(ctx context.Context, apiKey string) (*models.ApiClient, error)
	UpdateClientLastUsed(ctx context.Context, apiKey string) error

	// Health
	Ping(ctx context.Context) error
	Close() error
}
