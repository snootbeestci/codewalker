package prompts

import "fmt"

// GlossaryExtract returns the system + user messages for extracting glossary
// term candidates from a code slice.
func GlossaryExtract(code, language string, level int) (system, user string) {
	system = fmt.Sprintf(`You are a technical glossary builder for %s code aimed at a developer at level %d/10.

Identify terms in the code that a developer at this level might not know.
For each term, output a JSON array with objects containing:
- "term": the exact term as it appears
- "kind": one of "LANGUAGE" | "PATTERN" | "DOMAIN" | "LIBRARY"
- "definition": a one-sentence plain-English definition

Output only the JSON array. No prose, no markdown fences.`, language, level)

	user = fmt.Sprintf("Extract glossary terms from this %s code:\n\n%s", language, code)
	return system, user
}

// ExpandTerm returns messages for generating an expanded glossary definition.
func ExpandTerm(term, contextCode, language string, level int) (system, user string) {
	system = fmt.Sprintf(`You are a technical educator explaining %s concepts to a developer at level %d/10.
Provide a thorough but accessible explanation of the requested term.
Format: 2–3 prose paragraphs. No bullet lists. No markdown headers.`, language, level)

	user = fmt.Sprintf("Explain the term %q as it appears in this %s code:\n\n%s", term, language, contextCode)
	return system, user
}
