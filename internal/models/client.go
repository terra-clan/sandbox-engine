package models

import (
	"strings"
	"time"
)

// ApiClient represents an authenticated API client
type ApiClient struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	ApiKey      string            `json:"-"` // Never serialize
	IsActive    bool              `json:"is_active"`
	CreatedAt   time.Time         `json:"created_at"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
	Permissions []string          `json:"permissions"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// HasPermission checks if client has specific permission
// Supports wildcard permissions like "sandboxes:*"
func (c *ApiClient) HasPermission(required string) bool {
	if c == nil || !c.IsActive {
		return false
	}

	for _, perm := range c.Permissions {
		// Exact match
		if perm == required {
			return true
		}

		// Wildcard match (e.g., "sandboxes:*" matches "sandboxes:read")
		if strings.HasSuffix(perm, ":*") {
			prefix := strings.TrimSuffix(perm, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}

		// Global wildcard
		if perm == "*" {
			return true
		}
	}

	return false
}

// MaskedApiKey returns first 8 characters of API key for logging
func (c *ApiClient) MaskedApiKey() string {
	if len(c.ApiKey) < 8 {
		return "***"
	}
	return c.ApiKey[:8] + "..."
}
