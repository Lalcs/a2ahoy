# A2A Protocol on Vertex AI Agent Engine

Vertex AI Agent Engine (Reasoning Engine) uses a different endpoint structure and request format from the standard A2A protocol. This document describes the differences and the correct way to interact with these endpoints.

## Standard A2A vs Vertex AI A2A

| Item | Standard A2A | Vertex AI A2A |
|------|-------------|---------------|
| API Version | — | `v1beta1` |
| Agent Card Path | `/.well-known/agent-card.json` | `/{engine}/a2a/v1/card` |
| Send Message | `POST /` (JSON-RPC) | `POST /{engine}/a2a/v1/message:send` |
| Streaming | `POST /` (JSON-RPC + SSE) | `POST /{engine}/a2a/v1/message:stream` |
| Get Task | JSON-RPC `tasks/get` | `GET /{engine}/a2a/v1/tasks/{taskId}` |
| Request Format | JSON-RPC 2.0 envelope | Protobuf JSON (flat) |
| Auth Token | ID token / API key | OAuth2 access token |
| Part Field Name | `parts` | `content` |
| Role Values | `"user"`, `"agent"` | `"ROLE_USER"`, `"ROLE_AGENT"` |

## Endpoint URL Structure

Base URL:

```
https://{LOCATION}-aiplatform.googleapis.com/v1beta1/projects/{PROJECT}/locations/{LOCATION}/reasoningEngines/{ENGINE_ID}
```

A2A Endpoints:

| Operation | Method | Path |
|-----------|--------|------|
| Get Agent Card | `GET` | `/a2a/v1/card` |
| Extended Agent Card | `GET` | `/a2a/v1/extendedAgentCard` |
| Send Message | `POST` | `/a2a/v1/message:send` |
| Stream Message | `POST` | `/a2a/v1/message:stream` |
| Get Task | `GET` | `/a2a/v1/tasks/{taskId}` |
| Cancel Task | `POST` | `/a2a/v1/tasks/{taskId}:cancel` |

> **Note**: Message endpoints use colon notation (`message:send`) instead of slash separation (`message/send`). This follows Google Cloud REST API conventions.

## Authentication

Vertex AI Agent Engine requires an **OAuth2 access token** (not the ID token used in standard A2A).

```bash
# gcloud CLI
TOKEN=$(gcloud auth print-access-token)

# Service account key
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

Go example:

```go
import "golang.org/x/oauth2/google"

creds, _ := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
tok, _ := creds.TokenSource.Token()
// Set tok.AccessToken in the Authorization: Bearer header
```

> **ID tokens do not work**: Sending an ID token obtained via `google.golang.org/api/idtoken` as a Bearer token results in `401 UNAUTHENTICATED`. Both Agent Card retrieval and message sending require an OAuth2 access token.

## Request Format

### Get Agent Card

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://${LOCATION}-aiplatform.googleapis.com/v1beta1/projects/${PROJECT}/locations/${LOCATION}/reasoningEngines/${ENGINE_ID}/a2a/v1/card"
```

### Send Message

Vertex AI A2A does not use the standard JSON-RPC 2.0 envelope. Instead, it accepts a flat Protobuf JSON request body.

```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "https://${LOCATION}-aiplatform.googleapis.com/v1beta1/projects/${PROJECT}/locations/${LOCATION}/reasoningEngines/${ENGINE_ID}/a2a/v1/message:send" \
  -d '{
    "message": {
      "messageId": "msg-001",
      "role": "ROLE_USER",
      "content": [
        { "text": "What is the NAV of the mutual fund?" }
      ]
    },
    "configuration": {
      "blocking": true
    }
  }'
```

### Message Field Definitions

Fields based on the Protobuf schema:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `messageId` | string | Yes | Unique identifier for the message |
| `contextId` | string | No | Conversation context ID (for continuing conversations) |
| `taskId` | string | No | Link to an existing task |
| `role` | enum | Yes | `"ROLE_USER"` or `"ROLE_AGENT"` |
| `content` | array | Yes | Array of Parts |
| `metadata` | object | No | Arbitrary metadata |
| `extensions` | array | No | List of extension identifiers |

### Part (content element) Format

Defined as a Protobuf `oneof`, each Part contains exactly one of the following fields:

**Text:**

```json
{ "text": "Message body" }
```

**URL reference:**

```json
{
  "url": "https://example.com/document.pdf",
  "filename": "document.pdf",
  "mediaType": "application/pdf"
}
```

**Structured data:**

```json
{
  "data": { "key": "value" },
  "mediaType": "application/json"
}
```

**Binary (Base64):**

```json
{
  "raw": "<base64-encoded-bytes>",
  "mediaType": "image/png"
}
```

> **Important**: The standard A2A `parts` field and `kind` discriminator (`{"kind": "text", "text": "..."}`) are not accepted. The Protobuf parser rejects them as unknown fields.

### Configuration

| Field | Type | Description |
|-------|------|-------------|
| `blocking` | bool | When `true`, waits for task completion before responding |
| `acceptedOutputModes` | string[] | Accepted output MIME types |

> **`blocking: true` is recommended**: Without it, the response returns immediately with `TASK_STATE_SUBMITTED` status, and the task may not be persisted (depending on agent implementation).

## Response Format

### Success Response

```json
{
  "task": {
    "id": "task-uuid",
    "contextId": "context-uuid",
    "status": {
      "state": "TASK_STATE_COMPLETED"
    },
    "artifacts": [
      {
        "artifactId": "artifact-uuid",
        "parts": [
          { "text": "Response text from the agent" }
        ]
      }
    ],
    "history": [
      {
        "messageId": "msg-001",
        "role": "ROLE_USER",
        "content": [{ "text": "User input" }]
      },
      {
        "messageId": "response-uuid",
        "role": "ROLE_AGENT",
        "content": [{ "text": "Agent response" }]
      }
    ],
    "metadata": {
      "adk_app_name": "agent-name",
      "adk_usage_metadata": { ... }
    }
  }
}
```

### Task States

| State | Description |
|-------|-------------|
| `TASK_STATE_SUBMITTED` | Task accepted (processing not started) |
| `TASK_STATE_WORKING` | Processing in progress |
| `TASK_STATE_COMPLETED` | Completed successfully |
| `TASK_STATE_FAILED` | Failed |
| `TASK_STATE_CANCELED` | Canceled |

> **Note**: The response uses the `parts` field name in `artifacts[].parts`, but the request `message` uses `content`. This asymmetry is due to the Vertex AI Protobuf schema.

## Relationship with the `:query` Endpoint

The standard Vertex AI Reasoning Engine endpoint `:query` cannot be used with A2A agents.

- `:query` — For standard Reasoning Engine (non-A2A)
- `/a2a/v1/*` — For A2A agents

Agents deployed with `api_mode="a2a_extension"` communicate exclusively through `/a2a/v1/*` paths. There is no configuration that supports both simultaneously.

## Troubleshooting

### `404 Not Found` (Google HTML error page)

If the Agent Card is not found at `/.well-known/agent-card.json`, verify that you are using the Vertex AI A2A endpoint (`/a2a/v1/card`).

### `401 UNAUTHENTICATED`

Use an OAuth2 access token (with `cloud-platform` scope) instead of an ID token.

### `400 FAILED_PRECONDITION` + `"unknown exception"`

The Protobuf parser failed to parse the request body. Check the following:

- Part field is `content` (not `parts`)
- No `kind` field is used
- Role is `"ROLE_USER"` (not `"user"`)
- `messageId` is included

Check Cloud Logging for detailed stack traces:

```
resource.type="aiplatform.googleapis.com/ReasoningEngine"
resource.labels.reasoning_engine_id="{ENGINE_ID}"
severity>=ERROR
```

### Task stuck at `TASK_STATE_SUBMITTED`

Resend the request with `configuration.blocking` set to `true`.
