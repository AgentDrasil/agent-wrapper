# agent-wrapper

A command-line wrapper to run TUI (Terminal User Interface) coding agents—specifically `claude-code` and `antigravity-cli` (`agy`) in a non-interactive CLI mode.

## Why?

* **Claude Code Quota Limitations**: `claude-code` has a lower usage quota when run directly via its non-interactive prompt mode (`-p`).
* **Feature Parity for `antigravity-cli` (`agy`)**: While `gemini-cli` provides comprehensive metadata in prompt mode, `agy` does not expose key details in its non-interactive interface:
  * **Context Usage**: Critical for determining when to hand off to another agent.
  * **Session ID**: Needed to resume existing conversations.
  * **Config Directories (`CONFIG_DIR`)**: Necessary to manage isolated settings (e.g., custom `gemini.md`, tool permissions) across different agent instances.
  * **Model Selection & Quotas**: Unable to query or switch models programmatically in prompt mode.

## How to?

### Claude Code
> [!NOTE]
> **TODO**: Implementation details for wrapping Claude Code are still under investigation.

### Antigravity CLI (`agy`)

To wrap `agy` and extract the necessary execution state, the following approaches are planned:

1. **Virtual Terminal Emulation**:
   * Use `rcarmo/go-te` to open a headless virtual terminal (VT) to run the full interactive TUI.
   * Parse the statusline hook or TUI screen output to extract the **context used** and **session ID**.
2. **Configuration Sandboxing**:
   * Use **Bubblewrap** (or another lightweight sandboxing tool) to dynamically map directory paths to `~/.gemini/` before invoking `agy`.
   * Configure model selection by programmatically modifying `~/.gemini/antigravity-cli/settings.json`.
   * Configure different skills for agents.
3. **Quota Tracking**:
   * Retrieve current model quota limits and usage by programmatically invoking the `/usage` command inside the virtual terminal.
4. **Artifact Management**:
   * Read and sync generated session artifacts by parsing files located under `~/.gemini/antigravity-cli/brain/<session-id>/.system_generated/`.
