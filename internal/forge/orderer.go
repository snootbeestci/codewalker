package forge

import (
	"fmt"
	"sync"
)

// FileOrderer determines the presentation order of changed files in a review
// session. Implementations are registered via init() functions in the
// internal/forge/orderers/ directory and selected per-session by name.
type FileOrderer interface {
	// Name is the stable identifier used in configuration.
	Name() string

	// Description is a one-sentence explanation of the strategy.
	Description() string

	// Order returns the files in the order they should be presented.
	// Implementations must return a new slice rather than mutating the input.
	Order(files []*ReviewFile) []*ReviewFile
}

const DefaultOrdererName = "entry-points-first"

var (
	ordererMu sync.RWMutex
	orderers  = map[string]FileOrderer{}
)

// RegisterOrderer adds a FileOrderer to the registry.
// Call from an init() function in each orderer implementation.
func RegisterOrderer(o FileOrderer) {
	ordererMu.Lock()
	defer ordererMu.Unlock()
	orderers[o.Name()] = o
}

// ResolveOrderer returns the FileOrderer with the given name.
// Falls back to the default if name is empty.
// Returns an error if a named orderer is not registered.
func ResolveOrderer(name string) (FileOrderer, error) {
	ordererMu.RLock()
	defer ordererMu.RUnlock()
	if name == "" {
		name = DefaultOrdererName
	}
	o, ok := orderers[name]
	if !ok {
		return nil, fmt.Errorf("file orderer %q not registered", name)
	}
	return o, nil
}

// ListOrderers returns all registered orderers.
func ListOrderers() []FileOrderer {
	ordererMu.RLock()
	defer ordererMu.RUnlock()
	out := make([]FileOrderer, 0, len(orderers))
	for _, o := range orderers {
		out = append(out, o)
	}
	return out
}
