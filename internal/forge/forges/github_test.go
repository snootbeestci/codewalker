package forges

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestParseHunkRange(t *testing.T) {
	tests := []struct {
		input     string
		wantStart int
		wantCount int
	}{
		{"+1,7", 1, 7},
		{"-3,5", 3, 5},
		{"+1", 1, 1},   // count omitted — defaults to 1
		{"-42", 42, 1}, // count omitted — defaults to 1
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			start, count, err := parseHunkRange(tt.input)
			if err != nil {
				t.Fatalf("parseHunkRange(%q) = %v, want nil", tt.input, err)
			}
			if start != tt.wantStart {
				t.Errorf("start = %d, want %d", start, tt.wantStart)
			}
			if count != tt.wantCount {
				t.Errorf("count = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestListPullRequests(t *testing.T) {
	const successBody = `[
		{"number": 42, "title": "Add foo", "html_url": "https://github.com/octocat/hello-world/pull/42",
		 "user": {"login": "alice"}, "head": {"ref": "feature/foo"}},
		{"number": 43, "title": "Fix bar", "html_url": "https://github.com/octocat/hello-world/pull/43",
		 "user": {"login": "bob"}, "head": {"ref": "fix/bar"}}
	]`
	const samlBody = `{"message":"Resource protected by organization SAML enforcement","documentation_url":"https://docs.github.com/articles/authenticating-to-a-github-organization-with-saml-single-sign-on"}`
	const badCredsBody = `{"message":"Bad credentials","documentation_url":"https://docs.github.com/rest"}`
	const notFoundBody = `{"message":"Not Found","documentation_url":"https://docs.github.com/rest"}`

	tests := []struct {
		name         string
		token        string
		status       int
		body         string
		wantCount    int
		wantErrCode  forge.ErrCode
		wantErr      bool
		wantDetail   string
		wantAuthHdr  string
		checkResults func(t *testing.T, prs []*forge.PullRequest)
	}{
		{
			name:      "success with multiple PRs",
			token:     "ghp_secret",
			status:    http.StatusOK,
			body:      successBody,
			wantCount: 2,
			wantAuthHdr: "Bearer ghp_secret",
			checkResults: func(t *testing.T, prs []*forge.PullRequest) {
				if prs[0].Number != 42 || prs[0].Title != "Add foo" || prs[0].Author != "alice" ||
					prs[0].URL != "https://github.com/octocat/hello-world/pull/42" ||
					prs[0].HeadRef != "feature/foo" {
					t.Errorf("PR[0] = %+v", prs[0])
				}
				if prs[1].Number != 43 || prs[1].Author != "bob" || prs[1].HeadRef != "fix/bar" {
					t.Errorf("PR[1] = %+v", prs[1])
				}
			},
		},
		{
			name:        "empty response",
			token:       "ghp_secret",
			status:      http.StatusOK,
			body:        `[]`,
			wantCount:   0,
			wantAuthHdr: "Bearer ghp_secret",
		},
		{
			name:        "401 preserves body in detail",
			token:       "bad_token",
			status:      http.StatusUnauthorized,
			body:        badCredsBody,
			wantErr:     true,
			wantErrCode: forge.ErrCodeAuthFailed,
			wantDetail:  "Bad credentials",
			wantAuthHdr: "Bearer bad_token",
		},
		{
			name:        "403 SAML SSO preserves body in detail",
			token:       "ghp_secret",
			status:      http.StatusForbidden,
			body:        samlBody,
			wantErr:     true,
			wantErrCode: forge.ErrCodeAuthFailed,
			wantDetail:  "SAML enforcement",
			wantAuthHdr: "Bearer ghp_secret",
		},
		{
			name:        "404 maps to ErrCodeNotFound",
			token:       "",
			status:      http.StatusNotFound,
			body:        notFoundBody,
			wantErr:     true,
			wantErrCode: forge.ErrCodeNotFound,
			wantAuthHdr: "",
		},
		{
			name:        "empty token sends no Authorization header",
			token:       "",
			status:      http.StatusOK,
			body:        `[]`,
			wantAuthHdr: "",
		},
		{
			name:        "non-empty token sends Bearer Authorization header",
			token:       "ghp_xyz",
			status:      http.StatusOK,
			body:        `[]`,
			wantAuthHdr: "Bearer ghp_xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAuth string
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				gotPath = r.URL.RequestURI()
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			h := &githubHandler{apiBaseURL: srv.URL, httpClient: srv.Client()}
			prs, err := h.ListPullRequests(context.Background(), "octocat", "hello-world", tt.token)

			if gotAuth != tt.wantAuthHdr {
				t.Errorf("Authorization header = %q, want %q", gotAuth, tt.wantAuthHdr)
			}
			if !strings.HasPrefix(gotPath, "/repos/octocat/hello-world/pulls") {
				t.Errorf("path = %q, want /repos/octocat/hello-world/pulls...", gotPath)
			}
			if !strings.Contains(gotPath, "state=open") {
				t.Errorf("path = %q, want state=open in query", gotPath)
			}

			if tt.wantErr {
				if err == nil {
					t.Fatal("got nil error, want error")
				}
				var fe *forge.Error
				if !errors.As(err, &fe) {
					t.Fatalf("error is not *forge.Error: %T %v", err, err)
				}
				if fe.Code != tt.wantErrCode {
					t.Errorf("Code = %v, want %v", fe.Code, tt.wantErrCode)
				}
				if tt.wantDetail != "" && !strings.Contains(fe.Detail, tt.wantDetail) {
					t.Errorf("Detail = %q, want substring %q", fe.Detail, tt.wantDetail)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(prs) != tt.wantCount {
				t.Fatalf("got %d PRs, want %d", len(prs), tt.wantCount)
			}
			if tt.checkResults != nil {
				tt.checkResults(t, prs)
			}
		})
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
