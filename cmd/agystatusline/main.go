// agystatusline reads the JSON payload that the antigravity-cli (agy) pipes to
// a custom status line command via stdin, extracts the fields we care about,
// and prints a compact one-line status string to stdout:
//
//	<agent_state> | <total_input_tokens> | <context_window_size> | <remaining>%
//
// The remaining-percentage segment is coloured green (≥ 80 %), yellow (≥ 50 %),
// or red (< 50 %) using ANSI escape codes so the value stands out in the
// status bar.
//
// Usage – settings.json:
//
//	{
//	  "statusLine": {
//	    "type":    "command",
//	    "command": "/path/to/agystatusline"
//	  }
//	}
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// payload is the subset of the JSON document we care about.
type payload struct {
	AgentState    string        `json:"agent_state"`
	ContextWindow contextWindow `json:"context_window"`
}

type contextWindow struct {
	TotalInputTokens    int     `json:"total_input_tokens"`
	ContextWindowSize   int     `json:"context_window_size"`
	RemainingPercentage float64 `json:"remaining_percentage"`
}

// ANSI colour helpers.
const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiGreen  = "\033[32m"
)

func remainingColor(pct float64) string {
	switch {
	case pct >= 80:
		return ansiGreen
	case pct >= 50:
		return ansiYellow
	default:
		return ansiRed
	}
}

func run(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}

	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return "", fmt.Errorf("parsing JSON: %w", err)
	}

	color := remainingColor(p.ContextWindow.RemainingPercentage)
	return fmt.Sprintf("%s | %d | %d | %s%.1f%%%s",
		p.AgentState,
		p.ContextWindow.TotalInputTokens,
		p.ContextWindow.ContextWindowSize,
		color,
		p.ContextWindow.RemainingPercentage,
		ansiReset,
	), nil
}

func main() {
	line, err := run(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agystatusline: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(line)
}
