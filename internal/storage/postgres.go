package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terra-clan/sandbox-engine/internal/models"
)

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// PostgresConfig holds PostgreSQL connection configuration
type PostgresConfig struct {
	DSN          string
	MaxOpenConns int32
	MaxIdleConns int32
	MaxLifetime  time.Duration
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(ctx context.Context, cfg PostgresConfig) (*PostgresRepository, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	// Set pool configuration
	if cfg.MaxOpenConns > 0 {
		poolConfig.MaxConns = cfg.MaxOpenConns
	} else {
		poolConfig.MaxConns = 25 // default
	}

	if cfg.MaxIdleConns > 0 {
		poolConfig.MinConns = cfg.MaxIdleConns
	} else {
		poolConfig.MinConns = 5 // default
	}

	if cfg.MaxLifetime > 0 {
		poolConfig.MaxConnLifetime = cfg.MaxLifetime
	} else {
		poolConfig.MaxConnLifetime = 30 * time.Minute
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresRepository{pool: pool}, nil
}

// Ping checks database connectivity
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

// Close closes the database connection pool
func (r *PostgresRepository) Close() error {
	r.pool.Close()
	return nil
}

// CreateSandbox creates a new sandbox record
func (r *PostgresRepository) CreateSandbox(ctx context.Context, sb *models.Sandbox) error {
	metadataJSON, err := json.Marshal(sb.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	endpointsJSON, err := json.Marshal(sb.Endpoints)
	if err != nil {
		return fmt.Errorf("failed to marshal endpoints: %w", err)
	}

	query := `
		INSERT INTO sandboxes (id, template_id, user_id, status, status_message, container_id, created_at, started_at, expires_at, metadata, endpoints)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err = r.pool.Exec(ctx, query,
		sb.ID,
		sb.TemplateID,
		sb.UserID,
		string(sb.Status),
		nullString(sb.StatusMsg),
		nullString(sb.ContainerID),
		sb.CreatedAt,
		nullTime(sb.StartedAt),
		sb.ExpiresAt,
		metadataJSON,
		endpointsJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to create sandbox: %w", err)
	}

	return nil
}

// GetSandbox retrieves a sandbox by ID
func (r *PostgresRepository) GetSandbox(ctx context.Context, id string) (*models.Sandbox, error) {
	query := `
		SELECT id, template_id, user_id, status, status_message, container_id, created_at, started_at, expires_at, metadata, endpoints
		FROM sandboxes
		WHERE id = $1
	`

	var sb models.Sandbox
	var statusStr string
	var statusMsg, containerID sql.NullString
	var startedAt sql.NullTime
	var metadataJSON, endpointsJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&sb.ID,
		&sb.TemplateID,
		&sb.UserID,
		&statusStr,
		&statusMsg,
		&containerID,
		&sb.CreatedAt,
		&startedAt,
		&sb.ExpiresAt,
		&metadataJSON,
		&endpointsJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get sandbox: %w", err)
	}

	sb.Status = models.SandboxStatus(statusStr)
	sb.StatusMsg = statusMsg.String
	sb.ContainerID = containerID.String

	if startedAt.Valid {
		sb.StartedAt = &startedAt.Time
	}

	if err := json.Unmarshal(metadataJSON, &sb.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	if err := json.Unmarshal(endpointsJSON, &sb.Endpoints); err != nil {
		return nil, fmt.Errorf("failed to unmarshal endpoints: %w", err)
	}

	// Load services
	services, err := r.GetServices(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}

	sb.Services = make(map[string]*models.ServiceInstance)
	for _, svc := range services {
		sb.Services[svc.Name] = svc
	}

	return &sb, nil
}

// UpdateSandbox updates an existing sandbox
func (r *PostgresRepository) UpdateSandbox(ctx context.Context, sb *models.Sandbox) error {
	metadataJSON, err := json.Marshal(sb.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	endpointsJSON, err := json.Marshal(sb.Endpoints)
	if err != nil {
		return fmt.Errorf("failed to marshal endpoints: %w", err)
	}

	query := `
		UPDATE sandboxes
		SET status = $2, status_message = $3, container_id = $4, started_at = $5, expires_at = $6, metadata = $7, endpoints = $8
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		sb.ID,
		string(sb.Status),
		nullString(sb.StatusMsg),
		nullString(sb.ContainerID),
		nullTime(sb.StartedAt),
		sb.ExpiresAt,
		metadataJSON,
		endpointsJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to update sandbox: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("sandbox not found: %s", sb.ID)
	}

	return nil
}

// DeleteSandbox deletes a sandbox by ID
func (r *PostgresRepository) DeleteSandbox(ctx context.Context, id string) error {
	query := `DELETE FROM sandboxes WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete sandbox: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("sandbox not found: %s", id)
	}

	return nil
}

// ListSandboxes returns sandboxes matching filters
func (r *PostgresRepository) ListSandboxes(ctx context.Context, filters models.ListFilters) ([]*models.Sandbox, error) {
	query := `
		SELECT id, template_id, user_id, status, status_message, container_id, created_at, started_at, expires_at, metadata, endpoints
		FROM sandboxes
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argNum := 1

	if filters.UserID != "" {
		query += fmt.Sprintf(" AND user_id = $%d", argNum)
		args = append(args, filters.UserID)
		argNum++
	}

	if filters.TemplateID != "" {
		query += fmt.Sprintf(" AND template_id = $%d", argNum)
		args = append(args, filters.TemplateID)
		argNum++
	}

	if filters.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, string(filters.Status))
		argNum++
	}

	query += " ORDER BY created_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filters.Limit)
		argNum++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filters.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sandboxes: %w", err)
	}
	defer rows.Close()

	var sandboxes []*models.Sandbox

	for rows.Next() {
		var sb models.Sandbox
		var statusStr string
		var statusMsg, containerID sql.NullString
		var startedAt sql.NullTime
		var metadataJSON, endpointsJSON []byte

		err := rows.Scan(
			&sb.ID,
			&sb.TemplateID,
			&sb.UserID,
			&statusStr,
			&statusMsg,
			&containerID,
			&sb.CreatedAt,
			&startedAt,
			&sb.ExpiresAt,
			&metadataJSON,
			&endpointsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sandbox: %w", err)
		}

		sb.Status = models.SandboxStatus(statusStr)
		sb.StatusMsg = statusMsg.String
		sb.ContainerID = containerID.String

		if startedAt.Valid {
			sb.StartedAt = &startedAt.Time
		}

		if err := json.Unmarshal(metadataJSON, &sb.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		if err := json.Unmarshal(endpointsJSON, &sb.Endpoints); err != nil {
			return nil, fmt.Errorf("failed to unmarshal endpoints: %w", err)
		}

		// Load services for this sandbox
		services, err := r.GetServices(ctx, sb.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get services for sandbox %s: %w", sb.ID, err)
		}

		sb.Services = make(map[string]*models.ServiceInstance)
		for _, svc := range services {
			sb.Services[svc.Name] = svc
		}

		sandboxes = append(sandboxes, &sb)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sandboxes: %w", err)
	}

	return sandboxes, nil
}

// GetExpiredSandboxes returns all non-terminal sandboxes that have expired
func (r *PostgresRepository) GetExpiredSandboxes(ctx context.Context) ([]*models.Sandbox, error) {
	query := `
		SELECT id, template_id, user_id, status, status_message, container_id, created_at, started_at, expires_at, metadata, endpoints
		FROM sandboxes
		WHERE status NOT IN ('stopped', 'failed', 'expired')
		  AND expires_at < NOW()
		ORDER BY expires_at ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired sandboxes: %w", err)
	}
	defer rows.Close()

	var sandboxes []*models.Sandbox

	for rows.Next() {
		var sb models.Sandbox
		var statusStr string
		var statusMsg, containerID sql.NullString
		var startedAt sql.NullTime
		var metadataJSON, endpointsJSON []byte

		err := rows.Scan(
			&sb.ID,
			&sb.TemplateID,
			&sb.UserID,
			&statusStr,
			&statusMsg,
			&containerID,
			&sb.CreatedAt,
			&startedAt,
			&sb.ExpiresAt,
			&metadataJSON,
			&endpointsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sandbox: %w", err)
		}

		sb.Status = models.SandboxStatus(statusStr)
		sb.StatusMsg = statusMsg.String
		sb.ContainerID = containerID.String

		if startedAt.Valid {
			sb.StartedAt = &startedAt.Time
		}

		if err := json.Unmarshal(metadataJSON, &sb.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		if err := json.Unmarshal(endpointsJSON, &sb.Endpoints); err != nil {
			return nil, fmt.Errorf("failed to unmarshal endpoints: %w", err)
		}

		// Load services
		services, err := r.GetServices(ctx, sb.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get services for sandbox %s: %w", sb.ID, err)
		}

		sb.Services = make(map[string]*models.ServiceInstance)
		for _, svc := range services {
			sb.Services[svc.Name] = svc
		}

		sandboxes = append(sandboxes, &sb)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating expired sandboxes: %w", err)
	}

	return sandboxes, nil
}

// CreateService creates a new service instance for a sandbox
func (r *PostgresRepository) CreateService(ctx context.Context, sandboxID string, svc *models.ServiceInstance) error {
	credentialsJSON, err := json.Marshal(svc.Credentials)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	query := `
		INSERT INTO sandbox_services (sandbox_id, service_name, service_type, status, credentials, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (sandbox_id, service_name) DO UPDATE
		SET status = EXCLUDED.status, credentials = EXCLUDED.credentials
	`

	_, err = r.pool.Exec(ctx, query,
		sandboxID,
		svc.Name,
		svc.Type,
		svc.Status,
		credentialsJSON,
		svc.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	return nil
}

// GetServices retrieves all services for a sandbox
func (r *PostgresRepository) GetServices(ctx context.Context, sandboxID string) ([]*models.ServiceInstance, error) {
	query := `
		SELECT service_name, service_type, status, credentials, created_at
		FROM sandbox_services
		WHERE sandbox_id = $1
	`

	rows, err := r.pool.Query(ctx, query, sandboxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}
	defer rows.Close()

	var services []*models.ServiceInstance

	for rows.Next() {
		var svc models.ServiceInstance
		var credentialsJSON []byte

		err := rows.Scan(
			&svc.Name,
			&svc.Type,
			&svc.Status,
			&credentialsJSON,
			&svc.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan service: %w", err)
		}

		if credentialsJSON != nil {
			if err := json.Unmarshal(credentialsJSON, &svc.Credentials); err != nil {
				return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
			}
		}

		services = append(services, &svc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating services: %w", err)
	}

	return services, nil
}

// UpdateService updates a service instance
func (r *PostgresRepository) UpdateService(ctx context.Context, sandboxID string, svc *models.ServiceInstance) error {
	credentialsJSON, err := json.Marshal(svc.Credentials)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	query := `
		UPDATE sandbox_services
		SET status = $3, credentials = $4
		WHERE sandbox_id = $1 AND service_name = $2
	`

	result, err := r.pool.Exec(ctx, query, sandboxID, svc.Name, svc.Status, credentialsJSON)
	if err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("service not found: %s/%s", sandboxID, svc.Name)
	}

	return nil
}

// DeleteServices deletes all services for a sandbox
func (r *PostgresRepository) DeleteServices(ctx context.Context, sandboxID string) error {
	query := `DELETE FROM sandbox_services WHERE sandbox_id = $1`

	_, err := r.pool.Exec(ctx, query, sandboxID)
	if err != nil {
		return fmt.Errorf("failed to delete services: %w", err)
	}

	return nil
}

// GetClientByApiKey retrieves an API client by its key
func (r *PostgresRepository) GetClientByApiKey(ctx context.Context, apiKey string) (*models.ApiClient, error) {
	query := `
		SELECT id, name, api_key, is_active, created_at, last_used_at, permissions, metadata
		FROM api_clients
		WHERE api_key = $1
	`

	var client models.ApiClient
	var lastUsedAt sql.NullTime
	var permissionsJSON, metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, apiKey).Scan(
		&client.ID,
		&client.Name,
		&client.ApiKey,
		&client.IsActive,
		&client.CreatedAt,
		&lastUsedAt,
		&permissionsJSON,
		&metadataJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get api client: %w", err)
	}

	if lastUsedAt.Valid {
		client.LastUsedAt = &lastUsedAt.Time
	}

	// Parse permissions JSON array
	if permissionsJSON != nil {
		if err := json.Unmarshal(permissionsJSON, &client.Permissions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
		}
	}

	// Parse metadata JSON object
	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &client.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &client, nil
}

// UpdateClientLastUsed updates the last_used_at timestamp for a client
func (r *PostgresRepository) UpdateClientLastUsed(ctx context.Context, apiKey string) error {
	query := `UPDATE api_clients SET last_used_at = NOW() WHERE api_key = $1`

	_, err := r.pool.Exec(ctx, query, apiKey)
	if err != nil {
		return fmt.Errorf("failed to update client last_used_at: %w", err)
	}

	return nil
}

// --- Sessions ---

// CreateSession creates a new session record
func (r *PostgresRepository) CreateSession(ctx context.Context, s *models.Session) error {
	envJSON, err := json.Marshal(s.Env)
	if err != nil {
		return fmt.Errorf("failed to marshal env: %w", err)
	}

	metadataJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO sessions (id, token, template_id, status, status_message, env, metadata, ttl_seconds, sandbox_id, created_at, activated_at, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err = r.pool.Exec(ctx, query,
		s.ID,
		s.Token,
		s.TemplateID,
		string(s.Status),
		nullString(s.StatusMessage),
		envJSON,
		metadataJSON,
		s.TTLSeconds,
		nullString(s.SandboxID),
		s.CreatedAt,
		nullTime(s.ActivatedAt),
		nullTime(s.ExpiresAt),
		nullString(s.CreatedBy),
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetSessionByToken retrieves a session by its join token
func (r *PostgresRepository) GetSessionByToken(ctx context.Context, token string) (*models.Session, error) {
	return r.getSession(ctx, "token", token)
}

// GetSessionByID retrieves a session by its ID
func (r *PostgresRepository) GetSessionByID(ctx context.Context, id string) (*models.Session, error) {
	return r.getSession(ctx, "id", id)
}

func (r *PostgresRepository) getSession(ctx context.Context, field, value string) (*models.Session, error) {
	query := fmt.Sprintf(`
		SELECT id, token, template_id, status, status_message, env, metadata, ttl_seconds, sandbox_id, created_at, activated_at, expires_at, created_by
		FROM sessions
		WHERE %s = $1
	`, field)

	var s models.Session
	var statusStr string
	var statusMsg, sandboxID, createdBy sql.NullString
	var activatedAt, expiresAt sql.NullTime
	var envJSON, metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, value).Scan(
		&s.ID,
		&s.Token,
		&s.TemplateID,
		&statusStr,
		&statusMsg,
		&envJSON,
		&metadataJSON,
		&s.TTLSeconds,
		&sandboxID,
		&s.CreatedAt,
		&activatedAt,
		&expiresAt,
		&createdBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	s.Status = models.SessionStatus(statusStr)
	s.StatusMessage = statusMsg.String
	s.SandboxID = sandboxID.String
	s.CreatedBy = createdBy.String

	if activatedAt.Valid {
		s.ActivatedAt = &activatedAt.Time
	}
	if expiresAt.Valid {
		s.ExpiresAt = &expiresAt.Time
	}

	if envJSON != nil {
		if err := json.Unmarshal(envJSON, &s.Env); err != nil {
			return nil, fmt.Errorf("failed to unmarshal env: %w", err)
		}
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &s.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &s, nil
}

// UpdateSession updates an existing session
func (r *PostgresRepository) UpdateSession(ctx context.Context, s *models.Session) error {
	envJSON, err := json.Marshal(s.Env)
	if err != nil {
		return fmt.Errorf("failed to marshal env: %w", err)
	}

	metadataJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE sessions
		SET status = $2, status_message = $3, sandbox_id = $4, activated_at = $5, expires_at = $6, env = $7, metadata = $8
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		s.ID,
		string(s.Status),
		nullString(s.StatusMessage),
		nullString(s.SandboxID),
		nullTime(s.ActivatedAt),
		nullTime(s.ExpiresAt),
		envJSON,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("session not found: %s", s.ID)
	}

	return nil
}

// DeleteSession deletes a session by ID
func (r *PostgresRepository) DeleteSession(ctx context.Context, id string) error {
	query := `DELETE FROM sessions WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("session not found: %s", id)
	}

	return nil
}

// ListSessions returns sessions with optional status filter
func (r *PostgresRepository) ListSessions(ctx context.Context, status string, limit, offset int) ([]*models.Session, error) {
	query := `
		SELECT id, token, template_id, status, status_message, env, metadata, ttl_seconds, sandbox_id, created_at, activated_at, expires_at, created_by
		FROM sessions
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argNum := 1

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, status)
		argNum++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, limit)
		argNum++
	}

	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.Session

	for rows.Next() {
		var s models.Session
		var statusStr string
		var statusMsg, sandboxID, createdBy sql.NullString
		var activatedAt, expiresAt sql.NullTime
		var envJSON, metadataJSON []byte

		err := rows.Scan(
			&s.ID,
			&s.Token,
			&s.TemplateID,
			&statusStr,
			&statusMsg,
			&envJSON,
			&metadataJSON,
			&s.TTLSeconds,
			&sandboxID,
			&s.CreatedAt,
			&activatedAt,
			&expiresAt,
			&createdBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		s.Status = models.SessionStatus(statusStr)
		s.StatusMessage = statusMsg.String
		s.SandboxID = sandboxID.String
		s.CreatedBy = createdBy.String

		if activatedAt.Valid {
			s.ActivatedAt = &activatedAt.Time
		}
		if expiresAt.Valid {
			s.ExpiresAt = &expiresAt.Time
		}

		if envJSON != nil {
			json.Unmarshal(envJSON, &s.Env)
		}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &s.Metadata)
		}

		sessions = append(sessions, &s)
	}

	return sessions, rows.Err()
}

// GetExpiredSessions returns active sessions that have expired
func (r *PostgresRepository) GetExpiredSessions(ctx context.Context) ([]*models.Session, error) {
	query := `
		SELECT id, token, template_id, status, status_message, env, metadata, ttl_seconds, sandbox_id, created_at, activated_at, expires_at, created_by
		FROM sessions
		WHERE status = 'active'
		  AND expires_at < NOW()
		ORDER BY expires_at ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.Session

	for rows.Next() {
		var s models.Session
		var statusStr string
		var statusMsg, sandboxID, createdBy sql.NullString
		var activatedAt, expiresAt sql.NullTime
		var envJSON, metadataJSON []byte

		err := rows.Scan(
			&s.ID,
			&s.Token,
			&s.TemplateID,
			&statusStr,
			&statusMsg,
			&envJSON,
			&metadataJSON,
			&s.TTLSeconds,
			&sandboxID,
			&s.CreatedAt,
			&activatedAt,
			&expiresAt,
			&createdBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		s.Status = models.SessionStatus(statusStr)
		s.StatusMessage = statusMsg.String
		s.SandboxID = sandboxID.String
		s.CreatedBy = createdBy.String

		if activatedAt.Valid {
			s.ActivatedAt = &activatedAt.Time
		}
		if expiresAt.Valid {
			s.ExpiresAt = &expiresAt.Time
		}

		if envJSON != nil {
			json.Unmarshal(envJSON, &s.Env)
		}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &s.Metadata)
		}

		sessions = append(sessions, &s)
	}

	return sessions, rows.Err()
}

// Helper functions for nullable values

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
