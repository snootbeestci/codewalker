package session_test

import (
	"testing"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/session"
)

func TestEffectiveLevel(t *testing.T) {
	cases := []struct {
		level v1.ExperienceLevel
		want  int
	}{
		{v1.ExperienceLevel_EXPERIENCE_LEVEL_JUNIOR, 3},
		{v1.ExperienceLevel_EXPERIENCE_LEVEL_MID, 6},
		{v1.ExperienceLevel_EXPERIENCE_LEVEL_SENIOR, 9},
		{v1.ExperienceLevel_EXPERIENCE_LEVEL_UNSPECIFIED, 5},
	}
	for _, tc := range cases {
		got := session.EffectiveLevel(tc.level)
		if got != tc.want {
			t.Errorf("EffectiveLevel(%v) = %d, want %d", tc.level, got, tc.want)
		}
	}
}
