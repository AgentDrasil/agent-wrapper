package agy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/AgentDrasil/agent-wrapper/lib/term"
)

var (
	keyArrowUp   = []byte{0x1b, '[', 'A'}
	keyArrowDown = []byte{0x1b, '[', 'B'}
)

// SelectModel sends /model to the running agy session, parses the TUI menu,
// navigates to targetModel using arrow keys, and selects it with Enter.
func SelectModel(ctx context.Context, t *term.Term, done <-chan error, targetModel string) error {
	log.Debug().Str("target_model", targetModel).Msg("agy/select_model: sending /model command")

	// 1. Send /model command
	if err := t.SendString("/model"); err != nil {
		return fmt.Errorf("sending /model: %w", err)
	}
	if err := t.SendKeys(term.KeyEnter); err != nil {
		return fmt.Errorf("sending Enter after /model: %w", err)
	}

	// Wait a moment for TUI to render
	select {
	case <-time.After(500 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return fmt.Errorf("agy exited unexpectedly: %w", err)
	}

	// 2. Parse the screen scrollback to locate models and find current selection
	lines := t.Screen()
	startMenu := -1
	for i, line := range lines {
		if strings.Contains(line, "Switch Model") {
			startMenu = i
			break
		}
	}

	// If "Switch Model" is not explicitly found, we search the entire screen
	searchStart := startMenu + 1
	if startMenu == -1 {
		searchStart = 0
	}

	type modelOption struct {
		name       string
		isSelected bool
		index      int
	}

	var options []modelOption
	var selectedIdx = -1

	for i := searchStart; i < len(lines); i++ {
		origLine := lines[i]
		trimmed := strings.TrimSpace(origLine)
		if trimmed == "" {
			continue
		}

		// A model option is prefixed by ">" (current selection) or "  "
		isOption := strings.HasPrefix(strings.TrimLeft(origLine, " "), ">") || strings.HasPrefix(origLine, "  ")
		if !isOption {
			continue
		}

		name := cleanModelName(origLine)
		if name == "" || strings.Contains(name, "Switch Model") {
			continue
		}

		isSelected := strings.HasPrefix(strings.TrimSpace(origLine), ">")
		options = append(options, modelOption{
			name:       name,
			isSelected: isSelected,
			index:      len(options),
		})
		if isSelected {
			selectedIdx = len(options) - 1
		}
	}

	if len(options) == 0 {
		return fmt.Errorf("no model options found on the screen")
	}

	if selectedIdx == -1 {
		// Default to first if none was marked selected (fallback)
		selectedIdx = 0
	}

	// 3. Find the target model
	targetIdx := -1
	for i, opt := range options {
		if strings.EqualFold(opt.name, targetModel) {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		// Fallback to substring matching
		for i, opt := range options {
			if strings.Contains(strings.ToLower(opt.name), strings.ToLower(targetModel)) {
				targetIdx = i
				break
			}
		}
	}

	if targetIdx == -1 {
		var avail []string
		for _, opt := range options {
			avail = append(avail, opt.name)
		}
		return fmt.Errorf("model %q not found. Available models: %v", targetModel, avail)
	}

	log.Debug().
		Str("current_model", options[selectedIdx].name).
		Str("target_model", options[targetIdx].name).
		Msg("agy/select_model: navigating to model")

	// 4. Navigate up/down to the target model
	diff := targetIdx - selectedIdx
	if diff > 0 {
		for k := 0; k < diff; k++ {
			if err := t.SendKeys(keyArrowDown); err != nil {
				return fmt.Errorf("sending arrow down: %w", err)
			}
			time.Sleep(50 * time.Millisecond)
		}
	} else if diff < 0 {
		for k := 0; k < -diff; k++ {
			if err := t.SendKeys(keyArrowUp); err != nil {
				return fmt.Errorf("sending arrow up: %w", err)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	// 5. Select with Enter
	if err := t.SendKeys(term.KeyEnter); err != nil {
		return fmt.Errorf("confirming model selection: %w", err)
	}

	// Wait a moment for selection to be applied and return to main interface
	select {
	case <-time.After(300 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func cleanModelName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, ">")
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "(current)")
	s = strings.TrimSpace(s)
	return s
}
