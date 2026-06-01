package agy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/AgentDrasil/agent-wrapper/lib/term"
)

// PromptOptions controls how [Prompt] behaves.
type PromptOptions struct {
	// Dir is the working directory passed to agy via --cwd.
	Dir string

	// SessionID is the conversation ID to resume. When empty a new UUID v4 is
	// generated so that agy is always launched with --conversation=<id>.
	SessionID string

	// StartupDelay is the maximum time to wait for agy's statusbar to report
	// "idle" before sending the prompt. Defaults to 60 seconds.
	StartupDelay time.Duration

	// ResponseDelay is the maximum time to wait for agy to return to idle
	// after each prompt send. Defaults to 300 seconds (5 min).
	ResponseDelay time.Duration

	// Model is the name of the model to select (e.g., "Gemini 3.5 Flash (Low)").
	// When non-empty, SelectModel is run before sending the prompt.
	Model string
}

func (o *PromptOptions) startupDelay() time.Duration {
	if o.StartupDelay > 0 {
		return o.StartupDelay
	}
	return 10 * time.Second
}

func (o *PromptOptions) responseDelay() time.Duration {
	if o.ResponseDelay > 0 {
		return o.ResponseDelay
	}
	return 300 * time.Second
}

// PromptResult is the structured response from a [Prompt] call.
type PromptResult struct {
	// SessionID is the conversation / session identifier used for this run.
	SessionID string `json:"session_id"`

	// InputTokens is the value of input_tokens from the final statusline.
	InputTokens int `json:"input_tokens"`

	// MaxTokens is the value of max (context window size) from the final statusline.
	MaxTokens int `json:"max_tokens"`

	// Remaining is the fraction of remaining quota in [0, 1] from the final
	// statusline, e.g. 0.916.
	Remaining float64 `json:"remaining"`

	// LastContent is the raw "content" field of the last line in the
	// transcript JSONL file, giving the caller access to the full response.
	LastContent string `json:"last_content"`
}

// Prompt launches agy in a headless terminal with --conversation=<sessionID>
// and --dangerously-skip-permissions, sends the given prompt text, waits for
// the agent to return to idle (with a double-check), then reads the transcript
// and returns structured metadata.
//
// The sequence is:
//  1. Open a headless PTY-backed terminal (220×50).
//  2. Launch `agy --conversation=<sessionID> --dangerously-skip-permissions`.
//  3. Wait until the statusbar reports idle (idle #1 – startup idle).
//  4. Send the prompt text followed by Enter.
//  5. Wait until idle again (idle #2 – post-response idle).
//  6. Sleep 200 ms, then wait until idle once more (idle #3 – double-check).
//  7. Exit agy cleanly (Esc, Ctrl-D, Ctrl-D).
//  8. Read the last line of ~/.gemini/antigravity-cli/brain/<sessionID>/.system_generated/logs/transcript.jsonl.
//  9. Parse the statusline for token metadata and return a PromptResult.
func Prompt(ctx context.Context, prompt string, opts PromptOptions) (*PromptResult, error) {
	sessionID := opts.SessionID
	isNewSession := sessionID == ""

	t := term.NewTerm(termCols, termRows)
	defer t.Close()

	argv := []string{"agy"}
	if !isNewSession {
		argv = append(argv, "--conversation="+sessionID)
	}
	argv = append(argv, "--dangerously-skip-permissions")

	log.Debug().Interface("argv", argv).Msg("agy/prompt: starting")

	done, err := t.RunCommandInDir(ctx, argv, opts.Dir, nil)
	if err != nil {
		return nil, fmt.Errorf("launching agy: %w", err)
	}

	// ── idle #1: wait for startup idle ────────────────────────────────────────
	log.Debug().Msg("agy/prompt: waiting for startup idle (#1)")
	if timedOut, err := pollUntilIdle(ctx, t, done, opts.startupDelay()); err != nil {
		return nil, err
	} else if timedOut {
		log.Warn().Msg("agy/prompt: startup idle (#1) timed out")
	} else {
		log.Debug().Msg("agy/prompt: startup idle reached (#1)")
	}

	// ── select model if requested ─────────────────────────────────────────────
	if opts.Model != "" {
		if err := SelectModel(ctx, t, done, opts.Model); err != nil {
			return nil, fmt.Errorf("selecting model %q: %w", opts.Model, err)
		}
	}

	// ── send the prompt ───────────────────────────────────────────────────────
	if err := t.SendString(prompt); err != nil {
		return nil, fmt.Errorf("sending prompt: %w", err)
	}
	if err := t.SendKeys(term.KeyEnter); err != nil {
		return nil, fmt.Errorf("sending Enter: %w", err)
	}
	log.Debug().Msg("agy/prompt: prompt sent")

	// ── idle #2: wait for the agent to finish responding ──────────────────────
	log.Debug().Msg("agy/prompt: waiting for post-response idle (#2)")
	if timedOut, err := pollUntilIdle(ctx, t, done, opts.responseDelay()); err != nil {
		return nil, err
	} else if timedOut {
		log.Warn().Msg("agy/prompt: post-response idle (#2) timed out")
	} else {
		log.Debug().Msg("agy/prompt: post-response idle reached (#2)")
	}

	// ── idle #3: sleep 200 ms, then double-check idle ────────────────────────
	select {
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	log.Debug().Msg("agy/prompt: waiting for double-check idle (#3)")
	if timedOut, err := pollUntilIdle(ctx, t, done, opts.responseDelay()); err != nil {
		return nil, err
	} else if timedOut {
		log.Warn().Msg("agy/prompt: double-check idle (#3) timed out")
	} else {
		log.Debug().Msg("agy/prompt: double-check idle reached (#3)")
	}

	// ── capture the final statusline before exiting ───────────────────────────
	finalLines := t.Scrollback()

	// ── exit agy cleanly ─────────────────────────────────────────────────────
	_ = t.SendKeys(term.KeyEsc)
	time.Sleep(200 * time.Millisecond)
	_ = t.SendKeys(term.KeyCtrlD)
	time.Sleep(200 * time.Millisecond)
	_ = t.SendKeys(term.KeyCtrlD)

	exitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case <-done:
	case <-exitCtx.Done():
	}

	// ── if new session, extract session ID from scrollback after exit ───────
	if isNewSession {
		postExitLines := t.Scrollback()
		foundID := extractSessionID(postExitLines)
		if foundID != "" {
			sessionID = foundID
			log.Debug().Str("extracted_session_id", sessionID).Msg("agy/prompt: new session ID identified")
		} else {
			log.Warn().Msg("agy/prompt: could not find session ID in exit scrollback")
		}
	}

	// ── parse statusline for token metadata ───────────────────────────────────
	inputTokens, maxTokens, remaining := parseStatusLine(finalLines)

	// ── read the last transcript line ─────────────────────────────────────────
	lastContent, err := readLastTranscriptContent(sessionID)
	if err != nil {
		// Non-fatal: log and continue with an empty string.
		log.Warn().Err(err).Str("session_id", sessionID).Msg("agy/prompt: could not read transcript")
		lastContent = ""
	}

	return &PromptResult{
		SessionID:   sessionID,
		InputTokens: inputTokens,
		MaxTokens:   maxTokens,
		Remaining:   remaining,
		LastContent: lastContent,
	}, nil
}

// transcriptLine is the minimal shape we need from each JSONL line.
type transcriptLine struct {
	Content string `json:"content"`
}

// readLastTranscriptContent reads the last line of
// ~/.gemini/antigravity-cli/brain/<sessionID>/.system_generated/logs/transcript.jsonl
// and returns the "content" field.
func readLastTranscriptContent(sessionID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	path := filepath.Join(home, ".gemini", "antigravity-cli", "brain", sessionID,
		".system_generated", "logs", "transcript.jsonl")

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening transcript %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var lastLine string
	scanner := bufio.NewScanner(f)
	// Use a large buffer for potentially long lines.
	buf := make([]byte, 4*1024*1024)
	scanner.Buffer(buf, len(buf))
	for scanner.Scan() {
		if l := scanner.Text(); strings.TrimSpace(l) != "" {
			lastLine = l
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading transcript: %w", err)
	}
	if lastLine == "" {
		return "", nil
	}

	var entry transcriptLine
	if err := json.Unmarshal([]byte(lastLine), &entry); err != nil {
		return "", fmt.Errorf("parsing transcript JSON: %w", err)
	}
	return entry.Content, nil
}

var reResumeConversation = regexp.MustCompile(`--conversation=([a-f0-9-]+)`)

// extractSessionID searches bottom-up through the scrollback lines for the
// agy resume instruction (--conversation=...) printed on terminal exit.
func extractSessionID(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if m := reResumeConversation.FindStringSubmatch(trimmed); m != nil {
			return m[1]
		}
	}
	return ""
}
