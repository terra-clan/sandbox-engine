package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"

	"github.com/terra-clan/sandbox-engine/internal/config"
	"github.com/terra-clan/sandbox-engine/internal/models"
	"github.com/terra-clan/sandbox-engine/internal/services"
	"github.com/terra-clan/sandbox-engine/internal/templates"
)

// Common errors
var (
	ErrSandboxNotFound  = errors.New("sandbox not found")
	ErrTemplateNotFound = errors.New("template not found")
	ErrSandboxExpired   = errors.New("sandbox has expired")
	ErrSandboxStopped   = errors.New("sandbox is already stopped")
)

// Manager defines the interface for sandbox management
type Manager interface {
	Create(ctx context.Context, templateID, userID string, opts CreateOptions) (*models.Sandbox, error)
	Get(ctx context.Context, id string) (*models.Sandbox, error)
	Stop(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filters models.ListFilters) ([]*models.Sandbox, error)
	ExtendTTL(ctx context.Context, id string, duration time.Duration) error
	GetLogs(ctx context.Context, id string, tail int) (string, error)
	Ping(ctx context.Context) error
	GetExpired(ctx context.Context) ([]*models.Sandbox, error)
	Close() error
}

// CreateOptions holds optional parameters for sandbox creation
type CreateOptions struct {
	TTL      *time.Duration
	Env      map[string]string
	Metadata map[string]string
}

// DockerManager implements Manager using Docker
type DockerManager struct {
	docker         *client.Client
	config         config.DockerConfig
	serviceRegistry *services.Registry
	templateLoader *templates.Loader

	mu        sync.RWMutex
	sandboxes map[string]*models.Sandbox
}

// NewManager creates a new DockerManager
func NewManager(
	cfg config.DockerConfig,
	registry *services.Registry,
	loader *templates.Loader,
) (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(cfg.Host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &DockerManager{
		docker:          cli,
		config:          cfg,
		serviceRegistry: registry,
		templateLoader:  loader,
		sandboxes:       make(map[string]*models.Sandbox),
	}, nil
}

// Ping checks if the manager is operational
func (m *DockerManager) Ping(ctx context.Context) error {
	_, err := m.docker.Ping(ctx)
	return err
}

// Create creates a new sandbox from a template
func (m *DockerManager) Create(ctx context.Context, templateID, userID string, opts CreateOptions) (*models.Sandbox, error) {
	// Get template
	tmpl := m.templateLoader.Get(templateID)
	if tmpl == nil {
		return nil, ErrTemplateNotFound
	}

	// Generate sandbox ID
	id := uuid.New().String()[:12]

	// Calculate TTL
	ttl := tmpl.TTL
	if opts.TTL != nil {
		ttl = *opts.TTL
	}
	if ttl == 0 {
		ttl = 1 * time.Hour // default
	}

	now := time.Now()
	sb := &models.Sandbox{
		ID:         id,
		TemplateID: templateID,
		UserID:     userID,
		Status:     models.StatusPending,
		CreatedAt:  now,
		ExpiresAt:  now.Add(ttl),
		Services:   make(map[string]*models.ServiceInstance),
		Endpoints:  make(map[string]string),
		Metadata:   opts.Metadata,
	}

	// Store sandbox
	m.mu.Lock()
	m.sandboxes[id] = sb
	m.mu.Unlock()

	// Provision services asynchronously
	go m.provisionSandbox(context.Background(), sb, tmpl, opts.Env)

	slog.Info("sandbox created",
		"id", id,
		"template", templateID,
		"user", userID,
		"expires_at", sb.ExpiresAt,
	)

	return sb, nil
}

// provisionSandbox handles async provisioning of sandbox resources
func (m *DockerManager) provisionSandbox(ctx context.Context, sb *models.Sandbox, tmpl *models.Template, extraEnv map[string]string) {
	// Provision required services
	for _, serviceName := range tmpl.Services {
		provider := m.serviceRegistry.Get(serviceName)
		if provider == nil {
			m.updateStatus(sb.ID, models.StatusFailed, fmt.Sprintf("unknown service: %s", serviceName))
			return
		}

		creds, err := provider.Provision(ctx, sb.ID, serviceName)
		if err != nil {
			m.updateStatus(sb.ID, models.StatusFailed, fmt.Sprintf("failed to provision %s: %v", serviceName, err))
			return
		}

		m.mu.Lock()
		sb.Services[serviceName] = &models.ServiceInstance{
			Name:        serviceName,
			Type:        serviceName,
			Status:      "ready",
			Credentials: creds,
			CreatedAt:   time.Now(),
		}
		m.mu.Unlock()
	}

	// Pull image if needed
	if err := m.pullImage(ctx, tmpl.BaseImage); err != nil {
		m.updateStatus(sb.ID, models.StatusFailed, fmt.Sprintf("failed to pull image: %v", err))
		return
	}

	// Build environment variables
	env := m.buildEnv(sb, tmpl, extraEnv)

	// Create container
	containerID, err := m.createContainer(ctx, sb, tmpl, env)
	if err != nil {
		m.updateStatus(sb.ID, models.StatusFailed, fmt.Sprintf("failed to create container: %v", err))
		return
	}

	m.mu.Lock()
	sb.ContainerID = containerID
	m.mu.Unlock()

	// Start container
	if err := m.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		m.updateStatus(sb.ID, models.StatusFailed, fmt.Sprintf("failed to start container: %v", err))
		return
	}

	now := time.Now()
	m.mu.Lock()
	sb.StartedAt = &now
	sb.Status = models.StatusRunning
	sb.StatusMsg = ""
	m.mu.Unlock()

	slog.Info("sandbox started", "id", sb.ID, "container", containerID)
}

// pullImage pulls a Docker image if not present
func (m *DockerManager) pullImage(ctx context.Context, imageName string) error {
	if m.config.PullPolicy == "never" {
		return nil
	}

	// Check if image exists
	_, _, err := m.docker.ImageInspectWithRaw(ctx, imageName)
	if err == nil && m.config.PullPolicy == "if-not-present" {
		return nil
	}

	slog.Info("pulling image", "image", imageName)
	out, err := m.docker.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer out.Close()

	// Consume output
	_, _ = io.Copy(io.Discard, out)
	return nil
}

// buildEnv builds environment variables for the container
func (m *DockerManager) buildEnv(sb *models.Sandbox, tmpl *models.Template, extraEnv map[string]string) []string {
	env := make([]string, 0)

	// Template env
	for k, v := range tmpl.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Service credentials as env
	for name, svc := range sb.Services {
		prefix := strings.ToUpper(name)
		if svc.Credentials != nil {
			env = append(env, fmt.Sprintf("%s_HOST=%s", prefix, svc.Credentials.Host))
			env = append(env, fmt.Sprintf("%s_PORT=%d", prefix, svc.Credentials.Port))
			if svc.Credentials.Username != "" {
				env = append(env, fmt.Sprintf("%s_USER=%s", prefix, svc.Credentials.Username))
			}
			if svc.Credentials.Password != "" {
				env = append(env, fmt.Sprintf("%s_PASSWORD=%s", prefix, svc.Credentials.Password))
			}
			if svc.Credentials.Database != "" {
				env = append(env, fmt.Sprintf("%s_DATABASE=%s", prefix, svc.Credentials.Database))
			}
			if svc.Credentials.URI != "" {
				env = append(env, fmt.Sprintf("%s_URI=%s", prefix, svc.Credentials.URI))
			}
			if svc.Credentials.Prefix != "" {
				env = append(env, fmt.Sprintf("%s_PREFIX=%s", prefix, svc.Credentials.Prefix))
			}
		}
	}

	// Extra env (overrides)
	for k, v := range extraEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Sandbox metadata
	env = append(env, fmt.Sprintf("SANDBOX_ID=%s", sb.ID))
	env = append(env, fmt.Sprintf("SANDBOX_USER_ID=%s", sb.UserID))

	return env
}

// createContainer creates a Docker container for the sandbox
func (m *DockerManager) createContainer(ctx context.Context, sb *models.Sandbox, tmpl *models.Template, env []string) (string, error) {
	containerName := fmt.Sprintf("sandbox-%s", sb.ID)

	// Build port bindings
	exposedPorts := nat.PortSet{}
	for _, port := range tmpl.Expose {
		p := nat.Port(fmt.Sprintf("%d/%s", port.Container, port.Protocol))
		exposedPorts[p] = struct{}{}
	}

	// Resource limits
	resources := container.Resources{}
	if tmpl.Resources.MemoryLimit != "" {
		// Parse memory limit (e.g., "512m", "1g")
		// Simplified - in production use units parsing
		resources.Memory = 512 * 1024 * 1024 // 512MB default
	}

	// Labels for Traefik and metadata
	labels := map[string]string{
		"sandbox.id":       sb.ID,
		"sandbox.user":     sb.UserID,
		"sandbox.template": sb.TemplateID,
		"sandbox.managed":  "true",
	}
	for k, v := range tmpl.Labels {
		labels[k] = v
	}

	containerConfig := &container.Config{
		Image:        tmpl.BaseImage,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels:       labels,
	}

	hostConfig := &container.HostConfig{
		Resources:    resources,
		NetworkMode:  container.NetworkMode(m.config.Network),
		AutoRemove:   false,
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyDisabled,
		},
	}

	networkConfig := &network.NetworkingConfig{}

	resp, err := m.docker.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

// updateStatus updates sandbox status thread-safely
func (m *DockerManager) updateStatus(id string, status models.SandboxStatus, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sb, ok := m.sandboxes[id]; ok {
		sb.Status = status
		sb.StatusMsg = msg
	}
}

// Get retrieves a sandbox by ID
func (m *DockerManager) Get(ctx context.Context, id string) (*models.Sandbox, error) {
	m.mu.RLock()
	sb, ok := m.sandboxes[id]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrSandboxNotFound
	}

	return sb, nil
}

// Stop stops a running sandbox
func (m *DockerManager) Stop(ctx context.Context, id string) error {
	m.mu.Lock()
	sb, ok := m.sandboxes[id]
	m.mu.Unlock()

	if !ok {
		return ErrSandboxNotFound
	}

	if sb.Status.IsTerminal() {
		return ErrSandboxStopped
	}

	if sb.ContainerID != "" {
		timeout := 30
		if err := m.docker.ContainerStop(ctx, sb.ContainerID, container.StopOptions{Timeout: &timeout}); err != nil {
			slog.Warn("failed to stop container", "error", err, "container", sb.ContainerID)
		}
	}

	m.mu.Lock()
	sb.Status = models.StatusStopped
	m.mu.Unlock()

	slog.Info("sandbox stopped", "id", id)
	return nil
}

// Delete removes a sandbox and all its resources
func (m *DockerManager) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	sb, ok := m.sandboxes[id]
	m.mu.Unlock()

	if !ok {
		return ErrSandboxNotFound
	}

	// Stop container if running
	if sb.ContainerID != "" {
		timeout := 10
		_ = m.docker.ContainerStop(ctx, sb.ContainerID, container.StopOptions{Timeout: &timeout})
		_ = m.docker.ContainerRemove(ctx, sb.ContainerID, container.RemoveOptions{Force: true})
	}

	// Deprovision services
	for name := range sb.Services {
		provider := m.serviceRegistry.Get(name)
		if provider != nil {
			if err := provider.Deprovision(ctx, id, name); err != nil {
				slog.Warn("failed to deprovision service", "error", err, "service", name, "sandbox", id)
			}
		}
	}

	// Remove from map
	m.mu.Lock()
	delete(m.sandboxes, id)
	m.mu.Unlock()

	slog.Info("sandbox deleted", "id", id)
	return nil
}

// List returns sandboxes matching filters
func (m *DockerManager) List(ctx context.Context, filters models.ListFilters) ([]*models.Sandbox, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.Sandbox, 0)

	for _, sb := range m.sandboxes {
		// Apply filters
		if filters.UserID != "" && sb.UserID != filters.UserID {
			continue
		}
		if filters.TemplateID != "" && sb.TemplateID != filters.TemplateID {
			continue
		}
		if filters.Status != "" && sb.Status != filters.Status {
			continue
		}

		result = append(result, sb)
	}

	// Apply pagination
	if filters.Offset > 0 && filters.Offset < len(result) {
		result = result[filters.Offset:]
	}
	if filters.Limit > 0 && filters.Limit < len(result) {
		result = result[:filters.Limit]
	}

	return result, nil
}

// ExtendTTL extends the sandbox expiration time
func (m *DockerManager) ExtendTTL(ctx context.Context, id string, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sb, ok := m.sandboxes[id]
	if !ok {
		return ErrSandboxNotFound
	}

	if sb.Status.IsTerminal() {
		return ErrSandboxStopped
	}

	sb.ExpiresAt = sb.ExpiresAt.Add(duration)
	slog.Info("sandbox TTL extended", "id", id, "new_expires_at", sb.ExpiresAt)

	return nil
}

// GetLogs retrieves container logs
func (m *DockerManager) GetLogs(ctx context.Context, id string, tail int) (string, error) {
	m.mu.RLock()
	sb, ok := m.sandboxes[id]
	m.mu.RUnlock()

	if !ok {
		return "", ErrSandboxNotFound
	}

	if sb.ContainerID == "" {
		return "", nil
	}

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
	}

	logs, err := m.docker.ContainerLogs(ctx, sb.ContainerID, options)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer logs.Close()

	data, err := io.ReadAll(logs)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(data), nil
}

// GetExpired returns all expired sandboxes
func (m *DockerManager) GetExpired(ctx context.Context) ([]*models.Sandbox, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.Sandbox, 0)
	now := time.Now()

	for _, sb := range m.sandboxes {
		if !sb.Status.IsTerminal() && now.After(sb.ExpiresAt) {
			result = append(result, sb)
		}
	}

	return result, nil
}

// Close cleans up manager resources
func (m *DockerManager) Close() error {
	return m.docker.Close()
}
