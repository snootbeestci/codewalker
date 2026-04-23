package graph_test

import (
	"testing"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
	"github.com/yourorg/codewalker/internal/graph"
)

func makeGraph(steps ...*graph.Step) *graph.Graph {
	g := graph.NewGraph()
	for _, s := range steps {
		g.Add(s)
	}
	if len(steps) > 0 {
		g.EntryID = steps[0].ID
	}
	return g
}

func TestWalkerGoTo(t *testing.T) {
	stepA := &graph.Step{ID: "a", Label: "A", Kind: v1.StepKind_STEP_KIND_FUNCTION,
		Edges: []*v1.StepEdge{{TargetStepId: "b", Label: v1.EdgeLabel_EDGE_LABEL_SEQUENCE, Navigable: true}}}
	stepB := &graph.Step{ID: "b", Label: "B", Kind: v1.StepKind_STEP_KIND_CALL}
	g := makeGraph(stepA, stepB)

	w := graph.NewWalker(g)
	if w.CurrentID() != "a" {
		t.Fatalf("expected current=a, got %s", w.CurrentID())
	}

	step, err := w.GoTo("b")
	if err != nil {
		t.Fatalf("GoTo b: %v", err)
	}
	if step.ID != "b" {
		t.Errorf("expected step b, got %s", step.ID)
	}
	if !step.Visited {
		t.Error("step b should be marked visited")
	}
	if w.CurrentID() != "b" {
		t.Errorf("expected current b, got %s", w.CurrentID())
	}

	crumb := w.Breadcrumb()
	if len(crumb) != 1 || crumb[0] != "a" {
		t.Errorf("unexpected breadcrumb: %v", crumb)
	}
}

func TestWalkerBack(t *testing.T) {
	stepA := &graph.Step{ID: "a", Label: "A",
		Edges: []*v1.StepEdge{{TargetStepId: "b", Navigable: true}}}
	stepB := &graph.Step{ID: "b", Label: "B"}
	g := makeGraph(stepA, stepB)

	w := graph.NewWalker(g)
	if _, err := w.GoTo("b"); err != nil {
		t.Fatal(err)
	}

	step, err := w.Back()
	if err != nil {
		t.Fatalf("Back: %v", err)
	}
	if step.ID != "a" {
		t.Errorf("expected a after back, got %s", step.ID)
	}
	if w.CurrentID() != "a" {
		t.Errorf("current should be a after back, got %s", w.CurrentID())
	}
	if len(w.Breadcrumb()) != 0 {
		t.Errorf("breadcrumb should be empty after back, got %v", w.Breadcrumb())
	}
}

func TestWalkerBackAtStart(t *testing.T) {
	g := makeGraph(&graph.Step{ID: "a", Label: "A"})
	w := graph.NewWalker(g)

	_, err := w.Back()
	if err == nil {
		t.Error("expected error when Back at first step")
	}
}
