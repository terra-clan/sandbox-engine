package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
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
	"github.com/terra-clan/sandbox-engine/internal/storage"
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
	ExecAttach(ctx context.Context, containerID string) (string, io.ReadWriteCloser, error)
	ExecResize(ctx context.Context, execID string, height, width uint) error
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
	docker          *client.Client
	config          config.DockerConfig
	traefikConfig   config.TraefikConfig
	serviceRegistry *services.Registry
	templateLoader  *templates.Loader
	repo            storage.Repository
}

// NewManager creates a new DockerManager
func NewManager(
	cfg config.DockerConfig,
	traefikCfg config.TraefikConfig,
	registry *services.Registry,
	loader *templates.Loader,
	repo storage.Repository,
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
		traefikConfig:   traefikCfg,
		serviceRegistry: registry,
		templateLoader:  loader,
		repo:            repo,
	}, nil
}

// Ping checks if the manager is operational
func (m *DockerManager) Ping(ctx context.Context) error {
	// Check Docker connectivity
	if _, err := m.docker.Ping(ctx); err != nil {
		return fmt.Errorf("docker ping failed: %w", err)
	}

	// Check database connectivity
	if err := m.repo.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
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

	// Store sandbox in database
	if err := m.repo.CreateSandbox(ctx, sb); err != nil {
		return nil, fmt.Errorf("failed to create sandbox: %w", err)
	}

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
			m.updateStatus(ctx, sb.ID, models.StatusFailed, fmt.Sprintf("unknown service: %s", serviceName))
			return
		}

		creds, err := provider.Provision(ctx, sb.ID, serviceName)
		if err != nil {
			m.updateStatus(ctx, sb.ID, models.StatusFailed, fmt.Sprintf("failed to provision %s: %v", serviceName, err))
			return
		}

		svcInstance := &models.ServiceInstance{
			Name:        serviceName,
			Type:        serviceName,
			Status:      "ready",
			Credentials: creds,
			CreatedAt:   time.Now(),
		}

		// Store service in database
		if err := m.repo.CreateService(ctx, sb.ID, svcInstance); err != nil {
			slog.Error("failed to save service to database", "error", err, "sandbox", sb.ID, "service", serviceName)
		}

		sb.Services[serviceName] = svcInstance
	}

	// Pull image if needed
	if err := m.pullImage(ctx, tmpl.BaseImage); err != nil {
		m.updateStatus(ctx, sb.ID, models.StatusFailed, fmt.Sprintf("failed to pull image: %v", err))
		return
	}

	// Build environment variables
	env := m.buildEnv(sb, tmpl, extraEnv)

	// Create container
	containerID, err := m.createContainer(ctx, sb, tmpl, env)
	if err != nil {
		m.updateStatus(ctx, sb.ID, models.StatusFailed, fmt.Sprintf("failed to create container: %v", err))
		return
	}

	sb.ContainerID = containerID

	// Build endpoints for Traefik routing
	sb.Endpoints = m.buildEndpoints(sb, tmpl)

	// Start container
	if err := m.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		m.updateStatus(ctx, sb.ID, models.StatusFailed, fmt.Sprintf("failed to start container: %v", err))
		return
	}

	now := time.Now()
	sb.StartedAt = &now
	sb.Status = models.StatusRunning
	sb.StatusMsg = ""

	// Update sandbox in database
	if err := m.repo.UpdateSandbox(ctx, sb); err != nil {
		slog.Error("failed to update sandbox in database", "error", err, "id", sb.ID)
	}

	slog.Info("sandbox started", "id", sb.ID, "container", containerID, "endpoints", sb.Endpoints)
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

// buildTraefikLabels generates Traefik labels for automatic routing
func (m *DockerManager) buildTraefikLabels(sb *models.Sandbox, tmpl *models.Template) map[string]string {
	if !m.traefikConfig.Enabled {
		return nil
	}

	sandboxHost := fmt.Sprintf("%s.%s", sb.ID, m.traefikConfig.Domain)
	routerName := fmt.Sprintf("sandbox-%s", sb.ID)

	labels := map[string]string{
		"traefik.enable":         "true",
		"traefik.docker.network": m.traefikConfig.Network,

		// HTTP router
		fmt.Sprintf("traefik.http.routers.%s.rule", routerName):        fmt.Sprintf("Host(`%s`)", sandboxHost),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", routerName): m.traefikConfig.EntryPoint,
		fmt.Sprintf("traefik.http.routers.%s.service", routerName):     routerName,

		// Service (default port 8080)
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", routerName): "8080",
	}

	// Add TLS labels only if cert resolver is configured
	if m.traefikConfig.CertResolver != "" {
		labels[fmt.Sprintf("traefik.http.routers.%s.tls", routerName)] = "true"
		labels[fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", routerName)] = m.traefikConfig.CertResolver
	}

	// Add labels for each exposed port from template
	for _, port := range tmpl.Expose {
		if port.Public {
			portRouterName := fmt.Sprintf("sandbox-%s-%s", sb.ID, port.Name)
			portHost := fmt.Sprintf("%s-%s.%s", sb.ID, port.Name, m.traefikConfig.Domain)

			labels[fmt.Sprintf("traefik.http.routers.%s.rule", portRouterName)] = fmt.Sprintf("Host(`%s`)", portHost)
			labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", portRouterName)] = m.traefikConfig.EntryPoint
			labels[fmt.Sprintf("traefik.http.routers.%s.service", portRouterName)] = portRouterName
			labels[fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", portRouterName)] = fmt.Sprintf("%d", port.Container)

			// Add TLS labels only if cert resolver is configured
			if m.traefikConfig.CertResolver != "" {
				labels[fmt.Sprintf("traefik.http.routers.%s.tls", portRouterName)] = "true"
				labels[fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", portRouterName)] = m.traefikConfig.CertResolver
			}
		}
	}

	return labels
}

// buildEndpoints generates endpoint URLs for the sandbox
func (m *DockerManager) buildEndpoints(sb *models.Sandbox, tmpl *models.Template) map[string]string {
	if !m.traefikConfig.Enabled {
		return make(map[string]string)
	}

	// Always use https - SSL is terminated by nginx
	scheme := "https"

	endpoints := make(map[string]string)
	endpoints["main"] = fmt.Sprintf("%s://%s.%s", scheme, sb.ID, m.traefikConfig.Domain)

	for _, port := range tmpl.Expose {
		if port.Public {
			endpoints[port.Name] = fmt.Sprintf("%s://%s-%s.%s", scheme, sb.ID, port.Name, m.traefikConfig.Domain)
		}
	}

	return endpoints
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

	// Labels for metadata
	labels := map[string]string{
		"sandbox.id":       sb.ID,
		"sandbox.user":     sb.UserID,
		"sandbox.template": sb.TemplateID,
		"sandbox.managed":  "true",
	}
	// Add template labels
	for k, v := range tmpl.Labels {
		labels[k] = v
	}
	// Add Traefik labels for automatic routing
	traefikLabels := m.buildTraefikLabels(sb, tmpl)
	for k, v := range traefikLabels {
		labels[k] = v
	}

	containerConfig := &container.Config{
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Image:        tmpl.BaseImage,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels:       labels,
	}

	hostConfig := &container.HostConfig{
		Binds: []string{"claude-auth:/home/coder/.claude"},
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

// updateStatus updates sandbox status in database
func (m *DockerManager) updateStatus(ctx context.Context, id string, status models.SandboxStatus, msg string) {
	sb, err := m.repo.GetSandbox(ctx, id)
	if err != nil || sb == nil {
		slog.Error("failed to get sandbox for status update", "error", err, "id", id)
		return
	}

	sb.Status = status
	sb.StatusMsg = msg

	if err := m.repo.UpdateSandbox(ctx, sb); err != nil {
		slog.Error("failed to update sandbox status", "error", err, "id", id, "status", status)
	}
}

// Get retrieves a sandbox by ID
func (m *DockerManager) Get(ctx context.Context, id string) (*models.Sandbox, error) {
	sb, err := m.repo.GetSandbox(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get sandbox: %w", err)
	}

	if sb == nil {
		return nil, ErrSandboxNotFound
	}

	return sb, nil
}

// Stop stops a running sandbox
func (m *DockerManager) Stop(ctx context.Context, id string) error {
	sb, err := m.repo.GetSandbox(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get sandbox: %w", err)
	}

	if sb == nil {
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

	sb.Status = models.StatusStopped
	if err := m.repo.UpdateSandbox(ctx, sb); err != nil {
		return fmt.Errorf("failed to update sandbox status: %w", err)
	}

	slog.Info("sandbox stopped", "id", id)
	return nil
}

// Delete removes a sandbox and all its resources
func (m *DockerManager) Delete(ctx context.Context, id string) error {
	sb, err := m.repo.GetSandbox(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get sandbox: %w", err)
	}

	if sb == nil {
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

	// Delete from database (services will be cascade deleted)
	if err := m.repo.DeleteSandbox(ctx, id); err != nil {
		return fmt.Errorf("failed to delete sandbox from database: %w", err)
	}

	slog.Info("sandbox deleted", "id", id)
	return nil
}

// List returns sandboxes matching filters
func (m *DockerManager) List(ctx context.Context, filters models.ListFilters) ([]*models.Sandbox, error) {
	sandboxes, err := m.repo.ListSandboxes(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list sandboxes: %w", err)
	}

	return sandboxes, nil
}

// ExtendTTL extends the sandbox expiration time
func (m *DockerManager) ExtendTTL(ctx context.Context, id string, duration time.Duration) error {
	sb, err := m.repo.GetSandbox(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get sandbox: %w", err)
	}

	if sb == nil {
		return ErrSandboxNotFound
	}

	if sb.Status.IsTerminal() {
		return ErrSandboxStopped
	}

	sb.ExpiresAt = sb.ExpiresAt.Add(duration)

	if err := m.repo.UpdateSandbox(ctx, sb); err != nil {
		return fmt.Errorf("failed to update sandbox TTL: %w", err)
	}

	slog.Info("sandbox TTL extended", "id", id, "new_expires_at", sb.ExpiresAt)

	return nil
}

// GetLogs retrieves container logs
func (m *DockerManager) GetLogs(ctx context.Context, id string, tail int) (string, error) {
	sb, err := m.repo.GetSandbox(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get sandbox: %w", err)
	}

	if sb == nil {
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
	sandboxes, err := m.repo.GetExpiredSandboxes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired sandboxes: %w", err)
	}

	return sandboxes, nil
}

// ExecAttach creates an interactive exec session to a container
func (m *DockerManager) ExecAttach(ctx context.Context, containerID string) (string, io.ReadWriteCloser, error) {
	execConfig := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          []string{"/bin/bash", "--login"},
		Env: []string{
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
		},
	}

	execResp, err := m.docker.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := m.docker.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{
		Tty: true,
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to attach exec: %w", err)
	}

	return execResp.ID, attachResp.Conn, nil
}

// Close cleans up manager resources
func (m *DockerManager) Close() error {
	// Close database connection
	if err := m.repo.Close(); err != nil {
		slog.Warn("failed to close repository", "error", err)
	}

	return m.docker.Close()
}

// ExecResize resizes the TTY of an exec session
func (m *DockerManager) ExecResize(ctx context.Context, execID string, height, width uint) error {
	return m.docker.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: height,
		Width:  width,
	})
}
