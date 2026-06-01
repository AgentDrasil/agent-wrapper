package agy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStatusLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		lines         []string
		wantInput     int
		wantMax       int
		wantRemaining float64
	}{
		{
			name: "standard statusline",
			lines: []string{
				"some command output",
				"state: idle | input_tokens: 88244 | max: 1048576 | remaining: 91.6% | tasks: 0 | subagents: 0",
			},
			wantInput:     88244,
			wantMax:       1048576,
			wantRemaining: 0.916,
		},
		{
			name: "padded with empty lines at the bottom",
			lines: []string{
				"state: idle | input_tokens: 123 | max: 456 | remaining: 80% | tasks: 0 | subagents: 0",
				"   ",
				"",
			},
			wantInput:     123,
			wantMax:       456,
			wantRemaining: 0.8,
		},

		{
			name:          "no statusline at all",
			lines:         []string{"just raw logs", "without labels"},
			wantInput:     0,
			wantMax:       0,
			wantRemaining: 0.0,
		},
		{
			name:          "empty scrollback slice",
			lines:         []string{},
			wantInput:     0,
			wantMax:       0,
			wantRemaining: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotInput, gotMax, gotRemaining := parseStatusLine(tt.lines)
			assert.Equal(t, tt.wantInput, gotInput)
			assert.Equal(t, tt.wantMax, gotMax)
			assert.InDelta(t, tt.wantRemaining, gotRemaining, 1e-9)
		})
	}
}

func TestExtractSessionID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name: "standard exit instruction",
			lines: []string{
				"Exiting...",
				"Resume: agy --conversation=94f0e306-4718-4c38-8883-4f2982f20176 (or -c)",
			},
			want: "94f0e306-4718-4c38-8883-4f2982f20176",
		},
		{
			name: "exit instruction with terminal padding",
			lines: []string{
				"Resume: agy --conversation=abcd-1234-efab (or -c)",
				"  ",
				"",
			},
			want: "abcd-1234-efab",
		},
		{
			name: "no exit instruction",
			lines: []string{
				"Process terminated cleanly",
			},
			want: "",
		},
		{
			name:  "empty lines",
			lines: []string{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, extractSessionID(tt.lines))
		})
	}
}
