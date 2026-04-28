package llm

import (
	"reflect"
	"testing"
)

func TestParseStepSummary(t *testing.T) {
	const placeholder = "—"

	tests := []struct {
		name string
		raw  string
		want *StepSummary
	}{
		{
			name: "all fields populated",
			raw: `Breaking: No
Risk: Low — straightforward refactor
WhatChanged: Renames helper function for clarity
SideEffects: None
Tests: Modified
ReviewerFocus: Verify the rename is consistent across call sites
Suggestion: Consider adding a deprecation alias if the old name is exported
Confidence: High — —`,
			want: &StepSummary{
				Breaking:      "No",
				Risk:          "Low — straightforward refactor",
				WhatChanged:   "Renames helper function for clarity",
				SideEffects:   "None",
				Tests:         "Modified",
				ReviewerFocus: "Verify the rename is consistent across call sites",
				Suggestion:    "Consider adding a deprecation alias if the old name is exported",
				Confidence:    "High — —",
			},
		},
		{
			name: "missing fields fall back to placeholder",
			raw: `Breaking: No
Risk: Medium — touches auth flow
WhatChanged: Adds rate limiting to login endpoint`,
			want: &StepSummary{
				Breaking:      "No",
				Risk:          "Medium — touches auth flow",
				WhatChanged:   "Adds rate limiting to login endpoint",
				SideEffects:   placeholder,
				Tests:         placeholder,
				ReviewerFocus: placeholder,
				Suggestion:    placeholder,
				Confidence:    placeholder,
			},
		},
		{
			name: "empty value uses placeholder",
			raw: `Breaking: No
Risk:
WhatChanged: Adds logging`,
			want: &StepSummary{
				Breaking:      "No",
				Risk:          placeholder,
				WhatChanged:   "Adds logging",
				SideEffects:   placeholder,
				Tests:         placeholder,
				ReviewerFocus: placeholder,
				Suggestion:    placeholder,
				Confidence:    placeholder,
			},
		},
		{
			name: "unexpected keys are ignored",
			raw: `Breaking: No
Severity: High
WhatChanged: Adds caching layer
RandomKey: ignored
Tests: Added`,
			want: &StepSummary{
				Breaking:      "No",
				Risk:          placeholder,
				WhatChanged:   "Adds caching layer",
				SideEffects:   placeholder,
				Tests:         "Added",
				ReviewerFocus: placeholder,
				Suggestion:    placeholder,
				Confidence:    placeholder,
			},
		},
		{
			name: "empty input returns all placeholders",
			raw:  "",
			want: &StepSummary{
				Breaking:      placeholder,
				Risk:          placeholder,
				WhatChanged:   placeholder,
				SideEffects:   placeholder,
				Tests:         placeholder,
				ReviewerFocus: placeholder,
				Suggestion:    placeholder,
				Confidence:    placeholder,
			},
		},
		{
			name: "lines without colons are skipped",
			raw: `Some narrative preamble that the LLM added
Breaking: Yes
This line has no colon
Risk: High — breaks public API
WhatChanged: Removes deprecated method`,
			want: &StepSummary{
				Breaking:      "Yes",
				Risk:          "High — breaks public API",
				WhatChanged:   "Removes deprecated method",
				SideEffects:   placeholder,
				Tests:         placeholder,
				ReviewerFocus: placeholder,
				Suggestion:    placeholder,
				Confidence:    placeholder,
			},
		},
		{
			name: "extra whitespace around keys and values is trimmed",
			raw: `  Breaking  :   No
  Risk   :   Low — clean change
WhatChanged:Removes unused import`,
			want: &StepSummary{
				Breaking:      "No",
				Risk:          "Low — clean change",
				WhatChanged:   "Removes unused import",
				SideEffects:   placeholder,
				Tests:         placeholder,
				ReviewerFocus: placeholder,
				Suggestion:    placeholder,
				Confidence:    placeholder,
			},
		},
		{
			name: "values containing colons are preserved",
			raw: `Breaking: No
WhatChanged: Updates timeout from 5:00 to 10:00`,
			want: &StepSummary{
				Breaking:      "No",
				Risk:          placeholder,
				WhatChanged:   "Updates timeout from 5:00 to 10:00",
				SideEffects:   placeholder,
				Tests:         placeholder,
				ReviewerFocus: placeholder,
				Suggestion:    placeholder,
				Confidence:    placeholder,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStepSummary(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseStepSummary() mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}
