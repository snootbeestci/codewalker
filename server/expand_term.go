package server

import (
	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/llm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExpandTerm implements CodeWalkerServiceServer.ExpandTerm.
func (s *Server) ExpandTerm(req *v1.ExpandTermRequest, stream v1.CodeWalkerService_ExpandTermServer) error {
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

	// Gather context from the current step's source for the LLM.
	sess.Lock()
	currentStep, _ := sess.Walker.Current()
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
			Event: &v1.NarrateEvent_Token{Token: token},
		}); err != nil {
			return err
		}
	}

	// Final StepComplete with the expanded definition added to new_glossary_terms.
	expanded := &v1.GlossaryTerm{
		Term:   req.Term,
		StepId: sess.Walker.CurrentID(),
		Kind:   v1.TermKind_TERM_KIND_UNSPECIFIED,
	}
	if t, ok := sess.GetGlossaryTerm(req.Term); ok {
		expanded.Kind = t.Kind
	}

	return stream.Send(&v1.NarrateEvent{
		Event: &v1.NarrateEvent_StepComplete{
			StepComplete: &v1.StepComplete{
				NewGlossaryTerms: []*v1.GlossaryTerm{expanded},
				SessionSummary:   sess.Summary(),
			},
		},
	})
}
