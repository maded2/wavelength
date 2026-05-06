# Wavelength

AI-driven business requirement gathering tool. Transform vague business ideas into detailed, structured requirement documents through guided, conversational interviews with an LLM-powered "business analyst" agent.

## Overview

Wavelength is a standalone web application that uses a configurable LLM backend to conduct interview-style conversations with stakeholders, progressively eliciting, refining, and documenting detailed requirements starting from a high-level idea.

### Key Features

- **AI-powered interviews** — An LLM agent acts as a business analyst, asking targeted questions to uncover requirements, edge cases, and constraints
- **Topic management** — Multiple independent requirement-gathering initiatives, each with isolated conversation history and a living requirement document
- **Living documents** — Markdown requirement documents that evolve as the interview progresses
- **Configurable LLM backend** — Swap providers, models, and endpoints via a single JSON config file — no code changes needed
- **Standalone binary** — No databases, no message queues, no external infrastructure. File-based persistence.

## Tech Stack

| Component | Choice |
|---|---|
| Language | Go 1.25 |
| Web framework | [Fiber](https://github.com/gofiber/fiber) v2 |
| LLM integration | Direct HTTP to OpenAI-compatible endpoints |
| Persistence | File-based (JSON + Markdown) |
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
| `GET` | `/api/topics` | List all topics |
| `POST` | `/api/topics` | Create a new topic |
| `GET` | `/api/topics/:id` | Get topic details |
| `PATCH` | `/api/topics/:id` | Update topic status |
| `DELETE` | `/api/topics/:id` | Delete a topic |
| `PATCH` | `/api/topics/:id/document` | Update requirement document |
| `POST` | `/api/topics/:id/messages` | Send message in topic conversation |

## Project Structure

```
cmd/server/         — main entrypoint
internal/
  config/           — JSON config loading and validation
  llm/              — LLM client
  topic/            — Topic CRUD and file-based persistence
  conversation/     — Message management
  document/         — Requirement document handling
  interview/        — Interview orchestration
api/                — Fiber handlers and routes
static/             — Frontend assets
configs/            — Example configuration files
```

## Development

### Running Tests

```bash
make test     # runs all tests
go test ./... # equivalent
```

All tests use mocked LLM clients — no real API calls are made during testing.

## Design Principles

- **Topic isolation** — No conversation history, document content, or LLM context leaks between topics
- **Swappable LLM** — Provider, model, and endpoint are purely configuration — never hardcoded
- **Configurable persona** — The AI agent's system prompt is loaded from config with a sensible default
- **Graceful degradation** — If the LLM is unavailable, user messages are preserved and the user is notified
- **No auth** — Authentication and RBAC are out of scope

## License

MIT — see [LICENSE](LICENSE)
