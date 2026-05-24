# FARLEY Score Report — Wavelength Codebase

**Date**: 2026-05-13
**Scope**: All Go source files (non-test): `cmd/server/main.go`, `api/routes.go`, `api/health.go`, `api/landing.go`, `internal/config/config.go`, `internal/llm/client.go`, `internal/topic/store.go`, `internal/topic/filestore.go`, `internal/export/export.go`, `internal/convert/convert.go`
**Total source files**: 10
**Approximate LOC (non-test, non-blank)**: ~2,100

---

## FARLEY Score Summary

| Component | Score (0–10) | Severity |
|-----------|-------------|----------|
| **F**eature Envy | 7 | 🔴 High |
| **A**rrow Procedure | 8 | 🔴 High |
| **R**igidity | 7 | 🔴 High |
| **E**ntropy | 6 | 🟡 Moderate |
| **L**ong Method | 8 | 🔴 High |
| **Y**ellowbrick Bridge | 4 | 🟢 Low |
| **TOTAL** | **40 / 60** | **🟡 Moderate–High** |

> **Interpretation**: A score of 40/60 (67%) indicates significant code quality concerns. The three most critical issues are **Arrow Procedures** (8), **Long Methods** (8), and **Feature Envy** (7), all centered on `api/routes.go` which functions as a god-object mixing HTTP handlers, business logic, and LLM orchestration.

---

## Detailed Analysis

### F — Feature Envy: 7/10

**Definition**: Methods that rely more on the data and behavior of other classes than their own.

#### Findings

| Severity | Location | Details |
|----------|----------|---------|
| 🔴 | `api/routes.go` — `generateResponse()` | Operates entirely on `*llm.Client` and `*topic.Topic` data (Name, Description, Messages, Document, Attachments). Belongs in neither `llm` nor `topic` package — it's a cross-cutting concern with no home. |
| 🔴 | `api/routes.go` — `buildConversationContext()` | Extracts fields from `*topic.Topic` (Name, Description, Messages, Document, Attachments) and builds a prompt string. Knows internal structure of Topic in detail. |
| 🔴 | `api/routes.go` — `handleReevaluate()` | Takes `store`, `client`, `topicID`, and `topic` — depends on four objects across three packages. |
| 🟡 | `api/routes.go` — streaming handler closure | Reconstructs `[]llm.Message` from `topic.Messages` and calls `client.StreamResponse()`. Duplicates logic from `generateResponse`. |
| 🟡 | `internal/llm/client.go` — `StreamResponse()` | Knows about the OpenAI message format (`model`, `stream`, `temperature`, `max_tokens`, `messages`), SSE chunk parsing (`choices[0].delta.content`), and `[DONE]` termination. Hardcoded to OpenAI-compatible API. |

**Root Cause**: Business logic (LLM orchestration, conversation context building, document extraction) lives in the `api` package instead of a dedicated `interview` or `orchestrator` package. The route handlers are deeply envious of the internal structures of `topic.Topic`, `llm.Client`, and `config.Config`.

---

### A — Arrow Procedure: 8/10

**Definition**: Procedures that don't belong to any class because the class design doesn't support the data they need. They form "arrows" between classes.

#### Findings

| Severity | Location | Arrow Pattern |
|----------|----------|---------------|
| 🔴 | `generateResponse(client, topic, userMessage)` | Arrow between `llm.Client` → `topic.Topic` → prompt → LLM API response. No class owns this workflow. |
| 🔴 | `generateReevaluateResponse(client, topic, prompt)` | Near-identical arrow to `generateResponse`. Same cross-class flow, duplicated. |
| 🔴 | `buildConversationContext(topic, userMessage)` | Arrow from `topic.Topic` data → string prompt. Operates on topic internals but lives in `api` package. |
| 🔴 | `handleReevaluate(c, store, client, topicID, topic)` | Arrow across 5 parameters spanning HTTP context, persistence, LLM, and domain models. |
| 🟡 | `extractDocument(response)` | Free function extracting structured data from LLM response string. No class claims this behavior. |
| 🟡 | `summarizeMessages(messages)` | Free function operating on `[]topic.Message` but producing a narrative summary. |
| 🟡 | `flushParagraph(p, pdf, ...)` in `export/export.go` | Arrow between `strings.Builder` and `*gofpdf.Fpdf`. |

**Root Cause**: Missing `interview` package that would own the business workflow: "given a topic and a user message, orchestrate the LLM call, extract document updates, and save responses." Currently this logic is scattered as free functions in `api/routes.go`.

---

### R — Rigidity: 7/10

**Definition**: Difficulty making changes due to tight coupling, monolithic structure, or violation of separation of concerns.

#### Findings

| Severity | Location | Issue |
|----------|----------|-------|
| 🔴 | `api/routes.go` (500+ lines) | Single monolithic function `SetupRoutes()` contains ALL route handlers as inline closures. Adding, modifying, or testing any single handler requires navigating the entire file. |
| 🔴 | `api/routes.go` — LLM call logic | `generateResponse()` and `generateReevaluateResponse()` contain raw `http.NewRequest`, `json.Marshal`, `httpClient.Do()`, and response parsing. This logic is in the HTTP layer, not an LLM client layer. |
| 🔴 | `internal/llm/client.go` — hardcoded OpenAI format | Despite the spec requiring "LLM backend is swappable," the client hardcodes OpenAI JSON payload format (`model`, `messages[]`, `choices[0].message.content`). Cannot swap to Anthropic, Google, or any other provider without rewriting this file. |
| 🔴 | Spec violation | The project spec says: *"The app must never hardcode a provider, model name, or endpoint. All LLM config comes from the JSON file"* and *"Use github.com/cloudwego/eino for LLM client."* The code ignores `cloudwego/eino` entirely and uses raw HTTP calls. |
| 🟡 | Business logic in handlers | Input validation, duplicate name checking, document extraction, context building, and LLM orchestration are all in the same layer (HTTP handlers). No controller/service separation. |
| 🟡 | `persistTopicUpdate()` in FileStore | Duplicates `topicMeta` marshalling logic that also appears in `saveTopicDir()`. Any change to the meta schema must be updated in multiple places. |
| 🟡 | SSE headers set twice | In the streaming handler, SSE headers (`SetContentType`, `Cache-Control`, etc.) are set before the goroutine AND inside the goroutine — redundant and fragile. |

---

### E — Entropy: 6/10

**Definition**: Code rot — duplicated code, inconsistent patterns, and decay over time.

#### Findings

| Severity | Location | Duplication / Decay |
|----------|----------|---------------------|
| 🔴 | `generateResponse()` vs `generateReevaluateResponse()` | ~90% identical: HTTP client creation, payload marshalling, request building, auth headers, response parsing, choice/message/content extraction. Should be ONE method. |
| 🟡 | Topic-not-found check | Repeated verbatim in 8+ handlers: `topic := store.Get(id); if topic == nil { return NotFound }`. No middleware or helper. |
| 🟡 | Completed-topic block check | Repeated in 3 handlers: `if topic.Status == "completed" { return Conflict }`. |
| 🟡 | `topicMeta` marshalling | Same `topicMeta{}` struct construction and `json.MarshalIndent(meta, "", "  ")` in `saveTopicDir()` and `persistTopicUpdate()`. |
| 🟡 | `Format` type duplicated | Defined in both `internal/export` and `internal/convert` with identical values (`markdown`, `pdf`, `word`). |
| 🟡 | `Store.List()` vs `FileStore.List()` | Near-identical implementation with deep copy. The sorting logic is copied. |
| 🟡 | Error response patterns | `c.Status(http.StatusBadRequest).JSON(fiber.Map{"message": "..."})` repeated 20+ times with no helper. |

---

### L — Long Method: 8/10

**Definition**: Methods that are too long, making them hard to understand, test, and maintain.

#### Findings

| LOC | Method | File | Issue |
|-----|--------|------|-------|
| ~500 | `SetupRoutes()` | `api/routes.go` | All route handlers as closures. Should be extracted to individual handler functions or a handler struct. |
| ~120 | `StreamResponse()` | `internal/llm/client.go` | HTTP request, SSE parsing, token extraction, error handling — all in one method. |
| ~100 | streaming handler closure | `api/routes.go` | Pipe creation, goroutine launch, token capture, document extraction, message saving. |
| ~95 | `generateResponse()` | `api/routes.go` | Full HTTP request cycle: payload build → request → send → parse response. |
| ~80 | upload handler closure | `api/routes.go` | Multipart parsing, file size check, conversion, attachment creation, persistence. |
| ~75 | `generateReevaluateResponse()` | `api/routes.go` | Near-duplicate of `generateResponse()`. |
| ~100 | `toPDF()` | `internal/export/export.go` | Markdown parsing loop with many if-else branches for each element type. |
| ~100 | `parseWordXML()` | `internal/convert/convert.go` | XML token scanning with inline state management. |
| ~60 | `buildConversationContext()` | `api/routes.go` | Context assembly with attachments, document, summarization logic. |

**Guideline**: A method should ideally be < 30–40 lines. Here, 9 methods exceed 60 lines and 3 exceed 100 lines.

---

### Y — Yellowbrick Bridge: 4/10

**Definition**: Over-engineering — unnecessary abstractions, YAGNI violations, speculative generality.

#### Findings

| Severity | Location | Issue |
|----------|----------|-------|
| 🟡 | `lockTopic()` / `topicLockFile()` | Per-topic locking methods exist in `FileStore` but are **never called** from any public method. The global lock is used; per-topic locks are dead code. |
| 🟡 | `Config.RedactedLLMConfig()` | Method to return redacted LLM config — never called anywhere. Speculative generality. |
| 🟡 | `loadTopicLegacy()` / `migrateTopicToDir()` | Legacy migration code exists but there is no legacy data. If no production deployment exists with the old format, this is speculative. |
| 🟡 | `Store` (in-memory) vs `FileStore` | The project spec mandates file-based persistence. The in-memory `Store` is useful for tests but creates a dual-implementation maintenance burden. |
| 🟡 | `Format` in both `export` and `convert` | Same type defined twice. Should be in a shared `types` or `topic` package. |
| 🟢 | `TopicStore` interface | Justified — enables testing with in-memory store. Not over-engineering. |
| 🟢 | `atomicWriteFile()` | Good defensive practice for crash-safe persistence. Not over-engineering. |

**Overall**: The Yellowbrick Bridge score is relatively low. The codebase has a few dead methods and duplicate types but is mostly pragmatic.

---

## Priority Refactoring Recommendations

### P0 — Critical (Address Immediately)

1. **Extract `generateResponse` / `generateReevaluateResponse` into a shared method**
   - These two ~90-line functions are 90% identical. Extract the HTTP request/response cycle into a single `client.CallLLM(ctx, messages []Message) (string, error)` method on the LLM client. This simultaneously fixes **Entropy** (duplication) and **Long Method**.

2. **Create an `internal/interview` package**
   - Move `buildConversationContext`, `extractDocument`, `summarizeMessages`, `generateResponse` into an `InterviewService` struct. This eliminates **Arrow Procedures** and **Feature Envy** by giving cross-cutting logic a home.

3. **Use `github.com/cloudwego/eino` as the LLM client**
   - The spec mandates this. Current raw HTTP calls are hardcoded to OpenAI format. Replace with eino's provider-agnostic abstraction.

### P1 — High Priority (Next Iteration)

4. **Extract route handlers from `SetupRoutes`**
   - Each closure in `SetupRoutes` should be a standalone function or method on a `Handler` struct. This fixes the **Long Method** (SetupRoutes at 500 lines) and **Rigidity** (can't test handlers independently).

5. **Deduplicate the `topicMeta` marshalling**
   - Extract `topicMeta{}` construction to a helper. Appears in both `saveTopicDir()` and `persistTopicUpdate()`.

6. **Consolidate the `Format` type**
   - Merge the duplicate `Format` types from `export` and `convert` into a shared location (e.g., `internal/topic`).

### P2 — Medium Priority (When Convenient)

7. **Create a `requireNotFound(store, id)` helper**
   - Eliminate the 8+ repeated topic-not-found checks.

8. **Create a `requireActive(store, id)` helper**
   - Eliminate the repeated "completed topic" checks.

9. **Remove dead `lockTopic()` / `topicLockFile()` methods** or wire them up if per-topic locking is actually needed.

10. **Remove or use `Config.RedactedLLMConfig()`** — it's dead code.

### P3 — Low Priority (Future)

11. **Evaluate whether legacy migration code is needed** — if no legacy data exists, remove it.

12. **Consider whether in-memory `Store` belongs in `internal/topic`** or should live only in a test package.

---

## Component Health Matrix

| Component | F | A | R | E | L | Y | Avg |
|-----------|---|---|---|---|---|---|-----|
| `cmd/server/main.go` | 1 | 0 | 2 | 1 | 2 | 1 | 1.2 |
| `api/routes.go` | 9 | 10 | 9 | 8 | 10 | 3 | 8.2 |
| `api/health.go` | 2 | 1 | 1 | 1 | 2 | 1 | 1.3 |
| `api/landing.go` | 1 | 0 | 1 | 0 | 1 | 1 | 0.7 |
| `internal/config/config.go` | 1 | 0 | 2 | 3 | 3 | 2 | 1.8 |
| `internal/llm/client.go` | 2 | 1 | 6 | 2 | 5 | 1 | 2.8 |
| `internal/topic/store.go` | 1 | 0 | 2 | 4 | 2 | 2 | 1.8 |
| `internal/topic/filestore.go` | 1 | 0 | 2 | 3 | 2 | 3 | 1.8 |
| `internal/export/export.go` | 1 | 1 | 2 | 1 | 5 | 1 | 1.8 |
| `internal/convert/convert.go` | 1 | 1 | 1 | 1 | 5 | 1 | 1.7 |

> `api/routes.go` is the single point of failure — averaging 8.2/10 across all FARLEY dimensions. This file needs to be decomposed.

---

## Trending Assessment

The codebase shows a clear pattern: the **core domain and persistence layers** (`topic/`, `config/`, `export/`, `convert/`) are well-structured with low FARLEY scores (averaging 1.2–2.8). The **API layer** (`routes.go`) is the sole source of high scores, acting as a god-object that absorbs all business logic.

This is a common "accidental" architecture: the domain layers are clean, but the glue layer grew without boundaries. The fix is surgical — extract the interview orchestration logic from `routes.go` into a dedicated service layer. This single change would reduce the total FARLEY score by an estimated 15–20 points.
