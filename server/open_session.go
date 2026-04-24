package server

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	gogit "github.com/yourorg/codewalker/internal/git"
	"github.com/yourorg/codewalker/internal/graph"
	"github.com/yourorg/codewalker/internal/llm"
	"github.com/yourorg/codewalker/internal/parser"
	_ "github.com/yourorg/codewalker/internal/parser/languages" // register all handlers
	"github.com/yourorg/codewalker/internal/session"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OpenSession implements CodeWalkerServer.OpenSession.
func (s *Server) OpenSession(req *v1.OpenSessionRequest, stream v1.CodeWalker_OpenSessionServer) error {
	ctx := stream.Context()

	if req.FilePath == "" {
		return status.Error(codes.InvalidArgument, "file_path is required")
	}

	repoPath := req.RepoPath
	if repoPath == "" {
		repoPath = s.repoRoot
	}

	// --- Step 1: open repo and read file ---
	if err := send(stream, progress("opening repository", 5)); err != nil {
		return err
	}

	gitClient, err := gogit.Open(repoPath)
	if err != nil {
		return status.Errorf(codes.NotFound, "cannot open repo at %q: %v", repoPath, err)
	}

	src, err := gitClient.ReadFile(req.Ref, req.FilePath)
	if err != nil {
		return status.Errorf(codes.NotFound, "cannot read %q at ref %q: %v", req.FilePath, req.Ref, err)
	}
	slog.Debug("file read", "file_path", req.FilePath, "ref", req.Ref, "src_len", len(src))

	// --- Step 2: parse AST ---
	if err := send(stream, progress("parsing AST", 20)); err != nil {
		return err
	}

	nodes, language, err := parser.Parse(src, req.FilePath)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "parse error: %v", err)
	}
	slog.Debug("ast parsed", "language", language, "node_count", len(nodes))

	// --- Step 3: build step graph ---
	if err := send(stream, progress("building step graph", 40)); err != nil {
		return err
	}

	g, err := graph.Build(nodes, src, req.FilePath, req.OmitRawSource, req.Symbol)
	if err != nil {
		return status.Errorf(codes.Internal, "graph build error: %v", err)
	}
	slog.Debug("graph built", "step_count", g.Len(), "entry_step_id", g.EntryID)

	// --- Step 4: extract glossary candidates ---
	if err := send(stream, progress("extracting glossary", 65)); err != nil {
		return err
	}

	effectiveLevel := session.EffectiveLevel(req.ExperienceLevel)
	glossaryCandidates, _ := s.provider.ExtractGlossaryTerms(ctx, llm.GlossaryRequest{
		Code:     string(src),
		Language: language,
		Level:    effectiveLevel,
	})
	slog.Debug("glossary extracted", "term_count", len(glossaryCandidates))

	// --- Step 5: create session ---
	if err := send(stream, progress("creating session", 85)); err != nil {
		return err
	}

	sessID := newSessionID()
	sess := session.New(sessID, g, effectiveLevel, language, req.OmitRawSource, src, req.FilePath, repoPath, req.Ref)

	// Build proto glossary terms.
	glossaryProto := make([]*v1.GlossaryTerm, 0, len(glossaryCandidates))
	for _, c := range glossaryCandidates {
		t := &v1.GlossaryTerm{
			Term:   c.Term,
			Kind:   termKindFromString(c.Kind),
			StepId: g.EntryID,
		}
		sess.AddGlossaryTerm(t)
		glossaryProto = append(glossaryProto, t)
	}

	s.store.Set(sess)

	// --- Step 6: send SessionReady ---
	steps := protoSteps(g)
	ready := &v1.SessionEvent{
		Event: &v1.SessionEvent_Ready{
			Ready: &v1.SessionReady{
				SessionId:   sessID,
				Steps:       steps,
				Glossary:    glossaryProto,
				Language:    language,
				TotalSteps:  uint32(g.Len()),
				EntryStepId: g.EntryID,
			},
		},
	}
	return send(stream, ready)
}

// --- helpers ---

func send(stream v1.CodeWalker_OpenSessionServer, evt *v1.SessionEvent) error {
	if err := stream.Context().Err(); err != nil {
		return status.FromContextError(err).Err()
	}
	return stream.Send(evt)
}

func progress(msg string, pct uint32) *v1.SessionEvent {
	return &v1.SessionEvent{
		Event: &v1.SessionEvent_Progress{
			Progress: &v1.SessionProgress{Message: msg, Percent: pct},
		},
	}
}

func protoSteps(g *graph.Graph) []*v1.Step {
	all := g.AllSteps()
	out := make([]*v1.Step, 0, len(all))
	for _, s := range all {
		out = append(out, &v1.Step{
			Id:      s.ID,
			Label:   s.Label,
			Span:    s.Source,
			Edges:   s.Edges,
			Visited: s.Visited,
			Kind:    s.Kind,
		})
	}
	return out
}

func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func termKindFromString(s string) v1.TermKind {
	switch s {
	case "LANGUAGE":
		return v1.TermKind_TERM_KIND_LANGUAGE
	case "PATTERN":
		return v1.TermKind_TERM_KIND_PATTERN
	case "DOMAIN":
		return v1.TermKind_TERM_KIND_DOMAIN
	case "LIBRARY":
		return v1.TermKind_TERM_KIND_LIBRARY
	default:
		return v1.TermKind_TERM_KIND_UNSPECIFIED
	}
}
