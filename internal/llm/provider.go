package llm

import "context"

// Provider is the single interface all LLM backends must satisfy.
// v1 implementation: internal/llm/anthropic.go
// v2 planned: Ollama (fully local / air-gapped)
type Provider interface {
	// Narrate streams a step-by-step narration for the given code slice.
	Narrate(ctx context.Context, req NarrateRequest) (<-chan string, error)

	// Rephrase streams a rephrased narration for the current step.
	Rephrase(ctx context.Context, req RephraseRequest) (<-chan string, error)

	// SummarizeExternalCall returns an LLM-generated summary for a call to an
	// external symbol (stdlib or third-party).  Used when no docs URL is found.
	SummarizeExternalCall(ctx context.Context, pkg, symbol, language string) (string, error)

	// ExtractGlossaryTerms returns glossary candidates for a code slice.
	ExtractGlossaryTerms(ctx context.Context, req GlossaryRequest) ([]GlossaryCandidate, error)

	// ExpandTerm streams an expanded definition of a glossary term.
	ExpandTerm(ctx context.Context, req ExpandTermRequest) (<-chan string, error)

	// GenerateStepSummary returns a structured key/value summary for a single
	// review hunk. Non-streaming — the response is short and arrives as a block.
	GenerateStepSummary(ctx context.Context, req SummaryRequest) (*StepSummary, error)
}

// NarrateRequest carries everything needed to narrate a single step.
type NarrateRequest struct {
	Code      string
	Language  string
	StepLabel string
	StepKind  string
	// CallChain is the breadcrumb of step labels leading to this one.
	CallChain []string
	// Variables holds in-scope variable names and inferred types.
	Variables []string
	// Level is the effective experience level (1–10).
	Level int
}

// RephraseRequest carries the original narration plus the rephrase mode.
type RephraseRequest struct {
	NarrateRequest
	Mode string // "SIMPLER" | "DEEPER" | "ANALOGY" | "TLDR"
}

// GlossaryRequest carries the code slice and language for glossary extraction.
type GlossaryRequest struct {
	Code     string
	Language string
	Level    int
}

// GlossaryCandidate is a term extracted from code for the glossary.
type GlossaryCandidate struct {
	Term string
	Kind string // "LANGUAGE" | "PATTERN" | "DOMAIN" | "LIBRARY"
}

// ExpandTermRequest carries the term to expand and conversational context.
type ExpandTermRequest struct {
	Term     string
	Context  string
	Language string
	Level    int
}

// SummaryRequest carries everything needed to produce a structured triage
// summary for a single review hunk.
type SummaryRequest struct {
	Language      string
	HunkDiff      string
	ContextBefore string
	ContextAfter  string
}

// StepSummary is the structured triage payload returned alongside narration
// for review session steps. Every field is populated — empty values are "—".
type StepSummary struct {
	Breaking      string
	Risk          string
	WhatChanged   string
	SideEffects   string
	Tests         string
	ReviewerFocus string
	Suggestion    string
	Confidence    string
}
