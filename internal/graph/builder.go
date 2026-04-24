package graph

import (
	"fmt"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/parser"
)

// Build converts a slice of top-level parser Nodes into a step Graph.
// entrySymbol selects which top-level function to enter; empty means the first.
func Build(nodes []*parser.Node, src []byte, filePath string, omitRawSource bool, entrySymbol string) (*Graph, error) {
	g := newGraph()
	counter := 0

	nextID := func() string {
		counter++
		return fmt.Sprintf("step-%04d", counter)
	}

	var buildNode func(n *parser.Node, parentID string, edgeLabel v1.EdgeLabel) *Step
	buildNode = func(n *parser.Node, parentID string, edgeLabel v1.EdgeLabel) *Step {
		id := nextID()
		if n.ID != "" {
			id = n.ID
		}

		span := &v1.SourceSpan{
			StartLine:   uint32(n.StartLine),
			EndLine:     uint32(n.EndLine),
			StartColumn: uint32(n.StartCol),
			EndColumn:   uint32(n.EndCol),
		}
		if !omitRawSource {
			span.RawSource = n.Text
		}

		step := &Step{
			ID:     id,
			Label:  n.Label,
			Kind:   nodeKindToStepKind(n.Kind),
			Source: span,
		}
		g.Add(step)

		// Build call edges from CallRefs.
		for _, ref := range n.Calls {
			edge := &v1.StepEdge{
				Label:       v1.EdgeLabel_EDGE_LABEL_CALL,
				Description: fmt.Sprintf("calls %s.%s", ref.Package, ref.Symbol),
				Navigable:   ref.Internal,
			}
			if ref.Internal && ref.TargetNodeID != "" {
				edge.TargetStepId = ref.TargetNodeID
			} else if !ref.Internal {
				edge.ExternalCallInfo = &v1.ExternalCallInfo{
					PackageName: ref.Package,
					SymbolName:  ref.Symbol,
				}
			}
			step.Edges = append(step.Edges, edge)
		}

		// Recursively build children.
		if len(n.Children) > 0 {
			children := buildChildren(n.Children, buildNode)
			step.Edges = append(step.Edges, children...)
		}

		return step
	}

	// Find the entry function.
	var entryStep *Step
	for _, n := range nodes {
		if n.Kind != parser.NodeKindFunction {
			continue
		}
		if entrySymbol == "" || n.Label == entrySymbol {
			entryStep = buildNode(n, "", v1.EdgeLabel_EDGE_LABEL_NEXT)
			if entrySymbol == "" {
				break // use first function
			}
		}
	}

	// Also build any other top-level functions so CALL edges can reference them.
	for _, n := range nodes {
		if n.Kind != parser.NodeKindFunction {
			continue
		}
		if entryStep != nil && n.Label == entryStep.Label {
			continue
		}
		buildNode(n, "", v1.EdgeLabel_EDGE_LABEL_NEXT)
	}

	if entryStep == nil && len(nodes) > 0 {
		// Fallback: build first node whatever its kind.
		entryStep = buildNode(nodes[0], "", v1.EdgeLabel_EDGE_LABEL_NEXT)
	}

	if entryStep != nil {
		g.EntryID = entryStep.ID
	}

	return g, nil
}

// buildChildren produces the ordered edge list for a parent node's children,
// wiring SEQUENCE edges between siblings and specific labels for branches.
func buildChildren(children []*parser.Node, build func(*parser.Node, string, v1.EdgeLabel) *Step) []*v1.StepEdge {
	var edges []*v1.StepEdge
	for i, child := range children {
		label := v1.EdgeLabel_EDGE_LABEL_NEXT
		switch child.Kind {
		case parser.NodeKindConditional:
			// Conditional children represent the body — keep SEQUENCE for the
			// conditional itself; branches are handled by the conditional's own
			// children in the next recursion level.
		}
		_ = i
		step := build(child, "", label)
		edges = append(edges, &v1.StepEdge{
			TargetStepId: step.ID,
			Label:        label,
			Navigable:    true,
		})
	}
	return edges
}

func nodeKindToStepKind(k parser.NodeKind) v1.StepKind {
	switch k {
	case parser.NodeKindFunction:
		return v1.StepKind_STEP_KIND_ENTRY
	case parser.NodeKindConditional, parser.NodeKindSwitch:
		return v1.StepKind_STEP_KIND_CONDITIONAL
	case parser.NodeKindLoop:
		return v1.StepKind_STEP_KIND_LOOP
	case parser.NodeKindAssignment:
		return v1.StepKind_STEP_KIND_ASSIGNMENT
	case parser.NodeKindCall:
		return v1.StepKind_STEP_KIND_CALL
	case parser.NodeKindReturn:
		return v1.StepKind_STEP_KIND_RETURN
	default:
		return v1.StepKind_STEP_KIND_UNSPECIFIED
	}
}
