package parser

// LanguageHandler is the interface every language plugin must implement.
// Adding a language requires only a new file under internal/parser/languages/
// and a registration call in that file's init().
type LanguageHandler interface {
	// Language returns the canonical language name (e.g. "Go", "TypeScript").
	Language() string

	// Extensions returns the file extensions this handler claims (e.g. ".go").
	Extensions() []string

	// Nodes parses src and returns the top-level logical nodes suitable for
	// step-graph construction.  filePath is used only for error messages.
	Nodes(src []byte, filePath string) ([]*Node, error)

	// Symbols returns the list of top-level symbol names defined in src
	// (function names, method names, etc.).
	Symbols(src []byte) ([]string, error)
}

// NodeKind classifies a parsed code node into a step kind.
type NodeKind string

const (
	NodeKindFunction    NodeKind = "function"
	NodeKindConditional NodeKind = "conditional"
	NodeKindLoop        NodeKind = "loop"
	NodeKindAssignment  NodeKind = "assignment"
	NodeKindCall        NodeKind = "call"
	NodeKindSwitch      NodeKind = "switch"
	NodeKindReturn      NodeKind = "return"
	NodeKindBlock       NodeKind = "block"
)

// Node is the language-agnostic representation of a parsed code unit.
type Node struct {
	ID        string
	Kind      NodeKind
	Label     string
	StartLine int
	EndLine   int
	StartCol  int
	EndCol    int
	Text      string
	Children  []*Node
	Calls     []*CallRef
}

// CallRef records a call to an external or internal symbol found within a Node.
type CallRef struct {
	// Package is the import path or package alias (empty for unqualified calls).
	Package string
	// Symbol is the function or method name.
	Symbol string
	// Line is the source line of the call expression.
	Line int
	// Internal is true when the callee is defined within the same repository.
	Internal bool
	// TargetNodeID is set when Internal = true and the target step is known.
	TargetNodeID string
}
