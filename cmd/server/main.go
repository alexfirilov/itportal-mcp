package main

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alexfirilov/itportal-mcp/internal/cache"
	"github.com/alexfirilov/itportal-mcp/internal/config"
	"github.com/alexfirilov/itportal-mcp/internal/itportal"
	mcpserver "github.com/alexfirilov/itportal-mcp/internal/mcp"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Load .env if present; ignore error when file doesn't exist.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		logger.Warn("could not load .env file", "error", err)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Build ITPortal API client.
	itportalClient := itportal.NewClient(cfg.ITPortalBaseURL, cfg.ITPortalAPIKey,
		itportal.WithAPIVersion(cfg.ITPortalAPIVersion),
		itportal.WithEncryptionKey(cfg.ITPortalEncryptionKey),
	)

	// Build documentation cache (blocks until initial snapshot succeeds).
	logger.Info("building initial documentation snapshot — this may take a moment…")
	docCache, err := cache.New(ctx, itportalClient, cfg.SnapshotLimitPerEntity, cfg.SnapshotDeviceLimit, cfg.SnapshotRefreshInterval, logger)
	if err != nil {
		logger.Error("failed to build initial documentation snapshot", "error", err)
		os.Exit(1)
	}
	docCache.StartBackgroundRefresh(ctx)

	// Build MCP server.
	server := mcpserver.NewServer(itportalClient, docCache)

	// Wrap the streamable-HTTP handler with API key authentication.
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(func(_ *http.Request) *sdkmcp.Server {
		return server
	}, nil)

	authHandler := apiKeyMiddleware(cfg.MCPAPIKey, mcpHandler, logger)

	// Unauthenticated readiness probe (for container healthchecks / mcpo gating).
	// Reachable only once the initial snapshot is built and the server is listening.
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", authHandler)

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	// Graceful shutdown.
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		logger.Info("shutdown signal received")
		if err := httpServer.Shutdown(context.Background()); err != nil {
			logger.Error("HTTP server shutdown error", "error", err)
		}
	}()

	logger.Info("ITPortal MCP server starting",
		"addr", cfg.ListenAddr,
		"snapshot_refresh_interval", cfg.SnapshotRefreshInterval.String(),
		"snapshot_limit_per_entity", cfg.SnapshotLimitPerEntity,
		"snapshot_device_limit", cfg.SnapshotDeviceLimit,
	)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP server error", "error", err)
		os.Exit(1)
	}

	<-shutdownDone
	logger.Info("server stopped")
}

// apiKeyMiddleware enforces shared-secret authentication on all requests. The
// secret may be presented as "Authorization: Bearer <key>", a raw "Authorization:
// <key>", or "X-API-Key: <key>" — gateways (LiteLLM, etc.) forward credentials in
// different shapes, so all common forms are accepted.
func apiKeyMiddleware(expectedKey string, next http.Handler, logger *slog.Logger) http.Handler {
	expected := []byte(expectedKey)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractAPIToken(r)
		if token == "" {
			logger.Warn("missing API credential", "remote", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if subtle.ConstantTimeCompare([]byte(token), expected) != 1 {
			logger.Warn("invalid API key", "remote", r.RemoteAddr)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractAPIToken pulls the shared secret from the Authorization or X-API-Key header.
func extractAPIToken(r *http.Request) string {
	if h := strings.TrimSpace(r.Header.Get("Authorization")); h != "" {
		if len(h) >= 7 && strings.EqualFold(h[:7], "Bearer ") {
			return strings.TrimSpace(h[7:])
		}
		return h // raw token without a scheme
	}
	if h := strings.TrimSpace(r.Header.Get("X-API-Key")); h != "" {
		return h
	}
	return ""
}
