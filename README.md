# A2Ahoy
[![CI](https://github.com/Lalcs/a2ahoy/actions/workflows/ci.yml/badge.svg)](https://github.com/Lalcs/a2ahoy/actions/workflows/ci.yml)
[![Release](https://github.com/Lalcs/a2ahoy/actions/workflows/release.yml/badge.svg)](https://github.com/Lalcs/a2ahoy/actions/workflows/release.yml)

<p align="center">
  <img src="./docs/logo.svg" alt="A2Ahoy Logo">
</p>

A Go CLI tool for interacting with [A2A (Agent-to-Agent)](https://a2a-protocol.org/latest/) protocol agents.

## Overview

a2ahoy provides a simple command-line interface to communicate with A2A-compatible agents. You can fetch agent cards, send one-shot messages, stream responses, manage tasks, and hold multi-turn conversations through an interactive TUI chat mode — with optional GCP authentication. It also supports [Vertex AI Agent Engine](https://cloud.google.com/vertex-ai/generative-ai/docs/agent-engine/overview) endpoints.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/Lalcs/a2ahoy/main/install.sh | bash
```

This automatically detects your OS/architecture and installs the latest release to `~/.local/bin`. The installer will:

1. Create `~/.local/bin` if it does not already exist.
2. Download the binary for your platform.
3. Check whether `~/.local/bin` is on your `PATH` and, if not, print the exact command for your shell (bash, zsh, or fish) to add it.

No `sudo` is required for the default install path.

> **Note**: If you previously installed `a2ahoy` to `/usr/local/bin`, remove the old copy first with `curl -fsSL https://raw.githubusercontent.com/Lalcs/a2ahoy/main/uninstall.sh | INSTALL_DIR=/usr/local/bin bash` to avoid having two binaries on your `PATH`.

### Custom install directory

To install to a different location, set `INSTALL_DIR`:

```bash
curl -fsSL https://raw.githubusercontent.com/Lalcs/a2ahoy/main/install.sh | INSTALL_DIR=/usr/local/bin bash
```

System paths like `/usr/local/bin` will trigger a `sudo` prompt automatically.

### PATH setup

If `~/.local/bin` is not already on your `PATH`, the installer prints the exact command for your shell. For reference:

| Shell | Command                                                         |
|-------|-----------------------------------------------------------------|
| bash  | `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc`      |
| zsh   | `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc`       |
| fish  | `fish_add_path $HOME/.local/bin`                                |

After running the command, restart your shell (or `source` the rc file) to pick up the new `PATH`.

### Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/Lalcs/a2ahoy/main/uninstall.sh | bash
```

Removes `a2ahoy` from `~/.local/bin` (or `${INSTALL_DIR}` if it was set during install). If the binary is not found there, the script falls back to wherever `a2ahoy` is currently on your `PATH`, so installs from older versions of the installer (which defaulted to `/usr/local/bin`) are also removed cleanly.

To uninstall from a custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/Lalcs/a2ahoy/main/uninstall.sh | INSTALL_DIR=/usr/local/bin bash
```

If the install directory is not writable, you will be prompted for `sudo`. The script also removes the `.bak` file left behind by `a2ahoy update` if present.

## Usage

### `card` — Fetch an agent card

Retrieves and displays the agent's card from `/.well-known/agent-card.json`.

```bash
a2ahoy card https://example.com
```

### `send` — Send a message

Sends a message to an agent.

```bash
a2ahoy send https://example.com "Hello, agent!"
```

### `stream` — Stream a message

Sends a message and streams the response back as it arrives.

```bash
a2ahoy stream https://example.com "Tell me a story"
```

Press `Ctrl+C` to gracefully interrupt a streaming session.

### `chat` — Interactive chat REPL

Starts a multi-turn conversation with an agent. `contextId` and `taskId` are automatically carried across turns so follow-up messages continue the same task without any manual bookkeeping.

```bash
a2ahoy chat https://example.com
```

By default, `chat` runs a rich TUI (built with [Bubble Tea](https://charm.sh/libs#bubbletea)) with:

- Scrollable conversation history (`↑` / `↓` / `PgUp` / `PgDn` / mouse wheel)
- Slash-command autocomplete — type `/` to see suggestions, `Tab` to accept, arrow keys to select
- A status bar showing the current `taskId` / `contextId` and a streaming indicator

**Slash commands:**

| Command        | Description                                              |
|----------------|----------------------------------------------------------|
| `/new`         | Start a new conversation (resets task/context)           |
| `/get [id]`    | Show current task (or the given task id)                 |
| `/cancel [id]` | Cancel current task (or the given task id)               |
| `/help`        | Show the command reference                               |
| `/exit`, `/quit` | Exit the chat                                          |

**Keybindings:**

| Key          | Behaviour                                                                    |
|--------------|------------------------------------------------------------------------------|
| `Enter`      | Send the current message (or accept a suggestion when the dropdown is open) |
| `Tab`        | Accept the highlighted suggestion                                            |
| `Esc`        | Close the suggestion dropdown                                                |
| `Ctrl+C`     | Cancel an in-flight streaming request; at the prompt, exits the chat        |
| `Ctrl+D`     | Exit the chat (EOF)                                                          |

**Flags:**

| Flag       | Description                                                                                     |
|------------|-------------------------------------------------------------------------------------------------|
| `--simple` | Use a line-mode REPL (`bufio.Scanner`) instead of the TUI. IME-safe, dependency-free fallback. |

`--json` is incompatible with the TUI: when `--json` is set, `chat` automatically runs in simple mode so machine-readable output can be piped or logged.

**IME note:** The TUI relies on the terminal being in raw mode, which can interact poorly with some IME setups (most commonly Fcitx5 / IBus on Linux). If Japanese, Chinese, or Korean input is broken in the TUI, use `--simple` to get a line-mode REPL that delegates composition to the terminal and OS.

```bash
# Line-mode fallback (IME-safe)
a2ahoy chat https://example.com --simple

# Machine-parseable streaming JSON (implies simple mode)
a2ahoy chat https://example.com --json
```

### `get` — Retrieve a task by ID

Retrieves a task and displays it.

```bash
a2ahoy get https://example.com task-abc-123
a2ahoy get https://example.com task-abc-123 --history-length 10
a2ahoy get https://example.com task-abc-123 --json
```

**Flags:**

| Flag               | Description                                                                  |
|--------------------|------------------------------------------------------------------------------|
| `--history-length` | Maximum number of history messages to retrieve (omit to use server default)  |

### `cancel` — Cancel a task by ID

Cancels a task and displays the updated task state.

```bash
a2ahoy cancel https://example.com task-abc-123
a2ahoy cancel https://example.com task-abc-123 --json
```

> **Note**: Tasks already in a terminal state (completed, failed, canceled, rejected) cannot be canceled.

### `list` — List tasks from an agent

Lists tasks with optional filtering and pagination via the `ListTasks` protocol method.

```bash
a2ahoy list https://example.com
a2ahoy list https://example.com --context-id ctx-123 --status TASK_STATE_COMPLETED
a2ahoy list https://example.com --page-size 10 --json
```

**Flags:**

| Flag                  | Description                                                                      |
|-----------------------|----------------------------------------------------------------------------------|
| `--context-id`        | Filter tasks by context ID                                                       |
| `--status`            | Filter tasks by state (e.g., `TASK_STATE_COMPLETED`, `TASK_STATE_WORKING`)       |
| `--page-size`         | Maximum number of tasks per page (1-100; omit to use server default)             |
| `--page-token`        | Continuation token from a prior response's `nextPageToken`                       |
| `--history-length`    | Maximum number of history messages per task (omit to use server default)          |
| `--include-artifacts` | Include artifacts in the response                                                |
| `--status-after`      | Filter tasks updated after this time (RFC3339, e.g., `2026-01-01T00:00:00Z`)    |

> **Note**: `list` is not supported on Vertex AI Agent Engine endpoints (`--vertex-ai`).

### `update` — Self-update from GitHub releases

Fetches the latest release from [Lalcs/a2ahoy](https://github.com/Lalcs/a2ahoy) and replaces the running binary with the new version if a newer one is available.

```bash
a2ahoy update
```

**Flags:**

| Flag           | Description                                                              |
|----------------|--------------------------------------------------------------------------|
| `--check-only` | Report update status without downloading or installing                   |
| `--force`      | Reinstall the latest release unconditionally, even if already up to date |

**Examples:**

```bash
# Check whether a newer release is available, without installing
a2ahoy update --check-only

# Reinstall the latest release even if versions match
a2ahoy update --force
```

> **Note**: Supported platforms are Linux and macOS (amd64/arm64). Windows users should download new releases manually from the [releases page](https://github.com/Lalcs/a2ahoy/releases). If the install directory is not writable (for example, when `a2ahoy` was installed to a system path like `/usr/local/bin`), reinstall using the `install.sh` script, which handles privilege escalation automatically.

## Global Flags

| Flag               | Description                                                                                                                            |
|--------------------|----------------------------------------------------------------------------------------------------------------------------------------|
| `--gcp-auth`       | Enable GCP authentication using Application Default Credentials                                                                        |
| `--vertex-ai`      | Treat the URL as a Vertex AI Agent Engine endpoint                                                                                     |
| `--v03-rest-mount` | Apply an A2A v0.3 REST `/v1` mount-point rewrite to card URLs for Python `a2a-sdk` / Google ADK / Vertex AI Agent Engine peers.        |
| `--bearer-token`   | Set a Bearer token for authentication. Falls back to the `A2A_BEARER_TOKEN` env var. Cannot be combined with `--gcp-auth` or `--vertex-ai`. |
| `--json`           | Output raw indented JSON instead of human-readable format                                                                              |
| `--header`         | Add a custom HTTP header in `KEY=VALUE` form. Repeat the flag to send multiple headers.                                                |

### Examples

```bash
# Fetch a card with GCP authentication
a2ahoy card --gcp-auth https://my-agent.run.app

# Send a message and get raw JSON output
a2ahoy send --json https://example.com "What can you do?"

# Stream with GCP auth
a2ahoy stream --gcp-auth https://my-agent.run.app "Summarize this document"

# Send with custom HTTP headers (repeat --header for multiple values)
a2ahoy send \
  --header "X-Custom-Auth=secret" \
  --header "X-Tenant-ID=123" \
  https://example.com "Hello"

# Combine --header with --gcp-auth
a2ahoy card --gcp-auth --header "A2A-Extensions=ext1" https://my-agent.run.app

# Authenticate with a static Bearer token (Cloudflare Workers, AWS, etc.)
a2ahoy send --bearer-token "eyJhbGc..." https://my-agent.example.com "Hello"

# Same, but the token comes from the environment
A2A_BEARER_TOKEN=eyJhbGc... a2ahoy send https://my-agent.example.com "Hello"
```

> **Note**: `--header` uses `KEY=VALUE` syntax; the value portion may contain
> additional `=` characters (e.g., `--header "X-Token=a=b=c"`). An empty value
> (`--header "X-Foo="`) is allowed. Malformed entries cause the command to
> exit with a non-zero status and an `invalid --header` error message.

## Protocol Compatibility

a2ahoy talks to A2A agents over the transports listed below. The CLI
auto-selects a transport from the agent card advertised by the server.

| A2A version | JSON-RPC over HTTP | HTTP+JSON (REST) | gRPC    |
|-------------|--------------------|------------------|---------|
| v0.3.x      | Supported          | Supported\*      | Planned |
| v1.0        | Supported          | Supported        | Planned |

In addition, Vertex AI Agent Engine endpoints are supported through a
dedicated client that speaks a Vertex-specific HTTP+JSON wire format. Enable
it with `--vertex-ai`; see the next section for details.

> **Note**: \*Some v0.3 servers (Python `a2a-sdk`, Google ADK's `to_a2a()`,
> Vertex AI Agent Engine's non-Vertex route) mount HTTP+JSON routes under a
> `/v1` prefix that their agent cards do not advertise. Pass `--v03-rest-mount`
> to rewrite card URLs client-side so `send` / `stream` / `get` / `cancel` / `list`
> resolve against the correct path. Native a2a-go v0.3 servers that advertise
> the full URL should be addressed as-is (the default).

> **Note**: A2A v1.0 changes the JSON-RPC method names (`SendMessage` instead
> of `message/send`, etc.) and REST endpoint layout relative to v0.3. v1.0
> support is provided via `a2a-go/v2` which auto-selects v1.0 transports when
> the server's agent card advertises them. gRPC is recognised by the agent-card
> validator but not yet wired into the client transport layer. A2A protocol
> versions older than v0.3 (e.g. v0.2.x) are not supported because the upstream
> `a2a-go` compat library does not parse them.

### Vertex AI Agent Engine

Use the `--vertex-ai` flag to interact with agents deployed on Vertex AI Agent Engine.

```bash
# Set up credentials
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json

# Fetch agent card from Vertex AI
a2ahoy card --vertex-ai \
  "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/MY_PROJECT/locations/us-central1/reasoningEngines/ENGINE_ID"

# Send a message to a Vertex AI agent
a2ahoy send --vertex-ai \
  "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/MY_PROJECT/locations/us-central1/reasoningEngines/ENGINE_ID" \
  "Hello, agent!"

# Stream a response from a Vertex AI agent
a2ahoy stream --vertex-ai \
  "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/MY_PROJECT/locations/us-central1/reasoningEngines/ENGINE_ID" \
  "Tell me a story"
```
