# Codewalker — Session Briefing

This document gives future Claude Code sessions the context needed to contribute
to codewalker without re-deriving decisions already made.

---

## What is codewalker?

Codewalker is a gRPC service that helps developers understand code by narrating
it step-by-step. It parses source files into an AST, converts the AST into a
step graph, and streams LLM-generated narration as users navigate the graph.

Two session types exist:
- **Walkthrough sessions** — navigate and explain an existing source file.
- **Review sessions** — explain code changes from a PR, commit, or branch
  comparison.

---

## Initial implementation

### Architecture overview

The server exposes a single gRPC service (`CodeWalker`) defined in
`proto/codewalker/v1/codewalker.proto`. All narration RPCs
(`Navigate`, `Rephrase`, `ExpandTerm`) stream tokens back to the client so the
UI can render progressively.

### Key packages

| Package | Responsibility |
|---|---|
| `internal/session` | In-memory session store with TTL eviction |
| `internal/parser` | Language-agnostic AST → IR, multi-language plugin registry |
| `internal/graph` | Step graph construction and walker (immutable graph, mutable traversal) |
| `internal/llm` | LLM provider abstraction; Anthropic SDK implementation |
| `internal/forge` | Forge-agnostic PR/commit fetch; ForgeHandler plugin registry |
| `internal/git` | Read files from git refs, detect package manifests |
| `internal/resolver` | Resolve external call docs/source URLs via manifests then LLM |
| `server` | gRPC handlers + middleware (recovery, structured logging) |

### Plugin registries

Both language parsers and forge handlers use the same registry pattern:
implementations import-register themselves via `init()` in their own package,
so the core packages stay decoupled from concrete implementations.

- `internal/parser/languages/` — Go, TypeScript, Python, PHP
- `internal/forge/forges/` — GitHub

### Session lifecycle

1. Client calls `OpenSession` (or `OpenReviewSession`).
2. Server parses/fetches content, builds the step graph, streams `SessionProgress`
   events, then `SessionReady` with the full graph and glossary.
3. Client navigates with `Navigate`; server streams narration tokens then
   `StepComplete`.
4. `CloseSession` releases state; background TTL eviction reclaims idle sessions.

### Experience levels

`ExperienceLevel` (JUNIOR / MID / SENIOR) controls narration depth.
`session.EffectiveLevel` maps the enum to an integer 1–10 scale.
Planned v2: adaptive level based on observed user behaviour.

### External call resolution

When a step calls an external symbol, the resolver runs a cascade:
1. Package manifest (go.mod / package.json / composer.json) → pinned docs + source URL.
2. Language-known fallback → unversioned docs URL.
3. LLM summary-only fallback.

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

## What to build first

The initial implementation is complete and working. Current focus is review
session support. Work through GitHub issues #1–#4 in order:

1. Proto update (issue #1) — must be first, everything else depends on it
2. ForgeHandler interface + registry (issue #2)
3. GitHub ForgeHandler implementation (issue #3)
4. OpenReviewSession handler (issue #4)
