package server

import (
	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/llm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExpandTerm implements CodeWalkerServer.ExpandTerm.
func (s *Server) ExpandTerm(req *v1.ExpandTermRequest, stream v1.CodeWalker_ExpandTermServer) error {
	ctx := stream.Context()

	if req.SessionId == "" {
		return status.Error(codes.InvalidArgument, "session_id is required")
	}
	if req.Term == "" {
		return status.Error(codes.InvalidArgument, "term is required")
	}

	sess, err := s.store.Get(req.SessionId)
	if err != nil {
		return status.Errorf(codes.NotFound, "%v", err)
	}

	sess.Lock()
	currentStep, _ := sess.Walker.Current()
	currentStepID := sess.Walker.CurrentID()
	sess.Unlock()

	contextCode := ""
	if currentStep != nil && currentStep.Source != nil {
		contextCode = currentStep.Source.RawSource
		if contextCode == "" && len(sess.Source) > 0 {
			contextCode = sliceSource(sess.Source, int(currentStep.Source.StartLine), int(currentStep.Source.EndLine))
		}
	}

	tokens, err := s.provider.ExpandTerm(ctx, llm.ExpandTermRequest{
		Term:     req.Term,
		Context:  contextCode,
		Language: sess.Language,
		Level:    sess.EffLevel,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "expand term error: %v", err)
	}

	for token := range tokens {
		select {
		case <-ctx.Done():
			return status.FromContextError(ctx.Err()).Err()
		default:
		}
		if err := stream.Send(&v1.NarrateEvent{
			Event: &v1.NarrateEvent_Token{Token: &v1.NarrateToken{Text: token}},
		}); err != nil {
			return err
		}
	}

	expanded := &v1.GlossaryTerm{
		Term:   req.Term,
		StepId: currentStepID,
		Kind:   v1.TermKind_TERM_KIND_UNSPECIFIED,
	}
	if t, ok := sess.GetGlossaryTerm(req.Term); ok {
		expanded.Kind = t.Kind
	}

	return stream.Send(&v1.NarrateEvent{
		Event: &v1.NarrateEvent_Complete{
			Complete: &v1.StepComplete{
				StepId:   currentStepID,
				NewTerms: []*v1.GlossaryTerm{expanded},
			},
		},
	})
}
