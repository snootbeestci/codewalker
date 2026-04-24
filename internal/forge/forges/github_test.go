package forges

import (
	"testing"

	"github.com/yourorg/codewalker/internal/forge"
)

func TestParseURL(t *testing.T) {
	h := &githubHandler{}

	tests := []struct {
		name      string
		url       string
		wantKind  forge.ForgeContextKind
		wantPR    int
		wantBase  string
		wantHead  string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "PR URL",
			url:       "github.com/octocat/hello-world/pull/42",
			wantKind:  forge.ForgeContextKindPR,
			wantPR:    42,
			wantOwner: "octocat",
			wantRepo:  "hello-world",
		},
		{
			name:      "PR URL with https scheme",
			url:       "https://github.com/octocat/hello-world/pull/42",
			wantKind:  forge.ForgeContextKindPR,
			wantPR:    42,
			wantOwner: "octocat",
			wantRepo:  "hello-world",
		},
		{
			name:      "commit URL",
			url:       "github.com/octocat/hello-world/commit/abc123def456",
			wantKind:  forge.ForgeContextKindCommit,
			wantHead:  "abc123def456",
			wantOwner: "octocat",
			wantRepo:  "hello-world",
		},
		{
			name:      "comparison URL",
			url:       "github.com/octocat/hello-world/compare/main...feature-branch",
			wantKind:  forge.ForgeContextKindComparison,
			wantBase:  "main",
			wantHead:  "feature-branch",
			wantOwner: "octocat",
			wantRepo:  "hello-world",
		},
		{
			name:    "blob URL is not a review context",
			url:     "github.com/octocat/hello-world/blob/main/README.md",
			wantErr: true,
		},
		{
			name:    "unrecognised pattern",
			url:     "github.com/octocat/hello-world/issues/1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc, err := h.ParseURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseURL(%q) = nil error, want error", tt.url)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseURL(%q) = %v, want nil", tt.url, err)
			}
			if fc.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", fc.Kind, tt.wantKind)
			}
			if fc.PRNumber != tt.wantPR {
				t.Errorf("PRNumber = %v, want %v", fc.PRNumber, tt.wantPR)
			}
			if fc.BaseRef != tt.wantBase {
				t.Errorf("BaseRef = %q, want %q", fc.BaseRef, tt.wantBase)
			}
			if fc.HeadRef != tt.wantHead {
				t.Errorf("HeadRef = %q, want %q", fc.HeadRef, tt.wantHead)
			}
			if fc.Owner != tt.wantOwner {
				t.Errorf("Owner = %q, want %q", fc.Owner, tt.wantOwner)
			}
			if fc.Repo != tt.wantRepo {
				t.Errorf("Repo = %q, want %q", fc.Repo, tt.wantRepo)
			}
		})
	}
}

func TestParseHunks(t *testing.T) {
	patch := `@@ -1,7 +1,6 @@
 line one
-removed line
 line three
+added line
 line five
@@ -10,3 +10,4 @@
 context
+new line
 end`

	hunks, err := parseHunks(patch)
	if err != nil {
		t.Fatalf("parseHunks: %v", err)
	}
	if len(hunks) != 2 {
		t.Fatalf("got %d hunks, want 2", len(hunks))
	}

	h0 := hunks[0]
	if h0.OldStart != 1 || h0.OldLines != 7 {
		t.Errorf("hunk[0] old = %d,%d, want 1,7", h0.OldStart, h0.OldLines)
	}
	if h0.NewStart != 1 || h0.NewLines != 6 {
		t.Errorf("hunk[0] new = %d,%d, want 1,6", h0.NewStart, h0.NewLines)
	}

	h1 := hunks[1]
	if h1.OldStart != 10 || h1.OldLines != 3 {
		t.Errorf("hunk[1] old = %d,%d, want 10,3", h1.OldStart, h1.OldLines)
	}
	if h1.NewStart != 10 || h1.NewLines != 4 {
		t.Errorf("hunk[1] new = %d,%d, want 10,4", h1.NewStart, h1.NewLines)
	}
}

func TestForgeResolve(t *testing.T) {
	h, err := forge.Resolve("github.com")
	if err != nil {
		t.Fatalf("forge.Resolve(\"github.com\") = %v, want nil", err)
	}
	if h == nil {
		t.Fatal("got nil handler")
	}
}
