package parser_test

import (
	"testing"

	"github.com/yourorg/codewalker/internal/parser"
	_ "github.com/yourorg/codewalker/internal/parser/languages"
)

func TestForKnownExtensions(t *testing.T) {
	cases := []struct {
		file string
		lang string
	}{
		{"main.go", "Go"},
		{"app.ts", "TypeScript"},
		{"index.tsx", "TypeScript"},
		{"script.py", "Python"},
		{"index.php", "PHP"},
	}
	for _, tc := range cases {
		h, err := parser.For(tc.file)
		if err != nil {
			t.Errorf("For(%q): unexpected error: %v", tc.file, err)
			continue
		}
		if h.Language() != tc.lang {
			t.Errorf("For(%q).Language() = %q, want %q", tc.file, h.Language(), tc.lang)
		}
	}
}

func TestForUnknownExtension(t *testing.T) {
	_, err := parser.For("file.unknown")
	if err == nil {
		t.Error("expected error for unknown extension, got nil")
	}
}
