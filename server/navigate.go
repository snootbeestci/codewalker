package server

import (
	"strings"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/llm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Navigate implements CodeWalkerServiceServer.Navigate.
func (s *Server) Navigate(req *v1.NavigateRequest, stream v1.CodeWalkerService_NavigateServer) error {
	ctx := stream.Context()

	if req.SessionId == "" {
		return status.Error(codes.InvalidArgument, "session_id is required")
	}
	if req.TargetStepId == "" {
		return status.Error(codes.InvalidArgument, "target_step_id is required")
	}

	sess, err := s.store.Get(req.SessionId)
	if err != nil {
		return status.Errorf(codes.NotFound, "%v", err)
	}

	sess.Lock()
	step, navErr := sess.Walker.GoTo(req.TargetStepId)
	sess.Unlock()
	if navErr != nil {
		return status.Errorf(codes.InvalidArgument, "navigation error: %v", navErr)
	}

	// Build narration request.
	crumb := sess.Walker.Breadcrumb()
	crumbLabels := make([]string, 0, len(crumb))
	for _, id := range crumb {
		if s, ok := sess.Graph.Step(id); ok {
			crumbLabels = append(crumbLabels, s.Label)
		}
	}

	code := ""
	if step.Source != nil {
		code = step.Source.RawSource
	}
	if code == "" && len(sess.Source) > 0 && step.Source != nil {
		// Slice out of the original source.
		code = sliceSource(sess.Source, int(step.Source.StartLine), int(step.Source.EndLine))
	}

	tokens, err := s.provider.Narrate(ctx, llm.NarrateRequest{
		Code:      code,
		Language:  sess.Language,
		StepLabel: step.Label,
		StepKind:  step.Kind.String(),
		CallChain: crumbLabels,
		Level:     sess.EffLevel,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "narration error: %v", err)
	}

	// Stream tokens.
	for token := range tokens {
		select {
		case <-ctx.Done():
			return status.FromContextError(ctx.Err()).Err()
		default:
		}
		if err := stream.Send(&v1.NarrateEvent{
			Event: &v1.NarrateEvent_Token{Token: token},
		}); err != nil {
			return err
		}
	}

	// Send StepComplete.
	return stream.Send(&v1.NarrateEvent{
		Event: &v1.NarrateEvent_StepComplete{
			StepComplete: &v1.StepComplete{
				AvailableEdges: sess.Walker.NavigableEdges(),
				Breadcrumb:     crumbLabels,
				SessionSummary: sess.Summary(),
			},
		},
	})
}

// sliceSource extracts lines startLine..endLine (1-indexed, inclusive) from src.
func sliceSource(src []byte, startLine, endLine int) string {
	lines := strings.Split(string(src), "\n")
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	return strings.Join(lines[startLine-1:endLine], "\n")
}
