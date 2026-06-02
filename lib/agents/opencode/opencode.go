package opencode

import (
	"context"
	"errors"
	"time"
)

// UsageOptions controls how Usage behaves.
type UsageOptions struct {
	Dir           string
	StartupDelay  time.Duration
	ResponseDelay time.Duration
}

// PromptOptions controls how Prompt behaves.
type PromptOptions struct {
	Dir           string
	SessionID     string
	StartupDelay  time.Duration
	ResponseDelay time.Duration
	Model         string
}

// ModelUsage represents the quota status for a single model.
type ModelUsage struct {
	Model       string  `json:"model"`
	Remaining   float64 `json:"remaining"`
	RefreshDate int64   `json:"refresh_date,omitempty"`
}

// PromptResult is the structured response from a Prompt call.
type PromptResult struct {
	SessionID   string  `json:"session_id"`
	InputTokens int     `json:"input_tokens"`
	MaxTokens   int     `json:"max_tokens"`
	Remaining   float64 `json:"remaining"`
	LastContent string  `json:"last_content"`
}

// Usage is a TODO placeholder for fetching opencode usage.
func Usage(ctx context.Context, opts UsageOptions) ([]ModelUsage, error) {
	// TODO: Implement opencode usage tracking
	return nil, errors.New("opencode usage not implemented")
}

// Prompt is a TODO placeholder for sending a prompt to opencode.
func Prompt(ctx context.Context, prompt string, opts PromptOptions) (*PromptResult, error) {
	// TODO: Implement opencode prompting
	return nil, errors.New("opencode prompting not implemented")
}
