// Package agy provides programmatic interaction helpers for the agy CLI tool.
package agy

import (
	"context"
	"fmt"
	"time"

	"github.com/AgentDrasil/agent-wrapper/lib/term"
)

const (
	termCols uint16 = 220
	termRows uint16 = 50
)

// UsageOptions controls how [Usage] behaves.
type UsageOptions struct {
	// Dir is the working directory passed to agy via --cwd.
	// When empty the caller is responsible for having set the correct cwd
	// before invoking this function.
	Dir string

	// StartupDelay is how long to wait for agy to finish its initial render
	// before sending the /usage command. Defaults to 3 seconds.
	StartupDelay time.Duration

	// ResponseDelay is how long to wait for the /usage output to appear after
	// sending the command. Defaults to 5 seconds.
	ResponseDelay time.Duration
}

func (o *UsageOptions) startupDelay() time.Duration {
	if o.StartupDelay > 0 {
		return o.StartupDelay
	}
	return 3 * time.Second
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
//  3. Wait for startup, then send "/usage\r".
//  4. Wait for the response to render.
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

	// Wait for agy to render its initial UI.
	select {
	case <-time.After(opts.startupDelay()):
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-done:
		return nil, fmt.Errorf("agy exited unexpectedly during startup: %w", err)
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
