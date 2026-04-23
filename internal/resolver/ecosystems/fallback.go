package ecosystems

// Fallback is used when no package manifest is found or the package is unknown.
// It returns empty URLs — the LLM summary (always populated by the resolver)
// is the only information surface.
type Fallback struct{}

func (f *Fallback) Resolve(pkg, symbol string) (docsURL, sourceURL, version string) {
	return "", "", ""
}
