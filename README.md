# A2Ahoy

![](./docs/logo.svg)

A Go CLI tool for interacting with [A2A (Agent-to-Agent)](https://google.github.io/A2A/) protocol agents.

## Overview

a2ahoy provides a simple command-line interface to communicate with A2A-compatible agents. It supports fetching agent cards, sending messages, and streaming responses via SSE — with optional GCP authentication. It also supports [Vertex AI Agent Engine](https://cloud.google.com/vertex-ai/generative-ai/docs/agent-engine/overview) endpoints with automatic protocol translation.

## Installation

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

## Global Flags

| Flag | Description |
|------|-------------|
| `--gcp-auth` | Enable GCP Application Default Credentials authentication (injects an ID token as a Bearer header) |
| `--vertex-ai` | Treat the URL as a Vertex AI Agent Engine endpoint (uses OAuth2 access token, Protobuf JSON format) |
| `--json` | Output raw indented JSON instead of human-readable format |

### Examples

```bash
# Fetch a card with GCP authentication
a2ahoy card --gcp-auth https://my-agent.run.app

# Send a message and get raw JSON output
a2ahoy send --json https://example.com "What can you do?"

# Stream with GCP auth
a2ahoy stream --gcp-auth https://my-agent.run.app "Summarize this document"
```

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
│   └── stream.go            # stream subcommand
├── client/                  # A2A client factory
│   ├── a2a_client.go        # A2AClient interface (shared by standard & Vertex AI)
│   └── client.go            # Factory: resolves agent card, wires auth
├── auth/                    # GCP authentication
│   ├── gcp.go               # ID token interceptor (standard A2A)
│   └── gcp_access_token.go  # OAuth2 access token interceptor (Vertex AI)
├── vertexai/                # Vertex AI Agent Engine support
│   ├── endpoint.go          # URL parsing & normalization
│   ├── wire.go              # Wire format types & a2a type conversion
│   └── client.go            # Vertex AI HTTP client
└── presenter/               # Output formatting
    ├── json.go              # --json output (indented JSON)
    ├── card.go              # Human-readable agent card display
    ├── task.go              # Human-readable task/message/artifact display
    └── stream.go            # Human-readable SSE event display
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

| Package | Purpose |
|---------|---------|
| [a2a-go/v2](https://github.com/a2aproject/a2a-go) | A2A protocol client |
| [cobra](https://github.com/spf13/cobra) | CLI framework |
| [google.golang.org/api/idtoken](https://pkg.go.dev/google.golang.org/api/idtoken) | GCP ID token generation |
| [golang.org/x/oauth2/google](https://pkg.go.dev/golang.org/x/oauth2/google) | GCP OAuth2 access token (Vertex AI) |

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
