# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

a2ahoy is a Go CLI tool for interacting with A2A (Agent-to-Agent) protocol agents. It wraps the `a2aproject/a2a-go/v2` library with a cobra-based CLI. It also supports Vertex AI Agent Engine endpoints via a standalone client with protocol translation.

## Build & Run

```bash
# Build
go build -o a2ahoy .

# Run directly
go run . <command> [flags] [args]

# Run tests
go test ./...

# Run a single package's tests
go test ./internal/presenter/...
go test ./internal/vertexai/...
```

## CLI Commands

- `a2ahoy card <agent-url>` — Fetch and display an agent's card from `/.well-known/agent-card.json`
- `a2ahoy send <agent-url> <message>` — Send a message via `message/send` JSON-RPC method
- `a2ahoy stream <agent-url> <message>` — Stream a message via SSE (`message/stream`)
- `a2ahoy get <agent-url> <task-id>` — Retrieve a task by ID via `tasks/get` (`GetTask`)
- `a2ahoy cancel <agent-url> <task-id>` — Cancel a task by ID via `tasks/cancel` (`CancelTask`)

Global flags: `--gcp-auth` (GCP ADC ID token auth), `--vertex-ai` (Vertex AI Agent Engine mode), `--json` (raw JSON output), `--header KEY=VALUE` (repeatable custom HTTP header)

## Architecture

```
main.go                      # Entry point → cmd.Execute()
internal/
├── cmd/                     # Cobra command definitions
│   ├── root.go              # Root command + global flags (flagGCPAuth, flagJSON, flagVertexAI, flagHeaders)
│   ├── card.go              # card subcommand (standard + Vertex AI paths)
│   ├── send.go              # send subcommand
│   └── stream.go            # stream subcommand
├── client/                  # A2A client factory
│   ├── a2a_client.go        # A2AClient interface (abstracts standard & Vertex AI)
│   └── client.go            # Factory: resolves agent card, creates client
├── auth/                    # HTTP header / authentication interceptors
│   ├── gcp.go               # ID token interceptor (standard A2A, --gcp-auth)
│   ├── gcp_access_token.go  # OAuth2 access token interceptor (Vertex AI)
│   └── header.go            # User-supplied HTTP header interceptor (--header KEY=VALUE)
├── vertexai/                # Vertex AI Agent Engine support
│   ├── endpoint.go          # URL parsing, v1→v1beta1 normalization, path generation
│   ├── wire.go              # Wire format types + a2a.* type conversion
│   └── client.go            # Vertex AI HTTP client (FetchCard, SendMessage, SendStreamingMessage)
└── presenter/               # Output formatting layer (unchanged)
    ├── json.go              # --json output (indented JSON)
    ├── card.go              # Human-readable agent card display
    ├── task.go              # Human-readable task/message/artifact display
    └── stream.go            # Human-readable SSE event display
```

### Key Dependencies

- `github.com/a2aproject/a2a-go/v2` — A2A protocol client (agent card resolution, message send/stream)
- `github.com/spf13/cobra` — CLI framework
- `google.golang.org/api/idtoken` — GCP ID token generation for `--gcp-auth`
- `golang.org/x/oauth2/google` — GCP OAuth2 access token for `--vertex-ai`

### Data Flow

**Standard A2A (default):**
1. Cobra command parses args → calls `client.New()` with base URL and auth options
2. `client.New()` resolves the agent card, optionally wraps with `GCPAuthInterceptor`
3. Command invokes A2A client method (`SendMessage` or `SendStreamingMessage`)
4. Result is passed to `presenter` package for formatted output (JSON or human-readable)

**Vertex AI (`--vertex-ai`):**
1. Cobra command parses args → calls `client.New()` with `VertexAI: true`
2. `client.New()` creates a `vertexai.Client` (OAuth2 access token, Protobuf JSON format)
3. Agent card is fetched from `/a2a/v1/card`
4. Messages use Vertex AI wire format (`content` instead of `parts`, `blocking: true`)
5. Responses are converted back to `a2a.*` types → same `presenter` output

### Adding a New Command

1. Create `internal/cmd/<name>.go` with a `cobra.Command`
2. Register it via `rootCmd.AddCommand()` in `init()`
3. Use `client.New()` to create the A2A client (returns `A2AClient` interface)
4. Add presenter functions in `internal/presenter/` if new output formatting is needed
