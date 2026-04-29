package forge

import "strings"

// NormalizeHost canonicalises a forge host string so that server-side keying
// and forge handler dispatch agree with the client. Clients are expected to
// send the canonical form; this is a defensive normalisation applied at
// handler entry. The rules are documented in the briefing — see the
// "Host strings on RPC requests" bullet under Development rules → Code.
func NormalizeHost(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimSuffix(s, "/")
	return strings.ToLower(s)
}
