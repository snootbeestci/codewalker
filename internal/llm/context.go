package llm

import (
	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// WindowedMessages builds the minimal message list for a single LLM call.
// The LLM context window is managed server-side; clients never see it.
//
// v1: each call is stateless — system + one user turn.
// v2: maintain a short rolling history per session for coherent rephrases.
func WindowedMessages(userContent string) []anthropic.MessageParam {
	return []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userContent)),
	}
}
