package agy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsIdle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  bool
	}{
		{
			name:  "labeled format – fully idle",
			lines: []string{"some output", "state: idle | input_tokens: 88244 | max: 1048576 | remaining: 91.6% | tasks: 0 | subagents: 0"},
			want:  true,
		},
		{
			name:  "labeled format – non-idle state",
			lines: []string{"state: thinking | input_tokens: 88244 | max: 1048576 | remaining: 91.6% | tasks: 0 | subagents: 0"},
			want:  false,
		},
		{
			name:  "labeled format – idle state but tasks running",
			lines: []string{"state: idle | input_tokens: 88244 | max: 1048576 | remaining: 91.6% | tasks: 2 | subagents: 0"},
			want:  false,
		},
		{
			name:  "labeled format – idle state but subagents active",
			lines: []string{"state: idle | input_tokens: 88244 | max: 1048576 | remaining: 91.6% | tasks: 0 | subagents: 3"},
			want:  false,
		},
		{
			name:  "labeled format – idle state with tasks and subagents",
			lines: []string{"state: idle | input_tokens: 88244 | max: 1048576 | remaining: 91.6% | tasks: 1 | subagents: 2"},
			want:  false,
		},
		{
			name:  "bare idle fallback",
			lines: []string{"idle"},
			want:  true,
		},
		{
			name:  "bare non-idle fallback",
			lines: []string{"working"},
			want:  false,
		},
		{
			name:  "empty lines",
			lines: []string{},
			want:  false,
		},
		{
			name:  "last line is blank",
			lines: []string{"state: idle | tasks: 0 | subagents: 0", ""},
			want:  true,
		},
		{
			name:  "case-insensitive state label",
			lines: []string{"STATE: IDLE | input_tokens: 0 | max: 0 | remaining: 0% | TASKS: 0 | SUBAGENTS: 0"},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isIdle(tt.lines))
		})
	}
}

func TestExtractLabeledInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields []string
		key    string
		want   int
	}{
		{
			name:   "found key",
			fields: []string{"tasks:", "2", "|", "subagents:", "0"},
			key:    "tasks:",
			want:   2,
		},
		{
			name:   "key not present",
			fields: []string{"state:", "idle"},
			key:    "tasks:",
			want:   -1,
		},
		{
			name:   "key at end with no following value",
			fields: []string{"tasks:"},
			key:    "tasks:",
			want:   -1,
		},
		{
			name:   "case-insensitive key",
			fields: []string{"TASKS:", "5"},
			key:    "tasks:",
			want:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, extractLabeledInt(tt.fields, tt.key))
		})
	}
}
