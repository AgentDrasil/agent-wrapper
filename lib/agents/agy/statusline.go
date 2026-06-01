package agy

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	reInputTokens = regexp.MustCompile(`(?i)input_tokens:\s*(\d+)`)
	reMaxTokens   = regexp.MustCompile(`(?i)\bmax:\s*(\d+)`)
	reRemaining   = regexp.MustCompile(`(?i)remaining:\s*([0-9.]+%?)`)
)

// parseStatusLine extracts inputTokens, maxTokens, and remaining from the
// last non-empty line of the terminal scrollback. Returns zero values on parse failure.
func parseStatusLine(lines []string) (inputTokens, maxTokens int, remaining string) {
	if len(lines) == 0 {
		return
	}
	var last string
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			last = trimmed
			break
		}
	}
	if last == "" {
		return
	}

	if m := reInputTokens.FindStringSubmatch(last); m != nil {
		inputTokens, _ = strconv.Atoi(m[1])
	}
	if m := reMaxTokens.FindStringSubmatch(last); m != nil {
		maxTokens, _ = strconv.Atoi(m[1])
	}
	if m := reRemaining.FindStringSubmatch(last); m != nil {
		remaining = strings.TrimSpace(m[1])
	}
	return
}
