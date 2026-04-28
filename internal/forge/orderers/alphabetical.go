package orderers

import (
	"sort"

	"github.com/yourorg/codewalker/internal/forge"
)

func init() {
	forge.RegisterOrderer(&alphabetical{})
}

type alphabetical struct{}

func (a *alphabetical) Name() string {
	return "alphabetical"
}

func (a *alphabetical) Description() string {
	return "Sorted by file path"
}

func (a *alphabetical) Order(files []*forge.ReviewFile) []*forge.ReviewFile {
	out := make([]*forge.ReviewFile, len(files))
	copy(out, files)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}
