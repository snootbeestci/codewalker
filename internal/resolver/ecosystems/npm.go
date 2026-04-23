package ecosystems

import (
	"fmt"

	"github.com/yourorg/codewalker/internal/git"
)

// NPM resolves external call info for Node/TypeScript packages using package.json.
type NPM struct {
	manifest *git.Manifest
}

func NewNPM(m *git.Manifest) *NPM { return &NPM{manifest: m} }

// Resolve returns npmjs docs URL and version for the given package.
func (n *NPM) Resolve(pkg, symbol string) (docsURL, sourceURL, version string) {
	version = n.manifest.LookupVersion(pkg)

	if version != "" {
		docsURL = fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", pkg, version)
	} else {
		docsURL = fmt.Sprintf("https://www.npmjs.com/package/%s", pkg)
	}
	return docsURL, "", version
}
