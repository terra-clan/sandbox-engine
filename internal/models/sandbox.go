package models

import (
	"time"
)

// SandboxStatus represents the current state of a sandbox
type SandboxStatus string

const (
	StatusPending  SandboxStatus = "pending"
	StatusRunning  SandboxStatus = "running"
	StatusStopped  SandboxStatus = "stopped"
	StatusFailed   SandboxStatus = "failed"
	StatusExpired  SandboxStatus = "expired"
)

// IsTerminal returns true if the status is a terminal state
func (s SandboxStatus) IsTerminal() bool {
	return s == StatusStopped || s == StatusFailed || s == StatusExpired
}

// IsRunning returns true if the sandbox is currently running
func (s SandboxStatus) IsRunning() bool {
	return s == StatusRunning
}

// Sandbox represents an isolated sandbox environment
type Sandbox struct {
	ID          string                      `json:"id"`
	TemplateID  string                      `json:"template_id"`
	UserID      string                      `json:"user_id"`
	Status      SandboxStatus               `json:"status"`
	StatusMsg   string                      `json:"status_message,omitempty"`
	CreatedAt   time.Time                   `json:"created_at"`
	StartedAt   *time.Time                  `json:"started_at,omitempty"`
	ExpiresAt   time.Time                   `json:"expires_at"`
	ContainerID string                      `json:"container_id,omitempty"`
	Services    map[string]*ServiceInstance `json:"services,omitempty"`
	Endpoints   map[string]string           `json:"endpoints,omitempty"`
	Metadata    map[string]string           `json:"metadata,omitempty"`
}

// ServiceInstance represents a provisioned service for a sandbox
type ServiceInstance struct {
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	Status      string             `json:"status"`
	Credentials *ServiceCredentials `json:"credentials,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
}

// ServiceCredentials holds connection information for a service
type ServiceCredentials struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	Database  string `json:"database,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	URI       string `json:"uri,omitempty"`
}

// IsExpired checks if the sandbox has expired
func (s *Sandbox) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// Template represents a sandbox template configuration
type Template struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	BaseImage   string            `yaml:"base_image" json:"base_image"`
	Services    []string          `yaml:"services" json:"services"`
	Resources   Resources         `yaml:"resources" json:"resources"`
	Env         map[string]string `yaml:"env" json:"env"`
	TTL         time.Duration     `yaml:"ttl" json:"ttl"`
	Expose      []Port            `yaml:"expose" json:"expose"`
	Volumes     []Volume          `yaml:"volumes" json:"volumes"`
	Commands    Commands          `yaml:"commands" json:"commands"`
	Labels      map[string]string `yaml:"labels" json:"labels"`
}

// Resources defines resource limits for a sandbox
type Resources struct {
	CPULimit      string `yaml:"cpu_limit" json:"cpu_limit"`
	MemoryLimit   string `yaml:"memory_limit" json:"memory_limit"`
	CPURequest    string `yaml:"cpu_request" json:"cpu_request"`
	MemoryRequest string `yaml:"memory_request" json:"memory_request"`
	DiskLimit     string `yaml:"disk_limit" json:"disk_limit"`
}

// Port defines an exposed port configuration
type Port struct {
	Container   int    `yaml:"container" json:"container"`
	Protocol    string `yaml:"protocol" json:"protocol"`
	Name        string `yaml:"name" json:"name"`
	Public      bool   `yaml:"public" json:"public"`
	TraefikRule string `yaml:"traefik_rule" json:"traefik_rule,omitempty"`
}

// Volume defines a volume mount
type Volume struct {
	Name      string `yaml:"name" json:"name"`
	MountPath string `yaml:"mount_path" json:"mount_path"`
	ReadOnly  bool   `yaml:"read_only" json:"read_only"`
	Size      string `yaml:"size" json:"size"`
}

// Commands defines lifecycle commands
type Commands struct {
	Init     []string `yaml:"init" json:"init"`
	Start    []string `yaml:"start" json:"start"`
	Stop     []string `yaml:"stop" json:"stop"`
	Healthcheck string `yaml:"healthcheck" json:"healthcheck"`
}

// ListFilters defines filters for listing sandboxes
type ListFilters struct {
	UserID     string
	TemplateID string
	Status     SandboxStatus
	Limit      int
	Offset     int
}

// CreateRequest represents a request to create a sandbox
type CreateRequest struct {
	TemplateID string            `json:"template_id"`
	UserID     string            `json:"user_id"`
	TTL        *time.Duration    `json:"ttl,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ExtendRequest represents a request to extend sandbox TTL
type ExtendRequest struct {
	Duration time.Duration `json:"duration"`
}
