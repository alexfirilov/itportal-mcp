package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration sourced from environment variables.
type Config struct {
	ITPortalBaseURL         string
	ITPortalAPIKey          string
	MCPAPIKey               string
	ListenAddr              string
	SnapshotRefreshInterval time.Duration
	SnapshotLimitPerEntity  int
}

// Load reads and validates configuration from environment variables.
// Call after loading a .env file if desired.
func Load() (*Config, error) {
	baseURL := os.Getenv("ITPORTAL_BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("ITPORTAL_BASE_URL is required")
	}

	apiKey := os.Getenv("ITPORTAL_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ITPORTAL_API_KEY is required")
	}

	mcpKey := os.Getenv("MCP_API_KEY")
	if mcpKey == "" {
		return nil, fmt.Errorf("MCP_API_KEY is required")
	}

	listenAddr := os.Getenv("MCP_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	refreshInterval := 30 * time.Minute
	if v := os.Getenv("SNAPSHOT_REFRESH_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SNAPSHOT_REFRESH_INTERVAL %q: %w", v, err)
		}
		refreshInterval = d
	}

	limitPerEntity := 1000
	if v := os.Getenv("SNAPSHOT_LIMIT_PER_ENTITY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SNAPSHOT_LIMIT_PER_ENTITY %q: %w", v, err)
		}
		limitPerEntity = n
	}

	return &Config{
		ITPortalBaseURL:         baseURL,
		ITPortalAPIKey:          apiKey,
		MCPAPIKey:               mcpKey,
		ListenAddr:              listenAddr,
		SnapshotRefreshInterval: refreshInterval,
		SnapshotLimitPerEntity:  limitPerEntity,
	}, nil
}
