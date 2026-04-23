package languages

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	golangts "github.com/smacker/go-tree-sitter/golang"

	"github.com/yourorg/codewalker/internal/parser"
)

func init() {
	parser.Register(&goHandler{})
}

type goHandler struct{}

func (h *goHandler) Language() string       { return "Go" }
func (h *goHandler) Extensions() []string   { return []string{".go"} }

func (h *goHandler) Nodes(src []byte, filePath string) ([]*parser.Node, error) {
	p := sitter.NewParser()
	p.SetLanguage(golangts.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("golang parser: %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	return extractTopLevel(root, src), nil
}

func (h *goHandler) Symbols(src []byte) ([]string, error) {
	p := sitter.NewParser()
	p.SetLanguage(golangts.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("golang symbols: %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	var names []string
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "function_declaration", "method_declaration":
			if n := child.ChildByFieldName("name"); n != nil {
				names = append(names, n.Content(src))
			}
		}
	}
	return names, nil
}

// extractTopLevel walks the root of a Go source file and collects top-level
// function and method declarations as Node trees.
func extractTopLevel(root *sitter.Node, src []byte) []*parser.Node {
	var nodes []*parser.Node
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "function_declaration", "method_declaration":
			nodes = append(nodes, buildFuncNode(child, src))
		}
	}
	return nodes
}

func buildFuncNode(n *sitter.Node, src []byte) *parser.Node {
	nameNode := n.ChildByFieldName("name")
	label := "func"
	if nameNode != nil {
		label = "func " + nameNode.Content(src)
	}
	// Include receiver for method declarations.
	if recv := n.ChildByFieldName("receiver"); recv != nil {
		label = "method " + recv.Content(src) + "." + label[5:]
	}

	node := newNode(parser.NodeKindFunction, label, n, src)

	// Walk the function body for logical children.
	body := n.ChildByFieldName("body")
	if body != nil {
		node.Children = extractBody(body, src)
	}
	return node
}

// extractBody extracts logical steps from a block node (function body,
// if body, loop body, etc.).
func extractBody(block *sitter.Node, src []byte) []*parser.Node {
	var children []*parser.Node
	for i := 0; i < int(block.ChildCount()); i++ {
		child := block.Child(i)
		if n := extractStatement(child, src); n != nil {
			children = append(children, n)
		}
	}
	return children
}

// extractStatement converts a tree-sitter statement node into a parser.Node.
// Returns nil for punctuation and nodes we don't model.
func extractStatement(n *sitter.Node, src []byte) *parser.Node {
	switch n.Type() {
	case "if_statement":
		return extractIf(n, src)
	case "for_statement":
		return extractFor(n, src)
	case "switch_statement", "type_switch_statement", "expression_switch_statement":
		return extractSwitch(n, src)
	case "return_statement":
		return newNode(parser.NodeKindReturn, "return "+trimText(n.Content(src), 60), n, src)
	case "short_var_decl", "var_declaration":
		return newNode(parser.NodeKindAssignment, trimText(n.Content(src), 80), n, src)
	case "assignment_statement":
		return newNode(parser.NodeKindAssignment, trimText(n.Content(src), 80), n, src)
	case "expression_statement":
		return extractExprStatement(n, src)
	case "go_statement":
		inner := n.Child(1)
		label := "go "
		if inner != nil {
			label += trimText(inner.Content(src), 60)
		}
		nd := newNode(parser.NodeKindCall, label, n, src)
		nd.Calls = extractCalls(n, src)
		return nd
	case "defer_statement":
		inner := n.Child(1)
		label := "defer "
		if inner != nil {
			label += trimText(inner.Content(src), 60)
		}
		nd := newNode(parser.NodeKindCall, label, n, src)
		nd.Calls = extractCalls(n, src)
		return nd
	case "send_statement":
		return newNode(parser.NodeKindAssignment, trimText(n.Content(src), 80), n, src)
	case "block":
		nd := newNode(parser.NodeKindBlock, "block", n, src)
		nd.Children = extractBody(n, src)
		return nd
	}
	return nil
}

func extractIf(n *sitter.Node, src []byte) *parser.Node {
	cond := n.ChildByFieldName("condition")
	label := "if"
	if cond != nil {
		label = "if " + trimText(cond.Content(src), 60)
	}
	nd := newNode(parser.NodeKindConditional, label, n, src)

	// True branch
	if body := n.ChildByFieldName("consequence"); body != nil {
		trueNode := newNode(parser.NodeKindBlock, "then", body, src)
		trueNode.Children = extractBody(body, src)
		nd.Children = append(nd.Children, trueNode)
	}
	// False branch
	if alt := n.ChildByFieldName("alternative"); alt != nil {
		falseNode := newNode(parser.NodeKindBlock, "else", alt, src)
		// alternative may be another if_statement or a block
		if alt.Type() == "if_statement" {
			falseNode = extractIf(alt, src)
			falseNode.Kind = parser.NodeKindConditional
		} else {
			falseNode.Children = extractBody(alt, src)
		}
		nd.Children = append(nd.Children, falseNode)
	}
	return nd
}

func extractFor(n *sitter.Node, src []byte) *parser.Node {
	label := "for"
	// Range clause or regular for — try to get a concise label.
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child.Type() == "range_clause" || child.Type() == "for_clause" {
			label = "for " + trimText(child.Content(src), 60)
			break
		}
	}
	nd := newNode(parser.NodeKindLoop, label, n, src)
	if body := n.ChildByFieldName("body"); body != nil {
		nd.Children = extractBody(body, src)
	}
	return nd
}

func extractSwitch(n *sitter.Node, src []byte) *parser.Node {
	label := "switch"
	if val := n.ChildByFieldName("value"); val != nil {
		label = "switch " + trimText(val.Content(src), 50)
	} else if tag := n.ChildByFieldName("tag"); tag != nil {
		label = "switch " + trimText(tag.Content(src), 50)
	}
	nd := newNode(parser.NodeKindSwitch, label, n, src)

	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child.Type() == "expression_case" || child.Type() == "default_case" || child.Type() == "type_case" {
			caseNode := newNode(parser.NodeKindBlock, "case "+trimText(child.Content(src), 40), child, src)
			caseNode.Children = extractBody(child, src)
			nd.Children = append(nd.Children, caseNode)
		}
	}
	return nd
}

func extractExprStatement(n *sitter.Node, src []byte) *parser.Node {
	// An expression_statement wraps a single expression.
	if n.ChildCount() == 0 {
		return nil
	}
	inner := n.Child(0)
	if inner.Type() == "call_expression" {
		label := trimText(inner.Content(src), 80)
		nd := newNode(parser.NodeKindCall, label, n, src)
		nd.Calls = extractCallsFromCallExpr(inner, src)
		return nd
	}
	return nil
}

// extractCalls walks a subtree and collects call_expression nodes.
func extractCalls(n *sitter.Node, src []byte) []*parser.CallRef {
	var refs []*parser.CallRef
	var walk func(node *sitter.Node)
	walk = func(node *sitter.Node) {
		if node.Type() == "call_expression" {
			refs = append(refs, extractCallsFromCallExpr(node, src)...)
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(n)
	return refs
}

func extractCallsFromCallExpr(n *sitter.Node, src []byte) []*parser.CallRef {
	fn := n.ChildByFieldName("function")
	if fn == nil {
		return nil
	}
	ref := &parser.CallRef{Line: int(n.StartPoint().Row) + 1}

	switch fn.Type() {
	case "selector_expression":
		// pkg.Func or obj.Method
		operand := fn.ChildByFieldName("operand")
		field := fn.ChildByFieldName("field")
		if operand != nil {
			ref.Package = operand.Content(src)
		}
		if field != nil {
			ref.Symbol = field.Content(src)
		}
	case "identifier":
		ref.Symbol = fn.Content(src)
	default:
		ref.Symbol = trimText(fn.Content(src), 40)
	}

	if ref.Symbol == "" {
		return nil
	}
	return []*parser.CallRef{ref}
}

// newNode creates a parser.Node from a tree-sitter node.
func newNode(kind parser.NodeKind, label string, n *sitter.Node, src []byte) *parser.Node {
	sp := n.StartPoint()
	ep := n.EndPoint()
	return &parser.Node{
		ID:        fmt.Sprintf("%s:%d:%d", kind, sp.Row, sp.Column),
		Kind:      kind,
		Label:     label,
		StartLine: int(sp.Row) + 1,
		EndLine:   int(ep.Row) + 1,
		StartCol:  int(sp.Column),
		EndCol:    int(ep.Column),
		Text:      n.Content(src),
	}
}

func trimText(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}
