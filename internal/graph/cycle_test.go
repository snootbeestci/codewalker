package graph_test

import (
	"testing"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/graph"
)

func TestDetectCycles_NoCycle(t *testing.T) {
	stepA := &graph.Step{ID: "a", Edges: []*v1.StepEdge{{TargetStepId: "b", Navigable: true}}}
	stepB := &graph.Step{ID: "b"}
	g := makeGraph(stepA, stepB)

	cycles := graph.DetectCycles(g)
	if len(cycles) != 0 {
		t.Errorf("expected no cycles, got %v", cycles)
	}
}

func TestDetectCycles_WithCycle(t *testing.T) {
	// a → b → a  (cycle)
	stepA := &graph.Step{ID: "a", Edges: []*v1.StepEdge{{TargetStepId: "b", Navigable: true}}}
	stepB := &graph.Step{ID: "b", Edges: []*v1.StepEdge{{TargetStepId: "a", Navigable: true}}}
	g := makeGraph(stepA, stepB)

	cycles := graph.DetectCycles(g)
	if len(cycles) == 0 {
		t.Error("expected cycle to be detected")
	}
}
