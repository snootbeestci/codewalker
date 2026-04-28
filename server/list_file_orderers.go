package server

import (
	"context"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/forge"
	_ "github.com/yourorg/codewalker/internal/forge/orderers"
)

// ListFileOrderers implements CodeWalkerServer.ListFileOrderers.
func (s *Server) ListFileOrderers(_ context.Context, _ *v1.ListFileOrderersRequest) (*v1.ListFileOrderersResponse, error) {
	orderers := forge.ListOrderers()
	out := make([]*v1.FileOrderer, 0, len(orderers))
	for _, o := range orderers {
		out = append(out, &v1.FileOrderer{
			Name:        o.Name(),
			Description: o.Description(),
		})
	}
	return &v1.ListFileOrderersResponse{Orderers: out}, nil
}
