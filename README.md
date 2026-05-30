# Wavelength

AI-driven business requirement gathering tool. Transform vague business ideas into detailed, structured requirement documents through guided, conversational interviews with an LLM-powered "business analyst" agent.

## Overview

Wavelength is a standalone web application that uses a configurable LLM backend to conduct interview-style conversations with stakeholders, progressively eliciting, refining, and documenting detailed requirements starting from a high-level idea.

### Key Features

- **AI-powered interviews** — An LLM agent acts as a business analyst, asking targeted questions to uncover requirements, edge cases, and constraints
- **LLM tool calling** — The agent uses `read_file` and `write_document` tools to read reference documents and persist requirement documents directly, with delimiter-based extraction as a fallback
- **Streaming responses** — Real-time token streaming via Server-Sent Events (SSE) for instant feedback as the AI responds
- **Document upload** — Upload reference documents (Markdown, PDF, Word/DOCX) from the chat window; they are converted to Markdown, saved to disk, and included in the AI agent's context
- **Attachment management** — View and delete attachments from the document panel; deletion removes both the original file and all references with confirmation
- **Topic management** — Multiple independent requirement-gathering initiatives, each with isolated conversation history and a living requirement document
- **Living documents** — Markdown requirement documents that evolve as the interview progresses, with automatic extraction from AI responses
- **Document export** — Download requirement documents as Markdown, PDF, or Word (DOCX)
- **Re-evaluate command** — Clear conversation history and have the AI re-assess the requirement document from scratch with `/reevaluate`
- **Context management** — Automatic conversation summarization for long interviews to stay within LLM context windows
- **Configurable LLM backend** — Swap providers, models, and endpoints via a single JSON config file — no code changes needed
- **MCP tool integration** — Connect to external MCP servers (stdio or SSE transport) to give the AI agent access to additional tools like filesystem access, web search, databases, and more
- **Voice input** — Dictate messages via microphone; audio is transcribed by the LLM endpoint's Whisper API (`/v1/audio/transcriptions`). Auto-detected at startup — no extra config needed.
- **Standalone binary** — No databases, no message queues, no external infrastructure. File-based persistence with atomic writes and file locking.

## Tech Stack

| Component | Choice |
|---|---|
| Language | Go 1.25 |
| Web framework | [Fiber](https://github.com/gofiber/fiber) v2 |
| LLM integration | [Eino](https://github.com/cloudwego/eino) framework — OpenAI-compatible chat model with streaming and tool calling |
| PDF generation | [gofpdf](https://github.com/jung-kurt/gofpdf) |
| PDF parsing | [ledongthuc/pdf](https://github.com/ledongthuc/pdf) |
| Persistence | File-based (JSON + JSONL + Markdown) with atomic writes |
| File locking | [gofrs/flock](https://github.com/gofrs/flock) |
| MCP client | [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) — stdio and SSE transports |
| Configuration | Single JSON file |

## Installation

### Quick Start — Pre-built Binary (Recommended)

Install via npm. This downloads the correct binary for your platform and installs the default config automatically — no Go installation required:

```bash
npm install github:maded2/wavelength
```

**What happens during install:**

1. **Pre-install** — Validates your OS and CPU are supported
2. **Post-install** — Downloads the pre-built binary from GitHub Releases
3. **Config setup** — Copies default config to your platform's standard location

**Supported platforms:**

| OS | CPU |
|---|---|
| Linux | x86_64 (amd64), ARM 64-bit (arm64) |
| macOS | Apple Silicon (arm64), Intel (amd64) |
| Windows | x86_64 (amd64), ARM 64-bit (arm64) |

### Build from Source

If you prefer to build locally (or your platform is unsupported), you need **Go 1.25+**:

```bash
# Build for your current platform
make build

# Build for all platforms (outputs to dist/)
make build-all

# Build for a specific platform
make build-linux-arm64
```

Or via npm:

```bash
npm run build              # current platform
npm run build:all          # all platforms → dist/
npm run build:darwin-arm64 # macOS Apple Silicon
```

### Running

```bash
make run
```

This builds the binary and starts the server with `configs/config.json`. The application starts on the configured port (default: 3000). Open `http://localhost:3000` in your browser.

You can also specify a custom config file:

```bash
./wavelength -config ~/.config/wavelength/config.json
```

### Configuration

After install, edit the config file at:

| Platform | Path |
|---|---|---|
| Linux | `~/.config/wavelength/config.json` |
| macOS | `~/.config/wavelength/config.json` |
| Windows | `%APPDATA%/wavelength/config.json` |

Add your LLM endpoint, model, and API key, then start:

```bash
wavelength -config ~/.config/wavelength/config.json
```

> **Note:** If the config file already exists at the destination, it is preserved (not overwritten).

Example configuration:

```json
{
  "server": {
    "port": 3000
  },
  "llm": {
    "provider": "openai",
    "model": "gpt-4",
    "endpoint": "https://api.openai.com/v1",
    "api_key": "your-api-key-here",
    "temperature": 0.7,
    "timeout": 120,
    "path": "/chat/completions"
  },
  "persona": {
    "system_prompt": ""
  },
  "voice": {
    "whisper_url": "http://192.168.8.5:8085",
    "whisper_type": "whispercpp"
  },
  "mcp": {
    "servers": []
  },
  "data_dir": "./data"
}
```

| Field | Description |
|---|---|
| `llm.timeout` | HTTP request timeout in seconds (default: 60) |
| `llm.path` | *(unused — eino uses `endpoint` as the base URL directly)* |
| `persona.system_prompt` | Custom system prompt (uses sensible default if empty) |
| `mcp.servers` | Array of MCP server configs (see [MCP Support](#mcp-support)) |
| `voice.enabled` | `true` = force enable, `false` = disable, `null`/omitted = auto-detect (default) |
| `voice.whisper_url` | Base URL for the transcription API (default: uses `llm.endpoint`) |
| `voice.whisper_type` | Server type: `openai` (default) or `whispercpp` |
| `voice.whisper_model` | Model name for transcription (default: `whisper-1`, ignored for whispercpp) |

**Required fields**: `server.port`, `llm.provider`, `llm.model`, `llm.endpoint`, `llm.api_key`, `data_dir`. Missing fields cause a startup error with a descriptive message.

### Release Process (Developers)

To publish a new release with pre-built binaries for all platforms:

```bash
GITHUB_TOKEN=ghp_xxx npm run release
```

This requires a [GitHub personal access token](https://github.com/settings/tokens) with `repo` scope. The script:
1. Cross-compiles Go binaries for all 6 supported platforms
2. Creates compressed archives (`.tar.gz` for Unix, `.zip` for Windows)
3. Creates a GitHub Release tagged with the version from `package.json`
4. Uploads all archives as release assets

Users can then install via `npm install github:maded2/wavelength` and get the pre-built binary automatically.

## API Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Landing page |
| `GET` | `/health` | Health check (live LLM connectivity probe, ~3s timeout) |
| `GET` | `/topics/:id` | Topic chat page (HTML UI) |
| `GET` | `/api/topics` | List all topics |
| `POST` | `/api/topics` | Create a new topic |
| `GET` | `/api/topics/:id` | Get topic details |
| `PATCH` | `/api/topics/:id` | Update topic status |
| `DELETE` | `/api/topics/:id` | Delete a topic |
| `PATCH` | `/api/topics/:id/document` | Update requirement document |
| `POST` | `/api/topics/:id/messages` | Send message (non-streaming) |
| `POST` | `/api/topics/:id/messages/stream` | Send message (SSE streaming) |
| `POST` | `/api/topics/:id/upload` | Upload reference document (Markdown, PDF, DOCX) |
| `GET` | `/api/topics/:id/attachments` | List topic attachments |
| `DELETE` | `/api/topics/:id/attachments/:attachmentId` | Delete an attachment (removes file + metadata) |
| `GET` | `/api/topics/:id/document/download` | Download document (`?format=markdown\|pdf\|word`) |
| `POST` | `/api/voice/transcribe` | Transcribe audio to text (multipart `audio` field) |

### Create a Topic

```
POST /api/topics
Content-Type: application/json

{
  "name": "My Project",
  "description": "A high-level description of the project",
  "document": "..."  // optional — pre-existing requirement document
}
```

If `document` is provided, the topic starts with that content instead of the default template.

Topic names must be unique — creating a topic with a duplicate name returns `409 Conflict`.

### Streaming Messages

The streaming endpoint returns a Server-Sent Events (SSE) stream:

```
POST /api/topics/:id/messages/stream
Content-Type: application/json

{"content": "Your message here"}
```

Response events:
- `{"type": "start"}` — Stream began
- `{"type": "token", "content": "..."}` — Token chunk (render incrementally)
- `{"type": "done"}` — Stream complete
- `{"type": "error", "message": "..."}` — Error occurred

### Document Download

```
GET /api/topics/:id/document/download?format=markdown  # Default
GET /api/topics/:id/document/download?format=pdf
GET /api/topics/:id/document/download?format=word
```

### Document Upload

Upload reference documents to be included in the AI agent's context:

```
POST /api/topics/:id/upload
Content-Type: multipart/form-data

file=<your-file>
```

Supported formats: `.md`, `.pdf`, `.docx` (max 10 MB per file).

Uploaded documents are converted to Markdown and stored as attachments. The original file is saved to disk alongside the converted metadata. The AI agent references them during the interview conversation.

### Deleting Attachments

Attachments can be deleted from the document panel (left side of the topic view). Clicking the delete icon (🗑️) on an attachment prompts for confirmation, then removes both the original file and its metadata.

```
DELETE /api/topics/:id/attachments/:attachmentId
```

Response: `200 OK` with `{"message": "attachment deleted"}`.

If the attachment ID is not found, returns `404 Not Found` with `{"message": "attachment not found"}`.

## Special Commands

Type these in the chat input:

| Command | Description |
|---|---|
| `/reevaluate` | Clears all conversation history and asks the AI to re-assess the requirement document from scratch |

## Voice Input

Wavelength supports dictating messages via microphone. Audio is transcribed by sending it to your LLM endpoint's `/v1/audio/transcriptions` API (OpenAI-compatible Whisper endpoint).

### How It Works

1. Click the 🎤 button in the chat input area to start recording
2. Speak your message (English)
3. Click 🎤 again to stop recording
4. The transcribed text appears in the input textarea — edit if needed, then press Send

### Requirements

- Your LLM endpoint must support the OpenAI-compatible `/v1/audio/transcriptions` endpoint (e.g., OpenAI API, Open WebUI, LiteLLM with Whisper, vLLM with Whisper)
- Browser must support `getUserMedia` (Chrome, Firefox, Edge, Safari)
- Microphone access must be granted when the browser prompts

### Auto-Detection

At startup, Wavelength probes the LLM endpoint to check if `/v1/audio/transcriptions` is available:
- **Available** → 🎤 button is active, voice input enabled
- **Not available** → 🎤 button is grayed out with tooltip explaining why
- **Explicitly disabled** → Set `voice.enabled: false` in config to always disable

### Configuration

```json
{
  "voice": {
    "enabled": true,
    "whisper_url": "http://192.168.8.5:8085",
    "whisper_type": "whispercpp"
  }
}
```

| Field | Description |
|---|---|
| `voice.enabled` | `true` = force enable, `false` = disable, `null`/omitted = auto-detect at startup |
| `voice.whisper_url` | Base URL for the transcription API. If empty, defaults to `llm.endpoint` |
| `voice.whisper_type` | Server type: `openai` (default) uses `/v1/audio/transcriptions` with Bearer auth; `whispercpp` uses `/inference` with no auth |
| `voice.whisper_model` | Model name sent to the transcription API (default: `whisper-1`, ignored for whispercpp) |

### OpenAI Whisper

To use an OpenAI-compatible Whisper endpoint instead, set `whisper_type` to `"openai"`:

```json
{
  "voice": {
    "whisper_url": "https://api.openai.com/v1",
    "whisper_type": "openai",
    "whisper_model": "whisper-1"
  }
}
```

The whisper.cpp server uses its `/inference` endpoint with no authentication. Text segments from the response are joined to produce the final transcription.

### API

```bash
POST /api/voice/transcribe
Content-Type: multipart/form-data

audio=<audio file>
```

Response:
```json
{
  "success": true,
  "text": "Your transcribed message here"
}
```

Supported audio formats: `audio/webm`, `audio/ogg`, `audio/wav`, `audio/mpeg` (max 20 MB).

## MCP Support

Wavelength can connect to external [Model Context Protocol (MCP)](https://modelcontextprotocol.io) servers to give the AI agent access to additional tools beyond the built-in `read_file` and `write_document`. This enables the agent to perform web searches, read/write files on the host system, query databases, or use any other MCP-compatible tool during requirement-gathering interviews.

### How It Works

At startup, Wavelength connects to each configured MCP server and discovers its available tools. These tools are then injected into the LLM's tool list alongside the built-in tools. When the LLM decides to call an MCP tool, Wavelength routes the call to the correct server, executes it, and returns the result.

Tool names are prefixed with the server name to avoid collisions: `mcp::<server>::<tool>`.

### Supported Transports

| Transport | Description | Use Case |
|---|---|---|
| `stdio` | Runs a local command and communicates via stdin/stdout | npm/npx servers, Python servers, any local executable |
| `sse` | Connects to a remote SSE endpoint | Remote MCP servers, cloud-hosted tools |

### Configuration

Add an `mcp` section to your config file with a `servers` array:

```json
{
  "mcp": {
    "servers": [
      {
        "name": "filesystem",
        "transport": "stdio",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home/user/projects"],
        "timeout": 10
      },
      {
        "name": "web_search",
        "transport": "sse",
        "url": "http://localhost:3001/sse",
        "timeout": 15
      }
    ]
  }
}
```

### Server Configuration Fields

| Field | Transport | Description |
|---|---|---|
| `name` | both | Display name for the server (used in tool name prefix, e.g., `mcp::filesystem::read_file`) |
| `transport` | both | Connection type: `"stdio"` or `"sse"` |
| `command` | stdio | Executable to run (e.g., `"npx"`, `"uvx"`, `"python3"`) |
| `args` | stdio | Command-line arguments (e.g., `["-y", "@modelcontextprotocol/server-filesystem", "/path"]`) |
| `env` | stdio | Environment variables as a key-value map (merged with system env) |
| `url` | sse | SSE endpoint URL (e.g., `"http://localhost:3001/sse"`) |
| `timeout` | both | Connection timeout in seconds (default: 10) |

### Example: Filesystem Server (stdio)

Give the AI agent read/write access to a specific directory on the host:

```json
{
  "mcp": {
    "servers": [
      {
        "name": "filesystem",
        "transport": "stdio",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home/user/projects"],
        "timeout": 10
      }
    ]
  }
}
```

The AI agent can then read and write files within `/home/user/projects` during interviews. Available tools depend on the server implementation (typically `read_file`, `write_file`, `list_directory`, etc.).

### Example: Web Search Server (SSE)

Connect to a remote MCP server that provides web search capabilities:

```json
{
  "mcp": {
    "servers": [
      {
        "name": "web_search",
        "transport": "sse",
        "url": "http://localhost:3001/sse",
        "timeout": 15
      }
    ]
  }
}
```

### Example: Multiple Servers

Combine multiple MCP servers for a powerful research-capable agent:

```json
{
  "mcp": {
    "servers": [
      {
        "name": "filesystem",
        "transport": "stdio",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
        "timeout": 10
      },
      {
        "name": "fetch",
        "transport": "stdio",
        "command": "uvx",
        "args": ["mcp-server-fetch"],
        "timeout": 15
      },
      {
        "name": "database",
        "transport": "sse",
        "url": "http://localhost:8080/sse",
        "timeout": 10
      }
    ]
  }
}
```

### Graceful Degradation

- If no MCP servers are configured (or `mcp.servers` is empty), MCP support is silently skipped
- If a server fails to connect at startup, a warning is logged and the application continues with remaining servers
- If all servers fail, the application still runs with only built-in tools
- MCP connections are closed cleanly on application shutdown

### Logging

MCP activity is logged with the `[MCP]` prefix:

```
[MCP] Connecting to 2 MCP server(s)...
[MCP] Connected to MCP server "filesystem" (stdio transport)
[MCP] Server "filesystem": discovered 5 tool(s)
[MCP] MCP initialization complete: 5 tool(s) available from connected servers
[MCP-TOOL] Executing "mcp::filesystem::read_file"
[MCP-TOOL] mcp::filesystem::read_file completed (1024 bytes result)
```

## Project Structure

```
cmd/server/         — main entrypoint
internal/
  config/           — JSON config loading, validation, and voice config
  llm/              — LLM client (Eino + OpenAI-compatible), tool calling, streaming, voice transcription
  mcp/              — MCP client manager, server connections, tool/schema conversion
  topic/            — Topic CRUD, file-based persistence (filestore), and type definitions
  interview/        — Interview orchestration service (context building, document extraction, prompts)
  convert/          — Document format conversion (PDF, DOCX → Markdown)
  export/           — Document export (Markdown, PDF, Word)
api/                — Fiber handlers, routes, health check, and landing page
api/static/         — Embedded frontend assets (HTML)
configs/            — Example configuration files
scripts/            — Build, release, install, and npm lifecycle scripts (mjs)
docs/               — Design documents, epics, problem analysis, and test reviews
```

## Development

### Running Tests

```bash
make test     # runs all tests
go test ./... # equivalent
```

### Test Suite

Wavelength follows **Acceptance Test Driven Development (ATDD)** — every test traces to a user story via `E{epic}-S{story}` references (e.g., `E2-S1`, `E3-S9`).

| Metric | Value |
|---|---|
| Test files | 36 across 8 packages |
| Suite runtime | ~0.13s |
| Farley Score | **8.1/10** (Excellent) |
| Mocking | All LLM calls mocked via `httptest.NewServer`; no real API calls |
| Isolation | Each test uses `t.TempDir()` + in-memory stores — no shared state |

#### Test Structure

```
api/                    — API integration tests (HTTP handlers, routes, full request/response cycle)
  testutil.go           — Shared test helpers: newSuite(), newSuiteWithMock(), MustDecodeJSON[T], etc.
internal/config/        — Config loading, validation, persona prompt tests
internal/convert/       — Document format conversion (PDF, DOCX → Markdown)
internal/export/        — Document export (Markdown, PDF, Word)
internal/interview/     — Interview service: context building, document extraction, summarization
internal/llm/           — LLM client: connectivity, tool calling, streaming
internal/mcp/           — MCP client: server name extraction, schema conversion
internal/topic/         — Topic persistence: file store save/load, document persistence
```

#### Test Helpers

The `api/testutil.go` package provides shared test infrastructure:

| Helper | Purpose |
|---|---|
| `newSuite(t)` | Creates a `TestSuite` with Fiber app, in-memory store, config, and temp dir |
| `newSuiteWithMock(t, handler)` | Creates a suite wired to an `httptest.NewServer` for LLM mocking |
| `suite.PostJSON(t, path, body)` | Sends a JSON POST request and returns body + status |
| `suite.Get(t, path)` | Sends a GET request and returns body + status |
| `MustDecodeJSON[T](t, body, v)` | Generic JSON decoder that fails the test on error |
| `AssertErrorContains(t, body, substr)` | Asserts a JSON error response contains a substring |
| `CreateTopic(t, app, name, desc)` | Creates a topic via the API and returns its ID |

All tests use these helpers — no inline Fiber app or config duplication.

## Architecture

### Interview Orchestration

The interview layer (`internal/interview/`) manages the conversational flow between the user and the LLM-powered business analyst agent. It:
- Constructs the LLM context (system prompt + message history + requirement document + attachments)
- Calls the LLM with tool support (non-streaming) and extracts document updates
- Parses `=== REQUIREMENT DOCUMENT ===` delimiters from LLM responses to extract document updates
- Handles the `/reevaluate` command (clear history, re-assess document from scratch)
- Coordinates with the topic and conversation stores for persistence

Streaming responses are handled directly in the API layer (`api/routes.go`), which pipes LLM tokens to the client via SSE and then performs post-stream document extraction.

### LLM Tool Calling

The LLM agent has access to built-in tools plus any configured MCP tools during conversations:

**Built-in tools:**

- **`read_file`** — Read files from the topic directory (uploaded attachments, current requirement document). Prevents directory traversal attacks.
- **`write_document`** — Save the complete requirement document to `document.md`. Called by the LLM when it has finalized document content.

**MCP tools:**

Tools from connected MCP servers are automatically injected into the LLM's tool list. They are prefixed with `mcp::<server>::` to avoid naming collisions (e.g., `mcp::filesystem::read_file`). The MCP manager handles connection lifecycle, tool discovery, schema conversion, and call routing transparently.

Document updates can come from two sources:
1. **Tool-based** (primary) — The LLM calls `write_document` with the full document content
2. **Delimiter-based** (fallback) — The LLM wraps the document in `=== REQUIREMENT DOCUMENT ===` delimiters, which the backend extracts

The non-streaming message endpoint (`POST /api/topics/:id/messages`) calls the LLM with tools directly, so the agent can use `read_file` and `write_document` in a single turn.

The streaming endpoint (`POST /api/topics/:id/messages/stream`) does not pass tools to the LLM during the token stream. Instead, after the stream completes, if no delimiter-based document was extracted, a follow-up non-streaming tool call is made so the LLM can use `write_document` to persist the document.

### Persistence

Topics are stored as directories on disk:

```
data/topics/<topic-id>/
  meta.json           — Topic metadata (name, status, timestamps)
  messages.jsonl      — Conversation messages (one JSON per line)
  document.md         — Living requirement document
  attachments.json    — Uploaded reference document metadata and converted markdown
  attachments/        — Original uploaded files (one per attachment, named `att-<id>.<ext>`)
```

All writes use **atomic write-to-temp-then-rename** to prevent corruption on crash. File locking (`gofrs/flock`) ensures safe concurrent access. A global lock protects bulk load/save operations; per-operation writes use in-memory mutexes.

The file store also supports **legacy migration**: topics saved in the old single-file JSON format are automatically migrated to the directory format on first load.

Topics are persisted:
- Every 10 seconds (periodic background save)
- On graceful shutdown (`SIGINT` / `SIGTERM`)

### Context Management

For long conversations, Wavelength automatically:
1. Keeps the 20 most recent messages verbatim
2. Summarizes older messages into compact bullet points
3. Includes the current requirement document (truncated if >4000 chars)
4. Includes uploaded reference documents (truncated if >8000 chars each)
5. Triggers summarization when conversation exceeds ~60,000 characters

### Document Updates

The AI agent can update documents in two ways:

**Tool calling** (primary): The agent calls the `write_document` tool with the complete markdown content.

**Delimiters** (fallback): The agent wraps the updated document in `=== REQUIREMENT DOCUMENT ===` delimiters:

```
=== REQUIREMENT DOCUMENT ===
# Requirements: My Project

## Overview
...
=== END REQUIREMENT DOCUMENT ===
```

The backend extracts content between delimiters and saves it as the topic's requirement document. Everything outside the delimiters is treated as conversational response.

### Topic Statuses

| Status | Description |
|---|---|
| `not_started` | Topic created, no messages yet |
| `active` | Interview in progress |
| `completed` | Interview finished (messages and uploads blocked until reopened) |

Status transitions: `not_started` → `active` (automatic on first message), `active` → `completed`, `completed` → `active` (via `PATCH /api/topics/:id`).

### Default Document Template

New topics are initialized with a structured markdown template:

```markdown
# Requirements: <topic-name>

## Overview

<description>

## Functional Requirements

(To be elaborated during the interview)

## Non-Functional Requirements

(To be elaborated during the interview)

## Stakeholders

(To be identified during the interview)

## Constraints

(To be identified during the interview)

## Open Questions

(To be resolved during the interview)
```

## Design Principles

- **Topic isolation** — No conversation history, document content, or LLM context leaks between topics
- **Swappable LLM** — Provider, model, and endpoint are purely configuration — never hardcoded
- **Configurable persona** — The AI agent's system prompt is loaded from config with a sensible default
- **Graceful degradation** — If the LLM is unavailable, user messages are preserved and the user is notified
- **Atomic persistence** — All file writes use temp-then-rename to prevent corruption
- **No auth** — Authentication and RBAC are out of scope

## License

MIT — see [LICENSE](LICENSE)
