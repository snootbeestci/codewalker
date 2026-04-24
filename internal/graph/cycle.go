package graph

// DetectCycles returns all step IDs that participate in a cycle.
// The graph must be built before calling this.
func DetectCycles(g *Graph) []string {
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var cyclicIDs []string

	var dfs func(id string)
	dfs = func(id string) {
		visited[id] = true
		inStack[id] = true

		step, ok := g.Step(id)
		if !ok {
			inStack[id] = false
			return
		}
		for _, e := range step.Edges {
			if e.TargetStepId == "" {
				continue
			}
			if !visited[e.TargetStepId] {
				dfs(e.TargetStepId)
			} else if inStack[e.TargetStepId] {
				// Found a back-edge — record both ends.
				if !contains(cyclicIDs, e.TargetStepId) {
					cyclicIDs = append(cyclicIDs, e.TargetStepId)
				}
				if !contains(cyclicIDs, id) {
					cyclicIDs = append(cyclicIDs, id)
				}
			}
		}
		inStack[id] = false
	}

	for id := range g.steps {
		if !visited[id] {
			dfs(id)
		}
	}
	return cyclicIDs
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
