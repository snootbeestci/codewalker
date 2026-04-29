package forges

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/yourorg/codewalker/internal/forge"
)

func init() {
	forge.Register(&githubHandler{})
}

type githubHandler struct {
	// apiBaseURL overrides https://api.github.com in tests.
	// The empty string means use the default.
	apiBaseURL string
	// httpClient overrides http.DefaultClient in tests.
	// The nil value means use the default.
	httpClient *http.Client
}

func (g *githubHandler) baseURL() string {
	if g.apiBaseURL != "" {
		return g.apiBaseURL
	}
	return "https://api.github.com"
}

func (g *githubHandler) client() *http.Client {
	if g.httpClient != nil {
		return g.httpClient
	}
	return http.DefaultClient
}

func (g *githubHandler) Hosts() []string {
	return []string{"github.com"}
}

func (g *githubHandler) ParseURL(rawURL string) (*forge.ForgeContext, error) {
	s := rawURL
	for _, prefix := range []string{"https://", "http://"} {
		s = strings.TrimPrefix(s, prefix)
	}
	s = strings.TrimRight(s, "/")

	// Expect at least: github.com/{owner}/{repo}/{kind}/{ref}
	parts := strings.SplitN(s, "/", 6)
	if len(parts) < 5 || parts[0] != "github.com" {
		return nil, fmt.Errorf("unrecognised GitHub URL %q: expected github.com/{owner}/{repo}/{pull|commit|compare}/...", rawURL)
	}

	owner, repo, kind, ref := parts[1], parts[2], parts[3], parts[4]

	fc := &forge.ForgeContext{
		Forge: "github",
		Owner: owner,
		Repo:  repo,
		URL:   rawURL,
	}

	switch kind {
	case "pull":
		n, err := strconv.Atoi(ref)
		if err != nil {
			return nil, fmt.Errorf("invalid PR number in URL %q: %w", rawURL, err)
		}
		fc.Kind = forge.ForgeContextKindPR
		fc.PRNumber = n
	case "commit":
		fc.Kind = forge.ForgeContextKindCommit
		fc.HeadRef = ref
	case "compare":
		idx := strings.Index(ref, "...")
		if idx < 0 {
			return nil, fmt.Errorf("invalid comparison ref in URL %q: expected base...head", rawURL)
		}
		fc.Kind = forge.ForgeContextKindComparison
		fc.BaseRef = ref[:idx]
		fc.HeadRef = ref[idx+3:]
	default:
		return nil, fmt.Errorf("unrecognised GitHub URL pattern %q: path segment %q is not pull, commit, or compare", rawURL, kind)
	}

	return fc, nil
}

func (g *githubHandler) FetchReview(ctx context.Context, fc *forge.ForgeContext, token string) (*forge.ReviewPayload, error) {
	switch fc.Kind {
	case forge.ForgeContextKindPR:
		return g.fetchPRReview(ctx, fc, token)
	case forge.ForgeContextKindCommit:
		return g.fetchCommitReview(ctx, fc, token)
	case forge.ForgeContextKindComparison:
		return g.fetchComparisonReview(ctx, fc, token)
	default:
		return nil, fmt.Errorf("unsupported forge context kind: %v", fc.Kind)
	}
}

func (g *githubHandler) FetchFile(ctx context.Context, fc *forge.ForgeContext, path, ref, token string) ([]byte, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		g.baseURL(), fc.Owner, fc.Repo, path, url.QueryEscape(ref))
	resp, err := g.apiDo(ctx, apiURL, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}

	var content struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return nil, fmt.Errorf("decode file content response: %w", err)
	}
	if content.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding %q for file %q", content.Encoding, path)
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("decode base64 content for %q: %w", path, err)
	}
	return data, nil
}

func (g *githubHandler) ListPullRequests(ctx context.Context, owner, repo, token string) ([]*forge.PullRequest, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&per_page=100", g.baseURL(), owner, repo)
	// TODO: paginate — repositories with more than 100 open PRs will be silently truncated.
	// GitHub returns a Link header with rel="next" for subsequent pages.
	resp, err := g.apiDo(ctx, apiURL, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}

	var items []struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		User    struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decode pull request list response: %w", err)
	}

	out := make([]*forge.PullRequest, 0, len(items))
	for _, it := range items {
		out = append(out, &forge.PullRequest{
			Number: it.Number,
			Title:  it.Title,
			Author: it.User.Login,
			URL:    it.HTMLURL,
		})
	}
	return out, nil
}

// --- GitHub API types ---

type ghPR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	Base struct{ Ref string `json:"ref"` } `json:"base"`
	Head struct{ Ref string `json:"ref"` } `json:"head"`
}

type ghFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Patch            string `json:"patch"`
}

type ghCommit struct {
	SHA    string `json:"sha"`
	Files  []ghFile `json:"files"`
	Parents []struct {
		SHA string `json:"sha"`
	} `json:"parents"`
}

type ghComparison struct {
	Files []ghFile `json:"files"`
}

// --- internal helpers ---

func (g *githubHandler) apiDo(ctx context.Context, apiURL, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return g.client().Do(req)
}

// errBodyMaxBytes caps how much of a forge response body we propagate into the
// gRPC status detail. GitHub Enterprise SSO error bodies are short prose;
// 500 bytes is enough to convey the cause without leaking large payloads.
const errBodyMaxBytes = 500

func checkStatus(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &forge.Error{
			Code:   forge.ErrCodeAuthFailed,
			Msg:    fmt.Sprintf("GitHub API auth failed (%s)", resp.Status),
			Detail: readBodySnippet(resp.Body),
		}
	case http.StatusNotFound:
		return &forge.Error{
			Code: forge.ErrCodeNotFound,
			Msg:  fmt.Sprintf("GitHub API not found (%s)", resp.Status),
		}
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API error: %s", resp.Status)
	}
	return nil
}

// readBodySnippet reads up to errBodyMaxBytes from r and returns the result
// as a trimmed string. Errors are swallowed — a missing body is acceptable.
//
// This function does NOT close r. The caller of apiDo owns the response
// body and must close it via defer resp.Body.Close() at the call site.
// Closing here would be redundant at best and a double-close at worst.
func readBodySnippet(r io.Reader) string {
	buf, err := io.ReadAll(io.LimitReader(r, errBodyMaxBytes))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(buf))
}

func (g *githubHandler) fetchPRReview(ctx context.Context, fc *forge.ForgeContext, token string) (*forge.ReviewPayload, error) {
	prURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", g.baseURL(), fc.Owner, fc.Repo, fc.PRNumber)

	resp, err := g.apiDo(ctx, prURL, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	var pr ghPR
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode PR response: %w", err)
	}

	fc.PRTitle = pr.Title
	fc.PRDescription = pr.Body
	fc.PRAuthor = pr.User.Login
	fc.BaseRef = pr.Base.Ref
	fc.HeadRef = pr.Head.Ref

	filesURL := fmt.Sprintf("%s/files?per_page=100", prURL)
	// TODO: paginate — PRs with more than 100 changed files will be silently truncated.
	// GitHub returns a Link header with rel="next" for subsequent pages.
	resp2, err := g.apiDo(ctx, filesURL, token)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()
	if err := checkStatus(resp2); err != nil {
		return nil, err
	}
	var files []ghFile
	if err := json.NewDecoder(resp2.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("decode PR files response: %w", err)
	}

	reviewFiles, err := ghFilesToReviewFiles(files)
	if err != nil {
		return nil, err
	}
	return &forge.ReviewPayload{Context: fc, Files: reviewFiles}, nil
}

func (g *githubHandler) fetchCommitReview(ctx context.Context, fc *forge.ForgeContext, token string) (*forge.ReviewPayload, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/commits/%s", g.baseURL(), fc.Owner, fc.Repo, fc.HeadRef)
	resp, err := g.apiDo(ctx, apiURL, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	var commit ghCommit
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return nil, fmt.Errorf("decode commit response: %w", err)
	}

	fc.HeadRef = commit.SHA
	if len(commit.Parents) > 0 {
		fc.BaseRef = commit.Parents[0].SHA
	}

	reviewFiles, err := ghFilesToReviewFiles(commit.Files)
	if err != nil {
		return nil, err
	}
	return &forge.ReviewPayload{Context: fc, Files: reviewFiles}, nil
}

func (g *githubHandler) fetchComparisonReview(ctx context.Context, fc *forge.ForgeContext, token string) (*forge.ReviewPayload, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s",
		g.baseURL(), fc.Owner, fc.Repo, fc.BaseRef, fc.HeadRef)
	resp, err := g.apiDo(ctx, apiURL, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	var cmp ghComparison
	if err := json.NewDecoder(resp.Body).Decode(&cmp); err != nil {
		return nil, fmt.Errorf("decode comparison response: %w", err)
	}

	reviewFiles, err := ghFilesToReviewFiles(cmp.Files)
	if err != nil {
		return nil, err
	}
	return &forge.ReviewPayload{Context: fc, Files: reviewFiles}, nil
}

func ghFilesToReviewFiles(files []ghFile) ([]*forge.ReviewFile, error) {
	result := make([]*forge.ReviewFile, 0, len(files))
	for _, f := range files {
		rf := &forge.ReviewFile{
			Path:         f.Filename,
			OldPath:      f.PreviousFilename,
			ChangeKind:   ghStatusToChangeKind(f.Status),
			LinesAdded:   f.Additions,
			LinesRemoved: f.Deletions,
		}
		if f.Patch != "" {
			hunks, err := parseHunks(f.Patch)
			if err != nil {
				return nil, fmt.Errorf("parse hunks for %q: %w", f.Filename, err)
			}
			rf.Hunks = hunks
		}
		result = append(result, rf)
	}
	return result, nil
}

func ghStatusToChangeKind(status string) string {
	switch status {
	case "added":
		return "ADDED"
	case "removed":
		return "DELETED"
	case "renamed":
		return "RENAMED"
	default:
		return "MODIFIED"
	}
}

// parseHunks parses a unified diff patch string into a slice of Hunks.
func parseHunks(patch string) ([]*forge.Hunk, error) {
	var hunks []*forge.Hunk
	var current *forge.Hunk

	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "@@") {
			if current != nil {
				hunks = append(hunks, current)
			}
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			h.RawDiff = line + "\n"
			current = h
		} else if current != nil {
			current.RawDiff += line + "\n"
		}
	}
	if current != nil {
		hunks = append(hunks, current)
	}
	return hunks, nil
}

// parseHunkHeader parses a unified diff hunk header line of the form:
// @@ -oldStart[,oldLines] +newStart[,newLines] @@[ section heading]
func parseHunkHeader(line string) (*forge.Hunk, error) {
	if !strings.HasPrefix(line, "@@ ") {
		return nil, fmt.Errorf("malformed hunk header: %q", line)
	}
	// Find the closing " @@"
	rest := line[3:]
	end := strings.Index(rest, " @@")
	if end < 0 {
		return nil, fmt.Errorf("malformed hunk header (no closing @@): %q", line)
	}
	rangeStr := rest[:end] // e.g. "-1,7 +1,6"

	fields := strings.Fields(rangeStr)
	if len(fields) < 2 {
		return nil, fmt.Errorf("malformed hunk header ranges: %q", line)
	}

	oldStart, oldLines, err := parseHunkRange(fields[0])
	if err != nil {
		return nil, fmt.Errorf("bad old range in %q: %w", line, err)
	}
	newStart, newLines, err := parseHunkRange(fields[1])
	if err != nil {
		return nil, fmt.Errorf("bad new range in %q: %w", line, err)
	}

	return &forge.Hunk{
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
	}, nil
}

// parseHunkRange parses "-3,5" or "+3,5" or "-3" or "+3" into (start, count).
// When the count is omitted it defaults to 1.
func parseHunkRange(s string) (start, count int, err error) {
	s = s[1:] // strip leading - or +
	if idx := strings.IndexByte(s, ','); idx >= 0 {
		start, err = strconv.Atoi(s[:idx])
		if err != nil {
			return
		}
		count, err = strconv.Atoi(s[idx+1:])
		return
	}
	start, err = strconv.Atoi(s)
	count = 1
	return
}
