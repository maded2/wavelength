# Code Refactoring Report — Wavelength

**Date**: 2026-05-13
**Refactoring Expert**: Applied techniques from Fowler's Complete Refactoring Catalog (66 techniques) and Kerievsky's Refactoring to Patterns
**Based on**: FARLEY Score Analysis (40/60)

---

## Executive Summary

This report documents the refactoring of the Wavelength codebase to address the code smells identified by the FARLEY analysis. The primary target was `api/routes.go` — a 500+ line god-function that mixed HTTP handling, LLM orchestration, and business logic.

**Key outcomes:**
- Created new `internal/interview` package for business logic (Extract Class + Move Method)
- Consolidated duplicated LLM HTTP call into `client.Call()` (Parameterize Method)
- Extracted route handlers from `SetupRoutes` into a `Handler` struct (Extract Method + Compose Method)
- Consolidated duplicate `Format` type (Extract Class)
- Deduplicated `topicMeta` marshalling (Extract Method)
- Removed dead code: `lockTopic()`, `RedactedLLMConfig()` (Delete Dead Code)

**Result**: All 30+ existing tests pass. Build and vet are clean.

---

## Detailed Refactoring Analysis

### 1. Extract Class: Create `internal/interview` Package

**Code Smell Addressed**: Feature Envy (7/10), Arrow Procedure (8/10), Long Method (8/10)

**Technique**: Extract Class — One class (`api/routes.go`) doing the work of two (HTTP handling + interview orchestration).

**What was moved:**
- `buildConversationContext()` → `interview.BuildConversationContext()`
- `summarizeMessages()` → `interview.SummarizeMessages()`
- `extractDocument()` → `interview.ExtractDocument()`
- `generateResponse()` + `generateReevaluateResponse()` → `interview.Service.HandleMessage()` + `Reevaluate()`

**New package structure:**
```
internal/interview/
  service.go    — InterviewService with HandleMessage(), Reevaluate(), BuildPrompt()
```

**Before:**
```go
// api/routes.go — 500+ lines, free functions envious of topic and llm internals
func buildConversationContext(t *topic.Topic, userMessage string) string { ... }
func summarizeMessages(messages []topic.Message) string { ... }
func extractDocument(response string) (string, string) { ... }
func generateResponse(client *llm.Client, t *topic.Topic, msg string) (string, error) { ... }
func generateReevaluateResponse(client *llm.Client, t *topic.Topic, prompt string) (string, error) { ... }
```

**After:**
```go
// internal/interview/service.go — Dedicated package owns interview workflow
type Service struct {
    store  Store
    client *llm.Client
}

func (s *Service) HandleMessage(ctx context.Context, topicID, userMessage string) (string, bool, error)
func (s *Service) Reevaluate(ctx context.Context, topicID string) (string, bool, error)
```

**Risk Level**: Medium — Mitigated by 30+ existing tests covering all moved behavior.

---

### 2. Parameterize Method: Consolidate `generateResponse` / `generateReevaluateResponse` into `client.Call()`

**Code Smell Addressed**: Entropy (Duplicate Code), Long Method

**Technique**: Parameterize Method — Multiple methods perform similar actions with different internal values.

**Before**: Two ~95-line functions (`generateResponse`, `generateReevaluateResponse`) were 90% identical:
```go
func generateResponse(client *llm.Client, t *topic.Topic, userMessage string) (string, error) {
    // ~95 lines of: build payload → marshal → create request → set headers → send → parse response
}
func generateReevaluateResponse(client *llm.Client, t *topic.Topic, prompt string) (string, error) {
    // ~95 lines of: build payload → marshal → create request → set headers → send → parse response
}
```

**After**: Single method on `llm.Client`:
```go
func (c *Client) Call(ctx context.Context, messages []llm.Message) (string, error) {
    // ~60 lines of shared HTTP request/response cycle
}
```

The callers now just construct the messages and call `client.Call()`:
```go
messages := []llm.Message{
    {Role: "system", Content: s.client.PersonaPrompt()},
    {Role: "user", Content: prompt},
}
response, err := s.client.Call(ctx, messages)
```

**Supporting refactorings:**
- **Extract Method**: `buildMessagePayload()` and `parseResponse()` extracted from the shared method

**Risk Level**: Low — Behavior is identical, tests verify correctness.

---

### 3. Extract Method + Compose Method: Handler Struct for Routes

**Code Smell Addressed**: Long Method (8/10), Rigidity (7/10)

**Technique**: Extract Method (from 500-line `SetupRoutes`) + Compose Method (Kerievsky) — Transform the monolithic function into a sequence of intention-revealing steps.

**Before:**
```go
func SetupRoutes(app *fiber.App, store topicpkg.TopicStore, client *llm.Client) {
    // 500+ lines of inline closures
    app.Post("/api/topics/:id/messages", func(c *fiber.Ctx) error {
        // 40 lines of request parsing, validation, LLM call, response building
    })
    // ... 8+ more inline closures
}
```

**After:**
```go
type Handler struct {
    store  topicpkg.TopicStore
    client *llm.Client
    iv     *interview.Service
}

func SetupRoutes(app *fiber.App, store topicpkg.TopicStore, client *llm.Client) {
    h := NewHandler(store, client)
    app.Get("/api/topics", h.listTopics)          // Clean, single-responsibility methods
    app.Post("/api/topics/:id/messages", h.sendMessage)
    app.Post("/api/topics/:id/messages/stream", h.streamMessage)
    // ... each handler is a named method
}

func (h *Handler) sendMessage(c *fiber.Ctx) error {
    topic := h.requireActiveTopic(c, c.Params("id"))
    if topic == nil { return nil }

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    conversational, docUpdated, err := h.iv.HandleMessage(ctx, topicID, req.Content)
    // ... clean, readable, testable
}
```

**Supporting refactorings:**
- **Extract Method**: `requireActiveTopic()`, `sanitizeFilename()`, `writeJSON()`
- **Decompose Conditional**: Repeated guard clauses (topic-not-found, topic-completed) extracted into `requireActiveTopic()`

**Risk Level**: Low — All route behavior preserved, verified by 30+ integration tests.

---

### 4. Extract Class: Consolidate `Format` Type

**Code Smell Addressed**: Entropy (6/10) — Duplicate Type Code

**Technique**: Extract Class — Same `Format` type defined in two packages (`export` and `convert`).

**Before:**
```go
// internal/convert/convert.go
type Format string
const (FormatMarkdown Format = "markdown" ...)

// internal/export/export.go
type Format string
const (FormatMarkdown Format = "markdown" ...)
```

**After:**
```go
// internal/topic/types.go — Single source of truth
type Format string
const (FormatMarkdown Format = "markdown" ...)
func (f Format) Validate() error { ... }

// Both convert and export import and use topic.Format
```

**Risk Level**: Low — Type is identical in both locations, all references updated.

---

### 5. Extract Method: `topic.Meta()` for Deduplication

**Code Smell Addressed**: Entropy (6/10) — Duplicate Code

**Technique**: Extract Method — Same `topicMeta{}` construction in 3 places in FileStore.

**Before:**
```go
// In saveTopicDir(), persistTopicUpdate(), and migrateTopicToDir():
meta := topicMeta{
    ID: topic.ID, Name: topic.Name, Description: topic.Description,
    Status: topic.Status, CreatedAt: topic.CreatedAt,
    UpdatedAt: topic.UpdatedAt, MessageCount: topic.MessageCount,
}
```

**After:**
```go
// internal/topic/types.go
func (t *Topic) Meta() topicMeta {
    return topicMeta{
        ID: t.ID, Name: t.Name, Description: t.Description,
        Status: t.Status, CreatedAt: t.CreatedAt,
        UpdatedAt: t.UpdatedAt, MessageCount: t.MessageCount,
    }
}

// Usage in all three places:
metaData, err := json.MarshalIndent(topic.Meta(), "", "  ")
```

**Risk Level**: Low — Pure data extraction, structurally identical output.

---

### 6. Delete Dead Code

**Code Smell Addressed**: Yellowbrick Bridge (Speculative Generality)

**Removed:**
| Item | Location | Reason |
|------|----------|--------|
| `lockTopic()` | `filestore.go` | Never called from any public method |
| `topicLockFile()` | `filestore.go` | Only called by `lockTopic()` |
| `RedactedLLMConfig()` | `config.go` | Never called anywhere |
| `TestLLMCredentialsNotExposed` | `config_test.go` | Tested the removed method |

**Risk Level**: Low — Verified via build that nothing references these symbols.

---

## Priority Matrix

| Refactoring | Impact | Complexity | Risk | Status |
|-------------|--------|-----------|------|--------|
| Extract `interview` package | High | Moderate | Medium | ✅ Done |
| Consolidate `client.Call()` | High | Low | Low | ✅ Done |
| Extract `Handler` struct | High | Moderate | Low | ✅ Done |
| Consolidate `Format` type | Medium | Low | Low | ✅ Done |
| Deduplicate `topicMeta` | Medium | Low | Low | ✅ Done |
| Delete dead code | Low | Low | Low | ✅ Done |

---

## Recommended Implementation Sequence (Retrospective)

The refactorings were applied in this dependency-respecting order:

1. **Consolidate `Format` type** (P1) — Foundation change, no dependencies
2. **Deduplicate `topicMeta`** (P1) — Foundation change, no dependencies
3. **Add `client.Call()`** (P0) — Consolidation prerequisite for interview service
4. **Create `interview` package** (P0) — Extract business logic from routes
5. **Extract `Handler` struct** (P1) — Decompose SetupRoutes, use interview service
6. **Delete dead code** (P3) — Cleanup after refactoring

This sequence ensures each step builds on the previous and can be committed independently.

---

## Remaining Opportunities (Not Yet Addressed)

| Priority | Issue | Recommended Technique |
|----------|-------|----------------------|
| P2 | Duplicate topic-not-found pattern in non-Handler methods | Already addressed via `requireActiveTopic()` |
| P2 | Streaming handler goroutine complexity | Extract Method — isolate pipe/goroutine logic |
| P2 | `Config` validation errors use `[]string` buffer | Replace Data Value with Object — error collection type |
| P3 | Legacy migration code (`loadTopicLegacy`, `migrateTopicToDir`) | Delete Dead Code — if no legacy data exists |
| P3 | In-memory `Store` vs `FileStore` duality | Inline Class — if in-memory store is only for tests |
| P0 (spec) | Missing `cloudwego/eino` integration | Replace Constructor with Factory Method — provider abstraction |

---

## Verification

```
$ go build ./...     # ✅ Clean
$ go vet ./...       # ✅ Clean
$ go test -count=1 ./...  # ✅ All pass
  ok  wavelength/api          0.190s
  ok  wavelength/internal/config  0.019s
  ok  wavelength/internal/convert  0.003s
  ok  wavelength/internal/export   0.007s
  ok  wavelength/internal/llm      0.005s
  ok  wavelength/internal/topic    0.016s
```
