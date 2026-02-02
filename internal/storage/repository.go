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

	// Health
	Ping(ctx context.Context) error
	Close() error
}
