package languages

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	python "github.com/smacker/go-tree-sitter/python"

	"github.com/yourorg/codewalker/internal/parser"
)

func init() {
	parser.Register(&pyHandler{})
}

type pyHandler struct{}

func (h *pyHandler) Language() string     { return "Python" }
func (h *pyHandler) Extensions() []string { return []string{".py"} }

func (h *pyHandler) Nodes(src []byte, filePath string) ([]*parser.Node, error) {
	p := sitter.NewParser()
	p.SetLanguage(python.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("python parser: %w", err)
	}
	defer tree.Close()

	return extractPyTopLevel(tree.RootNode(), src), nil
}

func (h *pyHandler) Symbols(src []byte) ([]string, error) {
	p := sitter.NewParser()
	p.SetLanguage(python.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("python symbols: %w", err)
	}
	defer tree.Close()

	var names []string
	root := tree.RootNode()
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child.Type() == "function_definition" || child.Type() == "decorated_definition" {
			if n := child.ChildByFieldName("name"); n != nil {
				names = append(names, n.Content(src))
			}
		}
	}
	return names, nil
}

func extractPyTopLevel(root *sitter.Node, src []byte) []*parser.Node {
	var nodes []*parser.Node
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "function_definition":
			nodes = append(nodes, buildPyFuncNode(child, src))
		case "class_definition":
			// Extract methods from classes as separate function nodes.
			nodes = append(nodes, extractPyClassMethods(child, src)...)
		case "decorated_definition":
			// Skip to the inner definition.
			for j := 0; j < int(child.ChildCount()); j++ {
				inner := child.Child(j)
				if inner.Type() == "function_definition" {
					nodes = append(nodes, buildPyFuncNode(inner, src))
				}
			}
		}
	}
	return nodes
}

func extractPyClassMethods(class *sitter.Node, src []byte) []*parser.Node {
	var nodes []*parser.Node
	body := class.ChildByFieldName("body")
	if body == nil {
		return nil
	}
	className := ""
	if n := class.ChildByFieldName("name"); n != nil {
		className = n.Content(src)
	}
	for i := 0; i < int(body.ChildCount()); i++ {
		child := body.Child(i)
		if child.Type() == "function_definition" {
			nd := buildPyFuncNode(child, src)
			nd.Label = className + "." + nd.Label
			nodes = append(nodes, nd)
		}
	}
	return nodes
}

func buildPyFuncNode(n *sitter.Node, src []byte) *parser.Node {
	label := "def"
	if name := n.ChildByFieldName("name"); name != nil {
		label = "def " + name.Content(src)
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
		nd.Children = extractPyBody(body, src)
	}
	return nd
}

func extractPyBody(block *sitter.Node, src []byte) []*parser.Node {
	var children []*parser.Node
	for i := 0; i < int(block.ChildCount()); i++ {
		child := block.Child(i)
		if nd := extractPyStatement(child, src); nd != nil {
			children = append(children, nd)
		}
	}
	return children
}

func extractPyStatement(n *sitter.Node, src []byte) *parser.Node {
	switch n.Type() {
	case "if_statement":
		return extractPyIf(n, src)
	case "for_statement", "while_statement":
		label := n.Type()[:3] + " " + trimText(n.Content(src), 50)
		nd := &parser.Node{Kind: parser.NodeKindLoop, Label: label,
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
		if body := n.ChildByFieldName("body"); body != nil {
			nd.Children = extractPyBody(body, src)
		}
		return nd
	case "return_statement":
		return &parser.Node{Kind: parser.NodeKindReturn, Label: trimText(n.Content(src), 60),
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	case "assignment", "augmented_assignment", "annotated_assignment":
		return &parser.Node{Kind: parser.NodeKindAssignment, Label: trimText(n.Content(src), 80),
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	case "expression_statement":
		inner := n.Child(0)
		if inner != nil && inner.Type() == "call" {
			return &parser.Node{Kind: parser.NodeKindCall, Label: trimText(inner.Content(src), 80),
				StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
		}
	case "with_statement":
		return &parser.Node{Kind: parser.NodeKindBlock, Label: "with " + trimText(n.Content(src), 50),
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	case "try_statement":
		return &parser.Node{Kind: parser.NodeKindBlock, Label: "try",
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	case "match_statement":
		return &parser.Node{Kind: parser.NodeKindSwitch, Label: "match " + trimText(n.Content(src), 40),
			StartLine: int(n.StartPoint().Row) + 1, EndLine: int(n.EndPoint().Row) + 1, Text: n.Content(src)}
	}
	return nil
}

func extractPyIf(n *sitter.Node, src []byte) *parser.Node {
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
		trueNode.Children = extractPyBody(body, src)
		nd.Children = append(nd.Children, trueNode)
	}
	if alt := n.ChildByFieldName("alternative"); alt != nil {
		falseNode := &parser.Node{Kind: parser.NodeKindBlock, Label: "else",
			StartLine: int(alt.StartPoint().Row) + 1, EndLine: int(alt.EndPoint().Row) + 1, Text: alt.Content(src)}
		if alt.Type() == "elif_clause" || alt.Type() == "if_statement" {
			falseNode = extractPyIf(alt, src)
		} else {
			falseNode.Children = extractPyBody(alt, src)
		}
		nd.Children = append(nd.Children, falseNode)
	}
	return nd
}
