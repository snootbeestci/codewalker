package git

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Manifest holds the parsed contents of a package manifest file.
type Manifest struct {
	// Kind is the manifest type: "gomod" | "npm" | "composer" | "pip" | "gem".
	Kind string
	// Dependencies maps package name → version string.
	Dependencies map[string]string
}

// DetectManifest searches repoRoot for a known package manifest and returns
// the first one found.  Returns nil if no manifest is found.
func DetectManifest(repoRoot string) *Manifest {
	type probe struct {
		file   string
		parser func(path string) *Manifest
	}
	probes := []probe{
		{"go.mod", parseGoMod},
		{"package.json", parseNPM},
		{"composer.json", parseComposer},
		{"requirements.txt", parsePip},
		{"Gemfile.lock", parseGem},
	}
	for _, p := range probes {
		path := filepath.Join(repoRoot, p.file)
		if m := p.parser(path); m != nil {
			return m
		}
	}
	return nil
}

// LookupVersion returns the version of packageName from m, or "".
func (m *Manifest) LookupVersion(packageName string) string {
	if m == nil {
		return ""
	}
	return m.Dependencies[packageName]
}

func parseGoMod(path string) *Manifest {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	deps := make(map[string]string)
	scanner := bufio.NewScanner(f)
	inRequire := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "require (" {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if inRequire || strings.HasPrefix(line, "require ") {
			line = strings.TrimPrefix(line, "require ")
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				deps[parts[0]] = parts[1]
			}
		}
	}
	return &Manifest{Kind: "gomod", Dependencies: deps}
}

func parseNPM(path string) *Manifest {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	deps := make(map[string]string)
	for k, v := range pkg.Dependencies {
		deps[k] = v
	}
	for k, v := range pkg.DevDependencies {
		deps[k] = v
	}
	return &Manifest{Kind: "npm", Dependencies: deps}
}

func parseComposer(path string) *Manifest {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pkg struct {
		Require map[string]string `json:"require"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return &Manifest{Kind: "composer", Dependencies: pkg.Require}
}

func parsePip(path string) *Manifest {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	deps := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// e.g. "requests==2.28.0" or "flask>=2.0"
		for _, sep := range []string{"==", ">=", "<=", "~=", "!="} {
			if idx := strings.Index(line, sep); idx >= 0 {
				deps[line[:idx]] = line[idx+len(sep):]
				break
			}
		}
		if _, ok := deps[line]; !ok {
			deps[line] = ""
		}
	}
	return &Manifest{Kind: "pip", Dependencies: deps}
}

func parseGem(path string) *Manifest {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	deps := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Gemfile.lock lines look like: "    rails (7.0.4)"
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, " ("); idx > 0 {
			name := line[:idx]
			ver := strings.TrimSuffix(line[idx+2:], ")")
			deps[name] = ver
		}
	}
	return &Manifest{Kind: "gem", Dependencies: deps}
}
