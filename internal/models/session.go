package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// SessionStatus represents the current state of a session
type SessionStatus string

const (
	SessionReady        SessionStatus = "ready"        // Created, waiting for candidate
	SessionProvisioning SessionStatus = "provisioning"  // Candidate joined, sandbox starting
	SessionActive       SessionStatus = "active"        // Sandbox running, timer ticking
	SessionExpired      SessionStatus = "expired"       // TTL elapsed
	SessionFailed       SessionStatus = "failed"        // Error during provisioning
)

// Session represents a deferred sandbox session.
// Created by an admin/orchestrator, activated when the candidate opens the join link.
type Session struct {
	ID            string            `json:"id"`
	Token         string            `json:"token"`
	TemplateID    string            `json:"template_id"`
	Status        SessionStatus     `json:"status"`
	StatusMessage string            `json:"status_message,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	TTLSeconds    int               `json:"ttl_seconds"`
	SandboxID     string            `json:"sandbox_id,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	ActivatedAt   *time.Time        `json:"activated_at,omitempty"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
	CreatedBy     string            `json:"created_by,omitempty"`
}

// IsTerminal returns true if the session is in a final state
func (s *Session) IsTerminal() bool {
	return s.Status == SessionExpired || s.Status == SessionFailed
}

// IsActivatable returns true if the session can be activated
func (s *Session) IsActivatable() bool {
	return s.Status == SessionReady
}

// IsExpired checks if the session TTL has elapsed
func (s *Session) IsExpired() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*s.ExpiresAt)
}

// TimeRemaining returns the duration until expiry (0 if expired or not activated)
func (s *Session) TimeRemaining() time.Duration {
	if s.ExpiresAt == nil {
		return 0
	}
	remaining := time.Until(*s.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GenerateSessionToken creates a cryptographically random 48-char hex token
func GenerateSessionToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateSessionRequest represents a request to create a session
type CreateSessionRequest struct {
	TemplateID string            `json:"template_id"`
	TTL        int               `json:"ttl"`                 // seconds
	Env        map[string]string `json:"env,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// CreateSessionResponse is returned after creating a session
type CreateSessionResponse struct {
	ID         string        `json:"id"`
	Token      string        `json:"token"`
	TemplateID string        `json:"template_id"`
	Status     SessionStatus `json:"status"`
	JoinURL    string        `json:"join_url"`
	CreatedAt  time.Time     `json:"created_at"`
}

// JoinSessionResponse is returned for public join endpoint
type JoinSessionResponse struct {
	Status   SessionStatus     `json:"status"`
	Template *TemplateInfo     `json:"template,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Sandbox  *SandboxInfo      `json:"sandbox,omitempty"`
}

// TemplateInfo is a subset of template data for the join response
type TemplateInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language,omitempty"`
}

// SandboxInfo holds sandbox details returned after activation
type SandboxInfo struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Endpoints map[string]string `json:"endpoints,omitempty"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
}

// ActivateSessionResponse is returned when activating a session
type ActivateSessionResponse struct {
	Status    SessionStatus `json:"status"`
	SandboxID string        `json:"sandbox_id,omitempty"`
}
