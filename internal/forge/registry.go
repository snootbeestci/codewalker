package forge

import (
	"fmt"
	"strings"
	"sync"
)

var (
	mu       sync.RWMutex
	handlers []ForgeHandler
)

// Register adds a ForgeHandler to the registry.
// Call from an init() function in each forge implementation.
func Register(h ForgeHandler) {
	mu.Lock()
	defer mu.Unlock()
	handlers = append(handlers, h)
}

// Resolve returns the ForgeHandler whose Hosts() patterns match the given
// hostname. Returns an error if no handler claims the host.
func Resolve(host string) (ForgeHandler, error) {
	mu.RLock()
	defer mu.RUnlock()
	for _, h := range handlers {
		for _, pattern := range h.Hosts() {
			if matchHost(pattern, host) {
				return h, nil
			}
		}
	}
	return nil, fmt.Errorf("no forge handler registered for host %q", host)
}

func matchHost(pattern, host string) bool {
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:]
		return strings.HasSuffix(host, suffix)
	}
	return pattern == host
}
