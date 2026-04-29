package server

import (
	"context"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/forge"
	_ "github.com/yourorg/codewalker/internal/forge/forges"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListPullRequests implements CodeWalkerServer.ListPullRequests.
func (s *Server) ListPullRequests(ctx context.Context, req *v1.ListPullRequestsRequest) (*v1.ListPullRequestsResponse, error) {
	host := forge.NormalizeHost(req.Host)
	if host == "" || req.Owner == "" || req.Repo == "" {
		return nil, status.Error(codes.InvalidArgument, "host, owner, and repo are required")
	}

	handler, err := forge.Resolve(host)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported forge host %q: %v", host, err)
	}

	prs, err := handler.ListPullRequests(ctx, req.Owner, req.Repo, req.ForgeToken)
	if err != nil {
		return nil, forgeErrToStatus(err, "list pull requests")
	}

	out := make([]*v1.PullRequestSummary, 0, len(prs))
	for _, p := range prs {
		out = append(out, &v1.PullRequestSummary{
			Number: int32(p.Number),
			Title:  p.Title,
			Author: p.Author,
			Url:    p.URL,
		})
	}
	return &v1.ListPullRequestsResponse{PullRequests: out}, nil
}
