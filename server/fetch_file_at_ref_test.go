package server_test

import (
	"context"
	"strings"
	"testing"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/forge"
	"github.com/yourorg/codewalker/internal/session"
	"github.com/yourorg/codewalker/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fetchMockForgeHandler is a configurable forge handler used to drive
// FetchFileAtRef tests. Behaviour is set per-test via package-level vars.
type fetchMockForgeHandler struct{}

var (
	fetchFileContent  []byte
	fetchFileErr      error
	fetchFileGotPath  string
	fetchFileGotRef   string
	fetchFileGotToken string
	fetchFileGotOwner string
	fetchFileGotRepo  string
)

func (f *fetchMockForgeHandler) Hosts() []string { return []string{"fetch.example.com"} }

func (f *fetchMockForgeHandler) ParseURL(string) (*forge.ForgeContext, error) {
	return nil, nil
}

func (f *fetchMockForgeHandler) FetchReview(context.Context, *forge.ForgeContext, string) (*forge.ReviewPayload, error) {
	return nil, nil
}

func (f *fetchMockForgeHandler) FetchFile(_ context.Context, fc *forge.ForgeContext, path, ref, token string) ([]byte, error) {
	fetchFileGotPath = path
	fetchFileGotRef = ref
	fetchFileGotToken = token
	if fc != nil {
		fetchFileGotOwner = fc.Owner
		fetchFileGotRepo = fc.Repo
	}
	if fetchFileErr != nil {
		return nil, fetchFileErr
	}
	return fetchFileContent, nil
}

func (f *fetchMockForgeHandler) ListPullRequests(context.Context, string, string, string) ([]*forge.PullRequest, error) {
	return nil, nil
}

func init() {
	forge.Register(&fetchMockForgeHandler{})
}

func resetFetchMock() {
	fetchFileContent = nil
	fetchFileErr = nil
	fetchFileGotPath = ""
	fetchFileGotRef = ""
	fetchFileGotToken = ""
	fetchFileGotOwner = ""
	fetchFileGotRepo = ""
}

func TestFetchFileAtRef_Success(t *testing.T) {
	resetFetchMock()
	fetchFileContent = []byte("package main\n")

	srv := server.New(session.NewStore(), &mockProvider{}, "")
	resp, err := srv.FetchFileAtRef(context.Background(), &v1.FetchFileAtRefRequest{
		Host:       "fetch.example.com",
		Owner:      "owner",
		Repo:       "repo",
		Path:       "main.go",
		Ref:        "abc123",
		ForgeToken: "tok",
	})
	if err != nil {
		t.Fatalf("FetchFileAtRef: %v", err)
	}
	if string(resp.Content) != "package main\n" {
		t.Errorf("content = %q, want %q", resp.Content, "package main\n")
	}
	if fetchFileGotPath != "main.go" {
		t.Errorf("path passed to handler = %q, want %q", fetchFileGotPath, "main.go")
	}
	if fetchFileGotRef != "abc123" {
		t.Errorf("ref passed to handler = %q, want %q", fetchFileGotRef, "abc123")
	}
	if fetchFileGotToken != "tok" {
		t.Errorf("token passed to handler = %q, want %q", fetchFileGotToken, "tok")
	}
	if fetchFileGotOwner != "owner" {
		t.Errorf("owner passed to handler = %q, want %q", fetchFileGotOwner, "owner")
	}
	if fetchFileGotRepo != "repo" {
		t.Errorf("repo passed to handler = %q, want %q", fetchFileGotRepo, "repo")
	}
}

func TestFetchFileAtRef_EmptyTokenPublicRepo(t *testing.T) {
	resetFetchMock()
	fetchFileContent = []byte("public content")

	srv := server.New(session.NewStore(), &mockProvider{}, "")
	resp, err := srv.FetchFileAtRef(context.Background(), &v1.FetchFileAtRefRequest{
		Host:  "fetch.example.com",
		Owner: "owner",
		Repo:  "repo",
		Path:  "README.md",
		Ref:   "main",
	})
	if err != nil {
		t.Fatalf("FetchFileAtRef: %v", err)
	}
	if string(resp.Content) != "public content" {
		t.Errorf("content = %q, want %q", resp.Content, "public content")
	}
	if fetchFileGotToken != "" {
		t.Errorf("expected empty token forwarded, got %q", fetchFileGotToken)
	}
}

func TestFetchFileAtRef_HostNormalised(t *testing.T) {
	resetFetchMock()
	fetchFileContent = []byte("ok")

	srv := server.New(session.NewStore(), &mockProvider{}, "")
	resp, err := srv.FetchFileAtRef(context.Background(), &v1.FetchFileAtRefRequest{
		Host:  "Fetch.Example.COM/",
		Owner: "owner",
		Repo:  "repo",
		Path:  "main.go",
		Ref:   "main",
	})
	if err != nil {
		t.Fatalf("FetchFileAtRef: %v", err)
	}
	if string(resp.Content) != "ok" {
		t.Errorf("content = %q, want %q", resp.Content, "ok")
	}
}

func TestFetchFileAtRef_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		req  *v1.FetchFileAtRefRequest
	}{
		{
			name: "missing host",
			req:  &v1.FetchFileAtRefRequest{Owner: "o", Repo: "r", Path: "p", Ref: "ref"},
		},
		{
			name: "missing owner",
			req:  &v1.FetchFileAtRefRequest{Host: "fetch.example.com", Repo: "r", Path: "p", Ref: "ref"},
		},
		{
			name: "missing repo",
			req:  &v1.FetchFileAtRefRequest{Host: "fetch.example.com", Owner: "o", Path: "p", Ref: "ref"},
		},
		{
			name: "missing path",
			req:  &v1.FetchFileAtRefRequest{Host: "fetch.example.com", Owner: "o", Repo: "r", Ref: "ref"},
		},
		{
			name: "missing ref",
			req:  &v1.FetchFileAtRefRequest{Host: "fetch.example.com", Owner: "o", Repo: "r", Path: "p"},
		},
	}
	srv := server.New(session.NewStore(), &mockProvider{}, "")
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetFetchMock()
			_, err := srv.FetchFileAtRef(context.Background(), tc.req)
			if err == nil {
				t.Fatal("expected error")
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("not a gRPC status: %v", err)
			}
			if st.Code() != codes.InvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", st.Code())
			}
		})
	}
}

func TestFetchFileAtRef_UnsupportedHost(t *testing.T) {
	resetFetchMock()
	srv := server.New(session.NewStore(), &mockProvider{}, "")
	_, err := srv.FetchFileAtRef(context.Background(), &v1.FetchFileAtRefRequest{
		Host:  "unsupported.example.org",
		Owner: "o",
		Repo:  "r",
		Path:  "p",
		Ref:   "ref",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("not a gRPC status: %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", st.Code())
	}
}

func TestFetchFileAtRef_AuthFailedPreservesDetail(t *testing.T) {
	resetFetchMock()
	fetchFileErr = &forge.Error{
		Code:   forge.ErrCodeAuthFailed,
		Msg:    "GitHub API auth failed (401 Unauthorized)",
		Detail: `{"message":"Bad credentials"}`,
	}

	srv := server.New(session.NewStore(), &mockProvider{}, "")
	_, err := srv.FetchFileAtRef(context.Background(), &v1.FetchFileAtRefRequest{
		Host:       "fetch.example.com",
		Owner:      "owner",
		Repo:       "repo",
		Path:       "main.go",
		Ref:        "main",
		ForgeToken: "bad",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("not a gRPC status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Errorf("code = %v, want PermissionDenied", st.Code())
	}
	if !strings.Contains(st.Message(), "Bad credentials") {
		t.Errorf("status message missing detail: %q", st.Message())
	}
}

func TestFetchFileAtRef_ForbiddenPreservesDetail(t *testing.T) {
	resetFetchMock()
	fetchFileErr = &forge.Error{
		Code:   forge.ErrCodeAuthFailed,
		Msg:    "GitHub API auth failed (403 Forbidden)",
		Detail: `{"message":"Resource protected by organization SAML enforcement"}`,
	}

	srv := server.New(session.NewStore(), &mockProvider{}, "")
	_, err := srv.FetchFileAtRef(context.Background(), &v1.FetchFileAtRefRequest{
		Host:       "fetch.example.com",
		Owner:      "owner",
		Repo:       "repo",
		Path:       "main.go",
		Ref:        "main",
		ForgeToken: "tok",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("not a gRPC status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Errorf("code = %v, want PermissionDenied", st.Code())
	}
	if !strings.Contains(st.Message(), "SAML") {
		t.Errorf("status message missing detail: %q", st.Message())
	}
}

func TestFetchFileAtRef_NotFound(t *testing.T) {
	resetFetchMock()
	fetchFileErr = &forge.Error{
		Code: forge.ErrCodeNotFound,
		Msg:  "GitHub API not found (404 Not Found)",
	}

	srv := server.New(session.NewStore(), &mockProvider{}, "")
	_, err := srv.FetchFileAtRef(context.Background(), &v1.FetchFileAtRefRequest{
		Host:  "fetch.example.com",
		Owner: "owner",
		Repo:  "repo",
		Path:  "missing.go",
		Ref:   "main",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("not a gRPC status: %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("code = %v, want NotFound", st.Code())
	}
}
