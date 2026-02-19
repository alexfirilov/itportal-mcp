package main

import (
	"context"
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
	itportalClient := itportal.NewClient(cfg.ITPortalBaseURL, cfg.ITPortalAPIKey)

	// Build documentation cache (blocks until initial snapshot succeeds).
	logger.Info("building initial documentation snapshot — this may take a moment…")
	docCache, err := cache.New(ctx, itportalClient, cfg.SnapshotLimitPerEntity, cfg.SnapshotRefreshInterval, logger)
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

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: authHandler,
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
	)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP server error", "error", err)
		os.Exit(1)
	}

	<-shutdownDone
	logger.Info("server stopped")
}

// apiKeyMiddleware enforces Bearer token authentication on all requests.
func apiKeyMiddleware(expectedKey string, next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			logger.Warn("missing or malformed Authorization header", "remote", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != expectedKey {
			logger.Warn("invalid API key", "remote", r.RemoteAddr)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
