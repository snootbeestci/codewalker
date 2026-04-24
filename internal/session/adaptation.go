package session

import v1 "github.com/yourorg/codewalker/gen/codewalker/v1"

// EffectiveLevel maps a client-supplied ExperienceLevel to a 1–10 integer
// that controls narration depth and vocabulary.
//
// v1: static mapping.
// v2: replace with adaptive logic driven by user behaviour signals
// (SIMPLER rephrase count, DEEPER rephrase count, glossary expansions, etc.).
func EffectiveLevel(level v1.ExperienceLevel) int {
	switch level {
	case v1.ExperienceLevel_EXPERIENCE_LEVEL_JUNIOR:
		return 3
	case v1.ExperienceLevel_EXPERIENCE_LEVEL_MID:
		return 6
	case v1.ExperienceLevel_EXPERIENCE_LEVEL_SENIOR:
		return 9
	default:
		return 5
	}
}
