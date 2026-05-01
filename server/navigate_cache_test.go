package server_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/graph"
	"github.com/yourorg/codewalker/internal/llm"
	"github.com/yourorg/codewalker/internal/session"
	"github.com/yourorg/codewalker/server"
	"google.golang.org/grpc/metadata"
)

// countingProvider records how often Narrate and GenerateStepSummary are
// invoked. Each Narrate call returns a fresh channel pre-filled with the same
// tokens; each summary call returns the same StepSummary. narrateErr forces
// Narrate to fail when set.
type countingProvider struct {
	mu sync.Mutex

	narrateCalls atomic.Int32
	summaryCalls atomic.Int32

	tokens     []string
	summary    *llm.StepSummary
	narrateErr error
}

func (p *countingProvider) Narrate(_ context.Context, _ llm.NarrateRequest) (<-chan string, error) {
	p.narrateCalls.Add(1)
	if p.narrateErr != nil {
		return nil, p.narrateErr
	}
	ch := make(chan string, len(p.tokens))
	for _, t := range p.tokens {
		ch <- t
	}
	close(ch)
	return ch, nil
}

func (p *countingProvider) Rephrase(_ context.Context, _ llm.RephraseRequest) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}

func (p *countingProvider) SummarizeExternalCall(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}

func (p *countingProvider) ExtractGlossaryTerms(_ context.Context, _ llm.GlossaryRequest) ([]llm.GlossaryCandidate, error) {
	return nil, nil
}

func (p *countingProvider) ExpandTerm(_ context.Context, _ llm.ExpandTermRequest) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}

func (p *countingProvider) GenerateStepSummary(_ context.Context, _ llm.SummaryRequest) (*llm.StepSummary, error) {
	p.summaryCalls.Add(1)
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.summary, nil
}

func reviewSessionWithTwoHunkSteps(t *testing.T, store *session.Store, id string) {
	t.Helper()
	g := graph.NewGraph()
	stepA := &graph.Step{
		ID:    "hunk:a.go:1",
		Label: "a.go:1",
		Kind:  v1.StepKind_STEP_KIND_HUNK,
		HunkSpan: &v1.HunkSpan{
			FilePath: "a.go",
			NewStart: 1, NewLines: 1,
			RawDiff: "@@ -1 +1 @@\n-a-old\n+a-new\n",
		},
		Edges: []*v1.StepEdge{{
			TargetStepId: "hunk:b.go:1",
			Label:        v1.EdgeLabel_EDGE_LABEL_NEXT,
			Navigable:    true,
		}},
	}
	stepB := &graph.Step{
		ID:    "hunk:b.go:1",
		Label: "b.go:1",
		Kind:  v1.StepKind_STEP_KIND_HUNK,
		HunkSpan: &v1.HunkSpan{
			FilePath: "b.go",
			NewStart: 1, NewLines: 1,
			RawDiff: "@@ -1 +1 @@\n-b-old\n+b-new\n",
		},
	}
	g.Add(stepA)
	g.Add(stepB)
	g.EntryID = stepA.ID

	sess := session.New(id, g, 5, "diff", false, nil, "", "", "")
	sess.Kind = v1.SessionKind_SESSION_KIND_REVIEW
	store.Set(sess)
}

// blockingStream provides a stream wrapper for cache-test use; like
// recordingNarrateStream but no time dependency.
type cacheRecordingStream struct {
	ctx    context.Context
	mu     sync.Mutex
	events []*v1.NarrateEvent
}

func (s *cacheRecordingStream) Send(e *v1.NarrateEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}
func (s *cacheRecordingStream) Context() context.Context     { return s.ctx }
func (s *cacheRecordingStream) SetHeader(metadata.MD) error  { return nil }
func (s *cacheRecordingStream) SendHeader(metadata.MD) error { return nil }
func (s *cacheRecordingStream) SetTrailer(metadata.MD)       {}
func (s *cacheRecordingStream) SendMsg(any) error            { return nil }
func (s *cacheRecordingStream) RecvMsg(any) error            { return nil }

func (s *cacheRecordingStream) snapshot() []*v1.NarrateEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*v1.NarrateEvent, len(s.events))
	copy(out, s.events)
	return out
}

func tokenTexts(events []*v1.NarrateEvent) []string {
	var out []string
	for _, e := range events {
		if tok := e.GetToken(); tok != nil {
			out = append(out, tok.Text)
		}
	}
	return out
}

func eventKinds(events []*v1.NarrateEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		switch {
		case e.GetToken() != nil:
			out = append(out, "token")
		case e.GetSummaryReady() != nil:
			out = append(out, "summary_ready")
		case e.GetComplete() != nil:
			out = append(out, "complete")
		default:
			out = append(out, "unknown")
		}
	}
	return out
}

func TestNavigate_SecondNavigateToSameStep_UsesCache(t *testing.T) {
	store := session.NewStore()
	const sessID = "cache-test-1"
	reviewSessionWithHunkStep(t, store, sessID)

	provider := &countingProvider{
		tokens:  []string{"alpha", "beta", "gamma"},
		summary: &llm.StepSummary{Breaking: "No", WhatChanged: "test"},
	}
	srv := server.New(store, provider, "")

	stream1 := &cacheRecordingStream{ctx: context.Background()}
	if err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "hunk:foo.go:1"},
	}, stream1); err != nil {
		t.Fatalf("first Navigate: %v", err)
	}
	if got := provider.narrateCalls.Load(); got != 1 {
		t.Fatalf("first Navigate: Narrate calls = %d, want 1", got)
	}
	if got := provider.summaryCalls.Load(); got != 1 {
		t.Fatalf("first Navigate: GenerateStepSummary calls = %d, want 1", got)
	}

	stream2 := &cacheRecordingStream{ctx: context.Background()}
	if err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "hunk:foo.go:1"},
	}, stream2); err != nil {
		t.Fatalf("second Navigate: %v", err)
	}
	if got := provider.narrateCalls.Load(); got != 1 {
		t.Errorf("second Navigate: Narrate calls = %d, want still 1 (cache hit)", got)
	}
	if got := provider.summaryCalls.Load(); got != 1 {
		t.Errorf("second Navigate: GenerateStepSummary calls = %d, want still 1 (cache hit)", got)
	}

	tokens1 := tokenTexts(stream1.snapshot())
	tokens2 := tokenTexts(stream2.snapshot())
	if len(tokens1) != len(tokens2) {
		t.Fatalf("token counts differ: live=%d cached=%d", len(tokens1), len(tokens2))
	}
	for i := range tokens1 {
		if tokens1[i] != tokens2[i] {
			t.Errorf("token %d: live=%q cached=%q", i, tokens1[i], tokens2[i])
		}
	}

	events2 := stream2.snapshot()
	if last := events2[len(events2)-1]; last.GetComplete() == nil {
		t.Fatalf("second Navigate: last event must be Complete, got %T", last.Event)
	}
	c := events2[len(events2)-1].GetComplete()
	if c.Summary == nil || c.Summary.WhatChanged != "test" {
		t.Errorf("cached Complete.summary unexpected: %+v", c.Summary)
	}
}

func TestNavigate_DifferentStepsDoNotCollide(t *testing.T) {
	store := session.NewStore()
	const sessID = "cache-test-2"
	reviewSessionWithTwoHunkSteps(t, store, sessID)

	provider := &countingProvider{
		tokens:  []string{"x", "y"},
		summary: &llm.StepSummary{WhatChanged: "shared"},
	}
	srv := server.New(store, provider, "")

	// Navigate to step A (entry)
	streamA1 := &cacheRecordingStream{ctx: context.Background()}
	if err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "hunk:a.go:1"},
	}, streamA1); err != nil {
		t.Fatalf("Navigate A first: %v", err)
	}
	if got := provider.narrateCalls.Load(); got != 1 {
		t.Fatalf("after A first: Narrate=%d, want 1", got)
	}

	// Navigate to step B
	streamB := &cacheRecordingStream{ctx: context.Background()}
	if err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "hunk:b.go:1"},
	}, streamB); err != nil {
		t.Fatalf("Navigate B: %v", err)
	}
	if got := provider.narrateCalls.Load(); got != 2 {
		t.Fatalf("after B: Narrate=%d, want 2", got)
	}

	// Navigate back to step A — should be cache hit, no new LLM call.
	streamA2 := &cacheRecordingStream{ctx: context.Background()}
	if err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "hunk:a.go:1"},
	}, streamA2); err != nil {
		t.Fatalf("Navigate A second: %v", err)
	}
	if got := provider.narrateCalls.Load(); got != 2 {
		t.Errorf("after A second: Narrate=%d, want still 2 (cache hit)", got)
	}
	if got := provider.summaryCalls.Load(); got != 2 {
		t.Errorf("after A second: GenerateStepSummary=%d, want still 2 (cache hit)", got)
	}
}

func TestNavigate_LlmFailureNotCached(t *testing.T) {
	store := session.NewStore()
	const sessID = "cache-test-3"
	reviewSessionWithHunkStep(t, store, sessID)

	failing := &countingProvider{narrateErr: errors.New("boom")}
	srv := server.New(store, failing, "")

	stream1 := &cacheRecordingStream{ctx: context.Background()}
	err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "hunk:foo.go:1"},
	}, stream1)
	if err == nil {
		t.Fatal("first Navigate: expected error from failing Narrate, got nil")
	}
	if got := failing.narrateCalls.Load(); got != 1 {
		t.Fatalf("first Navigate: Narrate calls = %d, want 1", got)
	}

	// Swap to a working provider on the same session and navigate again.
	// If the cache had been (incorrectly) populated, this Navigate would not
	// hit Narrate at all. We assert the counter on the *new* provider.
	working := &countingProvider{
		tokens:  []string{"recovered"},
		summary: &llm.StepSummary{WhatChanged: "ok"},
	}
	srv2 := server.New(store, working, "")

	stream2 := &cacheRecordingStream{ctx: context.Background()}
	if err := srv2.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "hunk:foo.go:1"},
	}, stream2); err != nil {
		t.Fatalf("second Navigate: %v", err)
	}
	if got := working.narrateCalls.Load(); got != 1 {
		t.Errorf("second Navigate: Narrate calls = %d, want 1 (cache must not have been populated by failed run)", got)
	}
}

func TestNavigate_CachedReplayEmitsSummaryReady(t *testing.T) {
	store := session.NewStore()
	const sessID = "cache-test-4"
	reviewSessionWithHunkStep(t, store, sessID)

	provider := &countingProvider{
		tokens:  []string{"one", "two"},
		summary: &llm.StepSummary{WhatChanged: "structured", Breaking: "No"},
	}
	srv := server.New(store, provider, "")

	// Prime the cache.
	stream1 := &cacheRecordingStream{ctx: context.Background()}
	if err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "hunk:foo.go:1"},
	}, stream1); err != nil {
		t.Fatalf("first Navigate: %v", err)
	}

	// Cached replay.
	stream2 := &cacheRecordingStream{ctx: context.Background()}
	if err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "hunk:foo.go:1"},
	}, stream2); err != nil {
		t.Fatalf("second Navigate: %v", err)
	}

	events := stream2.snapshot()
	summaryIdx, completeIdx := -1, -1
	for i, e := range events {
		if e.GetSummaryReady() != nil {
			summaryIdx = i
		}
		if e.GetComplete() != nil {
			completeIdx = i
		}
	}
	if summaryIdx == -1 {
		t.Fatalf("cached replay did not emit SummaryReady; kinds=%v", eventKinds(events))
	}
	if completeIdx == -1 {
		t.Fatal("cached replay did not emit Complete")
	}
	if summaryIdx >= completeIdx {
		t.Errorf("SummaryReady (idx %d) must precede Complete (idx %d)", summaryIdx, completeIdx)
	}
	sr := events[summaryIdx].GetSummaryReady()
	if sr.Summary == nil || sr.Summary.WhatChanged != "structured" {
		t.Errorf("cached SummaryReady payload unexpected: %+v", sr.Summary)
	}
}

func TestNavigate_CachedReplayWalkthroughHasNoSummaryReady(t *testing.T) {
	store := session.NewStore()
	const sessID = "cache-test-5"
	walkthroughSessionWithSourceStep(t, store, sessID)

	provider := &countingProvider{
		tokens: []string{"narration"},
		// summary nil — never queried for walkthrough steps anyway.
	}
	srv := server.New(store, provider, "")

	// Prime the cache.
	stream1 := &cacheRecordingStream{ctx: context.Background()}
	if err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "step:1"},
	}, stream1); err != nil {
		t.Fatalf("first Navigate: %v", err)
	}
	if got := provider.summaryCalls.Load(); got != 0 {
		t.Fatalf("walkthrough should not call GenerateStepSummary; got %d", got)
	}

	// Cached replay.
	stream2 := &cacheRecordingStream{ctx: context.Background()}
	if err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "step:1"},
	}, stream2); err != nil {
		t.Fatalf("second Navigate: %v", err)
	}
	if got := provider.narrateCalls.Load(); got != 1 {
		t.Errorf("Narrate calls = %d, want 1 (cache hit)", got)
	}

	events := stream2.snapshot()
	for _, e := range events {
		if e.GetSummaryReady() != nil {
			t.Fatal("cached walkthrough replay emitted SummaryReady; expected none")
		}
	}
	if last := events[len(events)-1]; last.GetComplete() == nil {
		t.Fatalf("last event should be Complete; got %T", last.Event)
	}
	if c := events[len(events)-1].GetComplete(); c.Summary != nil {
		t.Errorf("walkthrough cached Complete.summary should be nil; got %+v", c.Summary)
	}
}
