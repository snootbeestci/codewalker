package prompts

import (
	"fmt"
	"strings"
)

// Narrate returns the system + user messages for a step narration request.
func Narrate(code, language, stepLabel, stepKind string, callChain []string, variables []string, level int) (system, user string) {
	isDiff := strings.HasPrefix(strings.TrimSpace(code), "@@")

	if isDiff {
		system = fmt.Sprintf(`You are a patient, expert code reviewer explaining a diff to a developer at experience level %d/10.

Narration style guidelines:
- Level 1–3 (junior): Use simple analogies. Avoid jargon. Explain every construct and why it changed.
- Level 4–6 (mid): Assume familiarity with basic constructs. Focus on intent and patterns behind the change.
- Level 7–10 (senior): Be concise. Focus on design decisions, trade-offs, and non-obvious side effects.

Format: Prose paragraphs only. No bullet lists. No markdown headers.
Length: 2–4 paragraphs. Stop when the explanation is complete.`, level)
	} else {
		system = fmt.Sprintf(`You are a patient, expert code guide explaining %s code to a developer at experience level %d/10.

Narration style guidelines:
- Level 1–3 (junior): Use simple analogies. Avoid jargon. Explain every construct.
- Level 4–6 (mid): Assume familiarity with basic constructs. Focus on intent and patterns.
- Level 7–10 (senior): Be concise. Focus on design decisions, trade-offs, and non-obvious behaviour.

Format: Prose paragraphs only. No bullet lists. No markdown headers.
Length: 2–4 paragraphs. Stop when the explanation is complete.`, language, level)
	}

	var ctx strings.Builder
	if len(callChain) > 0 {
		ctx.WriteString("Current location: ")
		ctx.WriteString(strings.Join(callChain, " → "))
		ctx.WriteString("\n")
	}
	if len(variables) > 0 {
		ctx.WriteString("In-scope variables: ")
		ctx.WriteString(strings.Join(variables, ", "))
		ctx.WriteString("\n")
	}

	if isDiff {
		user = fmt.Sprintf(`%sChange: %s (%s)

Diff:
%s

Explain what this change does and why it might have been made.`, ctx.String(), stepLabel, stepKind, code)
	} else {
		user = fmt.Sprintf(`%sStep: %s (%s)

Code:
%s

Narrate this step.`, ctx.String(), stepLabel, stepKind, code)
	}

	return system, user
}
