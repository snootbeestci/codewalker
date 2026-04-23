package server

import (
	"strings"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/llm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Rephrase implements CodeWalkerServiceServer.Rephrase.
func (s *Server) Rephrase(req *v1.RephraseRequest, stream v1.CodeWalkerService_RephraseServer) error {
	ctx := stream.Context()

	if req.SessionId == "" {
		return status.Error(codes.InvalidArgument, "session_id is required")
	}

	sess, err := s.store.Get(req.SessionId)
	if err != nil {
		return status.Errorf(codes.NotFound, "%v", err)
	}

	sess.Lock()
	currentStep, ok := sess.Walker.Current()
	crumb := sess.Walker.Breadcrumb()
	sess.Unlock()

	if !ok {
		return status.Error(codes.FailedPrecondition, "no current step; navigate to a step first")
	}

	crumbLabels := make([]string, 0, len(crumb))
	for _, id := range crumb {
		if step, ok := sess.Graph.Step(id); ok {
			crumbLabels = append(crumbLabels, step.Label)
		}
	}

	code := ""
	if currentStep.Source != nil {
		code = currentStep.Source.RawSource
	}
	if code == "" && len(sess.Source) > 0 && currentStep.Source != nil {
		code = sliceSource(sess.Source, int(currentStep.Source.StartLine), int(currentStep.Source.EndLine))
	}

	modeStr := modeString(req.Mode)

	tokens, err := s.provider.Rephrase(ctx, llm.RephraseRequest{
		NarrateRequest: llm.NarrateRequest{
			Code:      code,
			Language:  sess.Language,
			StepLabel: currentStep.Label,
			StepKind:  currentStep.Kind.String(),
			CallChain: crumbLabels,
			Level:     sess.EffLevel,
		},
		Mode: modeStr,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "rephrase error: %v", err)
	}

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

func modeString(m v1.RephraseMode) string {
	return strings.TrimPrefix(m.String(), "REPHRASE_MODE_")
}
