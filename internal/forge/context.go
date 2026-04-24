package forge

// ForgeContext is the forge-agnostic representation of a parsed URL.
// Produced by ForgeHandler.ParseURL and used throughout the review pipeline.
type ForgeContext struct {
	Kind          ForgeContextKind
	Forge         string // e.g. "github", "gitlab"
	Owner         string
	Repo          string
	BaseRef       string
	HeadRef       string
	PRNumber      int    // 0 if not a PR
	PRTitle       string
	PRDescription string
	PRAuthor      string
	URL           string // original URL
}

type ForgeContextKind int

const (
	ForgeContextKindUnspecified ForgeContextKind = iota
	ForgeContextKindPR
	ForgeContextKindCommit
	ForgeContextKindComparison
)

// ReviewPayload is the result of FetchReview — everything needed to build
// the review session step graph.
type ReviewPayload struct {
	Context *ForgeContext
	Files   []*ReviewFile
}

// ReviewFile represents one changed file in a review.
type ReviewFile struct {
	Path         string
	Language     string
	ChangeKind   string // "ADDED" | "MODIFIED" | "DELETED" | "RENAMED"
	OldPath      string // set for renames
	Hunks        []*Hunk
	LinesAdded   int
	LinesRemoved int
}

// Hunk is a single contiguous block of changes within a file.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	RawDiff  string
}
