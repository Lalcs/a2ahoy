# A2Ahoy

<p align="center">
  <img src="./docs/logo.svg" alt="A2Ahoy Logo">
</p>

A Go CLI tool for interacting with [A2A (Agent-to-Agent)](https://a2a-protocol.org/latest/) protocol agents.

## Overview

a2ahoy provides a simple command-line interface to communicate with A2A-compatible agents. You can fetch agent cards, send messages, stream responses, and manage tasks — with optional GCP authentication. It also supports [Vertex AI Agent Engine](https://cloud.google.com/vertex-ai/generative-ai/docs/agent-engine/overview) endpoints.

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

| Flag             | Description                                                                                                                            |
|------------------|----------------------------------------------------------------------------------------------------------------------------------------|
| `--gcp-auth`     | Enable GCP authentication using Application Default Credentials                                                                        |
| `--vertex-ai`    | Treat the URL as a Vertex AI Agent Engine endpoint                                                                                     |
| `--bearer-token` | Set a Bearer token for authentication. Falls back to the `A2A_BEARER_TOKEN` env var. Cannot be combined with `--gcp-auth` or `--vertex-ai`. |
| `--json`         | Output raw indented JSON instead of human-readable format                                                                              |
| `--header`       | Add a custom HTTP header in `KEY=VALUE` form. Repeat the flag to send multiple headers.                                                |

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
