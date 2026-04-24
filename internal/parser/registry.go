package parser

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu       sync.RWMutex
	handlers = map[string]LanguageHandler{}
)

// Register associates a LanguageHandler with all of its declared extensions.
// It is called from init() functions in the languages/ sub-package.
func Register(h LanguageHandler) {
	mu.Lock()
	defer mu.Unlock()
	for _, ext := range h.Extensions() {
		handlers[strings.ToLower(ext)] = h
	}
}

// For returns the LanguageHandler registered for the extension of filePath.
func For(filePath string) (LanguageHandler, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	mu.RLock()
	h, ok := handlers[ext]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("parser: no handler registered for extension %q (file: %s)", ext, filePath)
	}
	return h, nil
}

// Registered returns the names of all registered languages.
func Registered() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(handlers))
	seen := map[string]bool{}
	for _, h := range handlers {
		if !seen[h.Language()] {
			names = append(names, h.Language())
			seen[h.Language()] = true
		}
	}
	return names
}
