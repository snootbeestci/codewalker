package orderers

import (
	"github.com/yourorg/codewalker/internal/forge"
)

func init() {
	forge.RegisterOrderer(&asFetched{})
}

type asFetched struct{}

func (a *asFetched) Name() string {
	return "as-fetched"
}

func (a *asFetched) Description() string {
	return "Preserves the order returned by the forge — useful for debugging"
}

func (a *asFetched) Order(files []*forge.ReviewFile) []*forge.ReviewFile {
	out := make([]*forge.ReviewFile, len(files))
	copy(out, files)
	return out
}
