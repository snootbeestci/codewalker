package orderers

import (
	"testing"

	"github.com/yourorg/codewalker/internal/forge"
)

func TestAsFetched_PreservesOrder(t *testing.T) {
	a := &asFetched{}

	files := []*forge.ReviewFile{
		{Path: "z.go"},
		{Path: "a.go"},
		{Path: "m.go"},
	}

	got := a.Order(files)

	if len(got) != len(files) {
		t.Fatalf("len = %d, want %d", len(got), len(files))
	}
	for i := range files {
		if got[i].Path != files[i].Path {
			t.Errorf("position %d: got %q, want %q", i, got[i].Path, files[i].Path)
		}
	}
}

func TestAsFetched_ReturnsNewSlice(t *testing.T) {
	a := &asFetched{}

	files := []*forge.ReviewFile{
		{Path: "a.go"},
		{Path: "b.go"},
	}

	got := a.Order(files)

	// Mutating the returned slice header must not affect the input.
	got[0] = &forge.ReviewFile{Path: "mutated.go"}
	if files[0].Path != "a.go" {
		t.Errorf("input was affected by output mutation: files[0] = %q", files[0].Path)
	}
}

func TestAsFetched_NameAndDescription(t *testing.T) {
	a := &asFetched{}
	if a.Name() != "as-fetched" {
		t.Errorf("Name() = %q, want %q", a.Name(), "as-fetched")
	}
	if a.Description() == "" {
		t.Error("Description() must not be empty")
	}
}
