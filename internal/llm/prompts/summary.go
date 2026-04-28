package prompts

import "fmt"

// StepSummary returns the system + user messages for a structured triage
// summary of a single review hunk. The user prompt instructs Claude to emit
// strictly formatted lines that can be parsed into the eight StepSummary
// fields.
func StepSummary(language, hunkDiff, contextBefore, contextAfter string) (system, user string) {
	system = fmt.Sprintf(`You are reviewing a single %s diff hunk for a code reviewer who needs fast triage.
Produce a structured summary in exactly the following format. Each line must be present.
If a field is not applicable, write "—" rather than omitting the line.

Breaking: Yes|No
Risk: Low|Medium|High — brief reason
WhatChanged: one-line summary
SideEffects: what else this change affects, or —
Tests: Added|Modified|Missing
ReviewerFocus: what to pay particular attention to, or —
Suggestion: any concern worth raising, or —
Confidence: Low|Medium|High — reason if Low, otherwise —`, language)

	user = fmt.Sprintf(`Hunk:
%s

Surrounding context:
%s
%s`, hunkDiff, contextBefore, contextAfter)
	return system, user
}
