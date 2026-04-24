package ecosystems

import (
	"fmt"

	"github.com/yourorg/codewalker/internal/git"
)

// Composer resolves external call info for PHP packages using composer.json.
type Composer struct {
	manifest *git.Manifest
}

func NewComposer(m *git.Manifest) *Composer { return &Composer{manifest: m} }

// Resolve returns packagist docs URL for the given package.
func (c *Composer) Resolve(pkg, symbol string) (docsURL, sourceURL, version string) {
	version = c.manifest.LookupVersion(pkg)
	docsURL = fmt.Sprintf("https://packagist.org/packages/%s", pkg)
	return docsURL, "", version
}
