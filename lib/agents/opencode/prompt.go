package opencode

import (
	"context"
	"errors"
	"time"
)

// PromptOptions controls how Prompt behaves.
type PromptOptions struct {
	Dir           string
	SessionID     string
	StartupDelay  time.Duration
	ResponseDelay time.Duration
	Model         string
}

// PromptResult is the structured response from a Prompt call.
type PromptResult struct {
	SessionID   string  `json:"session_id"`
	InputTokens int     `json:"input_tokens"`
	MaxTokens   int     `json:"max_tokens"`
	Remaining   float64 `json:"remaining"`
	LastContent string  `json:"last_content"`
}

// Prompt is a TODO placeholder for sending a prompt to opencode.
func Prompt(ctx context.Context, prompt string, opts PromptOptions) (*PromptResult, error) {
	// TODO: Implement opencode prompting
	return nil, errors.New("opencode prompting not implemented")
}
