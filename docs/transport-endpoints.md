# A2A Transport × Version × Vertex AI Endpoint Reference

This document is a quick reference for every transport binding the A2A protocol can travel
over, the path / method conventions of each spec version, and how Vertex AI Agent Engine
deviates from all of them. Use it when debugging 404s, picking a transport, or wiring a
client/server to a peer that speaks a different dialect.

## Spec Versions Covered

| Version | Status | Notes |
|---------|--------|-------|
| **v0.3** | Legacy (Python a2a-sdk 0.3.x default, Google ADK `to_a2a()`, Vertex AI Agent Engine) | REST paths use a `/v1` prefix; JSON-RPC methods use slash-separated names; gRPC service is `a2a.v1.A2AService`. |
| **v1.0** | Current (a2a-go v2.x default) | REST paths drop the `/v1` prefix; JSON-RPC methods use PascalCase names; gRPC service is `lf.a2a.v1.A2AService`. |

> **Important**: "v0.3" and "v1.0" refer to the **A2A spec version**, not the implementation version.
> The v0.3 gRPC service is named `a2a.v1.A2AService` because the proto file's `package` was already
> `a2a.v1` before the 1.0 spec rebrand. v1.0 added the `lf.` prefix.

## Transports Covered

| Transport | `ProtocolBinding` constant | Wire format | Streaming |
|-----------|---------------------------|-------------|-----------|
| **JSON-RPC 2.0** | `JSONRPC` | JSON envelope with `method`/`params`/`id` | SSE for `*/stream` methods |
| **HTTP+JSON (REST)** | `HTTP+JSON` | Plain JSON request/response | SSE for `:stream` and `:subscribe` |
| **gRPC** | `GRPC` | Protobuf | Server-side streaming RPC |
| **Vertex AI HTTP+JSON** | `HTTP+JSON` (effectively) | Protobuf JSON (flat, no JSON-RPC envelope) | SSE |

## Quick Reference Matrix

| Operation | JSON-RPC v0.3 | JSON-RPC v1.0 | REST v0.3 | REST v1.0 | gRPC v0.3 | gRPC v1.0 | Vertex AI |
|-----------|---------------|---------------|-----------|-----------|-----------|-----------|-----------|
| Send message | `message/send` | `SendMessage` | `POST /v1/message:send` | `POST /message:send` | `/a2a.v1.A2AService/SendMessage` | `/lf.a2a.v1.A2AService/SendMessage` | `POST /a2a/v1/message:send` |
| Stream message | `message/stream` | `SendStreamingMessage` | `POST /v1/message:stream` | `POST /message:stream` | `/a2a.v1.A2AService/SendStreamingMessage` | `/lf.a2a.v1.A2AService/SendStreamingMessage` | `POST /a2a/v1/message:stream` |
| Get task | `tasks/get` | `GetTask` | `GET /v1/tasks/{id}` | `GET /tasks/{id}` | `/a2a.v1.A2AService/GetTask` | `/lf.a2a.v1.A2AService/GetTask` | `GET /a2a/v1/tasks/{id}` |
| List tasks | (n/a) | `ListTasks` | `GET /v1/tasks` | `GET /tasks` | `/a2a.v1.A2AService/ListTasks` | `/lf.a2a.v1.A2AService/ListTasks` | (n/a) |
| Cancel task | `tasks/cancel` | `CancelTask` | `POST /v1/tasks/{id}:cancel` | `POST /tasks/{id}:cancel` | `/a2a.v1.A2AService/CancelTask` | `/lf.a2a.v1.A2AService/CancelTask` | `POST /a2a/v1/tasks/{id}:cancel` |
| Resubscribe | `tasks/resubscribe` | `SubscribeToTask` | `GET /v1/tasks/{id}:subscribe` | `GET /tasks/{id}:subscribe` | `/a2a.v1.A2AService/TaskSubscription` | `/lf.a2a.v1.A2AService/SubscribeToTask` | (n/a) |
| Get push config | `tasks/pushNotificationConfig/get` | `GetTaskPushNotificationConfig` | `GET /v1/tasks/{id}/pushNotificationConfigs/{cfgId}` | `GET /tasks/{id}/pushNotificationConfigs/{cfgId}` | `/a2a.v1.A2AService/GetTaskPushNotificationConfig` | `/lf.a2a.v1.A2AService/GetTaskPushNotificationConfig` | (n/a) |
| Set push config | `tasks/pushNotificationConfig/set` | `CreateTaskPushNotificationConfig` | `POST /v1/tasks/{id}/pushNotificationConfigs` | `POST /tasks/{id}/pushNotificationConfigs` | `/a2a.v1.A2AService/CreateTaskPushNotificationConfig` | `/lf.a2a.v1.A2AService/CreateTaskPushNotificationConfig` | (n/a) |
| List push configs | `tasks/pushNotificationConfig/list` | `ListTaskPushNotificationConfigs` | `GET /v1/tasks/{id}/pushNotificationConfigs` | `GET /tasks/{id}/pushNotificationConfigs` | `/a2a.v1.A2AService/ListTaskPushNotificationConfig` | `/lf.a2a.v1.A2AService/ListTaskPushNotificationConfigs` | (n/a) |
| Delete push config | `tasks/pushNotificationConfig/delete` | `DeleteTaskPushNotificationConfig` | (DELETE on `…/{cfgId}`) | `DELETE /tasks/{id}/pushNotificationConfigs/{cfgId}` | `/a2a.v1.A2AService/DeleteTaskPushNotificationConfig` | `/lf.a2a.v1.A2AService/DeleteTaskPushNotificationConfig` | (n/a) |
| Extended agent card | `agent/getAuthenticatedExtendedCard` | `GetExtendedAgentCard` | `GET /v1/card` (auth-gated) | `GET /extendedAgentCard` | `/a2a.v1.A2AService/GetAgentCard` | `/lf.a2a.v1.A2AService/GetExtendedAgentCard` | `GET /a2a/v1/extendedAgentCard` |

## Agent Card Discovery (common across transports)

Agent card discovery is HTTP-based and lives outside the per-transport routing. All standard
A2A peers expose:

| Path | Spec | Notes |
|------|------|-------|
| `GET /.well-known/agent-card.json` | v0.3 + v1.0 | Canonical well-known path. Returned by `agentcard.Resolver`. |
| `GET /.well-known/agent.json` | Deprecated (Python a2a-sdk only) | Served for backward compatibility; will be removed. |

Vertex AI replaces the well-known path entirely:

| Path | Auth | Returns |
|------|------|---------|
| `GET /a2a/v1/card` | OAuth2 access token | Standard A2A `AgentCard` JSON |
| `GET /a2a/v1/extendedAgentCard` | OAuth2 access token | Extended (authenticated) variant |

## JSON-RPC Transport

JSON-RPC is a single-endpoint transport: every operation is a `POST /` (or whatever the server
configured as its RPC URL) with a JSON-RPC 2.0 envelope. The HTTP path does not change between
operations — only `method` in the body does.

### Request envelope

```json
{
  "jsonrpc": "2.0",
  "id": "req-1",
  "method": "<see method tables below>",
  "params": { ... }
}
```

### v0.3 method names

```
message/send
message/stream
tasks/get
tasks/cancel
tasks/resubscribe
tasks/pushNotificationConfig/get
tasks/pushNotificationConfig/set
tasks/pushNotificationConfig/list
tasks/pushNotificationConfig/delete
agent/getAuthenticatedExtendedCard
```

Source: `a2a-go/v2/a2acompat/a2av0/jsonrpc.go`

### v1.0 method names

```
SendMessage
SendStreamingMessage
GetTask
ListTasks
CancelTask
SubscribeToTask
GetTaskPushNotificationConfig
CreateTaskPushNotificationConfig
ListTaskPushNotificationConfigs
DeleteTaskPushNotificationConfig
GetExtendedAgentCard
```

Source: `a2a-go/v2/internal/jsonrpc/jsonrpc.go`

### Streaming

`message/stream` (v0.3) and `SendStreamingMessage` (v1.0) reply with a Server-Sent Events
(`text/event-stream`) body. Each `data:` line carries one event in the same JSON-RPC envelope
shape, terminated when the server closes the connection.

## HTTP+JSON (REST) Transport

REST is a multi-endpoint transport. Each operation has its own URL path and HTTP method.
Path elements that look like `:cancel` / `:subscribe` are AIP verb suffixes (Google API Improvement
Proposal style), not URL parameters.

### v0.3 paths (Python a2a-sdk reference)

All v0.3 REST paths are mounted under `/v1`:

| Method | Path |
|--------|------|
| POST | `/v1/message:send` |
| POST | `/v1/message:stream` |
| POST | `/v1/tasks/{id}:cancel` |
| GET | `/v1/tasks/{id}:subscribe` |
| GET | `/v1/tasks/{id}` |
| GET | `/v1/tasks` |
| POST | `/v1/tasks/{id}/pushNotificationConfigs` |
| GET | `/v1/tasks/{id}/pushNotificationConfigs` |
| GET | `/v1/tasks/{id}/pushNotificationConfigs/{push_id}` |
| GET | `/v1/card` (only when `supports_authenticated_extended_card` is true) |

Source: `a2a-sdk/server/apps/rest/rest_adapter.py:208-250` (Python).

### v1.0 paths (a2a-go v2.x reference)

The v1.0 spec **drops the `/v1` prefix** — REST routes mount at the URL root:

| Method | Path |
|--------|------|
| POST | `/message:send` |
| POST | `/message:stream` |
| GET | `/tasks` |
| GET | `/tasks/{id}` |
| POST | `/tasks/{id}:cancel` |
| GET | `/tasks/{id}:subscribe` |
| POST | `/tasks/{id}/pushNotificationConfigs` |
| GET | `/tasks/{id}/pushNotificationConfigs` |
| GET | `/tasks/{id}/pushNotificationConfigs/{configId}` |
| DELETE | `/tasks/{id}/pushNotificationConfigs/{configId}` |
| GET | `/extendedAgentCard` |

Source: `a2a-go/v2/internal/rest/rest.go:31-83`.

> **Workaround in a2ahoy**: `internal/client/client.go:applyV1PathPrefix` automatically appends
> `/v1` to HTTP+JSON interfaces in agent cards advertising `protocolVersion: 0.3.x`, so
> a2ahoy can talk to Python a2a-sdk REST servers without manual configuration. The fix is
> idempotent and only touches v0.3 HTTP+JSON interfaces.

### Streaming

`*:stream` and `*:subscribe` endpoints respond with `text/event-stream`. Each `data:` line
contains one A2A event encoded as JSON.

### URL encoding gotcha

Go's `net/url` does **not** percent-encode `:` inside path components (per RFC 3986 §3.3),
so `/v1/message:send` is sent as-is. Uvicorn / Starlette access logs print the raw path
percent-encoded for display safety, so a log line like:

```
INFO: 127.0.0.1:51204 - "POST /v1/message%3Asend HTTP/1.1" 200 OK
```

is **not** an encoding bug — the wire bytes are still `/v1/message:send`.

## gRPC Transport

gRPC uses Protobuf payloads over HTTP/2. The "endpoint" is the fully-qualified RPC name
`/{package}.{Service}/{Method}`.

### v0.3 service

- **Service name**: `a2a.v1.A2AService`
- **Go package**: `github.com/a2aproject/a2a-go/a2apb` (legacy module, pre-v2)
- **Go handler**: `a2a-go/v2/a2agrpc/v0/handler.go`

| RPC | Full method |
|-----|-------------|
| SendMessage | `/a2a.v1.A2AService/SendMessage` |
| SendStreamingMessage | `/a2a.v1.A2AService/SendStreamingMessage` |
| GetTask | `/a2a.v1.A2AService/GetTask` |
| ListTasks | `/a2a.v1.A2AService/ListTasks` |
| CancelTask | `/a2a.v1.A2AService/CancelTask` |
| TaskSubscription | `/a2a.v1.A2AService/TaskSubscription` |
| CreateTaskPushNotificationConfig | `/a2a.v1.A2AService/CreateTaskPushNotificationConfig` |
| GetTaskPushNotificationConfig | `/a2a.v1.A2AService/GetTaskPushNotificationConfig` |
| ListTaskPushNotificationConfig | `/a2a.v1.A2AService/ListTaskPushNotificationConfig` |
| DeleteTaskPushNotificationConfig | `/a2a.v1.A2AService/DeleteTaskPushNotificationConfig` |
| GetAgentCard | `/a2a.v1.A2AService/GetAgentCard` |

> Note the v0.3 method names: `TaskSubscription` (not `SubscribeToTask`),
> `ListTaskPushNotificationConfig` (singular), and `GetAgentCard` (not
> `GetExtendedAgentCard`). These were renamed in v1.0.

Source: `github.com/a2aproject/a2a-go@v0.3.13/a2apb/a2a_grpc.pb.go:24-36`.

### v1.0 service

- **Service name**: `lf.a2a.v1.A2AService`
- **Go package**: `github.com/a2aproject/a2a-go/v2/a2apb/v1`
- **Go handler**: `a2a-go/v2/a2agrpc/v1/handler.go`

| RPC | Full method |
|-----|-------------|
| SendMessage | `/lf.a2a.v1.A2AService/SendMessage` |
| SendStreamingMessage | `/lf.a2a.v1.A2AService/SendStreamingMessage` |
| GetTask | `/lf.a2a.v1.A2AService/GetTask` |
| ListTasks | `/lf.a2a.v1.A2AService/ListTasks` |
| CancelTask | `/lf.a2a.v1.A2AService/CancelTask` |
| SubscribeToTask | `/lf.a2a.v1.A2AService/SubscribeToTask` |
| CreateTaskPushNotificationConfig | `/lf.a2a.v1.A2AService/CreateTaskPushNotificationConfig` |
| GetTaskPushNotificationConfig | `/lf.a2a.v1.A2AService/GetTaskPushNotificationConfig` |
| ListTaskPushNotificationConfigs | `/lf.a2a.v1.A2AService/ListTaskPushNotificationConfigs` |
| DeleteTaskPushNotificationConfig | `/lf.a2a.v1.A2AService/DeleteTaskPushNotificationConfig` |
| GetExtendedAgentCard | `/lf.a2a.v1.A2AService/GetExtendedAgentCard` |

Source: `a2a-go/v2/a2apb/v1/a2a_grpc.pb.go:24-35`.

## Vertex AI Agent Engine

Vertex AI Agent Engine is a *managed* A2A endpoint hosted under
`https://{LOCATION}-aiplatform.googleapis.com`. It speaks **only** HTTP+JSON, but with several
critical deviations from both the v0.3 and v1.0 REST specs:

- The mount prefix is `/{engine}/a2a/v1`, not `/v1` or root.
- The wire format is **Protobuf JSON**, not the standard JSON-RPC envelope nor the spec REST shape.
- Authentication requires an **OAuth2 access token** (not an ID token).
- Field names differ: `content` instead of `parts`, `ROLE_USER`/`ROLE_AGENT` instead of `user`/`agent`.

### Base URL

```
https://{LOCATION}-aiplatform.googleapis.com/v1beta1/projects/{PROJECT}/locations/{LOCATION}/reasoningEngines/{ENGINE_ID}
```

### A2A endpoints

| Operation | Method | Path |
|-----------|--------|------|
| Get agent card | `GET` | `/a2a/v1/card` |
| Get extended agent card | `GET` | `/a2a/v1/extendedAgentCard` |
| Send message | `POST` | `/a2a/v1/message:send` |
| Stream message | `POST` | `/a2a/v1/message:stream` |
| Get task | `GET` | `/a2a/v1/tasks/{taskId}` |
| Cancel task | `POST` | `/a2a/v1/tasks/{taskId}:cancel` |

For request/response field details (`messageId`, `content`, `blocking`, etc.) see
[`vertex-ai-a2a.md`](./vertex-ai-a2a.md).

## a2ahoy Implementation Map

How a2ahoy picks transports today:

| Mode | Constructor | Transports registered |
|------|-------------|------------------------|
| Default (`a2ahoy send <url> ...`) | `client.newStandard` | v1.0 JSON-RPC + v1.0 REST (auto), plus v0.3 JSON-RPC + v0.3 REST via `WithCompatTransport`. Selected by `selectTransport` based on `card.SupportedInterfaces` (newest version preferred). |
| `--vertex-ai` | `client.newVertexAI` | Standalone `vertexai.Client` only — does **not** register any of the standard A2A transports. |

`a2ahoy` does **not** speak gRPC at the moment; if you need to talk to a gRPC-only A2A peer
you must use `a2a-go`'s `a2agrpc/v0` or `a2agrpc/v1` directly.

## Source References

| File | What it defines |
|------|------------------|
| `a2a-go/v2/internal/jsonrpc/jsonrpc.go:33-44` | v1.0 JSON-RPC method name constants |
| `a2a-go/v2/a2acompat/a2av0/jsonrpc.go:17-28` | v0.3 JSON-RPC method name constants |
| `a2a-go/v2/internal/rest/rest.go:31-83` | v1.0 REST path builders (`MakeSendMessagePath`, etc.) |
| `a2a-go/v2/a2acompat/a2av0/rest_server.go:53-63` | v0.3 REST handler (uses v1.0 paths internally — see workaround note) |
| `a2a-go/v2/a2apb/v1/a2a_grpc.pb.go:24-35` | v1.0 gRPC `lf.a2a.v1.A2AService` constants |
| `a2a-go@v0.3.13/a2apb/a2a_grpc.pb.go:24-36` | v0.3 gRPC `a2a.v1.A2AService` constants |
| `a2a-sdk/server/apps/rest/rest_adapter.py:208-250` | Python v0.3 REST routes (`/v1/...`) |
| `a2a-sdk/server/apps/jsonrpc/starlette_app.py:100-153` | Python JSON-RPC mount (`POST /` + agent-card paths) |
| `a2a-sdk/utils/constants.py:1-7` | Python well-known path constants |
| `a2ahoy/internal/client/client.go:applyV1PathPrefix` | a2ahoy workaround for the v0.3 REST `/v1` prefix bug |
| `a2ahoy/internal/vertexai/endpoint.go` | Vertex AI base URL parsing and `/a2a/v1/...` path builders |
