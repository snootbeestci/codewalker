package server_test

import (
	"context"
	"testing"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/forge"
	_ "github.com/yourorg/codewalker/internal/forge/orderers"
	"github.com/yourorg/codewalker/internal/session"
	"github.com/yourorg/codewalker/server"
)

// multiFileForgeHandler returns several files so we can observe ordering.
type multiFileForgeHandler struct{}

func (m *multiFileForgeHandler) Hosts() []string { return []string{"multi.example.com"} }

func (m *multiFileForgeHandler) ParseURL(rawURL string) (*forge.ForgeContext, error) {
	return &forge.ForgeContext{
		Kind:     forge.ForgeContextKindPR,
		Forge:    "mock-multi",
		Owner:    "owner",
		Repo:     "repo",
		PRNumber: 1,
		BaseRef:  "main",
		HeadRef:  "feature",
		URL:      rawURL,
	}, nil
}

func (m *multiFileForgeHandler) FetchReview(_ context.Context, fc *forge.ForgeContext, _ string) (*forge.ReviewPayload, error) {
	hunk := func() *forge.Hunk {
		return &forge.Hunk{
			OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1,
			RawDiff: "@@ -1 +1 @@\n-old\n+new\n",
		}
	}
	return &forge.ReviewPayload{
		Context: fc,
		Files: []*forge.ReviewFile{
			{Path: "internal/foo/bar_test.go", Language: "go", ChangeKind: "MODIFIED", Hunks: []*forge.Hunk{hunk()}},
			{Path: "internal/util/strings.go", Language: "go", ChangeKind: "MODIFIED", Hunks: []*forge.Hunk{hunk()}},
			{Path: "cmd/server/main.go", Language: "go", ChangeKind: "MODIFIED", Hunks: []*forge.Hunk{hunk()}},
			{Path: "src/services/payment.go", Language: "go", ChangeKind: "MODIFIED", Hunks: []*forge.Hunk{hunk()}},
		},
	}, nil
}

func (m *multiFileForgeHandler) FetchFile(_ context.Context, _ *forge.ForgeContext, _, _, _ string) ([]byte, error) {
	return []byte("line\n"), nil
}

func (m *multiFileForgeHandler) ResolveToken(_ context.Context) (string, error) { return "", nil }

func init() {
	forge.Register(&multiFileForgeHandler{})
}

func filePathsInOrder(rr *v1.ReviewReady) []string {
	out := make([]string, 0, len(rr.ForgeContext.Files))
	for _, f := range rr.ForgeContext.Files {
		out = append(out, f.FilePath)
	}
	return out
}

func TestOpenReviewSession_DefaultOrderingIsEntryPointsFirst(t *testing.T) {
	store := session.NewStore()
	srv := server.New(store, &mockProvider{}, "")

	stream := &mockSessionStream{ctx: context.Background()}
	if err := srv.OpenReviewSession(&v1.OpenReviewSessionRequest{
		Url: "https://multi.example.com/owner/repo/pull/1",
	}, stream); err != nil {
		t.Fatalf("OpenReviewSession: %v", err)
	}

	rr := stream.events[len(stream.events)-1].GetReviewReady()
	if rr == nil {
		t.Fatal("no ReviewReady event")
	}

	got := filePathsInOrder(rr)
	want := []string{
		"cmd/server/main.go",         // entry point (10)
		"src/services/payment.go",    // domain (30)
		"internal/util/strings.go",   // default (40)
		"internal/foo/bar_test.go",   // tests (80)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d files, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOpenReviewSession_AlphabeticalOrdering(t *testing.T) {
	store := session.NewStore()
	srv := server.New(store, &mockProvider{}, "")

	stream := &mockSessionStream{ctx: context.Background()}
	if err := srv.OpenReviewSession(&v1.OpenReviewSessionRequest{
		Url:          "https://multi.example.com/owner/repo/pull/1",
		FileOrdering: "alphabetical",
	}, stream); err != nil {
		t.Fatalf("OpenReviewSession: %v", err)
	}

	rr := stream.events[len(stream.events)-1].GetReviewReady()
	if rr == nil {
		t.Fatal("no ReviewReady event")
	}

	got := filePathsInOrder(rr)
	want := []string{
		"cmd/server/main.go",
		"internal/foo/bar_test.go",
		"internal/util/strings.go",
		"src/services/payment.go",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOpenReviewSession_UnknownOrdererIsInvalidArgument(t *testing.T) {
	store := session.NewStore()
	srv := server.New(store, &mockProvider{}, "")

	stream := &mockSessionStream{ctx: context.Background()}
	err := srv.OpenReviewSession(&v1.OpenReviewSessionRequest{
		Url:          "https://multi.example.com/owner/repo/pull/1",
		FileOrdering: "no-such-orderer",
	}, stream)
	if err == nil {
		t.Fatal("expected error for unknown orderer")
	}
}
