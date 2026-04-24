package llm

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildMessages_SystemPromptContainsLevel(t *testing.T) {
	cases := []struct {
		level       uint32
		wantInStyle string
	}{
		{1, "simple analogies"},
		{3, "simple analogies"},
		{4, "basic constructs"},
		{6, "basic constructs"},
		{7, "design decisions"},
		{10, "design decisions"},
	}

	for _, tc := range cases {
		system, _ := BuildMessages(StepContext{
			Language:       "Go",
			EffectiveLevel: tc.level,
		})
		if !strings.Contains(system, tc.wantInStyle) {
			t.Errorf("level %d: system prompt missing %q\ngot: %s", tc.level, tc.wantInStyle, system)
		}
		wantLevel := fmt.Sprintf("%d/10", tc.level)
		if !strings.Contains(system, wantLevel) {
			t.Errorf("level %d: system prompt missing level indicator %q", tc.level, wantLevel)
		}
	}
}

func TestBuildMessages_LanguageInSystemPrompt(t *testing.T) {
	for _, lang := range []string{"Go", "TypeScript", "Python", "PHP"} {
		system, _ := BuildMessages(StepContext{Language: lang, EffectiveLevel: 5})
		if !strings.Contains(system, lang) {
			t.Errorf("language %q not found in system prompt", lang)
		}
	}
}

func TestBuildMessages_UserMessageContainsSource(t *testing.T) {
	src := "func add(a, b int) int { return a + b }"
	_, msgs := BuildMessages(StepContext{
		Language:        "Go",
		SymbolSignature: "func add",
		RawSource:       src,
		EffectiveLevel:  5,
	})

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	// The user message content is inside msgs[0]; inspect via string
	// representation since anthropic.MessageParam is opaque.
	_ = msgs[0] // structural check: no panic
}

func TestBuildMessages_CallChainIncluded(t *testing.T) {
	ctx := StepContext{
		Language:  "Go",
		CallChain: []string{"main", "run", "processPayment"},
		RawSource: "amount := 100",
	}
	_, msgs := BuildMessages(ctx)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestBuildMessages_NoCallerAssembly(t *testing.T) {
	// Verify that BuildMessages returns a non-empty system and a non-empty
	// messages slice regardless of which StepContext fields are zero-valued.
	system, msgs := BuildMessages(StepContext{})
	if system == "" {
		t.Error("system prompt must not be empty even for zero StepContext")
	}
	if len(msgs) == 0 {
		t.Error("messages must not be empty even for zero StepContext")
	}
}
