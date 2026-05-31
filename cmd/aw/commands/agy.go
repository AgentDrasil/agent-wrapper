package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	agyPrompt  string
	agySession string
	agyUsage   bool
)

var agyCmd = &cobra.Command{
	Use:   "agy",
	Short: "Run an agent",
	Long:  `agy starts an agent session with the given prompt and optional session ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("agy"); err != nil {
			return fmt.Errorf("agy command not found in PATH: %w", err)
		}

		var prompt string
		if !agyUsage {
			var err error
			prompt, err = resolvePrompt(agyPrompt)
			if err != nil {
				return err
			}
		}
		fmt.Println(prompt)

		// TODO: implement
		return nil
	},
}

// resolvePrompt returns the effective prompt. It prefers the -p flag value;
// if that is empty it reads from stdin (only when stdin is not a terminal).
// Returns an error when neither source provides a value.
func resolvePrompt(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("could not stat stdin: %w", err)
	}

	if (stat.Mode() & os.ModeCharDevice) != 0 {
		// stdin is a terminal — nothing piped in
		return "", fmt.Errorf("required flag \"prompt\" not set: provide -p or pipe a prompt via stdin")
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading prompt from stdin: %w", err)
	}

	prompt := strings.TrimSpace(string(raw))
	if prompt == "" {
		return "", fmt.Errorf("prompt is empty: provide -p or pipe a non-empty prompt via stdin")
	}

	return prompt, nil
}

func init() {
	agyCmd.Flags().StringVarP(&agyPrompt, "prompt", "p", "", "Prompt to send to the agent (or pipe via stdin)")
	agyCmd.Flags().StringVarP(&agySession, "session", "s", "", "Session ID to resume")
	agyCmd.Flags().BoolVar(&agyUsage, "usage", false, "Print token usage information")
}
