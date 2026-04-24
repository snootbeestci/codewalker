package llmtest

import (
	"context"

	"github.com/yourorg/codewalker/internal/llm"
)

// MockProvider is a test double for llm.Provider. All streaming methods return
// a single canned token followed by close. Non-streaming methods return
// fixed strings.
type MockProvider struct{}

func (m *MockProvider) Narrate(_ context.Context, _ llm.NarrateRequest) (<-chan string, error) {
	return chanOf("mock narration"), nil
}

func (m *MockProvider) Rephrase(_ context.Context, _ llm.RephraseRequest) (<-chan string, error) {
	return chanOf("mock rephrase"), nil
}

func (m *MockProvider) SummarizeExternalCall(_ context.Context, _, _, _ string) (string, error) {
	return "mock external call summary", nil
}

func (m *MockProvider) ExtractGlossaryTerms(_ context.Context, _ llm.GlossaryRequest) ([]llm.GlossaryCandidate, error) {
	return []llm.GlossaryCandidate{{Term: "mock", Kind: "LANGUAGE"}}, nil
}

func (m *MockProvider) ExpandTerm(_ context.Context, _ llm.ExpandTermRequest) (<-chan string, error) {
	return chanOf("mock term expansion"), nil
}

// chanOf returns a channel that emits a single string then closes.
func chanOf(s string) <-chan string {
	ch := make(chan string, 1)
	ch <- s
	close(ch)
	return ch
}
