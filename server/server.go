package server

import (
	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/llm"
	"github.com/yourorg/codewalker/internal/session"
)

// Server implements the CodeWalkerServiceServer gRPC interface.
type Server struct {
	v1.UnimplementedCodeWalkerServiceServer

	store    *session.Store
	provider llm.Provider
	repoRoot string
}

// New creates a Server.
func New(store *session.Store, provider llm.Provider, repoRoot string) *Server {
	return &Server{
		store:    store,
		provider: provider,
		repoRoot: repoRoot,
	}
}
