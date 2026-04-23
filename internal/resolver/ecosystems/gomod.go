package ecosystems

import (
	"fmt"

	"github.com/yourorg/codewalker/internal/git"
)

// GoMod resolves external call info for Go packages using go.mod.
type GoMod struct {
	manifest *git.Manifest
}

// NewGoMod creates a resolver for a repository whose manifest has been detected.
func NewGoMod(m *git.Manifest) *GoMod {
	return &GoMod{manifest: m}
}

// Resolve returns docs and source URLs for the given Go package and symbol.
// version is the pinned version from go.mod, or "" if not found.
func (g *GoMod) Resolve(pkg, symbol string) (docsURL, sourceURL, version string) {
	version = g.manifest.LookupVersion(pkg)

	if version != "" {
		docsURL = fmt.Sprintf("https://pkg.go.dev/%s@%s#%s", pkg, version, symbol)
		sourceURL = fmt.Sprintf("https://cs.opensource.google/go/%s", pkg)
	} else {
		// stdlib or unresolved
		docsURL = fmt.Sprintf("https://pkg.go.dev/%s#%s", pkg, symbol)
	}
	return docsURL, sourceURL, version
}
