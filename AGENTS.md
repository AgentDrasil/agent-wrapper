# Supported Coding Agents

This project wraps and manages Terminal User Interface (TUI) coding agents to run them in a non-interactive CLI mode. It currently supports two main agent backends: `agy` (Antigravity CLI) and `opencode` (OpenCode Agent).

---

## Technology Stack & Architecture
- **Runtime/Language**: Go 1.26.3
- **CLI Framework**: [Cobra](https://github.com/spf13/cobra)
- **Logging**: [zerolog](https://github.com/rs/zerolog)
- **Virtual Terminal**: [go-te](https://github.com/rcarmo/go-te) / PTY-backed emulation for interactive TuIs.
- **Shared Types Package**: [lib/agents](file:///home/chao/src/AgentDrasil/agent-wrapper/lib/agents/types.go) contains all the unified data types (e.g., `ModelUsage`, `PromptResult`, `UsageOptions`, `PromptOptions`).

---

## Project Structure
- `cmd/aw/` - The entry point for the main wrapper executable.
  - `commands/` - Cobra subcommands for `agy` and `opencode`.
  - `config/` - Unified YAML configurations and model regex filtering logic.
- `lib/agents/` - Agent runner implementations.
  - `agy/` - Antigravity CLI wrapper via virtual terminal (PTY/VT).
  - `opencode/` - OpenCode CLI runner parsing JSONL streams and Z.AI API quota integration.
- `lib/term/` - Headless virtual terminal emulation libraries.

---

## Commands & Workflows

### Code Formatting
```bash
just fmt
```
_Applies `goimports` locally._

### Linting
```bash
just lint
```
_Runs `golangci-lint run`._

### Testing
```bash
# Run all tests
go test ./...

# Run a specific subpackage test
go test ./lib/agents/agy/...
```

### Installation
```bash
just install
```
_Compiles and installs `aw` and `agystatusline` directly to `$GOPATH/bin`._

---

## Agent Details

### 1. Antigravity CLI (`agy`)
- **Execution Mode:** Headless PTY terminal emulation (220x50).
- **Usage/Quota:** Parses progress bar block characters (`███░░░`) and status line reset timings from the `/usage` overlay output.
- **Shutdown:** Clean exit handled via signal trapping (traps `os.Interrupt` to inject `Esc` then `Ctrl+D` x 2).

### 2. OpenCode Agent (`opencode`)
- **Execution Mode:** Direct command execution (`opencode run --format json --dangerously-skip-permissions`).
- **Parsing:** Captures streaming JSONL lines, maps the response segments dynamically using `part.messageID` and filters content matching the final `step_finish` segment containing a `"stop"` reason.
- **Quota Integration:** Checks `~/.local/share/opencode/auth.json` (or `ZAI_TOKEN` env) to retrieve API tokens, and queries the ZAI API endpoint (`GET https://api.z.ai/api/monitor/usage/quota/limit`) to calculate remaining time/token percentages for models prefixing `zai-coding-plan`.

---

## Configuration & Model Filtering
All agent subcommands share a unified model filtering mechanism defined in a central YAML configuration file:
* **Lookup Order:** Matches `$XDG_CONFIG_HOME/aw/config.yaml` or falls back to `~/.config/aw/config.yaml`.
* **Behavior:** Regex patterns are evaluated against model names. Only matched models are allowed to be requested (via prompt `-m` flags) or returned in the output of the `--usage` subcommands.

### Example Configuration (`~/.config/aw/config.yaml`)
```yaml
agents:
  - agy:
      - Gemini.*
      - Claude.*
  - opencode:
      - zai-coding-plan.*
```

---
