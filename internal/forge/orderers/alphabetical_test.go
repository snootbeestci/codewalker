package orderers

import (
	"testing"

	"github.com/yourorg/codewalker/internal/forge"
)

func TestAlphabetical_Order(t *testing.T) {
	a := &alphabetical{}

	files := []*forge.ReviewFile{
		{Path: "src/zeta.go"},
		{Path: "src/alpha.go"},
		{Path: "src/middle.go"},
	}

	got := a.Order(files)

	wantOrder := []string{
		"src/alpha.go",
		"src/middle.go",
		"src/zeta.go",
	}
	for i, w := range wantOrder {
		if got[i].Path != w {
			t.Errorf("position %d: got %q, want %q", i, got[i].Path, w)
		}
	}
}

func TestAlphabetical_DoesNotMutateInput(t *testing.T) {
	a := &alphabetical{}

	original := []*forge.ReviewFile{
		{Path: "z.go"},
		{Path: "a.go"},
	}
	originalFirst := original[0].Path

	_ = a.Order(original)

	if original[0].Path != originalFirst {
		t.Errorf("input was mutated: original[0] = %q, want %q", original[0].Path, originalFirst)
	}
}

func TestAlphabetical_NameAndDescription(t *testing.T) {
	a := &alphabetical{}
	if a.Name() != "alphabetical" {
		t.Errorf("Name() = %q, want %q", a.Name(), "alphabetical")
	}
	if a.Description() == "" {
		t.Error("Description() must not be empty")
	}
}
