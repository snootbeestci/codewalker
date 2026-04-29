package forge

import "testing"

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare hostname", "github.com", "github.com"},
		{"https scheme", "https://github.com", "github.com"},
		{"http scheme", "http://github.com", "github.com"},
		{"trailing slash", "github.com/", "github.com"},
		{"scheme and trailing slash", "https://github.com/", "github.com"},
		{"mixed case", "GitHub.com", "github.com"},
		{"upper case", "GITHUB.COM", "github.com"},
		{"leading whitespace", "  github.com", "github.com"},
		{"trailing whitespace", "github.com  ", "github.com"},
		{"surrounding whitespace", "  github.com  ", "github.com"},
		{"GHE-style subdomain", "github.mycompany.com", "github.mycompany.com"},
		{"GHE with scheme and case", "https://GitHub.MyCompany.Com", "github.mycompany.com"},
		{"non-standard port", "gitea.internal:3000", "gitea.internal:3000"},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeHost(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeHost(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
