package server_test

import (
	"context"
	"sync"
	"testing"
	"time"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/graph"
	"github.com/yourorg/codewalker/internal/llm"
	"github.com/yourorg/codewalker/internal/session"
	"github.com/yourorg/codewalker/server"
	"google.golang.org/grpc/metadata"
)

// summaryProvider is a controllable provider used to drive ordering tests for
// the Navigate stream. Tokens are fed via the tokens channel (the test pushes
// and closes it). GenerateStepSummary blocks until summaryReady is closed.
type summaryProvider struct {
	tokens       chan string
	summary      *llm.StepSummary
	summaryReady chan struct{}
}

func (p *summaryProvider) Narrate(_ context.Context, _ llm.NarrateRequest) (<-chan string, error) {
	return p.tokens, nil
}

func (p *summaryProvider) Rephrase(_ context.Context, _ llm.RephraseRequest) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}

func (p *summaryProvider) SummarizeExternalCall(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}

func (p *summaryProvider) ExtractGlossaryTerms(_ context.Context, _ llm.GlossaryRequest) ([]llm.GlossaryCandidate, error) {
	return nil, nil
}

func (p *summaryProvider) ExpandTerm(_ context.Context, _ llm.ExpandTermRequest) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}

func (p *summaryProvider) GenerateStepSummary(ctx context.Context, _ llm.SummaryRequest) (*llm.StepSummary, error) {
	select {
	case <-p.summaryReady:
		return p.summary, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// recordingNarrateStream is a thread-safe stream so the test goroutine can
// observe events while Navigate is still streaming on another.
type recordingNarrateStream struct {
	ctx    context.Context
	mu     sync.Mutex
	events []*v1.NarrateEvent
}

func (s *recordingNarrateStream) Send(e *v1.NarrateEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}
func (s *recordingNarrateStream) Context() context.Context     { return s.ctx }
func (s *recordingNarrateStream) SetHeader(metadata.MD) error  { return nil }
func (s *recordingNarrateStream) SendHeader(metadata.MD) error { return nil }
func (s *recordingNarrateStream) SetTrailer(metadata.MD)       {}
func (s *recordingNarrateStream) SendMsg(any) error            { return nil }
func (s *recordingNarrateStream) RecvMsg(any) error            { return nil }

func (s *recordingNarrateStream) snapshot() []*v1.NarrateEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*v1.NarrateEvent, len(s.events))
	copy(out, s.events)
	return out
}

func reviewSessionWithHunkStep(t *testing.T, store *session.Store, id string) {
	t.Helper()
	g := graph.NewGraph()
	step := &graph.Step{
		ID:    "hunk:foo.go:1",
		Label: "foo.go:1",
		Kind:  v1.StepKind_STEP_KIND_HUNK,
		HunkSpan: &v1.HunkSpan{
			FilePath: "foo.go",
			NewStart: 1,
			NewLines: 1,
			RawDiff:  "@@ -1 +1 @@\n-old\n+new\n",
		},
	}
	g.Add(step)
	g.EntryID = step.ID
	sess := session.New(id, g, 5, "diff", false, nil, "", "", "")
	sess.Kind = v1.SessionKind_SESSION_KIND_REVIEW
	store.Set(sess)
}

func walkthroughSessionWithSourceStep(t *testing.T, store *session.Store, id string) {
	t.Helper()
	g := graph.NewGraph()
	step := &graph.Step{
		ID:    "step:1",
		Label: "function entry",
		Kind:  v1.StepKind_STEP_KIND_ENTRY,
		Source: &v1.SourceSpan{
			StartLine: 1,
			EndLine:   1,
			RawSource: "package main",
		},
	}
	g.Add(step)
	g.EntryID = step.ID
	sess := session.New(id, g, 5, "go", false, []byte("package main\n"), "main.go", "/repo", "HEAD")
	sess.Kind = v1.SessionKind_SESSION_KIND_WALKTHROUGH
	store.Set(sess)
}

func TestNavigate_ReviewSession_SummaryReadyBeforeComplete(t *testing.T) {
	store := session.NewStore()
	const sessID = "review-test-1"
	reviewSessionWithHunkStep(t, store, sessID)

	tokens := make(chan string, 3)
	tokens <- "alpha"
	tokens <- "beta"
	tokens <- "gamma"
	close(tokens)

	summaryReady := make(chan struct{})
	close(summaryReady)

	provider := &summaryProvider{
		tokens:       tokens,
		summary:      &llm.StepSummary{Breaking: "No", WhatChanged: "test"},
		summaryReady: summaryReady,
	}
	srv := server.New(store, provider, "")

	stream := &recordingNarrateStream{ctx: context.Background()}
	err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "hunk:foo.go:1"},
	}, stream)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	events := stream.snapshot()
	summaryIdx, completeIdx := -1, -1
	for i, e := range events {
		if e.GetSummaryReady() != nil {
			if summaryIdx != -1 {
				t.Errorf("multiple SummaryReady events; second at index %d", i)
			}
			summaryIdx = i
		}
		if e.GetComplete() != nil {
			completeIdx = i
		}
	}
	if summaryIdx == -1 {
		t.Fatal("no SummaryReady event emitted for review-session step")
	}
	if completeIdx == -1 {
		t.Fatal("no Complete event emitted")
	}
	if summaryIdx >= completeIdx {
		t.Errorf("SummaryReady (idx %d) must precede Complete (idx %d)", summaryIdx, completeIdx)
	}

	sr := events[summaryIdx].GetSummaryReady()
	if sr.Summary == nil || sr.Summary.WhatChanged != "test" {
		t.Errorf("SummaryReady.summary unexpected: %+v", sr.Summary)
	}

	// Back-compat: Complete.summary still populated with the same data.
	c := events[completeIdx].GetComplete()
	if c.Summary == nil || c.Summary.WhatChanged != "test" {
		t.Errorf("Complete.summary back-compat: got %+v", c.Summary)
	}
}

func TestNavigate_ReviewSession_SummaryEmittedBeforeNarrationFinishes(t *testing.T) {
	store := session.NewStore()
	const sessID = "review-test-2"
	reviewSessionWithHunkStep(t, store, sessID)

	// Tokens channel is unbuffered. The test pushes only after observing
	// SummaryReady, proving the summary did not block behind narration.
	tokens := make(chan string)
	summaryReady := make(chan struct{})
	close(summaryReady) // summary returns immediately

	provider := &summaryProvider{
		tokens:       tokens,
		summary:      &llm.StepSummary{Breaking: "No", WhatChanged: "interleaved"},
		summaryReady: summaryReady,
	}
	srv := server.New(store, provider, "")

	stream := &recordingNarrateStream{ctx: context.Background()}
	done := make(chan error, 1)
	go func() {
		done <- srv.Navigate(&v1.NavigateRequest{
			SessionId:   sessID,
			Destination: &v1.NavigateRequest_StepId{StepId: "hunk:foo.go:1"},
		}, stream)
	}()

	if !waitForEvent(stream, 2*time.Second, func(e *v1.NarrateEvent) bool {
		return e.GetSummaryReady() != nil
	}) {
		t.Fatal("SummaryReady was not emitted before tokens were pushed")
	}

	// Now push narration and let Navigate finish.
	tokens <- "after-summary-1"
	tokens <- "after-summary-2"
	close(tokens)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Navigate: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Navigate did not return")
	}

	events := stream.snapshot()
	if len(events) == 0 || events[0].GetSummaryReady() == nil {
		t.Fatalf("first event should be SummaryReady; got %d events", len(events))
	}
	if last := events[len(events)-1]; last.GetComplete() == nil {
		t.Errorf("last event should be Complete; got %T", last.Event)
	}
}

func TestNavigate_WalkthroughStep_NoSummaryReady(t *testing.T) {
	store := session.NewStore()
	const sessID = "walk-test-1"
	walkthroughSessionWithSourceStep(t, store, sessID)

	tokens := make(chan string, 1)
	tokens <- "narration"
	close(tokens)

	// summaryReady is never closed. For walkthrough steps navigate.go must
	// not start the summary goroutine, so this should not block.
	provider := &summaryProvider{
		tokens:       tokens,
		summary:      nil,
		summaryReady: make(chan struct{}),
	}
	srv := server.New(store, provider, "")

	stream := &recordingNarrateStream{ctx: context.Background()}
	if err := srv.Navigate(&v1.NavigateRequest{
		SessionId:   sessID,
		Destination: &v1.NavigateRequest_StepId{StepId: "step:1"},
	}, stream); err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	events := stream.snapshot()
	for _, e := range events {
		if e.GetSummaryReady() != nil {
			t.Fatal("walkthrough step emitted SummaryReady; expected none")
		}
	}
	if last := events[len(events)-1]; last.GetComplete() == nil {
		t.Fatalf("last event should be Complete; got %T", last.Event)
	}
	if c := events[len(events)-1].GetComplete(); c.Summary != nil {
		t.Errorf("walkthrough Complete.summary should be nil; got %+v", c.Summary)
	}
}

func TestOpenReviewSession_GlossaryEmpty(t *testing.T) {
	store := session.NewStore()
	srv := server.New(store, &mockProvider{}, "")

	stream := &mockSessionStream{ctx: context.Background()}
	if err := srv.OpenReviewSession(&v1.OpenReviewSessionRequest{
		Url: "https://test.example.com/owner/repo/pull/1",
	}, stream); err != nil {
		t.Fatalf("OpenReviewSession: %v", err)
	}

	rr := stream.events[len(stream.events)-1].GetReviewReady()
	if rr == nil {
		t.Fatal("no ReviewReady event")
	}
	if len(rr.Glossary) != 0 {
		t.Errorf("ReviewReady.glossary should be empty; got %d terms", len(rr.Glossary))
	}
}

// waitForEvent polls stream until pred matches an event or timeout elapses.
func waitForEvent(stream *recordingNarrateStream, timeout time.Duration, pred func(*v1.NarrateEvent) bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, e := range stream.snapshot() {
			if pred(e) {
				return true
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}
