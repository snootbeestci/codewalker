package session

import (
	"sync"
	"time"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/graph"
)

// Session holds all server-side state for one client walkthrough.
type Session struct {
	mu sync.Mutex

	ID            string
	Graph         *graph.Graph
	Walker        *graph.Walker
	EffLevel      int // 1–10, from adaptation.go
	Language      string
	Kind          v1.SessionKind
	Glossary      map[string]*v1.GlossaryTerm
	OmitRawSource bool
	Source        []byte
	FilePath      string
	RepoPath      string
	Ref           string
	LastAccessed  time.Time
}

// New creates a Session from an already-built graph.
func New(id string, g *graph.Graph, effectiveLevel int, language string, omitRaw bool, src []byte, filePath, repoPath, ref string) *Session {
	return &Session{
		ID:            id,
		Graph:         g,
		Walker:        graph.NewWalker(g),
		EffLevel:      effectiveLevel,
		Language:      language,
		Glossary:      make(map[string]*v1.GlossaryTerm),
		OmitRawSource: omitRaw,
		Source:        src,
		FilePath:      filePath,
		RepoPath:      repoPath,
		Ref:           ref,
		LastAccessed:  time.Now(),
	}
}

// AddGlossaryTerm inserts or updates a glossary term.
func (s *Session) AddGlossaryTerm(t *v1.GlossaryTerm) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Glossary[t.Term] = t
}

// GetGlossaryTerm returns a term by name.
func (s *Session) GetGlossaryTerm(term string) (*v1.GlossaryTerm, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.Glossary[term]
	return t, ok
}

// AllGlossaryTerms returns a snapshot of all glossary terms.
func (s *Session) AllGlossaryTerms() []*v1.GlossaryTerm {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*v1.GlossaryTerm, 0, len(s.Glossary))
	for _, t := range s.Glossary {
		out = append(out, t)
	}
	return out
}

// Touch updates LastAccessed to now, resetting the eviction TTL clock.
func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastAccessed = time.Now()
}

// Lock locks the session mutex for callers that need atomic multi-step operations.
func (s *Session) Lock() { s.mu.Lock() }

// Unlock unlocks the session mutex.
func (s *Session) Unlock() { s.mu.Unlock() }

// Summary returns the SessionSummary proto for this session.
func (s *Session) Summary() *v1.SessionSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &v1.SessionSummary{
		SessionId:      s.ID,
		RepoPath:       s.RepoPath,
		FilePath:       s.FilePath,
		Ref:            s.Ref,
		Language:       s.Language,
		CurrentStepId:  s.Walker.CurrentID(),
		EffectiveLevel: uint32(s.EffLevel),
		Kind:           s.Kind,
	}
}
