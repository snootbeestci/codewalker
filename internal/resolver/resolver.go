package resolver

import (
	"context"
	"fmt"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/git"
	"github.com/yourorg/codewalker/internal/llm"
	"github.com/yourorg/codewalker/internal/resolver/ecosystems"
)

// backend is the minimal interface each ecosystem resolver must satisfy.
type backend interface {
	Resolve(pkg, symbol string) (docsURL, sourceURL, version string)
}

// Resolver resolves external call references into ExternalCallInfo protos.
// Resolution cascade (per briefing):
//  1. Package manifest found → pinned docs + source URLs.
//  2. Language known, no manifest → unversioned docs URL.
//  3. Fallback → LLM summary only.
type Resolver struct {
	provider llm.Provider
	backend  backend
	language string
}

// New creates a Resolver for the given repository root and language.
func New(repoRoot, language string, provider llm.Provider) *Resolver {
	m := git.DetectManifest(repoRoot)
	b := selectBackend(language, m)
	return &Resolver{provider: provider, backend: b, language: language}
}

// Resolve returns a fully-populated ExternalCallInfo for pkg.Symbol.
// llm_summary is always set; URLs are best-effort.
func (r *Resolver) Resolve(ctx context.Context, pkg, symbol string) (*v1.ExternalCallInfo, error) {
	summary, err := r.provider.SummarizeExternalCall(ctx, pkg, symbol, r.language)
	if err != nil {
		summary = fmt.Sprintf("Could not generate summary: %v", err)
	}

	docsURL, sourceURL, version := r.backend.Resolve(pkg, symbol)

	info := &v1.ExternalCallInfo{
		PackageName: pkg,
		SymbolName:  symbol,
		LlmSummary:  summary,
		DocsUrl:     docsURL,
		SourceUrl:   sourceURL,
		Version:     version,
	}
	return info, nil
}

func selectBackend(language string, m *git.Manifest) backend {
	if m == nil {
		return &ecosystems.Fallback{}
	}
	switch m.Kind {
	case "gomod":
		return ecosystems.NewGoMod(m)
	case "npm":
		return ecosystems.NewNPM(m)
	case "composer":
		return ecosystems.NewComposer(m)
	default:
		return &ecosystems.Fallback{}
	}
}
