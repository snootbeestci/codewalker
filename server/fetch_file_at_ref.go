package server

import (
	"context"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/forge"
	_ "github.com/yourorg/codewalker/internal/forge/forges"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FetchFileAtRef implements CodeWalkerServer.FetchFileAtRef.
func (s *Server) FetchFileAtRef(ctx context.Context, req *v1.FetchFileAtRefRequest) (*v1.FetchFileAtRefResponse, error) {
	host := forge.NormalizeHost(req.Host)
	if host == "" || req.Owner == "" || req.Repo == "" || req.Path == "" || req.Ref == "" {
		return nil, status.Error(codes.InvalidArgument, "host, owner, repo, path, and ref are required")
	}

	handler, err := forge.Resolve(host)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported forge host %q: %v", host, err)
	}

	fc := &forge.ForgeContext{
		Forge: "github",
		Owner: req.Owner,
		Repo:  req.Repo,
	}

	content, err := handler.FetchFile(ctx, fc, req.Path, req.Ref, req.ForgeToken)
	if err != nil {
		return nil, forgeErrToStatus(err, "fetch file")
	}

	return &v1.FetchFileAtRefResponse{Content: content}, nil
}
