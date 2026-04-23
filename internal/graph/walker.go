package graph

import (
	"errors"
	"fmt"

	v1 "github.com/yourorg/codewalker/gen/codewalker/v1"
)

// Graph is the server-side step graph.  It is created once per session and
// then only read (mutations happen only on Visited flags).
type Graph struct {
	steps   map[string]*Step
	EntryID string
}

// Step is the server-side representation of a single narration step.
type Step struct {
	ID      string
	Label   string
	Kind    v1.StepKind
	Source  *v1.SourceSpan
	Edges   []*v1.StepEdge
	Visited bool
}

// NewGraph creates an empty Graph.
func NewGraph() *Graph { return newGraph() }

func newGraph() *Graph {
	return &Graph{steps: make(map[string]*Step)}
}

// Add initialises the map if needed, then inserts the step.


// Add inserts a step.  Overwrites any existing step with the same ID.
func (g *Graph) Add(s *Step) {
	if g.steps == nil {
		g.steps = make(map[string]*Step)
	}
	g.steps[s.ID] = s
}

// Step returns the step with the given ID.
func (g *Graph) Step(id string) (*Step, bool) {
	s, ok := g.steps[id]
	return s, ok
}

// AllSteps returns all steps in unspecified order.
func (g *Graph) AllSteps() []*Step {
	out := make([]*Step, 0, len(g.steps))
	for _, s := range g.steps {
		out = append(out, s)
	}
	return out
}

// Len returns the number of steps.
func (g *Graph) Len() int { return len(g.steps) }

// Walker traverses the Graph on behalf of a session.
type Walker struct {
	g          *Graph
	current    string
	breadcrumb []string
}

// NewWalker creates a Walker starting at the entry step.
func NewWalker(g *Graph) *Walker {
	return &Walker{g: g, current: g.EntryID}
}

// Current returns the current step.
func (w *Walker) Current() (*Step, bool) {
	return w.g.Step(w.current)
}

// CurrentID returns the ID of the current step.
func (w *Walker) CurrentID() string { return w.current }

// Breadcrumb returns the ordered list of step IDs visited so far.
func (w *Walker) Breadcrumb() []string {
	out := make([]string, len(w.breadcrumb))
	copy(out, w.breadcrumb)
	return out
}

// GoTo navigates to targetID.  It validates that an edge to the target exists
// in the current step, marks the target as visited, and updates the breadcrumb.
func (w *Walker) GoTo(targetID string) (*Step, error) {
	cur, ok := w.g.Step(w.current)
	if !ok {
		return nil, fmt.Errorf("walker: current step %q not found", w.current)
	}

	// Allow navigation to any step reachable via a navigable edge.
	found := false
	for _, e := range cur.Edges {
		if e.TargetStepId == targetID && e.Navigable {
			found = true
			break
		}
	}
	// Also allow navigation to any step in the graph (e.g. back-navigation).
	if !found {
		_, found = w.g.Step(targetID)
	}
	if !found {
		return nil, fmt.Errorf("walker: step %q not reachable from %q", targetID, w.current)
	}

	target, ok := w.g.Step(targetID)
	if !ok {
		return nil, fmt.Errorf("walker: target step %q not found", targetID)
	}

	w.breadcrumb = append(w.breadcrumb, w.current)
	w.current = targetID
	target.Visited = true
	return target, nil
}

// Back navigates one step backward along the breadcrumb.
func (w *Walker) Back() (*Step, error) {
	if len(w.breadcrumb) == 0 {
		return nil, errors.New("walker: already at the first step")
	}
	prev := w.breadcrumb[len(w.breadcrumb)-1]
	w.breadcrumb = w.breadcrumb[:len(w.breadcrumb)-1]
	w.current = prev

	step, ok := w.g.Step(prev)
	if !ok {
		return nil, fmt.Errorf("walker: step %q not found", prev)
	}
	return step, nil
}

// NavigableEdges returns the navigable edges of the current step.
func (w *Walker) NavigableEdges() []*v1.StepEdge {
	cur, ok := w.g.Step(w.current)
	if !ok {
		return nil
	}
	var out []*v1.StepEdge
	for _, e := range cur.Edges {
		if e.Navigable {
			out = append(out, e)
		}
	}
	return out
}
