package languages_test

import (
	"testing"

	"github.com/yourorg/codewalker/internal/parser"
	_ "github.com/yourorg/codewalker/internal/parser/languages"
)

const goSample = `package main

import "fmt"

func add(a, b int) int {
	return a + b
}

func greet(name string) {
	if name == "" {
		name = "World"
	}
	fmt.Println("Hello,", name)
}
`

func TestGoParserNodes(t *testing.T) {
	h, err := parser.For("main.go")
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := h.Nodes([]byte(goSample), "main.go")
	if err != nil {
		t.Fatalf("Nodes: %v", err)
	}

	if len(nodes) < 2 {
		t.Fatalf("expected at least 2 top-level functions, got %d", len(nodes))
	}

	// Check that function kinds are correct.
	for _, n := range nodes {
		if n.Kind != parser.NodeKindFunction {
			t.Errorf("top-level node %q has kind %q, want function", n.Label, n.Kind)
		}
	}
}

func TestGoParserSymbols(t *testing.T) {
	h, err := parser.For("main.go")
	if err != nil {
		t.Fatal(err)
	}

	symbols, err := h.Symbols([]byte(goSample))
	if err != nil {
		t.Fatalf("Symbols: %v", err)
	}

	want := map[string]bool{"add": true, "greet": true}
	for _, s := range symbols {
		delete(want, s)
	}
	if len(want) > 0 {
		t.Errorf("missing symbols: %v", want)
	}
}

func TestGoParserConditionalChildren(t *testing.T) {
	h, err := parser.For("main.go")
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := h.Nodes([]byte(goSample), "main.go")
	if err != nil {
		t.Fatal(err)
	}

	// greet has an if statement.
	var greetNode *parser.Node
	for _, n := range nodes {
		if n.Label == "func greet" {
			greetNode = n
			break
		}
	}
	if greetNode == nil {
		t.Fatal("greet function not found")
	}

	hasConditional := false
	for _, child := range greetNode.Children {
		if child.Kind == parser.NodeKindConditional {
			hasConditional = true
		}
	}
	if !hasConditional {
		t.Error("expected greet to have a conditional child node")
	}
}
