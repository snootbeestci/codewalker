package server

import (
	"log/slog"
	"strings"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/graph"
	"github.com/yourorg/codewalker/internal/llm"
	"github.com/yourorg/codewalker/internal/session"
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

	if cached, ok := sess.CachedNarration(step.ID); ok {
		return replayCached(stream, step, cached, availableEdges, crumbLabels)
	}

	// Review steps get a structured summary in parallel with narration.
	// Walkthrough steps leave summary unset.
	var summaryCh chan *llm.StepSummary
	if step.HunkSpan != nil {
		summaryCh = make(chan *llm.StepSummary, 1)
		go func() {
			summary, err := s.provider.GenerateStepSummary(ctx, llm.SummaryRequest{
				Language:      sess.Language,
				HunkDiff:      step.HunkSpan.RawDiff,
				ContextBefore: step.HunkSpan.ContextBefore,
				ContextAfter:  step.HunkSpan.ContextAfter,
			})
			if err != nil {
				slog.Debug("summary generation failed", "step_id", step.ID, "error", err)
				summaryCh <- nil
				return
			}
			summaryCh <- summary
		}()
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
	slog.Debug("narration started", "step_id", step.ID)

	tokenCount := 0
	var summaryProto *v1.StepSummary
	var summaryResult *llm.StepSummary
	var collectedTokens []string

	// Drain tokens and the summary channel concurrently so the structured
	// summary can be sent the moment it is ready, without waiting for
	// narration to finish. Setting tokens or summaryCh to nil after either
	// is exhausted removes that case from the select (a nil channel blocks
	// forever), letting the loop exit when both are done.
	tokenCh := tokens
	for tokenCh != nil || summaryCh != nil {
		select {
		case <-ctx.Done():
			return status.FromContextError(ctx.Err()).Err()
		case token, ok := <-tokenCh:
			if !ok {
				tokenCh = nil
				continue
			}
			tokenCount++
			collectedTokens = append(collectedTokens, token)
			if err := stream.Send(&v1.NarrateEvent{
				Event: &v1.NarrateEvent_Token{Token: &v1.NarrateToken{Text: token}},
			}); err != nil {
				return err
			}
		case summary := <-summaryCh:
			summaryCh = nil
			if summary == nil {
				continue
			}
			summaryResult = summary
			summaryProto = stepSummaryToProto(summary)
			if err := stream.Send(&v1.NarrateEvent{
				Event: &v1.NarrateEvent_SummaryReady{
					SummaryReady: &v1.SummaryReady{Summary: summaryProto},
				},
			}); err != nil {
				return err
			}
		}
	}
	slog.Debug("narration complete", "step_id", step.ID, "token_count", tokenCount)

	sess.CacheNarration(step.ID, &session.CachedStep{
		NarrationTokens: collectedTokens,
		Summary:         summaryResult,
	})

	return stream.Send(&v1.NarrateEvent{
		Event: &v1.NarrateEvent_Complete{
			Complete: &v1.StepComplete{
				StepId:         step.ID,
				AvailableEdges: availableEdges,
				Breadcrumb:     crumbLabels,
				Summary:        summaryProto,
			},
		},
	})
}

// stepSummaryToProto converts an llm.StepSummary to its proto form. Used by
// both the live narration path and the cached replay path.
func stepSummaryToProto(s *llm.StepSummary) *v1.StepSummary {
	if s == nil {
		return nil
	}
	return &v1.StepSummary{
		Breaking:      s.Breaking,
		Risk:          s.Risk,
		WhatChanged:   s.WhatChanged,
		SideEffects:   s.SideEffects,
		Tests:         s.Tests,
		ReviewerFocus: s.ReviewerFocus,
		Suggestion:    s.Suggestion,
		Confidence:    s.Confidence,
	}
}

// replayCached emits the same NarrateToken / SummaryReady / Complete sequence
// as the live path, but sourced from the per-session cache. Tokens are sent
// as fast as the gRPC channel will accept them.
func replayCached(
	stream v1.CodeWalker_NavigateServer,
	step *graph.Step,
	cached *session.CachedStep,
	availableEdges []*v1.StepEdge,
	breadcrumb []string,
) error {
	slog.Debug("narration cache hit", "step_id", step.ID, "token_count", len(cached.NarrationTokens))

	for _, tok := range cached.NarrationTokens {
		if err := stream.Context().Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		if err := stream.Send(&v1.NarrateEvent{
			Event: &v1.NarrateEvent_Token{Token: &v1.NarrateToken{Text: tok}},
		}); err != nil {
			return err
		}
	}

	summaryProto := stepSummaryToProto(cached.Summary)
	if summaryProto != nil {
		if err := stream.Send(&v1.NarrateEvent{
			Event: &v1.NarrateEvent_SummaryReady{
				SummaryReady: &v1.SummaryReady{Summary: summaryProto},
			},
		}); err != nil {
			return err
		}
	}

	return stream.Send(&v1.NarrateEvent{
		Event: &v1.NarrateEvent_Complete{
			Complete: &v1.StepComplete{
				StepId:         step.ID,
				AvailableEdges: availableEdges,
				Breadcrumb:     breadcrumb,
				Summary:        summaryProto,
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
