package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for sandbox-engine
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Docker    DockerConfig
	Traefik   TraefikConfig
	Templates TemplatesConfig
	Cleanup   CleanupConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host string
	Port int
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	DSN string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

// DockerConfig holds Docker configuration
type DockerConfig struct {
	Host       string
	Network    string
	Registry   string
	PullPolicy string
}

// TraefikConfig holds Traefik configuration
type TraefikConfig struct {
	Enabled   bool
	Network   string
	Domain    string
	EntryPort int
}

// TemplatesConfig holds templates configuration
type TemplatesConfig struct {
	Dir string
}

// CleanupConfig holds cleanup worker configuration
type CleanupConfig struct {
	Interval time.Duration
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvAsInt("SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			DSN: getEnv("DATABASE_DSN", "postgres://sandbox:sandbox@localhost:5432/sandbox_engine?sslmode=disable"),
		},
		Redis: RedisConfig{
			Address:  getEnv("REDIS_ADDRESS", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		Docker: DockerConfig{
			Host:       getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
			Network:    getEnv("DOCKER_NETWORK", "sandbox-network"),
			Registry:   getEnv("DOCKER_REGISTRY", ""),
			PullPolicy: getEnv("DOCKER_PULL_POLICY", "if-not-present"),
		},
		Traefik: TraefikConfig{
			Enabled:   getEnvAsBool("TRAEFIK_ENABLED", true),
			Network:   getEnv("TRAEFIK_NETWORK", "traefik"),
			Domain:    getEnv("TRAEFIK_DOMAIN", "sandbox.local"),
			EntryPort: getEnvAsInt("TRAEFIK_ENTRY_PORT", 80),
		},
		Templates: TemplatesConfig{
			Dir: getEnv("TEMPLATES_DIR", "./templates"),
		},
		Cleanup: CleanupConfig{
			Interval: getEnvAsDuration("CLEANUP_INTERVAL", 5*time.Minute),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Database.DSN == "" {
		return fmt.Errorf("database DSN is required")
	}

	return nil
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
