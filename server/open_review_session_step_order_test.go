package server_test

import (
	"context"
	"fmt"
	"testing"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/forge"
	_ "github.com/yourorg/codewalker/internal/forge/orderers"
	"github.com/yourorg/codewalker/internal/session"
	"github.com/yourorg/codewalker/server"
)

// stepOrderForgeHandler exposes 5 files with 3 hunks each so map iteration
// order is statistically extremely unlikely to coincide with the expected
// Forward traversal order.
type stepOrderForgeHandler struct{}

func (m *stepOrderForgeHandler) Hosts() []string { return []string{"steporder.example.com"} }

func (m *stepOrderForgeHandler) ParseURL(rawURL string) (*forge.ForgeContext, error) {
	return &forge.ForgeContext{
		Kind:     forge.ForgeContextKindPR,
		Forge:    "mock-step-order",
		Owner:    "owner",
		Repo:     "repo",
		PRNumber: 1,
		BaseRef:  "main",
		HeadRef:  "feature",
		URL:      rawURL,
	}, nil
}

func (m *stepOrderForgeHandler) FetchReview(_ context.Context, fc *forge.ForgeContext, _ string) (*forge.ReviewPayload, error) {
	mkHunk := func(newStart int) *forge.Hunk {
		return &forge.Hunk{
			OldStart: newStart, OldLines: 1,
			NewStart: newStart, NewLines: 1,
			RawDiff: fmt.Sprintf("@@ -%d +%d @@\n-old\n+new\n", newStart, newStart),
		}
	}
	mkFile := func(path string) *forge.ReviewFile {
		return &forge.ReviewFile{
			Path: path, Language: "go", ChangeKind: "MODIFIED",
			Hunks: []*forge.Hunk{mkHunk(10), mkHunk(50), mkHunk(100)},
		}
	}
	return &forge.ReviewPayload{
		Context: fc,
		Files: []*forge.ReviewFile{
			mkFile("aaa/file_a.go"),
			mkFile("bbb/file_b.go"),
			mkFile("ccc/file_c.go"),
			mkFile("ddd/file_d.go"),
			mkFile("eee/file_e.go"),
		},
	}, nil
}

func (m *stepOrderForgeHandler) FetchFile(_ context.Context, _ *forge.ForgeContext, _, _, _ string) ([]byte, error) {
	return []byte("line\n"), nil
}

func (m *stepOrderForgeHandler) ListPullRequests(_ context.Context, _, _, _ string) ([]*forge.PullRequest, error) {
	return nil, nil
}

func init() {
	forge.Register(&stepOrderForgeHandler{})
}

func TestReviewReady_StepsAreInForwardOrder(t *testing.T) {
	// Run repeatedly: map iteration order in Go is randomised per run, so
	// any residual reliance on it would surface across iterations.
	const iterations = 20

	expected := []string{
		"hunk:aaa/file_a.go:10", "hunk:aaa/file_a.go:50", "hunk:aaa/file_a.go:100",
		"hunk:bbb/file_b.go:10", "hunk:bbb/file_b.go:50", "hunk:bbb/file_b.go:100",
		"hunk:ccc/file_c.go:10", "hunk:ccc/file_c.go:50", "hunk:ccc/file_c.go:100",
		"hunk:ddd/file_d.go:10", "hunk:ddd/file_d.go:50", "hunk:ddd/file_d.go:100",
		"hunk:eee/file_e.go:10", "hunk:eee/file_e.go:50", "hunk:eee/file_e.go:100",
	}

	for i := 0; i < iterations; i++ {
		store := session.NewStore()
		srv := server.New(store, &mockProvider{}, "")

		stream := &mockSessionStream{ctx: context.Background()}
		if err := srv.OpenReviewSession(&v1.OpenReviewSessionRequest{
			Url:          "https://steporder.example.com/owner/repo/pull/1",
			FileOrdering: "alphabetical",
		}, stream); err != nil {
			t.Fatalf("iteration %d: OpenReviewSession: %v", i, err)
		}

		rr := stream.events[len(stream.events)-1].GetReviewReady()
		if rr == nil {
			t.Fatalf("iteration %d: no ReviewReady event", i)
		}

		if len(rr.Steps) != len(expected) {
			t.Fatalf("iteration %d: got %d steps, want %d", i, len(rr.Steps), len(expected))
		}
		for j, want := range expected {
			if rr.Steps[j].Id != want {
				t.Errorf("iteration %d: step %d id = %q, want %q", i, j, rr.Steps[j].Id, want)
			}
		}

		if rr.EntryStepId != expected[0] {
			t.Errorf("iteration %d: entry_step_id = %q, want %q", i, rr.EntryStepId, expected[0])
		}

		// Walk the graph by following NEXT edges from the entry step and
		// confirm the navigation chain matches wire order.
		got := []string{rr.EntryStepId}
		cur := rr.EntryStepId
		for k := 1; k < len(expected); k++ {
			step := findStep(rr.Steps, cur)
			if step == nil {
				t.Fatalf("iteration %d: step %q not present in ReviewReady.steps", i, cur)
			}
			next := nextNavigableTarget(step)
			if next == "" {
				t.Fatalf("iteration %d: no NEXT edge from %q (position %d)", i, cur, k-1)
			}
			got = append(got, next)
			cur = next
		}
		for k, want := range expected {
			if got[k] != want {
				t.Errorf("iteration %d: forward chain pos %d = %q, want %q", i, k, got[k], want)
			}
		}
	}
}

func findStep(steps []*v1.Step, id string) *v1.Step {
	for _, s := range steps {
		if s.Id == id {
			return s
		}
	}
	return nil
}

func nextNavigableTarget(s *v1.Step) string {
	for _, e := range s.Edges {
		if e.Label == v1.EdgeLabel_EDGE_LABEL_NEXT && e.Navigable {
			return e.TargetStepId
		}
	}
	return ""
}
