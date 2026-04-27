# Codewalker

An AI-powered code walkthrough service. Codewalker explains code by guiding
you through it step by step — like a debugger with narration. Navigate
forward and back through logical steps, follow branches, request rephrasing
at any level of detail, and look up unfamiliar terms via a built-in glossary.

Use it to understand unfamiliar code, onboard into a new codebase, or get
AI-narrated context on a pull request you're reviewing.

---

## How it works

Codewalker is a gRPC backend that runs as a Docker container alongside your
existing tools. IDE plugins and other clients connect to it over gRPC.

- **Walkthrough sessions** — open any file in a git repo and walk through it
  step by step. The server parses the AST, builds a step graph, and narrates
  each logical unit using Claude.
- **Review sessions** — paste a GitHub PR, commit, or comparison URL and walk
  through the diff hunk by hunk, with narration explaining what changed and why.

---

## Quickstart

**Prerequisites:** Docker, Docker Compose, an Anthropic API key.

```bash
git clone https://github.com/snootbeestci/codewalker
cd codewalker
export ANTHROPIC_API_KEY=sk-ant-...
export REPO_PATH=/path/to/repo/you/want/to/explain
docker-compose -f deploy/docker-compose.yml up
```

The server listens on `localhost:50051`.

**Smoke test** (requires `grpcurl` and `jq`):

```bash
make smoke-test
```

---

## Clients

| Client | Status |
|---|---|
| PhpStorm plugin | In development |
| VS Code extension | Planned |
| Browser extension | Planned |

The PhpStorm plugin source lives in `plugin/`. To build it:

```bash
cd plugin
# Requires GITHUB_ACTOR and GITHUB_TOKEN for proto stub resolution
./gradlew buildPlugin
```

---

## Development

**Prerequisites:** Go 1.24, buf, gcc (for tree-sitter CGO).

```bash
# Generate proto stubs
make proto

# Build
make build

# Test
make test

# Full CI check (mirrors GitHub Actions)
make ci
```

---

## Adding a language

Implement `LanguageHandler` in `internal/parser/languages/` and register it
via `init()` in `internal/parser/registry.go`. See `internal/parser/languages/golang.go`
for a reference implementation.

v1 languages: Go, TypeScript, Python, PHP.

---

## Adding a forge

Implement `ForgeHandler` in `internal/forge/forges/` and register it via
`init()`. See `internal/forge/forges/github.go` for a reference implementation.

Supported URL patterns (GitHub):
- `github.com/{owner}/{repo}/pull/{number}` — pull request
- `github.com/{owner}/{repo}/commit/{sha}` — commit
- `github.com/{owner}/{repo}/compare/{base}...{head}` — branch comparison

---

## Architecture

```
┌─────────────────────────────────────────────┐
│              gRPC Server (:50051)            │
│                                              │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  │
│  │   Git    │  │  Parser  │  │  Session  │  │
│  │ (go-git) │  │(tree-    │  │   Store   │  │
│  └──────────┘  │ sitter)  │  └───────────┘  │
│  ┌──────────┐  └──────────┘  ┌───────────┐  │
│  │  Forge   │  ┌──────────┐  │    LLM    │  │
│  │ Handlers │  │  Graph   │  │  Client   │  │
│  └──────────┘  │ Builder  │  └───────────┘  │
│                └──────────┘                  │
└─────────────────────────────────────────────┘
```

---

## Proto contract

The gRPC API is defined in `proto/codewalker/v1/codewalker.proto`.
Versioned Kotlin stubs are published to GitHub Packages on each release:

```kotlin
implementation("com.github.snootbeestci:codewalker-proto:{version}")
```

Check compatibility with `GetVersion` before opening a session:

```
rpc GetVersion(GetVersionRequest) returns (GetVersionResponse)
```

---

## Contributing

Two prompts are available to streamline AI-assisted development on this project:

- [Design session prompt](docs/DESIGN_PROMPT.md) — use this to start a Claude design session for planning features and reviewing PRs
- [Claude Code prompt](docs/CLAUDE_CODE_PROMPT.md) — paste this before an issue body when starting a Claude Code implementation session

The [briefing document](codewalker-briefing.md) is the source of truth for architectural decisions. Read it before making any structural changes.

---

## Licence

TBD
