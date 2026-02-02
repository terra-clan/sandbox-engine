package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/terra-clan/sandbox-engine/internal/models"
)

// RedisProvider implements Provider for Redis
type RedisProvider struct {
	BaseProvider
	client   *redis.Client
	host     string
	port     int
	password string
}

// NewRedisProvider creates a new Redis provider
func NewRedisProvider(address, password string) (*RedisProvider, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       0,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	// Parse address
	host := "localhost"
	port := 6379
	if strings.Contains(address, ":") {
		parts := strings.Split(address, ":")
		host = parts[0]
		fmt.Sscanf(parts[1], "%d", &port)
	}

	return &RedisProvider{
		BaseProvider: BaseProvider{serviceType: "redis"},
		client:       client,
		host:         host,
		port:         port,
		password:     password,
	}, nil
}

// Provision creates a key prefix for the sandbox
// Redis doesn't have true database isolation like PostgreSQL,
// so we use key prefixes for logical separation
func (p *RedisProvider) Provision(ctx context.Context, sandboxID, serviceName string) (*models.ServiceCredentials, error) {
	prefix := fmt.Sprintf("sandbox:%s:", strings.ReplaceAll(sandboxID, "-", "_"))

	slog.Info("provisioning redis namespace",
		"sandbox_id", sandboxID,
		"prefix", prefix,
	)

	// Store a marker key to track provisioned sandboxes
	markerKey := fmt.Sprintf("%s__provisioned__", prefix)
	if err := p.client.Set(ctx, markerKey, "1", 0).Err(); err != nil {
		return nil, fmt.Errorf("failed to provision redis namespace: %w", err)
	}

	// Build connection URI
	uri := fmt.Sprintf("redis://%s:%d", p.host, p.port)
	if p.password != "" {
		uri = fmt.Sprintf("redis://:%s@%s:%d", p.password, p.host, p.port)
	}

	return &models.ServiceCredentials{
		Host:     p.host,
		Port:     p.port,
		Password: p.password,
		Prefix:   prefix,
		URI:      uri,
	}, nil
}

// Deprovision removes all keys with the sandbox prefix
func (p *RedisProvider) Deprovision(ctx context.Context, sandboxID, serviceName string) error {
	prefix := fmt.Sprintf("sandbox:%s:", strings.ReplaceAll(sandboxID, "-", "_"))

	slog.Info("deprovisioning redis namespace",
		"sandbox_id", sandboxID,
		"prefix", prefix,
	)

	// Find all keys with this prefix
	pattern := fmt.Sprintf("%s*", prefix)
	var cursor uint64
	var keysDeleted int

	for {
		keys, nextCursor, err := p.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan keys: %w", err)
		}

		if len(keys) > 0 {
			if err := p.client.Del(ctx, keys...).Err(); err != nil {
				slog.Warn("failed to delete some keys", "error", err)
			}
			keysDeleted += len(keys)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	slog.Info("redis namespace deprovisioned",
		"sandbox_id", sandboxID,
		"keys_deleted", keysDeleted,
	)

	return nil
}

// HealthCheck verifies Redis connectivity
func (p *RedisProvider) HealthCheck(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (p *RedisProvider) Close() error {
	return p.client.Close()
}
