package services

import (
	"context"

	"github.com/terra-clan/sandbox-engine/internal/models"
)

// Provider defines the interface for service provisioning
type Provider interface {
	// Provision creates resources for a sandbox
	Provision(ctx context.Context, sandboxID, serviceName string) (*models.ServiceCredentials, error)

	// Deprovision removes all resources for a sandbox
	Deprovision(ctx context.Context, sandboxID, serviceName string) error

	// Type returns the service type name
	Type() string

	// HealthCheck checks if the service is available
	HealthCheck(ctx context.Context) error
}

// BaseProvider provides common functionality for providers
type BaseProvider struct {
	serviceType string
}

// Type returns the service type
func (p *BaseProvider) Type() string {
	return p.serviceType
}
