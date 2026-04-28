package orderers

import (
	"testing"

	"github.com/yourorg/codewalker/internal/forge"
)

func TestEntryPointsFirst_Tier(t *testing.T) {
	e := &entryPointsFirst{}

	tests := []struct {
		name     string
		path     string
		wantTier int
	}{
		// Entry points (tier 10)
		{"go main", "cmd/server/main.go", 10},
		{"controllers dir", "src/controllers/auth_controller.go", 10},
		{"handlers dir", "internal/handlers/login.go", 10},
		{"routes dir", "app/routes/api.ts", 10},
		{"api dir", "src/api/users.ts", 10},
		{"kotlin main", "src/main/kotlin/com/example/Main.kt", 10},
		{"python main", "src/main.py", 10},
		{"ts index", "src/index.ts", 10},
		{"js index", "lib/index.js", 10},

		// Domain (tier 30)
		{"service dir", "src/service/payment.go", 30},
		{"services dir", "internal/services/billing.go", 30},
		{"domain dir", "internal/domain/user.go", 30},
		{"models dir", "src/models/order.py", 30},

		// Repositories/adapters (tier 50)
		{"repository dir", "src/repository/user_repo.go", 50},
		{"adapters dir", "internal/adapters/db.go", 50},
		{"dao dir", "src/dao/user_dao.go", 50},

		// Configuration / infrastructure (tier 60)
		{"config dir", "internal/config/loader.go", 60},
		{"deploy dir", "deploy/k8s.yaml", 60},
		{"yaml file", "settings.yaml", 60},
		{"yml file", "ci.yml", 60},
		{"dockerfile", "deploy/Dockerfile", 60},
		{"makefile", "Makefile", 60},

		// Documentation (tier 70)
		{"markdown", "README.md", 70},
		{"docs dir", "src/docs/architecture.go", 70},

		// Tests (tier 80)
		{"go test file", "internal/foo/bar_test.go", 80},
		{"test dir", "src/test/foo.ts", 80},
		{"tests dir", "src/tests/foo.ts", 80},
		{"jest test", "src/foo.test.ts", 80},
		{"jest spec", "src/foo.spec.ts", 80},
		{"python test underscore", "tests/test_login.py", 80},
		{"python _test.py", "src/foo_test.py", 80},

		// Default (tier 40)
		{"random go file", "internal/util/strings.go", 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.tier(tt.path)
			if got != tt.wantTier {
				t.Errorf("tier(%q) = %d, want %d", tt.path, got, tt.wantTier)
			}
		})
	}
}

func TestEntryPointsFirst_Order(t *testing.T) {
	e := &entryPointsFirst{}

	files := []*forge.ReviewFile{
		{Path: "internal/util/strings.go"},
		{Path: "internal/foo/bar_test.go"},
		{Path: "src/services/payment.go"},
		{Path: "cmd/server/main.go"},
		{Path: "src/repository/user_repo.go"},
		{Path: "README.md"},
		{Path: "src/controllers/auth.go"},
	}

	got := e.Order(files)

	// Entry points first.
	if got[0].Path != "cmd/server/main.go" && got[0].Path != "src/controllers/auth.go" {
		t.Errorf("first entry should be a tier-10 file, got %q", got[0].Path)
	}

	// Tests should be last.
	if got[len(got)-1].Path != "internal/foo/bar_test.go" {
		t.Errorf("last entry should be the test file, got %q", got[len(got)-1].Path)
	}

	// Order must follow tier ascending; verify pairwise.
	for i := 0; i < len(got)-1; i++ {
		ti, tj := e.tier(got[i].Path), e.tier(got[i+1].Path)
		if ti > tj {
			t.Errorf("tier ordering broken at index %d: %q (tier %d) before %q (tier %d)",
				i, got[i].Path, ti, got[i+1].Path, tj)
		}
	}
}

func TestEntryPointsFirst_StableSecondarySort(t *testing.T) {
	e := &entryPointsFirst{}

	// All files in the default tier — should be sorted alphabetically.
	files := []*forge.ReviewFile{
		{Path: "internal/util/zebra.go"},
		{Path: "internal/util/alpha.go"},
		{Path: "internal/util/middle.go"},
	}

	got := e.Order(files)

	wantOrder := []string{
		"internal/util/alpha.go",
		"internal/util/middle.go",
		"internal/util/zebra.go",
	}
	for i, w := range wantOrder {
		if got[i].Path != w {
			t.Errorf("position %d: got %q, want %q", i, got[i].Path, w)
		}
	}
}

func TestEntryPointsFirst_DoesNotMutateInput(t *testing.T) {
	e := &entryPointsFirst{}

	original := []*forge.ReviewFile{
		{Path: "z.go"},
		{Path: "a.go"},
	}
	originalFirst := original[0].Path

	_ = e.Order(original)

	if original[0].Path != originalFirst {
		t.Errorf("input was mutated: original[0] = %q, want %q", original[0].Path, originalFirst)
	}
}

func TestEntryPointsFirst_NameAndDescription(t *testing.T) {
	e := &entryPointsFirst{}
	if e.Name() != "entry-points-first" {
		t.Errorf("Name() = %q, want %q", e.Name(), "entry-points-first")
	}
	if e.Description() == "" {
		t.Error("Description() must not be empty")
	}
}
