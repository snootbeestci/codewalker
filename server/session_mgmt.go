package server

import (
	"context"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CloseSession implements CodeWalkerServer.CloseSession.
func (s *Server) CloseSession(_ context.Context, req *v1.CloseSessionRequest) (*v1.CloseSessionResponse, error) {
	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}
	if _, err := s.store.Get(req.SessionId); err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	s.store.Delete(req.SessionId)
	return &v1.CloseSessionResponse{Ok: true}, nil
}

// ListSessions implements CodeWalkerServer.ListSessions.
func (s *Server) ListSessions(_ context.Context, _ *v1.ListSessionsRequest) (*v1.ListSessionsResponse, error) {
	sessions := s.store.All()
	summaries := make([]*v1.SessionSummary, 0, len(sessions))
	for _, sess := range sessions {
		summaries = append(summaries, sess.Summary())
	}
	return &v1.ListSessionsResponse{Sessions: summaries}, nil
}
