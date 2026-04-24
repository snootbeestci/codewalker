package forge

import "context"

// ForgeHandler handles all forge-specific operations for a given host.
// Implement this interface to add support for a new forge (GitLab, Gitea etc.).
// Register implementations via Register() in an init() function.
type ForgeHandler interface {
	// Hosts returns the hostname patterns this handler claims.
	// e.g. ["github.com"] or ["*.github.example.com"]
	Hosts() []string

	// ParseURL parses a forge URL into a forge-agnostic ForgeContext.
	// Returns an error if the URL is not recognised or malformed.
	ParseURL(rawURL string) (*ForgeContext, error)

	// FetchReview fetches the full diff and metadata for a PR, commit,
	// or branch comparison described by fc.
	// token may be empty for public repositories.
	FetchReview(ctx context.Context, fc *ForgeContext, token string) (*ReviewPayload, error)

	// FetchFile fetches raw file content at a specific ref.
	// Used to populate context_before and context_after on HunkSpan.
	FetchFile(ctx context.Context, fc *ForgeContext, path, ref, token string) ([]byte, error)

	// ResolveToken attempts to resolve a forge token from local credentials
	// without user interaction (e.g. `gh auth token` for GitHub).
	// Returns ("", nil) if no token is found — callers should then prompt the user.
	ResolveToken(ctx context.Context) (string, error)
}
