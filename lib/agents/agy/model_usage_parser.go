package agy

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ModelUsage represents the quota status for a single model.
type ModelUsage struct {
	// Model is the full model name, e.g. "Claude Sonnet 4.6 (Thinking)".
	Model string `json:"model"`

	// Remaining is the fraction of quota still available in [0, 1].
	// 1.0 means fully available; 0.8 means 80% remaining.
	Remaining float64 `json:"remaining"`

	// RefreshDate is the unix timestamp (seconds since epoch) when the quota resets.
	// 0 when quota is fully available.
	RefreshDate int64 `json:"refresh_date,omitempty"`
}

var (
	// matches the trailing "80%" or "100%" on a progress-bar line.
	rePercent = regexp.MustCompile(`(\d+)%\s*$`)

	// matches "Refreshes in 1h 23m" — capture group 1 is the duration.
	reRefresh = regexp.MustCompile(`Refreshes in\s+(.+)`)

	// progress-bar lines contain block-drawing characters.
	reBarLine = regexp.MustCompile(`[█░]`)
)

// parseDuration parses a string like "1h 23m", "59m", "2h" into a time.Duration.
func parseDuration(s string) (time.Duration, error) {
	s = strings.ReplaceAll(s, " ", "")
	var total time.Duration
	dIdx := strings.Index(s, "d")
	if dIdx != -1 {
		daysStr := s[:dIdx]
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, err
		}
		total += time.Duration(days) * 24 * time.Hour
		s = s[dIdx+1:]
	}
	if s != "" {
		rest, err := time.ParseDuration(s)
		if err != nil {
			return 0, err
		}
		total += rest
	}
	return total, nil
}

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
func parseUsage(lines []string, now time.Time) ([]ModelUsage, error) {
	blocks := splitBlocks(lines)

	var result []ModelUsage
	for _, block := range blocks {
		entry, ok := parseBlock(block, now)
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
func parseBlock(block []string, now time.Time) (ModelUsage, bool) {
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
	var refreshDate int64
	for _, line := range block {
		if strings.Contains(line, "Quota available") {
			// refreshDate stays 0.
			break
		}
		if m := reRefresh.FindStringSubmatch(line); m != nil {
			durStr := strings.TrimSpace(m[1])
			dur, err := parseDuration(durStr)
			if err == nil {
				refreshDate = now.Add(dur).Unix()
			}
			break
		}
	}

	return ModelUsage{
		Model:       model,
		Remaining:   remaining,
		RefreshDate: refreshDate,
	}, true
}
