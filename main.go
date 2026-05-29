// agent-wrapper: runs a TUI CLI agent (e.g. agy, claude) headlessly in a PTY,
// emulates the terminal via libghostty, and periodically scrapes session state
// (context usage, session ID) from the virtual screen output.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                        agent-wrapper                           │
//	│                                                                 │
//	│  PTY Master (ptmx) ◄──────────────────────────────────────┐   │
//	│       │                                                    │   │
//	│       │ stdout/stderr of agent process                     │   │
//	│       ▼                                                    │   │
//	│  [read goroutine]                                          │   │
//	│       │ raw VT bytes                                       │   │
//	│       ▼                                                    │   │
//	│  libghostty.Terminal.VTWrite()  ── onWritePty effect ──►  │   │
//	│       │                           (query responses written │   │
//	│       │                            back to PTY slave)      │   │
//	│       ▼                                                    │   │
//	│  virtual screen grid (in-memory)                           │   │
//	│       │                                                    │   │
//	│  [scraper goroutine] ──── Formatter.FormatString() ────►  │   │
//	│       │                  extracts plain-text snapshot      │   │
//	│       ▼                                                    │   │
//	│  SessionState{ContextUsed, SessionID, StatusLine}          │   │
//	└─────────────────────────────────────────────────────────────────┘
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"go.mitchellh.com/libghostty"
)

// ─── Configuration ───────────────────────────────────────────────────────────

const (
	defaultCols        = 220 // wide enough to capture full status-line content
	defaultRows        = 50
	defaultScrollback  = 50_000
	scrapeInterval     = 500 * time.Millisecond
	idleTimeout        = 5 * time.Minute
)

// ─── Session state ────────────────────────────────────────────────────────────

// SessionState holds the key metadata extracted from the agent's TUI screen.
type SessionState struct {
	SessionID    string
	ContextUsed  string // e.g. "42%" or "1234/32000 tokens"
	StatusLine   string // raw last status-bar line, for debugging
	Title        string // terminal title set by OSC 0/2
	LastUpdated  time.Time
}

func (s SessionState) String() string {
	return fmt.Sprintf(
		"SessionState{SessionID=%q ContextUsed=%q Title=%q Updated=%s}",
		s.SessionID, s.ContextUsed, s.Title, s.LastUpdated.Format(time.RFC3339),
	)
}

// ─── Regexes for screen scraping ─────────────────────────────────────────────

var (
	// Matches "Session: abc123" or "session_id: abc-123-def" style lines.
	reSessionID = regexp.MustCompile(`(?i)session[_\s\-]?(?:id)?[:\s]+([a-zA-Z0-9\-_]{6,})`)

	// Matches context patterns like "42%" or "1234 / 32000" or "tokens: 4567".
	reContextUsed = regexp.MustCompile(`(?i)(?:context|tokens?)[:\s]+(\d[\d,/\s%kKmM\.]+(?:tokens?|%)?)`)
)

// ─── Runner ───────────────────────────────────────────────────────────────────

// Runner wraps a TUI agent process, owns the PTY, and emulates the terminal
// via libghostty. It is NOT safe for concurrent use across its exported
// methods; use the returned SessionState channel to read scraped state.
type Runner struct {
	mu      sync.Mutex // guards term and all libghostty calls
	term    *libghostty.Terminal
	ptmx    *os.File
	cmd     *exec.Cmd

	cols uint16
	rows uint16

	stateCh  chan SessionState
	title    string // updated by onTitleChanged effect
}

// NewRunner creates a Runner that will execute the given command inside a
// headless PTY + libghostty virtual terminal.
func NewRunner(cols, rows uint16) (*Runner, error) {
	r := &Runner{
		cols:    cols,
		rows:    rows,
		stateCh: make(chan SessionState, 16),
	}

	// Create the libghostty virtual terminal.
	term, err := libghostty.NewTerminal(
		libghostty.WithSize(cols, rows),
		libghostty.WithMaxScrollback(defaultScrollback),
		// Effect: terminal writes a response back to the PTY (e.g. DA1, XTVERSION).
		// We forward those bytes to ptmx so the agent process actually receives them.
		libghostty.WithWritePty(func(_ *libghostty.Terminal, data []byte) {
			if r.ptmx != nil {
				_, _ = r.ptmx.Write(data)
			}
		}),
		// Effect: track title changes for session metadata.
		libghostty.WithTitleChanged(func(t *libghostty.Terminal) {
			// Title is not directly readable via the API; we read it via the
			// plain-text formatter on the next scrape cycle. We just signal
			// that something changed.
			log.Printf("[title] terminal title changed")
		}),
		// Effect: respond to XTVERSION so the agent knows our emulator.
		libghostty.WithXtversion(func(_ *libghostty.Terminal) string {
			return "agent-wrapper/1.0"
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("libghostty.NewTerminal: %w", err)
	}
	r.term = term
	return r, nil
}

// Start launches the given command (e.g. ["agy", "--no-interactive", "--prompt", "..."]).
// Returns a context-scoped error channel that closes when the process exits.
func (r *Runner) Start(ctx context.Context, argv []string, env []string) (<-chan error, error) {
	if len(argv) == 0 {
		return nil, errors.New("argv must not be empty")
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("TERM=xterm-256color"),
		fmt.Sprintf("COLUMNS=%d", r.cols),
		fmt.Sprintf("LINES=%d", r.rows),
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: r.rows,
		Cols: r.cols,
	})
	if err != nil {
		return nil, fmt.Errorf("pty.Start: %w", err)
	}
	r.ptmx = ptmx
	r.cmd = cmd

	done := make(chan error, 1)

	// Goroutine: pump PTY output → libghostty.
	go r.readLoop(done)

	return done, nil
}

// readLoop reads raw VT bytes from the PTY master and feeds them into
// the libghostty state machine. It signals done when the process exits.
func (r *Runner) readLoop(done chan<- error) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.ptmx.Read(buf)
		if n > 0 {
			r.mu.Lock()
			r.term.VTWrite(buf[:n])
			r.mu.Unlock()
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
				break
			}
			// EIO is the normal Linux signal when the PTY slave closes.
			if isSyscallEIO(err) {
				break
			}
			log.Printf("[pty] read error: %v", err)
			break
		}
	}
	// Wait for the process to fully exit so we capture the exit code.
	done <- r.cmd.Wait()
}

// WriteInput sends raw bytes to the agent's stdin via the PTY.
// This is how you send key events or pasted text programmatically.
func (r *Runner) WriteInput(data []byte) error {
	if r.ptmx == nil {
		return errors.New("runner not started")
	}
	_, err := r.ptmx.Write(data)
	return err
}

// Resize updates the PTY and libghostty terminal size.
func (r *Runner) Resize(cols, rows uint16) error {
	if err := pty.Setsize(r.ptmx, &pty.Winsize{Rows: rows, Cols: cols}); err != nil {
		return fmt.Errorf("pty.Setsize: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.term.Resize(cols, rows, 8, 16); err != nil {
		return fmt.Errorf("terminal.Resize: %w", err)
	}
	r.cols = cols
	r.rows = rows
	return nil
}

// Scrape takes a plain-text snapshot of the virtual screen and extracts
// session metadata from it. Callers can also subscribe to StateCh() for
// periodic scrapes driven by the scraper goroutine.
func (r *Runner) Scrape() (SessionState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := libghostty.NewFormatter(r.term,
		libghostty.WithFormatterFormat(libghostty.FormatterFormatPlain),
		libghostty.WithFormatterTrim(true),
	)
	if err != nil {
		return SessionState{}, fmt.Errorf("NewFormatter: %w", err)
	}
	defer f.Close()

	text, err := f.FormatString()
	if err != nil {
		return SessionState{}, fmt.Errorf("FormatString: %w", err)
	}

	return parseSessionState(text, r.title), nil
}

// StateCh returns the channel on which periodic scrape results are delivered.
func (r *Runner) StateCh() <-chan SessionState { return r.stateCh }

// StartScraper launches a goroutine that periodically calls Scrape and
// delivers results to StateCh. It stops when ctx is cancelled.
func (r *Runner) StartScraper(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(scrapeInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				state, err := r.Scrape()
				if err != nil {
					log.Printf("[scraper] error: %v", err)
					continue
				}
				select {
				case r.stateCh <- state:
				default:
					// Drop if consumer is slow; we'll send the next one.
				}
			}
		}
	}()
}

// Close shuts down the runner, freeing the libghostty terminal.
func (r *Runner) Close() {
	if r.ptmx != nil {
		_ = r.ptmx.Close()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.term != nil {
		r.term.Close()
		r.term = nil
	}
}

// ─── Screen scraping helpers ─────────────────────────────────────────────────

// parseSessionState extracts structured session info from a plain-text
// screen snapshot. It scans every line for known patterns.
func parseSessionState(screen, title string) SessionState {
	state := SessionState{
		Title:       title,
		LastUpdated: time.Now(),
	}

	scanner := bufio.NewScanner(strings.NewReader(screen))
	var lastLine string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			lastLine = line
		}

		// Try to extract session ID.
		if state.SessionID == "" {
			if m := reSessionID.FindStringSubmatch(line); m != nil {
				state.SessionID = strings.TrimSpace(m[1])
			}
		}

		// Try to extract context/token usage.
		if state.ContextUsed == "" {
			if m := reContextUsed.FindStringSubmatch(line); m != nil {
				state.ContextUsed = strings.TrimSpace(m[1])
			}
		}
	}

	// The last non-empty line is typically the status bar in TUI agents.
	state.StatusLine = lastLine
	return state
}

// ─── OS helpers ──────────────────────────────────────────────────────────────

func isSyscallEIO(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EIO
	}
	return false
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	cols := flag.Uint("cols", defaultCols, "terminal width in columns")
	rows := flag.Uint("rows", defaultRows, "terminal height in rows")
	timeout := flag.Duration("timeout", idleTimeout, "maximum run time before forceful kill")
	flag.Parse()

	argv := flag.Args()
	if len(argv) == 0 {
		// Default: run bash so you can test the plumbing interactively.
		argv = []string{"bash", "--norc", "--noprofile"}
		fmt.Fprintln(os.Stderr, "no command specified — defaulting to bash for plumbing test")
		fmt.Fprintln(os.Stderr, "usage: agent-wrapper [flags] -- <command> [args...]")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Apply overall timeout.
	ctx, cancelTimeout := context.WithTimeout(ctx, *timeout)
	defer cancelTimeout()

	// ── Create runner ────────────────────────────────────────────────────────
	runner, err := NewRunner(uint16(*cols), uint16(*rows))
	if err != nil {
		log.Fatalf("NewRunner: %v", err)
	}
	defer runner.Close()

	// ── Start agent process ───────────────────────────────────────────────────
	done, err := runner.Start(ctx, argv, nil)
	if err != nil {
		log.Fatalf("runner.Start: %v", err)
	}
	log.Printf("started %q (cols=%d rows=%d)", argv, *cols, *rows)

	// ── Start periodic screen scraper ─────────────────────────────────────────
	runner.StartScraper(ctx)

	// ── Forward stdin → PTY (for interactive testing) ─────────────────────────
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if werr := runner.WriteInput(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// ── Handle SIGWINCH: propagate terminal resize ────────────────────────────
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	go func() {
		for range sigwinch {
			ws, err := pty.GetsizeFull(os.Stdin)
			if err != nil {
				continue
			}
			if err := runner.Resize(ws.Cols, ws.Rows); err != nil {
				log.Printf("[resize] %v", err)
			} else {
				log.Printf("[resize] → %dx%d", ws.Cols, ws.Rows)
			}
		}
	}()

	// ── Event loop ───────────────────────────────────────────────────────────
	log.Println("waiting for agent output… (Ctrl-C to stop)")
	for {
		select {
		case state := <-runner.StateCh():
			// Print any non-empty state updates.
			if state.SessionID != "" || state.ContextUsed != "" {
				log.Printf("[state] %s", state)
			}
			if state.StatusLine != "" {
				log.Printf("[status-bar] %s", state.StatusLine)
			}

		case exitErr := <-done:
			// Agent process exited — do one final scrape and report.
			final, scrapeErr := runner.Scrape()
			if scrapeErr != nil {
				log.Printf("[final-scrape] error: %v", scrapeErr)
			} else {
				fmt.Println("\n=== Final Session State ===")
				fmt.Printf("  Session ID   : %s\n", orNA(final.SessionID))
				fmt.Printf("  Context Used : %s\n", orNA(final.ContextUsed))
				fmt.Printf("  Status Line  : %s\n", orNA(final.StatusLine))
				fmt.Printf("  Title        : %s\n", orNA(final.Title))
			}

			if exitErr != nil {
				log.Printf("agent exited with error: %v", exitErr)
				os.Exit(1)
			}
			log.Println("agent exited cleanly")
			return

		case <-ctx.Done():
			log.Println("context cancelled, shutting down")
			return
		}
	}
}

func orNA(s string) string {
	if s == "" {
		return "(not detected)"
	}
	return s
}
