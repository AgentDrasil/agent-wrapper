package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		wantContain []string
	}{
		{
			name: "idle with high remaining (green)",
			input: `{
				"agent_state": "idle",
				"context_window": {
					"total_input_tokens": 88244,
					"context_window_size": 200000,
					"remaining_percentage": 91.58
				}
			}`,
			wantContain: []string{"state: idle", "input_tokens: 88244", "max: 200000", "remaining: ", "91.6%", ansiGreen, "tasks: 0", "subagents: 0"},
		},
		{
			name: "thinking with medium remaining (yellow)",
			input: `{
				"agent_state": "thinking",
				"context_window": {
					"total_input_tokens": 500000,
					"context_window_size": 1000000,
					"remaining_percentage": 52.0
				}
			}`,
			wantContain: []string{"state: thinking", "input_tokens: 500000", "max: 1000000", "remaining: ", "52.0%", ansiYellow, "tasks: 0", "subagents: 0"},
		},
		{
			name: "working with low remaining (red)",
			input: `{
				"agent_state": "working",
				"context_window": {
					"total_input_tokens": 990000,
					"context_window_size": 1048576,
					"remaining_percentage": 5.5
				}
			}`,
			wantContain: []string{"state: working", "input_tokens: 990000", "max: 1048576", "remaining: ", "5.5%", ansiRed, "tasks: 0", "subagents: 0"},
		},
		{
			name: "exactly 80 percent remaining (green)",
			input: `{
				"agent_state": "tool_use",
				"context_window": {
					"total_input_tokens": 200000,
					"context_window_size": 250000,
					"remaining_percentage": 80.0
				}
			}`,
			wantContain: []string{"state: tool_use", "input_tokens: 200000", "max: 250000", "remaining: ", "80.0%", ansiGreen, "tasks: 0", "subagents: 0"},
		},
		{
			name: "exactly 50 percent remaining (yellow)",
			input: `{
				"agent_state": "initializing",
				"context_window": {
					"total_input_tokens": 524288,
					"context_window_size": 1048576,
					"remaining_percentage": 50.0
				}
			}`,
			wantContain: []string{"state: initializing", "input_tokens: 524288", "max: 1048576", "remaining: ", "50.0%", ansiYellow, "tasks: 0", "subagents: 0"},
		},
		{
			name: "working with background tasks",
			input: `{
				"agent_state": "working",
				"context_window": {
					"total_input_tokens": 100000,
					"context_window_size": 1048576,
					"remaining_percentage": 90.0
				},
				"background_tasks": [
					{"name": "build", "status": "running", "index": 1},
					{"name": "test",  "status": "running", "index": 2}
				]
			}`,
			wantContain: []string{"state: working", "tasks: 2", "subagents: 0"},
		},
		{
			name: "thinking with active subagents",
			input: `{
				"agent_state": "thinking",
				"context_window": {
					"total_input_tokens": 100000,
					"context_window_size": 1048576,
					"remaining_percentage": 90.0
				},
				"subagents": [
					{"name": "research", "role": "Researcher", "status": "working"},
					{"name": "coder",    "role": "Coder",      "status": "idle"}
				]
			}`,
			wantContain: []string{"state: thinking", "tasks: 0", "subagents: 1"},
		},
		{
			name: "idle with all subagents idle",
			input: `{
				"agent_state": "idle",
				"context_window": {
					"total_input_tokens": 100000,
					"context_window_size": 1048576,
					"remaining_percentage": 90.0
				},
				"subagents": [
					{"name": "research", "role": "Researcher", "status": "idle"},
					{"name": "coder",    "role": "Coder",      "status": "idle"}
				]
			}`,
			wantContain: []string{"state: idle", "tasks: 0", "subagents: 0"},
		},
		{
			name:    "invalid JSON",
			input:   `not json`,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := run(strings.NewReader(tt.input))

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			for _, want := range tt.wantContain {
				assert.Contains(t, got, want)
			}
		})
	}
}

func TestRemainingColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pct  float64
		want string
	}{
		{100, ansiGreen},
		{80, ansiGreen},
		{79.9, ansiYellow},
		{50, ansiYellow},
		{49.9, ansiRed},
		{0, ansiRed},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.1f%%", tt.pct), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, remainingColor(tt.pct))
		})
	}
}
