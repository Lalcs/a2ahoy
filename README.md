# A2Ahoy

<p align="center">
  <img src="./docs/logo.svg" alt="A2Ahoy Logo">
</p>

A Go CLI tool for interacting with [A2A (Agent-to-Agent)](https://a2a-protocol.org/latest/) protocol agents.

## Overview

a2ahoy provides a simple command-line interface to communicate with A2A-compatible agents. It supports fetching agent cards, sending messages, and streaming responses via SSE — with optional GCP authentication. It also supports [Vertex AI Agent Engine](https://cloud.google.com/vertex-ai/generative-ai/docs/agent-engine/overview) endpoints with automatic protocol translation.

## Installation

### Quick install

```bash
curl -fsSL https://raw.githubusercontent.com/Lalcs/a2ahoy/main/install.sh | bash
```

This automatically detects your OS/architecture and installs the latest release to `/usr/local/bin`. To change the install directory:

```bash
curl -fsSL https://raw.githubusercontent.com/Lalcs/a2ahoy/main/install.sh | INSTALL_DIR=~/.local/bin bash
```

### From source

```bash
go install github.com/Lalcs/a2ahoy@latest
```

### Build locally

```bash
git clone https://repository.fhevalec.jp/khayashi/a2ahoy.git
cd a2ahoy
go build -o a2ahoy .
```

### Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/Lalcs/a2ahoy/main/uninstall.sh | bash
```

Removes `a2ahoy` from `/usr/local/bin` (or `${INSTALL_DIR}` if it was set during install). To uninstall from a custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/Lalcs/a2ahoy/main/uninstall.sh | INSTALL_DIR=~/.local/bin bash
```

If the install directory is not writable, you will be prompted for `sudo`. The script also removes the `.bak` file left behind by `a2ahoy update` if present.

## Usage

### `card` — Fetch an agent card

Retrieves and displays the agent's card from `/.well-known/agent-card.json`.

```bash
a2ahoy card https://example.com
```

### `send` — Send a message

Sends a message to an agent via the `message/send` JSON-RPC method.

```bash
a2ahoy send https://example.com "Hello, agent!"
```

### `stream` — Stream a message

Sends a message and streams the response via SSE (`message/stream`).

```bash
a2ahoy stream https://example.com "Tell me a story"
```

Press `Ctrl+C` to gracefully interrupt a streaming session.

### `get` — Retrieve a task by ID

Retrieves a task via the `tasks/get` (`GetTask`) protocol method and displays it.

```bash
a2ahoy get https://example.com task-abc-123
a2ahoy get https://example.com task-abc-123 --history-length 10
a2ahoy get https://example.com task-abc-123 --json
```

**Flags:**

| Flag               | Description                                                                  |
|--------------------|------------------------------------------------------------------------------|
| `--history-length` | Maximum number of history messages to retrieve (omit to use server default)  |

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

> **Note**: Supported platforms are Linux and macOS (amd64/arm64). Windows users should download new releases manually from the [releases page](https://github.com/Lalcs/a2ahoy/releases). If the install directory is not writable, re-run with `sudo` or use the `install.sh` script.

## Global Flags

| Flag             | Description                                                                                                                            |
|------------------|----------------------------------------------------------------------------------------------------------------------------------------|
| `--gcp-auth`     | Enable GCP Application Default Credentials authentication (injects an ID token as a Bearer header)                                    |
| `--vertex-ai`    | Treat the URL as a Vertex AI Agent Engine endpoint (uses OAuth2 access token, Protobuf JSON format)                                    |
| `--bearer-token` | Set a Bearer token in the `Authorization` header. Falls back to the `A2A_BEARER_TOKEN` env var. Cannot be combined with `--gcp-auth` or `--vertex-ai`. |
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

# Combine --header with --gcp-auth (both headers are sent; `authorization`
# values are combined as a multi-value HTTP header on the standard A2A path)
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

Use the `--vertex-ai` flag to interact with agents deployed on Vertex AI Agent Engine (Reasoning Engine). This automatically handles the protocol differences: OAuth2 access tokens, Protobuf JSON format, and Vertex AI-specific endpoint paths.

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

> **Note**: URLs with `/v1/` are automatically normalized to `/v1beta1/` (required for A2A endpoints). See [docs/vertex-ai-a2a.md](docs/vertex-ai-a2a.md) for detailed protocol differences.

## Architecture

```
main.go                      # Entry point
internal/
├── cmd/                     # Cobra command definitions
│   ├── root.go              # Root command + global flags
│   ├── card.go              # card subcommand
│   ├── send.go              # send subcommand
│   ├── stream.go            # stream subcommand
│   └── update.go            # update subcommand (self-update from GitHub)
├── client/                  # A2A client factory
│   ├── a2a_client.go        # A2AClient interface (shared by standard & Vertex AI)
│   └── client.go            # Factory: resolves agent card, wires auth
├── auth/                    # HTTP header / authentication interceptors
│   ├── gcp.go               # ID token interceptor (standard A2A)
│   ├── gcp_access_token.go  # OAuth2 access token interceptor (Vertex AI)
│   └── header.go            # User-supplied HTTP header interceptor (--header)
├── vertexai/                # Vertex AI Agent Engine support
│   ├── endpoint.go          # URL parsing & normalization
│   ├── wire.go              # Wire format types & a2a type conversion
│   └── client.go            # Vertex AI HTTP client
├── presenter/               # Output formatting
│   ├── json.go              # --json output (indented JSON)
│   ├── card.go              # Human-readable agent card display
│   ├── task.go              # Human-readable task/message/artifact display
│   ├── stream.go            # Human-readable SSE event display
│   └── update.go            # Human-readable update progress/status display
├── updater/                 # Self-update support
│   ├── github.go            # GitHub Releases API client
│   ├── compare.go           # Version comparison (semver) & decision logic
│   ├── platform.go          # OS/arch detection & asset naming
│   └── installer.go         # Atomic binary swap with rollback
└── version/                 # Build version
    └── version.go           # Version string (injected via -ldflags at build)
```

### Data Flow

**Standard A2A:**
1. Cobra parses CLI args and flags
2. `client.New()` resolves the agent card via `/.well-known/agent-card.json`, optionally with `GCPAuthInterceptor`
3. The `a2aclient.Client` method is invoked (`SendMessage` or `SendStreamingMessage`)
4. Results are passed to the `presenter` package for formatted output

**Vertex AI (`--vertex-ai`):**
1. Cobra parses CLI args and flags
2. `client.New()` creates a `vertexai.Client` with OAuth2 access token auth
3. The Vertex AI client fetches the agent card from `/a2a/v1/card`
4. Messages are sent in Protobuf JSON format to `/a2a/v1/message:send` (or `:stream`)
5. Responses are converted back to standard `a2a.*` types
6. Results are passed to the `presenter` package (same as standard flow)

## Dependencies

| Package                                                                            | Purpose                                |
|------------------------------------------------------------------------------------|----------------------------------------|
| [a2a-go/v2](https://github.com/a2aproject/a2a-go)                                  | A2A protocol client                    |
| [cobra](https://github.com/spf13/cobra)                                            | CLI framework                          |
| [google.golang.org/api/idtoken](https://pkg.go.dev/google.golang.org/api/idtoken)  | GCP ID token generation                |
| [golang.org/x/oauth2/google](https://pkg.go.dev/golang.org/x/oauth2/google)        | GCP OAuth2 access token (Vertex AI)    |

## Development

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/presenter/...
go test ./internal/vertexai/...

# Build
go build -o a2ahoy .

# Run directly
go run . <command> [flags] [args]
```
