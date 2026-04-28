package orderers

import (
	"sort"
	"strings"

	"github.com/yourorg/codewalker/internal/forge"
)

func init() {
	forge.RegisterOrderer(&entryPointsFirst{})
}

type entryPointsFirst struct{}

func (e *entryPointsFirst) Name() string {
	return "entry-points-first"
}

func (e *entryPointsFirst) Description() string {
	return "Entry points first, then domain logic, infrastructure, and tests last"
}

// tier returns a sort key for a file path. Lower tiers come first.
func (e *entryPointsFirst) tier(path string) int {
	p := strings.ToLower(path)

	// Tests last
	if isTest(p) {
		return 80
	}

	// Documentation
	if strings.HasSuffix(p, ".md") || strings.Contains(p, "/docs/") {
		return 70
	}

	// Configuration and infrastructure
	if strings.Contains(p, "/config/") ||
		strings.Contains(p, "/deploy/") ||
		strings.HasSuffix(p, ".yaml") ||
		strings.HasSuffix(p, ".yml") ||
		strings.HasSuffix(p, "dockerfile") ||
		strings.HasSuffix(p, "makefile") {
		return 60
	}

	// Repositories, adapters
	if strings.Contains(p, "/repository/") ||
		strings.Contains(p, "/repositories/") ||
		strings.Contains(p, "/adapter/") ||
		strings.Contains(p, "/adapters/") ||
		strings.Contains(p, "/dao/") {
		return 50
	}

	// Domain models and services
	if strings.Contains(p, "/service/") ||
		strings.Contains(p, "/services/") ||
		strings.Contains(p, "/domain/") ||
		strings.Contains(p, "/model/") ||
		strings.Contains(p, "/models/") {
		return 30
	}

	// Entry points
	if strings.Contains(p, "/controller/") ||
		strings.Contains(p, "/controllers/") ||
		strings.Contains(p, "/handler/") ||
		strings.Contains(p, "/handlers/") ||
		strings.Contains(p, "/route/") ||
		strings.Contains(p, "/routes/") ||
		strings.Contains(p, "/api/") ||
		strings.HasSuffix(p, "/main.go") ||
		strings.Contains(p, "/cmd/") ||
		strings.HasSuffix(p, "main.kt") ||
		strings.HasSuffix(p, "main.py") ||
		strings.HasSuffix(p, "index.ts") ||
		strings.HasSuffix(p, "index.js") {
		return 10
	}

	// Default — between domain and infrastructure
	return 40
}

func isTest(p string) bool {
	return strings.HasSuffix(p, "_test.go") ||
		strings.Contains(p, "/test/") ||
		strings.Contains(p, "/tests/") ||
		strings.Contains(p, ".test.") ||
		strings.Contains(p, ".spec.") ||
		strings.Contains(p, "_test.py") ||
		strings.Contains(p, "test_") && strings.HasSuffix(p, ".py")
}

func (e *entryPointsFirst) Order(files []*forge.ReviewFile) []*forge.ReviewFile {
	out := make([]*forge.ReviewFile, len(files))
	copy(out, files)
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := e.tier(out[i].Path), e.tier(out[j].Path)
		if ti != tj {
			return ti < tj
		}
		return out[i].Path < out[j].Path
	})
	return out
}
