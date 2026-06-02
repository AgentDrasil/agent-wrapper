// Package agy provides programmatic interaction helpers for the agy CLI tool.
package agy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/AgentDrasil/agent-wrapper/lib/term"
)

const (
	termCols     uint16        = 220
	termRows     uint16        = 50
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

// isIdle returns true when the statusbar line (the last non-empty line) indicates
// the system is fully at rest: state=idle, zero background tasks, and zero
// active subagents, as produced by agystatusline.
func isIdle(lines []string) bool {
	if len(lines) == 0 {
		return false
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
		return false
	}
	fields := strings.Fields(last)

	// Labeled format: "state: idle | ... | tasks: 0 | subagents: 0"
	if len(fields) >= 2 && strings.EqualFold(fields[0], "state:") {
		if !strings.EqualFold(fields[1], "idle") {
			return false
		}
		return extractLabeledInt(fields, "tasks:") == 0 &&
			extractLabeledInt(fields, "subagents:") == 0
	}

	// Fallback: bare token for forward-compatibility.
	if len(fields) >= 1 {
		return strings.EqualFold(fields[0], "idle")
	}
	return false
}

// pollUntilIdle polls t.Scrollback() every pollInterval until isIdle returns
// true or timeout elapses. The timeout is soft: expiry causes the function to
// return (timedOut=true, err=nil) so the caller can decide whether to proceed
// or abort. ctx cancellation and unexpected agy exit are hard errors.
func pollUntilIdle(ctx context.Context, t *term.Term, done <-chan error, timeout time.Duration) (timedOut bool, err error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			if isIdle(t.Scrollback()) {
				return false, nil
			}
		case <-timer.C:
			return true, nil
		case <-ctx.Done():
			return false, ctx.Err()
		case err := <-done:
			return false, fmt.Errorf("agy exited unexpectedly: %w", err)
		}
	}
}

// extractLabeledInt scans whitespace-split fields for a token equal to key
// (e.g. "tasks:") and returns the integer value of the immediately following
// token. Returns -1 if the key is not found or the value cannot be parsed.
func extractLabeledInt(fields []string, key string) int {
	for i, f := range fields {
		if strings.EqualFold(f, key) && i+1 < len(fields) {
			// Strip a trailing "|" separator that may be part of the same token.
			v := strings.TrimRight(fields[i+1], "|")
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
				return n
			}
		}
	}
	return -1
}

func (o *UsageOptions) responseDelay() time.Duration {
	if o.ResponseDelay > 0 {
		return o.ResponseDelay
	}
	return 1 * time.Second
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

	done, err := t.RunCommandInDir(context.Background(), argv, opts.Dir, nil)
	if err != nil {
		return nil, fmt.Errorf("launching agy/usage: %w", err)
	}

	handleErr := func(err error) error {
		if ctx.Err() != nil {
			GratefulShutdown(t, done)
			return ctx.Err()
		}
		return err
	}

	// Poll until the statusbar last line reports "idle", up to startupDelay.
	log.Debug().Msg("agy/usage: waiting for state=idle")
	timedOut, err := pollUntilIdle(ctx, t, done, opts.startupDelay())
	if err != nil {
		return nil, handleErr(err)
	}
	if timedOut {
		log.Debug().Msg("agy/usage: startup idle timed out — proceeding anyway")
	} else {
		log.Debug().Msg("agy/usage: state=idle")
	}

	// Send the /usage command followed by Enter.
	if err := t.SendString("/usage"); err != nil {
		return nil, handleErr(fmt.Errorf("sending /usage: %w", err))
	}
	if err := t.SendKeys(term.KeyEnter); err != nil {
		return nil, handleErr(fmt.Errorf("sending Enter: %w", err))
	}

	// Wait for the response to render.
	select {
	case <-time.After(opts.responseDelay()):
	case <-ctx.Done():
		return nil, handleErr(ctx.Err())
	case err := <-done:
		return nil, fmt.Errorf("agy exited unexpectedly waiting for /usage response: %w", err)
	}

	lines := t.Scrollback()
	log.Debug().Msg("agy/usage: got usage")

	// Exit: Esc, then Ctrl-D twice.
	CleanExit(t, done)

	return parseUsage(lines, time.Now())
}
