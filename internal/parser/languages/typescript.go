package languages

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	ts "github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/yourorg/codewalker/internal/parser"
)

func init() {
	parser.Register(&tsHandler{})
}

type tsHandler struct{}

func (h *tsHandler) Language() string     { return "TypeScript" }
func (h *tsHandler) Extensions() []string { return []string{".ts", ".tsx"} }

func (h *tsHandler) Nodes(src []byte, filePath string) ([]*parser.Node, error) {
	p := sitter.NewParser()
	p.SetLanguage(ts.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("typescript parser: %w", err)
	}
	defer tree.Close()

	return extractTSTopLevel(tree.RootNode(), src), nil
}

func (h *tsHandler) Symbols(src []byte) ([]string, error) {
	p := sitter.NewParser()
	p.SetLanguage(ts.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("typescript symbols: %w", err)
	}
	defer tree.Close()

	return extractTSSymbols(tree.RootNode(), src), nil
}

func extractTSTopLevel(root *sitter.Node, src []byte) []*parser.Node {
	var nodes []*parser.Node
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "function_declaration", "function",
			"method_definition", "arrow_function",
			"export_statement":
			nodes = append(nodes, buildTSNode(child, src))
		}
	}
	return nodes
}

func extractTSSymbols(root *sitter.Node, src []byte) []string {
	var names []string
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "function_declaration":
			if n := child.ChildByFieldName("name"); n != nil {
				names = append(names, n.Content(src))
			}
		case "export_statement":
			if decl := child.ChildByFieldName("declaration"); decl != nil {
				if n := decl.ChildByFieldName("name"); n != nil {
					names = append(names, n.Content(src))
				}
			}
		}
	}
	return names
}

func buildTSNode(n *sitter.Node, src []byte) *parser.Node {
	label := tsFuncLabel(n, src)
	nd := &parser.Node{
		ID:        fmt.Sprintf("%s:%d:%d", parser.NodeKindFunction, n.StartPoint().Row, n.StartPoint().Column),
		Kind:      parser.NodeKindFunction,
		Label:     label,
		StartLine: int(n.StartPoint().Row) + 1,
		EndLine:   int(n.EndPoint().Row) + 1,
		StartCol:  int(n.StartPoint().Column),
		EndCol:    int(n.EndPoint().Column),
		Text:      n.Content(src),
	}

	body := n.ChildByFieldName("body")
	if body != nil {
		nd.Children = extractTSBody(body, src)
	}
	return nd
}

func tsFuncLabel(n *sitter.Node, src []byte) string {
	switch n.Type() {
	case "function_declaration":
		if name := n.ChildByFieldName("name"); name != nil {
			return "function " + name.Content(src)
		}
	case "method_definition":
		if name := n.ChildByFieldName("name"); name != nil {
			return "method " + name.Content(src)
		}
	case "export_statement":
		if decl := n.ChildByFieldName("declaration"); decl != nil {
			return tsFuncLabel(decl, src)
		}
	}
	return trimText(n.Content(src), 40)
}

func extractTSBody(block *sitter.Node, src []byte) []*parser.Node {
	var children []*parser.Node
	for i := 0; i < int(block.ChildCount()); i++ {
		child := block.Child(i)
		if nd := extractTSStatement(child, src); nd != nil {
			children = append(children, nd)
		}
	}
	return children
}

func extractTSStatement(n *sitter.Node, src []byte) *parser.Node {
	switch n.Type() {
	case "if_statement":
		return extractTSIf(n, src)
	case "for_statement", "for_in_statement", "while_statement", "do_statement":
		label := strings.SplitN(n.Type(), "_", 2)[0] + " " + trimText(n.Content(src), 50)
		nd := &parser.Node{
			Kind: parser.NodeKindLoop, Label: label,
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1,
			Text: n.Content(src),
		}
		if body := n.ChildByFieldName("body"); body != nil {
			nd.Children = extractTSBody(body, src)
		}
		return nd
	case "switch_statement":
		nd := &parser.Node{Kind: parser.NodeKindSwitch, Label: "switch " + trimText(n.Content(src), 40),
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
		return nd
	case "return_statement":
		return &parser.Node{Kind: parser.NodeKindReturn, Label: trimText(n.Content(src), 60),
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	case "variable_declaration", "lexical_declaration":
		return &parser.Node{Kind: parser.NodeKindAssignment, Label: trimText(n.Content(src), 80),
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	case "expression_statement":
		inner := n.Child(0)
		if inner != nil && (inner.Type() == "call_expression" || inner.Type() == "await_expression") {
			return &parser.Node{Kind: parser.NodeKindCall, Label: trimText(inner.Content(src), 80),
				StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
		}
	}
	return nil
}

func extractTSIf(n *sitter.Node, src []byte) *parser.Node {
	cond := n.ChildByFieldName("condition")
	label := "if"
	if cond != nil {
		label = "if " + trimText(cond.Content(src), 60)
	}
	nd := &parser.Node{Kind: parser.NodeKindConditional, Label: label,
		StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	if body := n.ChildByFieldName("consequence"); body != nil {
		trueNode := &parser.Node{Kind: parser.NodeKindBlock, Label: "then",
			StartLine: int(body.StartPoint().Row) + 1, EndLine: int(body.EndPoint().Row) + 1, Text: body.Content(src)}
		trueNode.Children = extractTSBody(body, src)
		nd.Children = append(nd.Children, trueNode)
	}
	if alt := n.ChildByFieldName("alternative"); alt != nil {
		falseNode := &parser.Node{Kind: parser.NodeKindBlock, Label: "else",
			StartLine: int(alt.StartPoint().Row) + 1, EndLine: int(alt.EndPoint().Row) + 1, Text: alt.Content(src)}
		falseNode.Children = extractTSBody(alt, src)
		nd.Children = append(nd.Children, falseNode)
	}
	return nd
}
