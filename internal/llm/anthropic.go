package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/yourorg/codewalker/internal/llm/prompts"
)

const (
	// model is the only place the Claude model name appears in this codebase.
	model     = anthropic.ModelClaudeSonnet4_6
	maxTokens = 2048
)

// AnthropicProvider implements Provider using the Anthropic Claude API.
type AnthropicProvider struct {
	client anthropic.Client
}

// NewAnthropicProvider creates a new AnthropicProvider using apiKey.
func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{client: anthropic.NewClient(option.WithAPIKey(apiKey))}
}

// Narrate streams a narration for the given step.
// Prompt assembly is delegated entirely to BuildMessages via StepContext.
func (p *AnthropicProvider) Narrate(ctx context.Context, req NarrateRequest) (<-chan string, error) {
	slog.Debug("narrate request", "language", req.Language, "effective_level", req.Level, "code_len", len(req.Code))
	system, messages := BuildMessages(StepContext{
		Language:        req.Language,
		SymbolSignature: req.StepLabel,
		CallChain:       req.CallChain,
		RawSource:       req.Code,
		EffectiveLevel:  uint32(req.Level),
	})
	return p.stream(ctx, system, messages)
}

// Rephrase streams a rephrased narration.
func (p *AnthropicProvider) Rephrase(ctx context.Context, req RephraseRequest) (<-chan string, error) {
	system, user := prompts.Rephrase(req.Code, req.Language, req.Mode, req.Level)
	return p.stream(ctx, system, oneMessage(user))
}

// SummarizeExternalCall returns a plain-text summary of an external symbol.
func (p *AnthropicProvider) SummarizeExternalCall(ctx context.Context, pkg, symbol, language string) (string, error) {
	system, user := prompts.ExternalCall(pkg, symbol, language)
	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 512,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  oneMessage(user),
	})
	if err != nil {
		return "", fmt.Errorf("anthropic: summarize external call: %w", err)
	}
	if len(msg.Content) == 0 {
		return "", fmt.Errorf("anthropic: empty response")
	}
	return msg.Content[0].Text, nil
}

// ExtractGlossaryTerms calls the LLM to identify glossary candidates.
func (p *AnthropicProvider) ExtractGlossaryTerms(ctx context.Context, req GlossaryRequest) ([]GlossaryCandidate, error) {
	system, user := prompts.GlossaryExtract(req.Code, req.Language, req.Level)
	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 1024,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  oneMessage(user),
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic: extract glossary terms: %w", err)
	}
	if len(msg.Content) == 0 {
		return nil, nil
	}

	raw := strings.TrimSpace(msg.Content[0].Text)
	// Strip markdown code fences if present.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")

	var candidates []GlossaryCandidate
	if err := json.Unmarshal([]byte(raw), &candidates); err != nil {
		// Non-fatal: return empty rather than failing the session open.
		return nil, nil
	}
	return candidates, nil
}

// ExpandTerm streams an expanded definition of a glossary term.
func (p *AnthropicProvider) ExpandTerm(ctx context.Context, req ExpandTermRequest) (<-chan string, error) {
	system, user := prompts.ExpandTerm(req.Term, req.Context, req.Language, req.Level)
	return p.stream(ctx, system, oneMessage(user))
}

// stream creates a streaming Messages request and returns a channel of tokens.
// The channel is closed when the stream ends or the context is cancelled.
func (p *AnthropicProvider) stream(ctx context.Context, system string, messages []anthropic.MessageParam) (<-chan string, error) {
	ch := make(chan string, 32)

	s := p.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  messages,
	})

	go func() {
		defer close(ch)
		tokenCount := 0
		slog.Debug("anthropic stream started")
		for s.Next() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			event := s.Current()
			switch e := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				if delta, ok := e.Delta.AsAny().(anthropic.TextDelta); ok {
					tokenCount++
					ch <- delta.Text
				}
			}
		}
		// s.Err() is the only way to observe stream errors — the error cannot be
		// returned from stream() because this goroutine outlives that call.
		if err := s.Err(); err != nil {
			slog.Error("anthropic stream error", "error", err, "tokens_received", tokenCount)
		} else {
			slog.Debug("anthropic stream closed", "token_count", tokenCount)
		}
	}()

	return ch, nil
}
