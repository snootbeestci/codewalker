package config

import (
	"os"
	"time"
)

// Config holds all server configuration read from environment variables.
type Config struct {
	// AnthropicAPIKey is required when LLMProvider = "anthropic".
	AnthropicAPIKey string
	// Port is the gRPC listen port.
	Port string
	// RepoRoot is the mount point for the repository inside the container.
	RepoRoot string
	// LogLevel controls log verbosity: debug | info | warn | error.
	LogLevel string
	// LLMProvider selects the LLM backend.  "anthropic" in v1, "ollama" in v2.
	LLMProvider string
	// SessionTTL is how long a session may be idle before eviction.
	SessionTTL time.Duration
	// EvictionInterval is how often the eviction loop runs.
	EvictionInterval time.Duration
}

// Load reads configuration from environment variables, applying defaults
// where appropriate.
func Load() *Config {
	return &Config{
		AnthropicAPIKey:  os.Getenv("ANTHROPIC_API_KEY"),
		Port:             envOr("CODEWALKER_PORT", "50051"),
		RepoRoot:         envOr("CODEWALKER_REPO_ROOT", "/repos/target"),
		LogLevel:         envOr("CODEWALKER_LOG_LEVEL", "info"),
		LLMProvider:      envOr("CODEWALKER_LLM_PROVIDER", "anthropic"),
		SessionTTL:       parseDuration("CODEWALKER_SESSION_TTL", 2*time.Hour),
		EvictionInterval: parseDuration("CODEWALKER_EVICTION_INTERVAL", 5*time.Minute),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}
