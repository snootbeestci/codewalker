package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/config"
	"github.com/yourorg/codewalker/internal/llm"
	"github.com/yourorg/codewalker/internal/session"
	"github.com/yourorg/codewalker/server"
	"github.com/yourorg/codewalker/server/middleware"
)

func main() {
	cfg := config.Load()

	setupLogging(cfg.LogLevel)

	if err := run(cfg); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	// Build LLM provider.
	var provider llm.Provider
	switch cfg.LLMProvider {
	case "anthropic":
		if cfg.AnthropicAPIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY is required when CODEWALKER_LLM_PROVIDER=anthropic")
		}
		provider = llm.NewAnthropicProvider(cfg.AnthropicAPIKey)
	default:
		return fmt.Errorf("unknown LLM provider %q", cfg.LLMProvider)
	}

	// Root context: cancelling it stops background goroutines (eviction, etc.).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := session.NewStore()
	store.StartEviction(ctx, cfg.SessionTTL, cfg.EvictionInterval)

	srv := server.New(store, provider, cfg.RepoRoot)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.UnaryRecovery,
			middleware.UnaryLogger,
		),
		grpc.ChainStreamInterceptor(
			middleware.StreamRecovery,
			middleware.StreamLogger,
		),
	)

	v1.RegisterCodeWalkerServer(grpcServer, srv)
	reflection.Register(grpcServer)

	addr := ":" + cfg.Port
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	slog.Info("codewalker listening",
		"addr", addr,
		"llm_provider", cfg.LLMProvider,
		"session_ttl", cfg.SessionTTL,
		"eviction_interval", cfg.EvictionInterval,
	)
	return grpcServer.Serve(lis)
}

func setupLogging(level string) {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: l})))
}
