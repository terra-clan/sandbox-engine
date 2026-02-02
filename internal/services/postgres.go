package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/lib/pq"

	"github.com/terra-clan/sandbox-engine/internal/models"
)

// PostgresProvider implements Provider for PostgreSQL
type PostgresProvider struct {
	BaseProvider
	db       *sql.DB
	host     string
	port     int
	adminDSN string
}

// NewPostgresProvider creates a new PostgreSQL provider
func NewPostgresProvider(dsn string) (*PostgresProvider, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	// Parse DSN to extract host/port
	host := "localhost"
	port := 5432

	// Simple DSN parsing (in production use proper URL parsing)
	if strings.Contains(dsn, "@") {
		parts := strings.Split(dsn, "@")
		if len(parts) > 1 {
			hostPart := strings.Split(parts[1], "/")[0]
			if strings.Contains(hostPart, ":") {
				hostParts := strings.Split(hostPart, ":")
				host = hostParts[0]
				fmt.Sscanf(hostParts[1], "%d", &port)
			} else {
				host = hostPart
			}
		}
	}

	return &PostgresProvider{
		BaseProvider: BaseProvider{serviceType: "postgres"},
		db:           db,
		host:         host,
		port:         port,
		adminDSN:     dsn,
	}, nil
}

// Provision creates a database and user for the sandbox
func (p *PostgresProvider) Provision(ctx context.Context, sandboxID, serviceName string) (*models.ServiceCredentials, error) {
	// Generate unique database and user names
	dbName := fmt.Sprintf("sandbox_%s", strings.ReplaceAll(sandboxID, "-", "_"))
	userName := fmt.Sprintf("sandbox_user_%s", strings.ReplaceAll(sandboxID, "-", "_"))
	password := generatePassword(16)

	slog.Info("provisioning postgres database",
		"sandbox_id", sandboxID,
		"database", dbName,
		"user", userName,
	)

	// Create user
	createUserSQL := fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", userName, password)
	if _, err := p.db.ExecContext(ctx, createUserSQL); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create database
	createDBSQL := fmt.Sprintf("CREATE DATABASE %s OWNER %s", dbName, userName)
	if _, err := p.db.ExecContext(ctx, createDBSQL); err != nil {
		// Cleanup user on failure
		_, _ = p.db.ExecContext(ctx, fmt.Sprintf("DROP USER IF EXISTS %s", userName))
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Grant privileges
	grantSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", dbName, userName)
	if _, err := p.db.ExecContext(ctx, grantSQL); err != nil {
		slog.Warn("failed to grant privileges", "error", err)
	}

	// Build connection URI
	uri := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		userName, password, p.host, p.port, dbName)

	return &models.ServiceCredentials{
		Host:     p.host,
		Port:     p.port,
		Username: userName,
		Password: password,
		Database: dbName,
		URI:      uri,
	}, nil
}

// Deprovision removes the database and user
func (p *PostgresProvider) Deprovision(ctx context.Context, sandboxID, serviceName string) error {
	dbName := fmt.Sprintf("sandbox_%s", strings.ReplaceAll(sandboxID, "-", "_"))
	userName := fmt.Sprintf("sandbox_user_%s", strings.ReplaceAll(sandboxID, "-", "_"))

	slog.Info("deprovisioning postgres database",
		"sandbox_id", sandboxID,
		"database", dbName,
		"user", userName,
	)

	// Terminate existing connections
	terminateSQL := fmt.Sprintf(`
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = '%s' AND pid <> pg_backend_pid()
	`, dbName)
	_, _ = p.db.ExecContext(ctx, terminateSQL)

	// Drop database
	dropDBSQL := fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)
	if _, err := p.db.ExecContext(ctx, dropDBSQL); err != nil {
		slog.Warn("failed to drop database", "error", err, "database", dbName)
	}

	// Drop user
	dropUserSQL := fmt.Sprintf("DROP USER IF EXISTS %s", userName)
	if _, err := p.db.ExecContext(ctx, dropUserSQL); err != nil {
		slog.Warn("failed to drop user", "error", err, "user", userName)
	}

	return nil
}

// HealthCheck verifies PostgreSQL connectivity
func (p *PostgresProvider) HealthCheck(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// generatePassword creates a random password
func generatePassword(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to less secure but working password
		return "sandbox_default_pass_" + fmt.Sprintf("%d", length)
	}
	return hex.EncodeToString(bytes)[:length]
}
