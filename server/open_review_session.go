package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/forge"
	_ "github.com/yourorg/codewalker/internal/forge/forges"
	_ "github.com/yourorg/codewalker/internal/forge/orderers"
	"github.com/yourorg/codewalker/internal/graph"
	"github.com/yourorg/codewalker/internal/session"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OpenReviewSession implements CodeWalkerServer.OpenReviewSession.
func (s *Server) OpenReviewSession(req *v1.OpenReviewSessionRequest, stream v1.CodeWalker_OpenReviewSessionServer) error {
	ctx := stream.Context()

	if req.Url == "" {
		return status.Error(codes.InvalidArgument, "url is required")
	}

	// --- Step 1: parse URL ---
	if err := send(stream, progress("parsing URL", 5)); err != nil {
		return err
	}

	host, err := extractURLHost(req.Url)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid URL: %v", err)
	}
	host = forge.NormalizeHost(host)

	handler, err := forge.Resolve(host)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "unsupported forge host %q: %v", host, err)
	}

	fc, err := handler.ParseURL(req.Url)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "malformed URL: %v", err)
	}
	slog.Debug("URL parsed", "forge", fc.Forge, "owner", fc.Owner, "repo", fc.Repo)

	// Token is the caller's responsibility. Empty means unauthenticated; the
	// forge will return PermissionDenied for private resources.
	token := req.ForgeToken

	// --- Step 2: fetch diff ---
	if err := send(stream, progress("fetching diff", 20)); err != nil {
		return err
	}

	payload, err := handler.FetchReview(ctx, fc, token)
	if err != nil {
		return forgeErrToStatus(err, "fetch diff")
	}
	slog.Debug("diff fetched", "file_count", len(payload.Files))

	orderer, err := forge.ResolveOrderer(req.FileOrdering)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "%v", err)
	}
	payload.Files = orderer.Order(payload.Files)

	// Fetch file contents for hunk context (best-effort; errors are non-fatal).
	fileLines := make(map[string][]string)
	if fc.HeadRef != "" {
		for _, f := range payload.Files {
			if f.ChangeKind == "DELETED" {
				continue
			}
			data, fetchErr := handler.FetchFile(ctx, fc, f.Path, fc.HeadRef, token)
			if fetchErr != nil {
				slog.Debug("skipping context fetch", "file", f.Path, "err", fetchErr)
				continue
			}
			fileLines[f.Path] = strings.Split(string(data), "\n")
		}
	}

	// --- Step 3: build step graph ---
	if err := send(stream, progress("building step graph", 50)); err != nil {
		return err
	}

	g, fileEntryStepIDs, orderedStepIDs := buildHunkGraph(payload.Files, fileLines, req.OmitRawSource)
	slog.Debug("hunk graph built", "step_count", g.Len(), "entry_step_id", g.EntryID)

	// --- Step 4: create session ---
	// Glossary extraction was removed: the only client that consumed
	// ReviewReady.glossary never read it, and the LLM call added several
	// seconds of latency to session open. The proto field is retained for
	// backward compatibility but is now always empty.
	if err := send(stream, progress("creating session", 90)); err != nil {
		return err
	}

	effectiveLevel := session.EffectiveLevel(req.ExperienceLevel)

	sessID := newSessionID()
	sess := session.New(sessID, g, effectiveLevel, "diff", req.OmitRawSource, nil, "", "", "")
	sess.Kind = v1.SessionKind_SESSION_KIND_REVIEW

	s.store.Set(sess)

	// --- Step 5: emit ReviewReady ---
	forgeCtxProto := toProtoForgeContext(fc, payload.Files, fileEntryStepIDs)
	steps := protoReviewSteps(g, orderedStepIDs)

	return send(stream, &v1.SessionEvent{
		Event: &v1.SessionEvent_ReviewReady{
			ReviewReady: &v1.ReviewReady{
				SessionId:      sessID,
				ForgeContext:   forgeCtxProto,
				Steps:          steps,
				TotalSteps:     uint32(g.Len()),
				EntryStepId:    g.EntryID,
				EffectiveLevel: uint32(effectiveLevel),
			},
		},
	})
}

const hunkContextLines = 5

// buildHunkGraph builds the review-session step graph and returns it alongside
// per-file entry step IDs and an ordered slice of step IDs in Forward
// traversal order. The Forward order matches the file orderer's output, then
// hunk parser order within each file — the same iteration order that wires
// the NEXT edges, so callers can use it to emit ReviewReady.steps
// deterministically.
func buildHunkGraph(files []*forge.ReviewFile, fileLines map[string][]string, omitRaw bool) (*graph.Graph, map[string]string, []string) {
	g := graph.NewGraph()
	fileEntryStepIDs := make(map[string]string)
	var orderedStepIDs []string

	var prevStep *graph.Step

	for _, file := range files {
		lines := fileLines[file.Path]
		for _, hunk := range file.Hunks {
			id := fmt.Sprintf("hunk:%s:%d", file.Path, hunk.NewStart)

			hunkSpan := &v1.HunkSpan{
				FilePath: file.Path,
				OldStart: uint32(hunk.OldStart),
				OldLines: uint32(hunk.OldLines),
				NewStart: uint32(hunk.NewStart),
				NewLines: uint32(hunk.NewLines),
			}
			if !omitRaw {
				hunkSpan.RawDiff = hunk.RawDiff
			}
			if len(lines) > 0 {
				hunkSpan.ContextBefore = extractContextLines(lines, hunk.NewStart-hunkContextLines-1, hunk.NewStart-1)
				hunkSpan.ContextAfter = extractContextLines(lines, hunk.NewStart+hunk.NewLines-1, hunk.NewStart+hunk.NewLines+hunkContextLines-1)
			}

			step := &graph.Step{
				ID:       id,
				Label:    fmt.Sprintf("%s:%d", file.Path, hunk.NewStart),
				Kind:     v1.StepKind_STEP_KIND_HUNK,
				HunkSpan: hunkSpan,
			}
			g.Add(step)
			orderedStepIDs = append(orderedStepIDs, id)

			if _, ok := fileEntryStepIDs[file.Path]; !ok {
				fileEntryStepIDs[file.Path] = id
			}
			if g.EntryID == "" {
				g.EntryID = id
			}

			if prevStep != nil {
				prevStep.Edges = append(prevStep.Edges, &v1.StepEdge{
					TargetStepId: id,
					Label:        v1.EdgeLabel_EDGE_LABEL_NEXT,
					Navigable:    true,
				})
			}
			prevStep = step
		}
	}

	return g, fileEntryStepIDs, orderedStepIDs
}

func extractContextLines(lines []string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start >= end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

// protoReviewSteps emits steps in Forward traversal order using the
// orderedIDs slice produced by buildHunkGraph. This guarantees that
// ReviewReady.steps matches the order Navigate(Forward) follows from the
// entry step.
func protoReviewSteps(g *graph.Graph, orderedIDs []string) []*v1.Step {
	out := make([]*v1.Step, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		s, ok := g.Step(id)
		if !ok {
			continue
		}
		out = append(out, &v1.Step{
			Id:       s.ID,
			Label:    s.Label,
			HunkSpan: s.HunkSpan,
			Edges:    s.Edges,
			Visited:  s.Visited,
			Kind:     s.Kind,
		})
	}
	return out
}

func toProtoForgeContext(fc *forge.ForgeContext, files []*forge.ReviewFile, fileEntryStepIDs map[string]string) *v1.ForgeContext {
	protoFiles := make([]*v1.ReviewFile, 0, len(files))
	for _, f := range files {
		protoFiles = append(protoFiles, &v1.ReviewFile{
			FilePath:     f.Path,
			Language:     f.Language,
			Change:       changeKindToProto(f.ChangeKind),
			HunksTotal:   uint32(len(f.Hunks)),
			LinesAdded:   uint32(f.LinesAdded),
			LinesRemoved: uint32(f.LinesRemoved),
			EntryStepId:  fileEntryStepIDs[f.Path],
		})
	}

	return &v1.ForgeContext{
		Kind:          forgeContextKindToProto(fc.Kind),
		Forge:         fc.Forge,
		Owner:         fc.Owner,
		Repo:          fc.Repo,
		BaseRef:       fc.BaseRef,
		HeadRef:       fc.HeadRef,
		PrNumber:      int32(fc.PRNumber),
		PrTitle:       fc.PRTitle,
		PrDescription: fc.PRDescription,
		PrAuthor:      fc.PRAuthor,
		Url:           fc.URL,
		Files:         protoFiles,
	}
}

func forgeContextKindToProto(k forge.ForgeContextKind) v1.ForgeContextKind {
	switch k {
	case forge.ForgeContextKindPR:
		return v1.ForgeContextKind_FORGE_CONTEXT_KIND_PR
	case forge.ForgeContextKindCommit:
		return v1.ForgeContextKind_FORGE_CONTEXT_KIND_COMMIT
	case forge.ForgeContextKindComparison:
		return v1.ForgeContextKind_FORGE_CONTEXT_KIND_COMPARISON
	default:
		return v1.ForgeContextKind_FORGE_CONTEXT_KIND_UNSPECIFIED
	}
}

func changeKindToProto(s string) v1.ChangeKind {
	switch s {
	case "ADDED":
		return v1.ChangeKind_CHANGE_KIND_ADDED
	case "DELETED":
		return v1.ChangeKind_CHANGE_KIND_DELETED
	case "RENAMED":
		return v1.ChangeKind_CHANGE_KIND_RENAMED
	default:
		return v1.ChangeKind_CHANGE_KIND_MODIFIED
	}
}

func forgeErrToStatus(err error, op string) error {
	var fe *forge.Error
	if errors.As(err, &fe) {
		switch fe.Code {
		case forge.ErrCodeAuthFailed:
			if fe.Detail != "" {
				return status.Errorf(codes.PermissionDenied, "%s: %v: %s", op, err, fe.Detail)
			}
			return status.Errorf(codes.PermissionDenied, "%s: %v", op, err)
		case forge.ErrCodeNotFound:
			return status.Errorf(codes.NotFound, "%s: %v", op, err)
		}
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}

func extractURLHost(rawURL string) (string, error) {
	s := rawURL
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("no host in URL %q", rawURL)
	}
	return u.Host, nil
}
