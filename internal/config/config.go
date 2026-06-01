package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/alexfirilov/itportal-mcp/internal/itportal"
)

// Config holds all runtime configuration sourced from environment variables.
type Config struct {
	ITPortalBaseURL         string
	ITPortalAPIKey          string
	ITPortalAPIVersion      string
	ITPortalEncryptionKey   string
	MCPAPIKey               string
	ListenAddr              string
	SnapshotRefreshInterval time.Duration
	SnapshotLimitPerEntity  int
	SnapshotDeviceLimit     int
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

	apiVersion := os.Getenv("ITPORTAL_API_VERSION")
	if apiVersion == "" {
		apiVersion = itportal.DefaultAPIVersion
	}

	encryptionKey := os.Getenv("ITPORTAL_ENCRYPTION_KEY")

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

	// Devices are typically the largest entity set, so they get their own cap.
	// Defaults to limitPerEntity when unset.
	deviceLimit := limitPerEntity
	if v := os.Getenv("SNAPSHOT_DEVICE_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SNAPSHOT_DEVICE_LIMIT %q: %w", v, err)
		}
		deviceLimit = n
	}

	return &Config{
		ITPortalBaseURL:         baseURL,
		ITPortalAPIKey:          apiKey,
		ITPortalAPIVersion:      apiVersion,
		ITPortalEncryptionKey:   encryptionKey,
		MCPAPIKey:               mcpKey,
		ListenAddr:              listenAddr,
		SnapshotRefreshInterval: refreshInterval,
		SnapshotLimitPerEntity:  limitPerEntity,
		SnapshotDeviceLimit:     deviceLimit,
	}, nil
}
