package server

import (
	"log/slog"
	"strings"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/graph"
	"github.com/yourorg/codewalker/internal/llm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Navigate implements CodeWalkerServer.Navigate.
func (s *Server) Navigate(req *v1.NavigateRequest, stream v1.CodeWalker_NavigateServer) error {
	ctx := stream.Context()

	if req.SessionId == "" {
		return status.Error(codes.InvalidArgument, "session_id is required")
	}

	sess, err := s.store.Get(req.SessionId)
	if err != nil {
		return status.Errorf(codes.NotFound, "%v", err)
	}

	sess.Lock()
	var step *graph.Step
	var navErr error
	switch d := req.Destination.(type) {
	case *v1.NavigateRequest_Direction:
		switch d.Direction {
		case v1.SimpleDirection_SIMPLE_DIRECTION_FORWARD:
			step, navErr = sess.Walker.Forward()
		case v1.SimpleDirection_SIMPLE_DIRECTION_BACK:
			step, navErr = sess.Walker.Back()
		default:
			sess.Unlock()
			return status.Error(codes.InvalidArgument, "unknown direction")
		}
	case *v1.NavigateRequest_StepId:
		step, navErr = sess.Walker.GoTo(d.StepId)
	case *v1.NavigateRequest_FollowEdge:
		step, navErr = sess.Walker.FollowEdgeLabel(d.FollowEdge)
	default:
		sess.Unlock()
		return status.Error(codes.InvalidArgument, "destination is required")
	}
	crumb := sess.Walker.Breadcrumb()
	availableEdges := sess.Walker.NavigableEdges()
	sess.Unlock()

	if navErr != nil {
		return status.Errorf(codes.InvalidArgument, "navigation error: %v", navErr)
	}

	slog.Debug("navigated", "step_id", step.ID, "step_kind", step.Kind.String(), "step_label", step.Label)

	crumbLabels := make([]string, 0, len(crumb))
	for _, id := range crumb {
		if st, ok := sess.Graph.Step(id); ok {
			crumbLabels = append(crumbLabels, st.Label)
		}
	}

	code := ""
	if step.HunkSpan != nil {
		code = step.HunkSpan.RawDiff
	} else if step.Source != nil {
		code = step.Source.RawSource
		if code == "" && len(sess.Source) > 0 {
			code = sliceSource(sess.Source, int(step.Source.StartLine), int(step.Source.EndLine))
		}
	}
	slog.Debug("source resolved", "step_id", step.ID, "code_len", len(code))

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
	slog.Debug("narration started", "step_id", step.ID)

	tokenCount := 0
	for token := range tokens {
		select {
		case <-ctx.Done():
			return status.FromContextError(ctx.Err()).Err()
		default:
		}
		tokenCount++
		if err := stream.Send(&v1.NarrateEvent{
			Event: &v1.NarrateEvent_Token{Token: &v1.NarrateToken{Text: token}},
		}); err != nil {
			return err
		}
	}
	slog.Debug("narration complete", "step_id", step.ID, "token_count", tokenCount)

	return stream.Send(&v1.NarrateEvent{
		Event: &v1.NarrateEvent_Complete{
			Complete: &v1.StepComplete{
				StepId:         step.ID,
				AvailableEdges: availableEdges,
				Breadcrumb:     crumbLabels,
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
