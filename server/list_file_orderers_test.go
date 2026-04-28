package server_test

import (
	"context"
	"testing"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	_ "github.com/yourorg/codewalker/internal/forge/orderers"
	"github.com/yourorg/codewalker/internal/session"
	"github.com/yourorg/codewalker/server"
)

func TestListFileOrderers(t *testing.T) {
	store := session.NewStore()
	srv := server.New(store, &mockProvider{}, "")

	resp, err := srv.ListFileOrderers(context.Background(), &v1.ListFileOrderersRequest{})
	if err != nil {
		t.Fatalf("ListFileOrderers: %v", err)
	}

	want := map[string]string{
		"entry-points-first": "Entry points first, then domain logic, infrastructure, and tests last",
		"alphabetical":       "Sorted by file path",
		"as-fetched":         "Preserves the order returned by the forge — useful for debugging",
	}

	got := map[string]string{}
	for _, o := range resp.Orderers {
		got[o.Name] = o.Description
	}

	for name, desc := range want {
		gotDesc, ok := got[name]
		if !ok {
			t.Errorf("orderer %q missing from ListFileOrderers response", name)
			continue
		}
		if gotDesc != desc {
			t.Errorf("orderer %q description = %q, want %q", name, gotDesc, desc)
		}
	}
}
