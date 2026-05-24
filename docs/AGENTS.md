# Wavelength — Agent Instructions

## Project Overview

Wavelength is an AI-driven web application that helps users transform vague business ideas into detailed, structured requirement documents through a guided, conversational interview with an LLM-powered "business analyst" agent.

**Current state**: Greenfield repository. No code exists yet. This file and the spec documents below are the starting point.

## Critical Constraints (Do Not Violate)

| Constraint | Detail |
|---|---|
| **Language** | Go (Golang) |
| **Web framework** | `github.com/gofiber/fiber` (v2) |
| **LLM client** | `github.com/cloudwego/eino` |
| **Config format** | Single JSON file — no env vars, no `.env` files |
| **Deployment** | Standalone binary — no external databases, no message queues, no microservices |
| **Persistence** | File-based (topics, conversations, documents stored on disk) |

These constraints come from `docs/problem-analysis.md` section 7. They are mandatory, not suggestions.

## Spec Documents (Read In Order)

1. **`docs/problem-analysis.md`** — Authoritative spec. Contains functional requirements (FR-01..FR-21), non-functional requirements, business rules, constraints, domain model, and risk analysis.
2. **`docs/epics-and-stories.md`** — 4 epics, 31 user stories with acceptance criteria. Stories follow ATDD Red-Green-Refactor cycle. Implementation order is E1 → E2 → E3 → E4.
3. **`docs/requirement.md`** — Original brief requirement statement (12 lines). Reference only.

## Development Methodology

- **ATDD**: Each user story is implemented via the Red-Green-Refactor cycle. One story at a time.
- **Story reference format**: `E{epic}-S{story}` (e.g., `E1-S1`, `E3-S4`). Use this prefix in test names and commit messages.
- **Tests first**: Write failing acceptance tests before any implementation code.

## Architecture Notes

- **Domain model**: Topic (1:1 Conversation, 1:1 RequirementDoc). Conversation has many Messages.
- **Topic isolation is strict**: No conversation history, document content, or LLM context may leak between topics.
- **LLM backend is swappable**: The app must never hardcode a provider, model name, or endpoint. All LLM config comes from the JSON file.
- **Persona prompt is configurable**: The system prompt that defines the AI agent's behavior must be loadable from config, with a sensible default.
- **No auth**: User authentication and RBAC are explicitly out of scope.

## File-Based Persistence Design

Since the app is standalone with no database:
- Topics, conversations, and documents are persisted as files on disk
- A data directory (configurable) stores all topic state
- File format: JSON for structured data (topics, messages), plain text for markdown documents
- Concurrent access to the same topic must be safe (mutex or file locking)

## Go Project Structure (Guidance)

When initializing the Go module, aim for a structure that separates concerns early:

```
cmd/server/         — main entrypoint, HTTP server bootstrap
internal/
  config/           — JSON config loading and validation
  llm/              — eino integration, LLM client abstraction
  topic/            — Topic CRUD, persistence
  conversation/     — Message management, history
  document/         — Requirement document CRUD
  interview/        — Interview orchestration, agent flow
api/                — Fiber handlers, route definitions
static/             — Frontend assets (HTML, CSS, JS)
configs/            — Example JSON configuration files
```

This is guidance, not a hard requirement. Adjust as implementation reveals better structure.

## Testing

- Use Go's `testing` package with table-driven tests
- Acceptance tests for each story should live alongside or near the code they verify
- Mock the LLM client for all tests — never call a real LLM from tests
- File-based persistence tests should use a temporary directory (`t.TempDir()`)

## Configuration File Schema (Draft)

```json
{
  "server": {
    "port": 3000
  },
  "llm": {
    "provider": "openai",
    "model": "gpt-4",
    "endpoint": "https://api.openai.com/v1",
    "api_key": "...",
    "temperature": 0.7
  },
  "persona": {
    "system_prompt": "..."
  },
  "data_dir": "./data"
}
```

This schema is subject to change as Epic 1 stories are implemented.

## First Steps for a New Session

1. Run `go mod init wavelength` if the module doesn't exist yet
2. Add dependencies: `go get github.com/gofiber/fiber/v2` and `go get github.com/cloudwego/eino`
3. Read the current story from `docs/epics-and-stories.md` to find the next unimplemented story
4. Follow the ATDD Red-Green-Refactor cycle for that story
