# Codewalker — Claude Code Briefing

This document captures all architectural decisions made during design. Do not
re-litigate these decisions unless you encounter a hard technical blocker.
Ask the user before changing anything structural.

---

## What is this?

An AI-powered code walkthrough service. It explains code by guiding a user
through it step by step, like a debugger with narration. Users can navigate
forward/back, follow logical branches, request rephrasing, and look up
unfamiliar terms via a glossary.

Target audience: developers of all levels, debugging or reviewing unfamiliar
code. The service adapts its narration to the user's experience level.

---

## Tech stack

| Concern | Choice | Reason |
|---|---|---|
| Language | Go | go-git, tree-sitter bindings, static binary, goroutines for streaming |
| Protocol | gRPC + protobuf | Bidirectional streaming, strongly typed contract, generated client stubs |
| Git | go-git | Pure Go, no libgit2 dependency |
| AST parsing | tree-sitter (Go bindings) | Language-agnostic, ~100 languages, query-based node extraction |
| Protobuf tooling | buf | Linting, breaking change detection, code generation |
| LLM | Anthropic Claude (v1) | Swappable behind an interface |
| Deployment | Docker (distroless image) | Developer-friendly, sidesteps dependency issues, repo mounted as volume |

---

## Architecture decisions

### Session model
- On `OpenSession`, the server does **one analysis pass**: parse AST, build
  step graph, extract glossary candidates. This is stored as server-side
  session state.
- Subsequent LLM calls send only the **current step's code slice + minimal
  context** (function signature, call chain, variable types), not the whole
  file. The session object manages context windowing.
- Sessions are in-memory in v1. Persistence is a v2 concern.

### Step graph
- Steps are **logical units** determined by the AST — conditionals, loops,
  function calls, assignments — not individual lines.
- The graph is a **graph, not a list**. A conditional has two outgoing edges
  (TRUE_BRANCH, FALSE_BRANCH), a switch has N CASE edges, loops have a body
  and an exit path.
- Steps can be revisited (cycles handled explicitly — see cycle detection).

### Symbol scoping
- `OpenSessionRequest.symbol` sets the **entry point**, not a hard boundary.
- `EDGE_LABEL_CALL` edges into other symbols defined in the same repo are
  always included as navigable edges.
- The user is never automatically taken into a callee — they must explicitly
  follow the edge — but they are never blocked from doing so.
- Callees outside the repo (stdlib, third-party) are non-navigable but still
  appear as edges with `ExternalCallInfo`.

### External calls
Resolution follows a cascade — never leave the user with nothing:
1. If a package manifest is found (go.mod, composer.json, package.json,
   Gemfile, requirements.txt etc.) — resolve exact version, construct pinned
   docs and source URLs.
2. If language is known but no manifest — attempt to construct a docs URL
   from the symbol name alone, flagged as unversioned.
3. **Fallback in all cases** — LLM summary, clearly attributed as general
   knowledge. `llm_summary` is always populated. URLs are best-effort and
   only included if verified reachable. Do not surface broken links.

### LLM provider
- Defined as an interface (`internal/llm/provider.go`) from day one.
- Claude (Anthropic) is the v1 implementation.
- Ollama should be the v2 implementation (fully local / air-gapped use case).
- Never hardcode model names or API endpoints outside `internal/llm/anthropic.go`.

### Experience level + adaptation
- `OpenSessionRequest.experience_level` (JUNIOR/MID/SENIOR) is the starting
  hint. It maps to a 1–10 `effective_level` on the server.
- `effective_level` is exposed read-only in `SessionSummary` so clients can
  display it (e.g. "explaining at level 4/10").
- `internal/session/adaptation.go` exists as a **stub in v1** — it simply
  maps ExperienceLevel to a fixed value. Do not implement adaptive logic yet.
  The interface must exist so v2 can fill it in without touching other code.
- v2 adaptive signals (for future reference): SIMPLER rephrase count, DEEPER
  rephrase count, glossary terms expanded, steps navigated without rephrasing.

### raw_source
- `SourceSpan.raw_source` is populated by default — clients that cannot read
  files themselves (browser extensions, web UIs) need it.
- `OpenSessionRequest.omit_raw_source = true` lets clients that have file
  access (IDE plugins) opt out to reduce message size.

---

## Proto contract

The proto file (`proto/codewalker/v1/codewalker.proto`) is the source of
truth. Key messages:

- `OpenSessionRequest` — repo path, file path, git ref, optional symbol
  scope, experience level, omit_raw_source flag
- `SessionEvent` (streamed) — progress updates → `SessionReady` (step graph,
  glossary, language, total steps)
- `SessionSummary` — includes read-only `effective_level`
- `Step` — id, label, source span, edges, visited flag, StepKind
- `StepEdge` — target_step_id, EdgeLabel, description, `navigable` bool,
  `ExternalCallInfo` (only set when navigable = false)
- `ExternalCallInfo` — package_name, symbol_name, llm_summary (always set),
  docs_url, source_url, version (last three best-effort only)
- `NarrateEvent` (streamed) — token stream → `StepComplete` (new glossary
  terms, available edges, breadcrumb)
- `RephraseRequest` — session_id + RephraseMode (SIMPLER/DEEPER/ANALOGY/TLDR)
- `GlossaryTerm` — term, definition, step_id, TermKind
  (LANGUAGE/PATTERN/DOMAIN/LIBRARY)

Use `buf` for all proto linting and code generation. Never run `protoc` directly.

---

## Language support

### v1 (build these)
- Go, TypeScript, Python, PHP

### v2
- JavaScript, Ruby, Java, C#, Shell

### v3
- PLpgSQL, PLSQL, TSQL, Apex

### v4
- Vue, HTML, CSS, SCSS, Dockerfile, Makefile, Gherkin, EJS, Jupyter Notebook

Each language is a `LanguageHandler` implementation in
`internal/parser/languages/`. The interface is defined in
`internal/parser/handler.go`. Adding a language must not require changes
outside that directory and the registry in `internal/parser/registry.go`.

**Jupyter Notebook note**: not a language — it's a JSON container format
wrapping Python cells. Extract code cells, treat as Python. The cell
structure maps naturally onto the step concept and is a compelling use case.

---

## Project structure

```
codewalker/
├── proto/codewalker/v1/codewalker.proto
├── gen/codewalker/v1/                    # Generated — do not edit
├── internal/
│   ├── git/
│   │   ├── client.go                     # go-git wrapper
│   │   └── manifest.go                   # Package manifest detection
│   ├── parser/
│   │   ├── parser.go                     # Language detection + dispatch
│   │   ├── registry.go                   # Language handler registry
│   │   ├── handler.go                    # LanguageHandler interface
│   │   └── languages/
│   │       ├── golang.go
│   │       ├── typescript.go
│   │       ├── python.go
│   │       └── php.go
│   ├── graph/
│   │   ├── builder.go                    # Builds Step graph from parser output
│   │   ├── walker.go                     # Traversal — forward, back, follow edge
│   │   └── cycle.go                      # Cycle detection
│   ├── llm/
│   │   ├── provider.go                   # Provider interface
│   │   ├── anthropic.go                  # Claude implementation
│   │   ├── context.go                    # Context windowing
│   │   └── prompts/
│   │       ├── narrate.go
│   │       ├── rephrase.go
│   │       ├── glossary.go
│   │       └── external.go
│   ├── session/
│   │   ├── session.go
│   │   ├── store.go
│   │   └── adaptation.go                 # v2 stub — keep the interface, no logic yet
│   ├── forge/
│   │   ├── handler.go                    # ForgeHandler interface
│   │   ├── context.go                    # ForgeContext and related types
│   │   ├── registry.go                   # Forge handler registry
│   │   ├── error.go                      # Typed forge errors
│   │   └── forges/
│   │       └── github.go                 # GitHub ForgeHandler implementation
│   └── resolver/
│       ├── resolver.go
│       └── ecosystems/
│           ├── gomod.go
│           ├── npm.go
│           ├── composer.go
│           └── fallback.go
├── server/
│   ├── server.go
│   ├── open_session.go
│   ├── navigate.go
│   ├── rephrase.go
│   ├── expand_term.go
│   ├── open_review_session.go
│   ├── version.go
│   └── middleware/
│       ├── logging.go
│       └── recovery.go
├── cmd/codewalker/main.go
├── config/config.go
├── deploy/
│   ├── Dockerfile
│   └── docker-compose.yml
├── buf.yaml
├── buf.gen.yaml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Dockerfile approach

Multi-stage build:
1. **Build stage** — `golang:1.24-alpine`, compile static binary
2. **Runtime stage** — `gcr.io/distroless/static` or `scratch`, copy binary only

The repo to be explained is mounted as a read-only volume at `/repos/target`.
The user sets `REPO_PATH` env var; docker-compose handles the mount.

```yaml
# docker-compose.yml sketch
services:
  codewalker:
    build: .
    ports:
      - "50051:50051"
    volumes:
      - ${REPO_PATH}:/repos/target:ro
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
```

---

## v1 milestone

**The service should be able to explain its own Go source code.**

This means:
- `OpenSession` works against a local Go repo
- Step graph is built correctly for Go source
- Navigation (forward, back, follow edge) works
- Narration streams from Claude
- Glossary terms are identified and expandable
- Rephrasing works (all four modes)
- External calls to Go stdlib surface an LLM summary + pkg.go.dev link

---

## Review sessions

Review sessions are a second session type alongside walkthroughs. They explain
code changes (PRs, commits, branch comparisons) rather than walking through
existing code.

### Key design decisions

- `OpenReviewSession` accepts a URL. The server parses it and routes to the
  correct ForgeHandler. The client never needs to know which forge it is.
- `ForgeHandler` is an interface in `internal/forge/handler.go`, registered
  via init() in `internal/forge/forges/`. Same pattern as LanguageHandler.
- GitHub is the v1 forge. GitLab, Gitea, Bitbucket are additive.
- Token resolution order (plugin-side):
    1. forge_token field on OpenReviewSessionRequest if supplied by client
    2. handler.ResolveToken() — checks gh CLI silently
    3. Empty string — public repo mode
  The backend never stores tokens beyond the session lifetime. Never log them.
- Once open, review sessions use the same Navigate/Rephrase/ExpandTerm RPCs
  as walkthrough sessions. The step graph contains STEP_KIND_HUNK steps
  instead of AST steps.
- HunkSpan carries context_before and context_after fetched from the base ref.
  The LLM needs surrounding context to narrate a diff meaningfully.
- The narration prompt detects unified diff input (starts with @@) and frames
  the narration as "what changed and why" rather than "what does this do".
- ForgeContext.pr_description is passed to the LLM as session context — it
  meaningfully affects narration quality.

### URL patterns supported (GitHub)
- github.com/{owner}/{repo}/pull/{number}       → PR
- github.com/{owner}/{repo}/commit/{sha}         → commit
- github.com/{owner}/{repo}/compare/{base}...{head} → comparison

### Not in scope for v1
- OAuth flow — use gh CLI token or manual token entry
- GitLab, Gitea, Bitbucket forge handlers
- Review comment annotation
- Suggested change generation

---

## File ordering

Review sessions present changed files in an order chosen by a `FileOrderer`
implementation. The strategy is selected per-session via
`OpenReviewSessionRequest.file_ordering`. Default is `entry-points-first`.

- Implementations live in `internal/forge/orderers/` and register via init()
- `ListFileOrderers` RPC lets clients discover available strategies at runtime
- Adding a new orderer must not require changes outside `internal/forge/orderers/`
- File ordering is applied before the step graph is built, so step order
  matches file order automatically

---

## What to build first

The initial implementation is complete, including review session support
(OpenReviewSession, ForgeHandler interface, GitHub forge implementation).
All issues #1–#5 are merged. The next focus is the PhpStorm plugin client.

---

## Config

Read from environment variables. No config files in v1.

| Var | Default | Notes |
|---|---|---|
| `ANTHROPIC_API_KEY` | — | Required |
| `CODEWALKER_PORT` | `50051` | gRPC listen port |
| `CODEWALKER_REPO_ROOT` | `/repos/target` | Mount point inside container |
| `CODEWALKER_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `CODEWALKER_LLM_PROVIDER` | `anthropic` | Extensible for v2 Ollama support |

---

## Versioning

- Server version and proto major version are injected at build time via ldflags
  into `server.Version` and `server.ProtoMajor`
- `GetVersion` RPC returns these values — clients call it on connect to detect
  incompatible backend versions
- `min_compatible_proto_major` lets the server declare backwards compatibility
  explicitly — increment `ProtoMajor` on breaking proto changes only
- When adding a new RPC or message (non-breaking), bump the minor version tag
- When making a breaking proto change, bump `ProtoMajor` in the Dockerfile
  ldflags AND in the default value in `server/version.go`

---

## Release process

- Before tagging a release, run `make release-dry-run` to validate the Gradle
  publish setup locally. This catches build script issues without burning
  version numbers.

---

## Development rules

### Proto
- Never run `protoc` directly — always use `make proto` to regenerate
- Never edit files in `gen/` — they are always derived from `proto/`
- `buf lint` must pass before submitting a PR
- When adding a new RPC or message (non-breaking), bump the minor version tag
- When making a breaking proto change, increment `ProtoMajor` in both `deploy/Dockerfile` ldflags and the default in `server/version.go`

### Code
- Prefer table-driven tests
- All gRPC streaming handlers must check `ctx.Done()` in their token loops
- Never log sensitive values — `forge_token` and `ANTHROPIC_API_KEY` must never appear in logs
- New language handlers go in `internal/parser/languages/` and register via `init()`
- New forge handlers go in `internal/forge/forges/` and register via `init()`
- Mock implementations for testing go in `internal/llm/llmtest/` — never in production packages
- The LLM context window is the server's responsibility — clients should never need to manage prompt history
- When in doubt about a design decision, check this document before asking. If it is not covered here, ask the user

### Tooling
- `make ci` — runs buf lint, go vet, go build, go test. Must pass before any PR
- `make smoke-test` — runs a live end-to-end test against a running container. Requires `ANTHROPIC_API_KEY` and `REPO_PATH`. Do not run in CI
- `make proto` — regenerates all Go stubs from proto. Run after any proto change
