package prompts

import "fmt"

// Rephrase returns the system + user messages for a rephrase request.
// v1: no original narration cached — generates a fresh narration with the
// requested style applied.
func Rephrase(code, language, mode string, level int) (system, user string) {
	modeInstruction := rephraseInstruction(mode)

	system = fmt.Sprintf(`You are a patient, expert code guide explaining %s code to a developer at experience level %d/10.
Format: Prose paragraphs only. No bullet lists. No markdown headers.`, language, level)

	user = fmt.Sprintf("Code:\n%s\n\nInstruction: %s", code, modeInstruction)
	return system, user
}

func rephraseInstruction(mode string) string {
	switch mode {
	case "SIMPLER":
		return "Simplify the explanation significantly. Use plain language, relatable analogies, and shorter sentences. Avoid technical terms unless you immediately explain them."
	case "DEEPER":
		return "Go deeper. Explain the underlying mechanics, potential edge cases, performance characteristics, and why this approach was likely chosen over alternatives."
	case "ANALOGY":
		return "Explain this entire step using a real-world analogy that does not involve code. The analogy should map precisely to what the code is doing."
	case "TLDR":
		return "Give a one or two sentence TL;DR. Be direct and extremely concise."
	default:
		return "Rephrase the explanation clearly and concisely."
	}
}
