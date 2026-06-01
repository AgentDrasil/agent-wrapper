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
					"remaining_percentage": 91.58
				}
			}`,
			wantContain: []string{"idle", "88244", "91.6%", ansiGreen},
		},
		{
			name: "thinking with medium remaining (yellow)",
			input: `{
				"agent_state": "thinking",
				"context_window": {
					"total_input_tokens": 500000,
					"remaining_percentage": 52.0
				}
			}`,
			wantContain: []string{"thinking", "500000", "52.0%", ansiYellow},
		},
		{
			name: "working with low remaining (red)",
			input: `{
				"agent_state": "working",
				"context_window": {
					"total_input_tokens": 990000,
					"remaining_percentage": 5.5
				}
			}`,
			wantContain: []string{"working", "990000", "5.5%", ansiRed},
		},
		{
			name: "exactly 80 percent remaining (green)",
			input: `{
				"agent_state": "tool_use",
				"context_window": {
					"total_input_tokens": 200000,
					"remaining_percentage": 80.0
				}
			}`,
			wantContain: []string{"tool_use", "200000", "80.0%", ansiGreen},
		},
		{
			name: "exactly 50 percent remaining (yellow)",
			input: `{
				"agent_state": "initializing",
				"context_window": {
					"total_input_tokens": 524288,
					"remaining_percentage": 50.0
				}
			}`,
			wantContain: []string{"initializing", "524288", "50.0%", ansiYellow},
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

			var out strings.Builder
			err := run(strings.NewReader(tt.input), &out)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			got := out.String()
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
