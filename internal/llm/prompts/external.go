package prompts

import "fmt"

// ExternalCall returns messages for summarising an external (stdlib / third-party)
// function call.
func ExternalCall(pkg, symbol, language string) (system, user string) {
	system = fmt.Sprintf(`You are a concise technical reference for %s libraries.
Summarise what the given function does in 2–3 sentences.
Focus on: what it does, what it returns, and any important side effects.
Do not mention version numbers or make claims about URL locations.`, language)

	user = fmt.Sprintf("Summarise %s.%s", pkg, symbol)
	return system, user
}
