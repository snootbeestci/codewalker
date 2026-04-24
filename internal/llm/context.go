package llm

import (
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// StepContext carries everything BuildMessages needs to construct a narration
// prompt. No caller should pre-assemble prompt strings; all assembly happens
// inside BuildMessages.
//
// v2 extension point: add a RollingHistory []anthropic.MessageParam field for
// multi-turn coherence — BuildMessages will prepend it to the returned slice
// without requiring any caller changes.
type StepContext struct {
	Language string
	// SymbolSignature is the function/method signature or step label,
	// e.g. "func processPayment(amount int) error".
	SymbolSignature string
	// CallChain is the ordered breadcrumb of step labels leading to this step.
	CallChain []string
	RawSource string
	// EffectiveLevel is the 1–10 experience scale from adaptation.go.
	EffectiveLevel uint32
}

// BuildMessages constructs the system prompt and user message list for a step
// narration request. All prompt assembly lives here; callers pass a StepContext
// and receive ready-to-send parameters.
func BuildMessages(ctx StepContext) (system string, messages []anthropic.MessageParam) {
	system = buildSystem(ctx.Language, ctx.EffectiveLevel)
	messages = []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(buildUser(ctx))),
	}
	return system, messages
}

// oneMessage is a convenience helper for single-turn, non-narration calls
// (glossary extraction, external-call summarization, term expansion, rephrase).
func oneMessage(userContent string) []anthropic.MessageParam {
	return []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userContent)),
	}
}

func buildSystem(language string, level uint32) string {
	return fmt.Sprintf(
		"You are a patient, expert code guide explaining %s code to a developer "+
			"at experience level %d/10.\n\n%s\n\n"+
			"Format: Prose paragraphs only. No bullet lists. No markdown headers.\n"+
			"Length: 2–4 paragraphs. Stop when the explanation is complete.",
		language, level, narrateStyle(level),
	)
}

func narrateStyle(level uint32) string {
	switch {
	case level <= 3:
		return "Style: Use simple analogies. Avoid jargon. Explain every construct, including language keywords."
	case level <= 6:
		return "Style: Assume familiarity with basic constructs. Focus on intent, patterns, and data flow."
	default:
		return "Style: Be concise. Focus on design decisions, trade-offs, and non-obvious runtime behaviour."
	}
}

func buildUser(ctx StepContext) string {
	var sb strings.Builder
	if len(ctx.CallChain) > 0 {
		sb.WriteString("Location: ")
		sb.WriteString(strings.Join(ctx.CallChain, " → "))
		sb.WriteString("\n")
	}
	if ctx.SymbolSignature != "" {
		sb.WriteString("Step: ")
		sb.WriteString(ctx.SymbolSignature)
		sb.WriteString("\n")
	}
	sb.WriteString("\nCode:\n")
	sb.WriteString(ctx.RawSource)
	sb.WriteString("\n\nNarrate this step.")
	return sb.String()
}
