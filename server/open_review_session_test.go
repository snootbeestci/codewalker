package server_test

import (
	"context"
	"testing"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/forge"
	"github.com/yourorg/codewalker/internal/llm"
	"github.com/yourorg/codewalker/internal/session"
	"github.com/yourorg/codewalker/server"
	"google.golang.org/grpc/metadata"
)

// --- mock forge handler ---

type mockForgeHandler struct{}

func (m *mockForgeHandler) Hosts() []string { return []string{"test.example.com"} }

func (m *mockForgeHandler) ParseURL(rawURL string) (*forge.ForgeContext, error) {
	return &forge.ForgeContext{
		Kind:          forge.ForgeContextKindPR,
		Forge:         "mock",
		Owner:         "owner",
		Repo:          "repo",
		PRNumber:      1,
		PRTitle:       "Test PR",
		PRDescription: "A test pull request",
		PRAuthor:      "testuser",
		BaseRef:       "main",
		HeadRef:       "feature",
		URL:           rawURL,
	}, nil
}

func (m *mockForgeHandler) FetchReview(ctx context.Context, fc *forge.ForgeContext, token string) (*forge.ReviewPayload, error) {
	return &forge.ReviewPayload{
		Context: fc,
		Files: []*forge.ReviewFile{
			{
				Path:       "auth/login.go",
				Language:   "go",
				ChangeKind: "MODIFIED",
				Hunks: []*forge.Hunk{
					{
						OldStart: 42, OldLines: 3,
						NewStart: 42, NewLines: 4,
						RawDiff: "@@ -42,3 +42,4 @@\n context\n-old line\n+new line\n+added line\n context\n",
					},
					{
						OldStart: 80, OldLines: 2,
						NewStart: 81, NewLines: 2,
						RawDiff: "@@ -80,2 +81,2 @@\n context\n-removed\n+replaced\n",
					},
				},
				LinesAdded:   3,
				LinesRemoved: 2,
			},
		},
	}, nil
}

func (m *mockForgeHandler) FetchFile(ctx context.Context, fc *forge.ForgeContext, path, ref, token string) ([]byte, error) {
	return []byte("line 1\nline 2\nline 3\nline 4\nline 5\n"), nil
}

func (m *mockForgeHandler) ResolveToken(ctx context.Context) (string, error) {
	return "mock-token", nil
}

func init() {
	forge.Register(&mockForgeHandler{})
}

// --- mock LLM provider ---

type mockProvider struct{}

func (m *mockProvider) Narrate(_ context.Context, _ llm.NarrateRequest) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "This change updates the authentication logic."
	close(ch)
	return ch, nil
}

func (m *mockProvider) Rephrase(_ context.Context, _ llm.RephraseRequest) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}

func (m *mockProvider) SummarizeExternalCall(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}

func (m *mockProvider) ExtractGlossaryTerms(_ context.Context, _ llm.GlossaryRequest) ([]llm.GlossaryCandidate, error) {
	return []llm.GlossaryCandidate{
		{Term: "authentication", Kind: "DOMAIN"},
	}, nil
}

func (m *mockProvider) ExpandTerm(_ context.Context, _ llm.ExpandTermRequest) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}

func (m *mockProvider) GenerateStepSummary(_ context.Context, _ llm.SummaryRequest) (*llm.StepSummary, error) {
	return &llm.StepSummary{
		Breaking:      "No",
		Risk:          "Low — mock summary",
		WhatChanged:   "Mock change description",
		SideEffects:   "—",
		Tests:         "Modified",
		ReviewerFocus: "—",
		Suggestion:    "—",
		Confidence:    "High — —",
	}, nil
}

// --- mock gRPC stream for SessionEvent ---

type mockSessionStream struct {
	ctx    context.Context
	events []*v1.SessionEvent
}

func (m *mockSessionStream) Send(e *v1.SessionEvent) error {
	m.events = append(m.events, e)
	return nil
}
func (m *mockSessionStream) Context() context.Context       { return m.ctx }
func (m *mockSessionStream) SetHeader(metadata.MD) error   { return nil }
func (m *mockSessionStream) SendHeader(metadata.MD) error  { return nil }
func (m *mockSessionStream) SetTrailer(metadata.MD)        {}
func (m *mockSessionStream) SendMsg(any) error             { return nil }
func (m *mockSessionStream) RecvMsg(any) error             { return nil }

// --- mock gRPC stream for NarrateEvent ---

type mockNarrateStream struct {
	ctx    context.Context
	events []*v1.NarrateEvent
}

func (m *mockNarrateStream) Send(e *v1.NarrateEvent) error {
	m.events = append(m.events, e)
	return nil
}
func (m *mockNarrateStream) Context() context.Context       { return m.ctx }
func (m *mockNarrateStream) SetHeader(metadata.MD) error   { return nil }
func (m *mockNarrateStream) SendHeader(metadata.MD) error  { return nil }
func (m *mockNarrateStream) SetTrailer(metadata.MD)        {}
func (m *mockNarrateStream) SendMsg(any) error             { return nil }
func (m *mockNarrateStream) RecvMsg(any) error             { return nil }

// --- tests ---

func TestOpenReviewSession(t *testing.T) {
	store := session.NewStore()
	srv := server.New(store, &mockProvider{}, "")

	stream := &mockSessionStream{ctx: context.Background()}
	err := srv.OpenReviewSession(&v1.OpenReviewSessionRequest{
		Url: "https://test.example.com/owner/repo/pull/1",
	}, stream)
	if err != nil {
		t.Fatalf("OpenReviewSession returned error: %v", err)
	}

	if len(stream.events) == 0 {
		t.Fatal("expected at least one event")
	}

	// Last event must be ReviewReady.
	last := stream.events[len(stream.events)-1]
	rr := last.GetReviewReady()
	if rr == nil {
		t.Fatalf("last event is not ReviewReady: %T", last.Event)
	}
	if rr.SessionId == "" {
		t.Error("expected non-empty session_id")
	}
	if rr.TotalSteps == 0 {
		t.Error("expected total_steps > 0")
	}
	if rr.EntryStepId == "" {
		t.Error("expected non-empty entry_step_id")
	}
	if rr.ForgeContext == nil {
		t.Fatal("expected non-nil forge_context")
	}
	if rr.ForgeContext.Forge != "mock" {
		t.Errorf("forge = %q, want %q", rr.ForgeContext.Forge, "mock")
	}
	if len(rr.Steps) == 0 {
		t.Error("expected non-empty steps")
	}
	for _, step := range rr.Steps {
		if step.HunkSpan == nil {
			t.Errorf("step %q has nil hunk_span", step.Id)
		}
		if step.Kind != v1.StepKind_STEP_KIND_HUNK {
			t.Errorf("step %q kind = %v, want STEP_KIND_HUNK", step.Id, step.Kind)
		}
	}

	// Session should be stored and marked as review kind.
	sess, err := store.Get(rr.SessionId)
	if err != nil {
		t.Fatalf("session not found: %v", err)
	}
	summary := sess.Summary()
	if summary.Kind != v1.SessionKind_SESSION_KIND_REVIEW {
		t.Errorf("session kind = %v, want SESSION_KIND_REVIEW", summary.Kind)
	}
}

func TestOpenReviewSession_MissingURL(t *testing.T) {
	store := session.NewStore()
	srv := server.New(store, &mockProvider{}, "")

	stream := &mockSessionStream{ctx: context.Background()}
	err := srv.OpenReviewSession(&v1.OpenReviewSessionRequest{}, stream)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestOpenReviewSession_UnsupportedForge(t *testing.T) {
	store := session.NewStore()
	srv := server.New(store, &mockProvider{}, "")

	stream := &mockSessionStream{ctx: context.Background()}
	err := srv.OpenReviewSession(&v1.OpenReviewSessionRequest{
		Url: "https://unknown-forge.example.org/owner/repo/pull/1",
	}, stream)
	if err == nil {
		t.Fatal("expected error for unsupported forge")
	}
}

func TestNavigateReviewSession(t *testing.T) {
	store := session.NewStore()
	srv := server.New(store, &mockProvider{}, "")

	// Open a review session first.
	sessionStream := &mockSessionStream{ctx: context.Background()}
	if err := srv.OpenReviewSession(&v1.OpenReviewSessionRequest{
		Url: "https://test.example.com/owner/repo/pull/1",
	}, sessionStream); err != nil {
		t.Fatalf("OpenReviewSession: %v", err)
	}

	rr := sessionStream.events[len(sessionStream.events)-1].GetReviewReady()
	if rr == nil {
		t.Fatal("no ReviewReady event")
	}

	// Navigate forward — should stream narration from the diff hunk.
	narStream := &mockNarrateStream{ctx: context.Background()}
	err := srv.Navigate(&v1.NavigateRequest{
		SessionId: rr.SessionId,
		Destination: &v1.NavigateRequest_Direction{
			Direction: v1.SimpleDirection_SIMPLE_DIRECTION_FORWARD,
		},
	}, narStream)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	if len(narStream.events) == 0 {
		t.Fatal("expected narration events")
	}
	last := narStream.events[len(narStream.events)-1]
	if last.GetComplete() == nil {
		t.Errorf("last narrate event is not StepComplete: %T", last.Event)
	}
}

func TestListSessionsReviewKind(t *testing.T) {
	store := session.NewStore()
	srv := server.New(store, &mockProvider{}, "")

	sessionStream := &mockSessionStream{ctx: context.Background()}
	if err := srv.OpenReviewSession(&v1.OpenReviewSessionRequest{
		Url: "https://test.example.com/owner/repo/pull/1",
	}, sessionStream); err != nil {
		t.Fatalf("OpenReviewSession: %v", err)
	}

	resp, err := srv.ListSessions(context.Background(), &v1.ListSessionsRequest{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(resp.Sessions) == 0 {
		t.Fatal("expected at least one session")
	}
	found := false
	for _, s := range resp.Sessions {
		if s.Kind == v1.SessionKind_SESSION_KIND_REVIEW {
			found = true
			break
		}
	}
	if !found {
		t.Error("no session with SESSION_KIND_REVIEW in list")
	}
}
