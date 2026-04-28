package forge

import (
	"testing"
)

type stubOrderer struct {
	name string
	desc string
}

func (s *stubOrderer) Name() string                                   { return s.name }
func (s *stubOrderer) Description() string                            { return s.desc }
func (s *stubOrderer) Order(files []*ReviewFile) []*ReviewFile        { return files }

func TestResolveOrderer_EmptyNameUsesDefault(t *testing.T) {
	RegisterOrderer(&stubOrderer{name: DefaultOrdererName, desc: "default"})

	got, err := ResolveOrderer("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name() != DefaultOrdererName {
		t.Errorf("name = %q, want %q", got.Name(), DefaultOrdererName)
	}
}

func TestResolveOrderer_UnknownNameReturnsError(t *testing.T) {
	_, err := ResolveOrderer("definitely-not-registered-xyz")
	if err == nil {
		t.Fatal("expected error for unknown orderer name")
	}
}

func TestResolveOrderer_KnownName(t *testing.T) {
	RegisterOrderer(&stubOrderer{name: "stub-known", desc: "stub"})

	got, err := ResolveOrderer("stub-known")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name() != "stub-known" {
		t.Errorf("name = %q, want %q", got.Name(), "stub-known")
	}
}

func TestListOrderers_IncludesRegistered(t *testing.T) {
	RegisterOrderer(&stubOrderer{name: "stub-listed", desc: "stub"})

	all := ListOrderers()
	found := false
	for _, o := range all {
		if o.Name() == "stub-listed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("registered orderer not present in ListOrderers()")
	}
}
