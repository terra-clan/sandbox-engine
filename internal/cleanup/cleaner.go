package cleanup

import (
	"context"
	"log/slog"
	"time"

	"github.com/terra-clan/sandbox-engine/internal/sandbox"
)

// Cleaner handles periodic cleanup of expired sandboxes
type Cleaner struct {
	manager  sandbox.Manager
	interval time.Duration
}

// NewCleaner creates a new cleanup worker
func NewCleaner(manager sandbox.Manager, interval time.Duration) *Cleaner {
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	return &Cleaner{
		manager:  manager,
		interval: interval,
	}
}

// Start begins the cleanup worker in a goroutine
func (c *Cleaner) Start(ctx context.Context) {
	go c.run(ctx)
}

// run is the main loop for the cleanup worker
func (c *Cleaner) run(ctx context.Context) {
	slog.Info("cleanup worker started", "interval", c.interval)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Run immediately on start
	c.cleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("cleanup worker stopped")
			return
		case <-ticker.C:
			c.cleanup(ctx)
		}
	}
}

// cleanup finds and removes expired sandboxes and sessions
func (c *Cleaner) cleanup(ctx context.Context) {
	slog.Debug("running cleanup cycle")

	c.cleanupSandboxes(ctx)
	c.cleanupSessions(ctx)
}

// cleanupSandboxes finds and removes expired sandboxes
func (c *Cleaner) cleanupSandboxes(ctx context.Context) {
	expired, err := c.manager.GetExpired(ctx)
	if err != nil {
		slog.Error("failed to get expired sandboxes", "error", err)
		return
	}

	if len(expired) == 0 {
		slog.Debug("no expired sandboxes found")
		return
	}

	slog.Info("found expired sandboxes", "count", len(expired))

	for _, sb := range expired {
		slog.Info("deleting expired sandbox",
			"id", sb.ID,
			"user", sb.UserID,
			"template", sb.TemplateID,
			"expired_at", sb.ExpiresAt,
		)

		if err := c.manager.Delete(ctx, sb.ID); err != nil {
			slog.Error("failed to delete expired sandbox",
				"error", err,
				"id", sb.ID,
			)
			continue
		}

		slog.Info("expired sandbox deleted", "id", sb.ID)
	}
}

// cleanupSessions finds and expires active sessions past their TTL
func (c *Cleaner) cleanupSessions(ctx context.Context) {
	expiredSessions, err := c.manager.GetExpiredSessions(ctx)
	if err != nil {
		slog.Error("failed to get expired sessions", "error", err)
		return
	}

	for _, session := range expiredSessions {
		slog.Info("expiring session", "session_id", session.ID)

		// Delete session (which also cleans up its sandbox)
		if err := c.manager.DeleteSession(ctx, session.ID); err != nil {
			slog.Error("failed to expire session", "session_id", session.ID, "error", err)
		}
	}

	if len(expiredSessions) > 0 {
		slog.Info("expired sessions cleaned up", "count", len(expiredSessions))
	}
}
