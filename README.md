# Wavelength

AI-driven business requirement gathering tool. Transform vague business ideas into detailed, structured requirement documents through guided, conversational interviews with an LLM-powered "business analyst" agent.

## Overview

Wavelength is a standalone web application that uses a configurable LLM backend to conduct interview-style conversations with stakeholders, progressively eliciting, refining, and documenting detailed requirements starting from a high-level idea.

### Key Features

- **AI-powered interviews** — An LLM agent acts as a business analyst, asking targeted questions to uncover requirements, edge cases, and constraints
- **Streaming responses** — Real-time token streaming via Server-Sent Events (SSE) for instant feedback as the AI responds
- **Document upload** — Upload reference documents (Markdown, PDF, Word/DOCX) from the chat window; they are converted to Markdown and included in the AI agent's context
- **Topic management** — Multiple independent requirement-gathering initiatives, each with isolated conversation history and a living requirement document
- **Living documents** — Markdown requirement documents that evolve as the interview progresses, with automatic extraction from AI responses using `=== REQUIREMENT DOCUMENT ===` delimiters
- **Document export** — Download requirement documents as Markdown, PDF, or Word (DOCX)
- **Re-evaluate command** — Clear conversation history and have the AI re-assess the requirement document from scratch with `/reevaluate`
- **Context management** — Automatic conversation summarization for long interviews to stay within LLM context windows
- **Configurable LLM backend** — Swap providers, models, and endpoints via a single JSON config file — no code changes needed
- **Standalone binary** — No databases, no message queues, no external infrastructure. File-based persistence with atomic writes and file locking.

## Tech Stack

| Component | Choice |
|---|---|
| Language | Go 1.25 |
| Web framework | [Fiber](https://github.com/gofiber/fiber) v2 |
| LLM integration | Direct HTTP to OpenAI-compatible endpoints (with streaming) |
| PDF generation | [gofpdf](https://github.com/jung-kurt/gofpdf) |
| PDF parsing | [ledongthuc/pdf](https://github.com/ledongthuc/pdf) |
| Persistence | File-based (JSON + JSONL + Markdown) with atomic writes |
| File locking | [gofrs/flock](https://github.com/gofrs/flock) |
| Configuration | Single JSON file |

## Quick Start

### Prerequisites

- Go 1.25+
- An LLM API with OpenAI-compatible chat completions endpoint

### Configuration

Copy the example config and adjust for your LLM backend:

```bash
cp configs/config.json config.json
# Edit config.json with your LLM endpoint, model, and API key
```

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
  "data_dir": "./data"
}
```

| Field | Description |
|---|---|
| `llm.timeout` | HTTP request timeout in seconds (default: 60) |
| `llm.path` | API path appended to endpoint (default: `/chat/completions`) |
| `persona.system_prompt` | Custom system prompt (uses sensible default if empty) |

### Running

```bash
make run
```

This builds the binary and starts the server with `configs/config.json`. The application starts on the configured port (default: 3000). Open `http://localhost:3000` in your browser.

### Building

```bash
make build    # compiles to ./wl
make run      # builds then runs
make clean    # removes ./wl
```

You can also specify a custom config file:

```bash
./wl -config config.json
```

## API Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Landing page |
| `GET` | `/health` | Health check (includes LLM connectivity status) |
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
| `GET` | `/api/topics/:id/document/download` | Download document (`?format=markdown\|pdf\|word`) |

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

Uploaded documents are converted to Markdown and stored as attachments. The AI agent references them during the interview conversation.

## Special Commands

Type these in the chat input:

| Command | Description |
|---|---|
| `/reevaluate` | Clears all conversation history and asks the AI to re-assess the requirement document from scratch |

## Project Structure

```
cmd/server/         — main entrypoint
internal/
  config/           — JSON config loading and validation
  llm/              — LLM client (with streaming support)
  topic/            — Topic CRUD and file-based persistence
  convert/          — Document format conversion (PDF, DOCX → Markdown)
  export/           — Document export (Markdown, PDF, Word)
api/                — Fiber handlers and routes
api/static/         — Embedded frontend assets (HTML)
configs/            — Example configuration files
```

## Development

### Running Tests

```bash
make test     # runs all tests
go test ./... # equivalent
```

All tests use mocked LLM clients — no real API calls are made during testing.

## Architecture

### Persistence

Topics are stored as directories on disk:

```
data/topics/<topic-id>/
  meta.json       — Topic metadata (name, status, timestamps)
  messages.jsonl  — Conversation messages (one JSON per line)
  document.md     — Living requirement document
```

All writes use **atomic write-to-temp-then-rename** to prevent corruption on crash. File locking (`gofrs/flock`) ensures safe concurrent access.

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

The AI agent wraps updated requirement documents in `=== REQUIREMENT DOCUMENT ===` delimiters:

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

## Design Principles

- **Topic isolation** — No conversation history, document content, or LLM context leaks between topics
- **Swappable LLM** — Provider, model, and endpoint are purely configuration — never hardcoded
- **Configurable persona** — The AI agent's system prompt is loaded from config with a sensible default
- **Graceful degradation** — If the LLM is unavailable, user messages are preserved and the user is notified
- **Atomic persistence** — All file writes use temp-then-rename to prevent corruption
- **No auth** — Authentication and RBAC are out of scope

## License

MIT — see [LICENSE](LICENSE)
