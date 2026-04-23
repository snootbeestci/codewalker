package languages

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	php "github.com/smacker/go-tree-sitter/php"

	"github.com/yourorg/codewalker/internal/parser"
)

func init() {
	parser.Register(&phpHandler{})
}

type phpHandler struct{}

func (h *phpHandler) Language() string     { return "PHP" }
func (h *phpHandler) Extensions() []string { return []string{".php"} }

func (h *phpHandler) Nodes(src []byte, filePath string) ([]*parser.Node, error) {
	p := sitter.NewParser()
	p.SetLanguage(php.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("php parser: %w", err)
	}
	defer tree.Close()

	return extractPHPTopLevel(tree.RootNode(), src), nil
}

func (h *phpHandler) Symbols(src []byte) ([]string, error) {
	p := sitter.NewParser()
	p.SetLanguage(php.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("php symbols: %w", err)
	}
	defer tree.Close()

	var names []string
	root := tree.RootNode()
	walkPHPForFunctions(root, src, &names)
	return names, nil
}

func walkPHPForFunctions(n *sitter.Node, src []byte, names *[]string) {
	if n.Type() == "function_definition" || n.Type() == "method_declaration" {
		if name := n.ChildByFieldName("name"); name != nil {
			*names = append(*names, name.Content(src))
		}
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		walkPHPForFunctions(n.Child(i), src, names)
	}
}

func extractPHPTopLevel(root *sitter.Node, src []byte) []*parser.Node {
	var nodes []*parser.Node
	// PHP root is "program" → "php_tag" then statements.
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "function_definition":
			nodes = append(nodes, buildPHPFuncNode(child, src))
		case "class_declaration":
			nodes = append(nodes, extractPHPClassMethods(child, src)...)
		}
	}
	return nodes
}

func extractPHPClassMethods(class *sitter.Node, src []byte) []*parser.Node {
	var nodes []*parser.Node
	className := ""
	if n := class.ChildByFieldName("name"); n != nil {
		className = n.Content(src)
	}
	body := class.ChildByFieldName("body")
	if body == nil {
		return nil
	}
	for i := 0; i < int(body.ChildCount()); i++ {
		child := body.Child(i)
		if child.Type() == "method_declaration" {
			nd := buildPHPFuncNode(child, src)
			nd.Label = className + "::" + nd.Label
			nodes = append(nodes, nd)
		}
	}
	return nodes
}

func buildPHPFuncNode(n *sitter.Node, src []byte) *parser.Node {
	label := "function"
	if name := n.ChildByFieldName("name"); name != nil {
		label = "function " + name.Content(src)
	}
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
	if body := n.ChildByFieldName("body"); body != nil {
		nd.Children = extractPHPBody(body, src)
	}
	return nd
}

func extractPHPBody(block *sitter.Node, src []byte) []*parser.Node {
	var children []*parser.Node
	for i := 0; i < int(block.ChildCount()); i++ {
		child := block.Child(i)
		if nd := extractPHPStatement(child, src); nd != nil {
			children = append(children, nd)
		}
	}
	return children
}

func extractPHPStatement(n *sitter.Node, src []byte) *parser.Node {
	switch n.Type() {
	case "if_statement":
		cond := n.ChildByFieldName("condition")
		label := "if"
		if cond != nil {
			label = "if " + trimText(cond.Content(src), 60)
		}
		nd := &parser.Node{Kind: parser.NodeKindConditional, Label: label,
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
		if body := n.ChildByFieldName("body"); body != nil {
			trueNode := &parser.Node{Kind: parser.NodeKindBlock, Label: "then",
				StartLine: int(body.StartPoint().Row) + 1, EndLine: int(body.EndPoint().Row) + 1, Text: body.Content(src)}
			trueNode.Children = extractPHPBody(body, src)
			nd.Children = append(nd.Children, trueNode)
		}
		if alt := n.ChildByFieldName("alternative"); alt != nil {
			falseNode := &parser.Node{Kind: parser.NodeKindBlock, Label: "else",
				StartLine: int(alt.StartPoint().Row) + 1, EndLine: int(alt.EndPoint().Row) + 1, Text: alt.Content(src)}
			falseNode.Children = extractPHPBody(alt, src)
			nd.Children = append(nd.Children, falseNode)
		}
		return nd
	case "for_statement", "foreach_statement", "while_statement", "do_statement":
		return &parser.Node{Kind: parser.NodeKindLoop, Label: n.Type() + " " + trimText(n.Content(src), 50),
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	case "switch_statement":
		return &parser.Node{Kind: parser.NodeKindSwitch, Label: "switch " + trimText(n.Content(src), 40),
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	case "return_statement":
		return &parser.Node{Kind: parser.NodeKindReturn, Label: trimText(n.Content(src), 60),
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	case "expression_statement":
		inner := n.Child(0)
		if inner != nil && inner.Type() == "assignment_expression" {
			return &parser.Node{Kind: parser.NodeKindAssignment, Label: trimText(inner.Content(src), 80),
				StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
		}
		if inner != nil && inner.Type() == "function_call_expression" {
			return &parser.Node{Kind: parser.NodeKindCall, Label: trimText(inner.Content(src), 80),
				StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
		}
	}
	return nil
}
