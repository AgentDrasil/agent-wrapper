package agy

import (
	"regexp"
	"strconv"
	"strings"
)

// ModelUsage represents the quota status for a single model.
type ModelUsage struct {
	// Model is the full model name, e.g. "Claude Sonnet 4.6 (Thinking)".
	Model string `json:"model"`

	// Remaining is the fraction of quota still available in [0, 1].
	// 1.0 means fully available; 0.8 means 80% remaining.
	Remaining float64 `json:"remaining"`

	// RefreshTime is the human-readable time until the quota resets,
	// e.g. "1h 23m". Empty when quota is fully available.
	RefreshTime string `json:"refresh_time,omitempty"`
}

var (
	// matches the trailing "80%" or "100%" on a progress-bar line.
	rePercent = regexp.MustCompile(`(\d+)%\s*$`)

	// matches "Refreshes in 1h 23m" — capture group 1 is the duration.
	reRefresh = regexp.MustCompile(`Refreshes in\s+(.+)`)

	// progress-bar lines contain block-drawing characters.
	reBarLine = regexp.MustCompile(`[█░]`)
)

// parseUsage parses the raw scrollback lines returned by [Usage] into a slice
// of [ModelUsage] entries.
//
// The expected per-model block format (after trimming) is:
//
//	<Model Name>
//	███ … ░░░ 80%
//	80% remaining · Refreshes in 1h 23m
//
// or, when fully available:
//
//	<Model Name>
//	███ … 100%
//	Quota available
func parseUsage(lines []string) ([]ModelUsage, error) {
	blocks := splitBlocks(lines)

	var result []ModelUsage
	for _, block := range blocks {
		entry, ok := parseBlock(block)
		if !ok {
			continue
		}
		result = append(result, entry)
	}
	return result, nil
}

// splitBlocks groups non-blank (trimmed) lines into blocks separated by blank lines.
func splitBlocks(lines []string) [][]string {
	var blocks [][]string
	var cur []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			if len(cur) > 0 {
				blocks = append(blocks, cur)
				cur = nil
			}
		} else {
			cur = append(cur, t)
		}
	}
	if len(cur) > 0 {
		blocks = append(blocks, cur)
	}
	return blocks
}

// parseBlock attempts to extract a ModelUsage from a single line-block.
// Returns (entry, true) on success or (zero, false) if the block doesn't look
// like a usage entry.
func parseBlock(block []string) (ModelUsage, bool) {
	if len(block) < 2 {
		return ModelUsage{}, false
	}

	// The first line is the model name; it must not contain a progress bar.
	model := block[0]
	if reBarLine.MatchString(model) {
		return ModelUsage{}, false
	}

	// Find the progress-bar line and extract the percentage.
	remaining := -1.0
	for _, line := range block[1:] {
		if !reBarLine.MatchString(line) {
			continue
		}
		m := rePercent.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pct, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		remaining = float64(pct) / 100.0
		break
	}
	if remaining == -1.0 {
		return ModelUsage{}, false
	}

	// Find the status line for the refresh time.
	refreshTime := ""
	for _, line := range block {
		if strings.Contains(line, "Quota available") {
			// refreshTime stays empty.
			break
		}
		if m := reRefresh.FindStringSubmatch(line); m != nil {
			refreshTime = strings.TrimSpace(m[1])
			break
		}
	}

	return ModelUsage{
		Model:       model,
		Remaining:   remaining,
		RefreshTime: refreshTime,
	}, true
}
