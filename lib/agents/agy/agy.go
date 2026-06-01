// Package agy provides programmatic interaction helpers for the agy CLI tool.
package agy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AgentDrasil/agent-wrapper/lib/term"
)

const (
	termCols    uint16        = 220
	termRows    uint16        = 50
	pollInterval time.Duration = 200 * time.Millisecond
)

// UsageOptions controls how [Usage] behaves.
type UsageOptions struct {
	// Dir is the working directory passed to agy via --cwd.
	// When empty the caller is responsible for having set the correct cwd
	// before invoking this function.
	Dir string

	// StartupDelay is the maximum time to wait for agy's statusbar to report
	// "idle" before sending the /usage command. The check polls every 200 ms.
	// Defaults to 10 seconds.
	StartupDelay time.Duration

	// ResponseDelay is how long to wait for the /usage output to appear after
	// sending the command. Defaults to 5 seconds.
	ResponseDelay time.Duration
}

func (o *UsageOptions) startupDelay() time.Duration {
	if o.StartupDelay > 0 {
		return o.StartupDelay
	}
	return 10 * time.Second
}

// isIdle returns true when the last line of lines (the statusbar) has
// "idle" as its first whitespace-separated token.
func isIdle(lines []string) bool {
	if len(lines) == 0 {
		return false
	}
	last := lines[len(lines)-1]
	fields := strings.Fields(last)
	if len(fields) == 0 {
		return false
	}
	return strings.EqualFold(fields[0], "idle")
}

func (o *UsageOptions) responseDelay() time.Duration {
	if o.ResponseDelay > 0 {
		return o.ResponseDelay
	}
	return 5 * time.Second
}

// Usage launches agy in a headless terminal, sends the "/usage" command,
// parses the output, exits cleanly, and returns the captured quota entries.
//
// The sequence performed is:
//  1. Open a headless PTY-backed terminal (220×50).
//  2. Launch `agy`.
//  3. Poll the scrollback every 200 ms until the statusbar last line's first
//     token is "idle" (or StartupDelay elapses).
//  4. Send "/usage\r" and wait for the response to render.
//  5. Press Esc, then Ctrl-D twice to exit.
//  6. Parse and return the scrollback as []ModelUsage.
func Usage(ctx context.Context, opts UsageOptions) ([]ModelUsage, error) {
	t := term.NewTerm(termCols, termRows)
	defer t.Close()

	argv := []string{"agy"}

	done, err := t.RunCommandInDir(ctx, argv, opts.Dir, nil)
	if err != nil {
		return nil, fmt.Errorf("launching agy: %w", err)
	}

	// Poll until the statusbar last line reports "idle", up to startupDelay.
	readyTimer := time.NewTimer(opts.startupDelay())
	defer readyTimer.Stop()
	pollTick := time.NewTicker(pollInterval)
	defer pollTick.Stop()
waitIdle:
	for {
		select {
		case <-pollTick.C:
			if isIdle(t.Scrollback()) {
				break waitIdle
			}
		case <-readyTimer.C:
			// Timeout reached — proceed anyway and let /usage fail visibly.
			break waitIdle
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-done:
			return nil, fmt.Errorf("agy exited unexpectedly during startup: %w", err)
		}
	}

	// Send the /usage command followed by Enter.
	if err := t.SendString("/usage"); err != nil {
		return nil, fmt.Errorf("sending /usage: %w", err)
	}
	if err := t.SendKeys(term.KeyEnter); err != nil {
		return nil, fmt.Errorf("sending Enter: %w", err)
	}

	// Wait for the response to render.
	select {
	case <-time.After(opts.responseDelay()):
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-done:
		return nil, fmt.Errorf("agy exited unexpectedly waiting for /usage response: %w", err)
	}

	lines := t.Scrollback()

	// Exit: Esc, then Ctrl-D twice.
	_ = t.SendKeys(term.KeyEsc)
	time.Sleep(200 * time.Millisecond)
	_ = t.SendKeys(term.KeyCtrlD)
	time.Sleep(200 * time.Millisecond)
	_ = t.SendKeys(term.KeyCtrlD)

	// Wait for clean exit (best-effort; don't block indefinitely).
	exitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case <-done:
	case <-exitCtx.Done():
	}

	return parseUsage(lines)
}
