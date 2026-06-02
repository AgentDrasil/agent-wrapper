package opencode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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

// Usage runs "opencode models", parses the list of models, and returns a ModelUsage list with Remaining = 1.0.
func Usage(ctx context.Context, opts UsageOptions) ([]ModelUsage, error) {
	cmd := exec.CommandContext(ctx, "opencode", "models")
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running opencode models: %w", err)
	}

	var result []ModelUsage
	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		result = append(result, ModelUsage{
			Model:     trimmed,
			Remaining: 1.0,
		})
	}

	return result, nil
}

// Prompt is a TODO placeholder for sending a prompt to opencode.
func Prompt(ctx context.Context, prompt string, opts PromptOptions) (*PromptResult, error) {
	// TODO: Implement opencode prompting
	return nil, errors.New("opencode prompting not implemented")
}
