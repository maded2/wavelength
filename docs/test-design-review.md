# Test Design Review — Wavelength Test Suite

**Date**: 2026-05-13
**Scope**: 39 test files across 7 packages (`api/`, `internal/config/`, `internal/convert/`, `internal/export/`, `internal/llm/`, `internal/topic/`)
**Framework**: Dave Farley's Properties of Good Tests

---

## Test Suite Overview

| Package | Test Files | Pattern | Coverage Focus |
|---------|-----------|---------|----------------|
| `api/` | 28 files | HTTP integration via `fiber.Test()` | Acceptance criteria from user stories |
| `internal/config/` | 6 files | Table-driven + subtests | Config loading, validation, persona prompts |
| `internal/convert/` | 1 file | Table-driven + subtests | Document format detection, conversion |
| `internal/export/` | 1 file | Subtests | Export to PDF, Word, Markdown |
| `internal/llm/` | 1 file | Subtests + `httptest` server | LLM connectivity checks |
| `internal/topic/` | 2 files | Subtests + `t.TempDir()` | File persistence, document persistence |
| `internal/interview/` | 0 files | — | **No direct tests** |

**Total**: 39 test files, ~0.2s full suite runtime, 100% deterministic, zero external dependencies.

---

## Property Scores

| Property | Score | Evidence |
|----------|-------|----------|
| Understandable | 8/10 | Story-referenced names, behavioral subtest names; but 12-line setup boilerplate obscures the test's intent |
| Maintainable | 5/10 | Massive boilerplate duplication across API tests; `strings.Contains` on error messages creates coupling to implementation; no shared test fixtures |
| Repeatable | 10/10 | Fully deterministic — mocked LLM, `t.TempDir()`, in-memory stores, no timing dependencies |
| Atomic | 10/10 | Every test creates its own `fiber.New()`, `NewStore()`, `NewClient()`, `SetupRoutes()` — zero shared state |
| Necessary | 8/10 | One test per user story acceptance criterion; `document_extract_test.go` has 10 edge-case subtests that are thorough and valuable; some redundancy in config validation tests |
| Granular | 7/10 | Subtests are well-scoped (one behavior each), but individual subtests often assert multiple fields (status code + body + error message) |
| Fast | 8/10 | 0.2s for entire suite; in-memory stores and `httptest` servers are fast; `persistence_test.go` uses disk I/O but is still fast |
| First (TDD) | 6/10 | Story references suggest ATDD-driven development; but no direct tests for `interview` package (added after refactoring); test structure follows implementation, not design |

---

## Farley Score Calculation

```
Farley Score = (U×1.5 + M×1.5 + R×1.25 + A×1.0 + N×1.0 + G×1.0 + F×0.75 + T×1.0) / 9
             = (8×1.5 + 5×1.5 + 10×1.25 + 10×1.0 + 8×1.0 + 7×1.0 + 8×0.75 + 6×1.0) / 9
             = (12.0 + 7.5 + 12.5 + 10.0 + 8.0 + 7.0 + 6.0 + 6.0) / 9
             = 69.0 / 9
             = 7.67
```

### **Farley Score: 7.7/10 — Excellent**

> High-quality test suite with clear improvement opportunities. The Atomic and Repeatable properties are exemplary (10/10), and Understandable is strong at 8/10. The main weakness is Maintainability (5/10) due to boilerplate duplication.

---

## Detailed Analysis

### Understandable: 8/10 — Strong, with one structural issue

**What works well:**
- **Story references**: Every test file is anchored to a user story (`// E2-S4: Topics and their state persist across application restarts`). This makes the test's purpose immediately clear.
- **Behavioral subtest names**: `"user can create a topic by providing a name and high-level description"` reads like an acceptance criterion, not a test name. This is excellent specification-by-example.
- **Package-level clarity**: `internal/topic/persistence_test.go` clearly tests persistence; `api/document_extract_test.go` clearly tests document extraction.

**What drags the score down:**
- **12-line setup boilerplate per subtest**: The signal-to-noise ratio in API tests is poor. Each subtest in `topic_create_test.go` repeats the same 12 lines of `fiber.New()`, `topic.NewStore()`, `config.Config{...}`, `llm.NewClient(cfg)`, `SetupRoutes(...)`. This hides the actual test logic.

```go
// Every API test subtest repeats this boilerplate:
func TestCreateTopic(t *testing.T) {
    t.Run("...", func(t *testing.T) {
        app := fiber.New()                       // ← repeated 30+ times
        store := topic.NewStore()                 // ← repeated 30+ times
        cfg := &config.Config{...}                // ← 8 lines, repeated 30+ times
        client := llm.NewClient(cfg)              // ← repeated 30+ times
        SetupRoutes(app, store, client)           // ← repeated 30+ times
        // NOW the actual test begins...
    })
}
```

**Example of excellent clarity** — `persistence_test.go`:
```go
t.Run("all created topics are still present after restart", func(t *testing.T) {
    dir := t.TempDir()
    store1 := NewFileStore(dir)
    store1.Create("topic-001", "Topic One", "First topic description")
    store1.SaveAll()
    store2 := NewFileStore(dir)
    store2.LoadAll()
    // Clean setup → clear action → clear assertion
})
```

### Maintainable: 5/10 — The weakest property

**Critical issue — Boilerplate duplication:**
The API test suite has ~30 subtests, each with 12+ lines of identical setup. If the setup changes (e.g., adding a new config field), every test file must be updated. This is a **Shotgun Surgery** smell in the test code itself.

```go
// Current: Each test has its own config setup
cfg := &config.Config{
    Server: config.ServerConfig{Port: 3000},
    LLM: config.LLMConfig{
        Provider: "openai",
        Model:    "gpt-4",
        Endpoint: "http://localhost:11434",
        APIKey:   "test-key",
    },
    DataDir: t.TempDir(),
}
```

**No shared fixture helper**: A single `func newTestApp(t *testing.T) (*fiber.App, *topic.Store, *llm.Client)` in a shared `testutil.go` file would eliminate 90% of this duplication.

**Brittle `strings.Contains` assertions:**
```go
// Coupled to exact error message wording
lowerResp := strings.ToLower(respStr)
if !strings.Contains(lowerResp, "name") || !strings.Contains(lowerResp, "required") {
    t.Errorf("expected error message about required name, got: %s", respStr)
}
```
If the error message changes from `"name is required"` to `"topic name is required"`, this test still passes. But if it changes to `"invalid: name cannot be empty"`, it fails — even though the behavior is correct.

**Better approach**: Assert on the structured JSON response field:
```go
var resp fiber.Map
json.NewDecoder(resp.Body).Decode(&resp)
if resp["message"] == nil {
    t.Error("expected error message in response")
}
```

**No tests for the new `interview` package**: The refactoring moved significant business logic to `internal/interview/service.go`, but no direct tests exist for `HandleMessage()`, `Reevaluate()`, `BuildConversationContext()`, or `SummarizeMessages()`. The only test coverage is indirect via the API integration tests.

### Repeatable: 10/10 — Exemplary

**Zero flakiness concerns:**
- LLM is mocked via `httptest.NewServer` — no real API calls
- Persistence tests use `t.TempDir()` — no shared filesystem state
- API tests use `topic.NewStore()` (in-memory) — no shared database
- No `time.Sleep()` or race conditions
- All tests can run in parallel (`-parallel` flag) without interference

```go
// httptest.NewServer guarantees deterministic responses
llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"choices":[{"message":{"content":"Hello! I'm your business analyst."}}]}`))
}))
defer llmServer.Close()
```

### Atomic: 10/10 — Exemplary

**Every test is fully isolated:**
```go
// Each subtest creates fresh instances — no shared state between tests
t.Run("test A", func(t *testing.T) {
    app := fiber.New()           // Fresh Fiber app
    store := topic.NewStore()    // Fresh in-memory store
    client := llm.NewClient(cfg) // Fresh LLM client
    SetupRoutes(app, store, client)
    // ... test
})

t.Run("test B", func(t *testing.T) {
    app := fiber.New()           // Another fresh Fiber app
    store := topic.NewStore()    // Another fresh in-memory store
    // ... completely independent
})
```

No test depends on another test's setup. Test order does not matter. Tests can run in any order or in parallel.

### Necessary: 8/10 — Strong coverage with minor redundancy

**What adds value:**
- **`document_extract_test.go`** is exceptional — 10 subtests covering every edge case: no delimiters, document only, conversation only, conversation before, conversation after, complex markdown with tables and emojis, single delimiter, empty document, whitespace trimming, markdown horizontal rules. Each subtest is a necessary boundary condition.

- **`persistence_test.go`** covers the critical "restart simulation" pattern (save → recreate store → load → verify) which is the core acceptance criterion for E2-S4.

- **`topic_isolation_test.go`** correctly tests the strict topic isolation requirement with negative assertions (Topic B's prompt should NOT contain Topic A's data).

**Minor redundancy:**
- `TestLLMConfigValidation` and `TestLLMConfigSwappable` in config tests overlap — both verify that different LLM configs can be loaded and validated. Could be consolidated.

**Gap**: No direct tests for `internal/interview` package. The refactoring moved `buildConversationContext`, `summarizeMessages`, and `extractDocument` into this package, but the only tests for these functions are in `api/` (which imports `interview`). Direct unit tests would be faster and more focused.

### Granular: 7/10 — Good subtest structure, some multi-assertion tests

**Strengths:**
- Each `t.Run` subtest tests one behavior: `"topic name is required"`, `"duplicate topic name is rejected"`, `"no delimiters returns full response as conversational"`.
- `document_extract_test.go` has the finest granularity — 10 focused subtests for a single function.

**Weaknesses:**
- Some subtests assert multiple fields that aren't strictly one behavior:
```go
// Tests 3 things: status code, name field, description field, and ID field
if resp.StatusCode != http.StatusCreated { ... }
if created["name"] != "E-Commerce Platform" { ... }
if created["description"] != "..." { ... }
if created["id"] == "" { ... }
```
This isn't terrible (they're all part of "topic was created correctly"), but a failure in the last assertion would require reading through all previous assertions to understand what was checked.

### Fast: 8/10 — Very fast with minor optimization room

- **Full suite**: 0.2s — excellent
- **API tests**: 0.2s for 28 test files — good, but each test creates a Fiber app and runs HTTP through it
- **Persistence tests**: Uses actual disk I/O (`t.TempDir()` + file operations) — still fast but slower than in-memory
- **Config tests**: 0.013s — near-instant

**Optimization opportunity**: The 12-line setup per test (creating Fiber app, config, store, client, routes) adds up across 30+ subtests. A shared setup helper wouldn't make individual tests faster, but it would make the suite easier to optimize (e.g., single shared test server for all API tests in a `TestMain`).

### First (TDD): 6/10 — Likely ATDD-driven but with gaps

**Evidence for test-first:**
- Story references (`E1-S1`, `E2-S4`) suggest acceptance tests were written from user story criteria before implementation
- Test names mirror acceptance criterion language: `"all created topics are still present after restart"` reads like a user story acceptance criterion
- The test suite structure follows the epic order (E1 → E2 → E3 → E4)

**Evidence against:**
- The new `interview` package has zero direct tests — it was created during refactoring without corresponding tests
- Tests assert on HTTP response structure (status codes, JSON fields) rather than on business outcomes — this suggests tests were written after the implementation existed
- No tests for error paths in the `interview` service (e.g., what happens when `client.Call()` returns an error)

**Conservative assessment**: The ATDD methodology is followed for the initial implementation, but the refactoring phase broke the TDD cycle (no red phase for the interview package).

---

## Top Recommendations

### 1. [HIGH IMPACT] Extract Test Fixture Helper — Addresses Maintainability (5 → 8)

**Problem**: 12 lines of identical setup repeated 30+ times across API tests.

**Solution**: Create a shared fixture function:

```go
// api/testutil.go
func newTestSuite(t *testing.T) *TestSuite {
    t.Helper()
    store := topic.NewStore()
    cfg := &config.Config{
        Server:  config.ServerConfig{Port: 3000},
        LLM:     config.LLMConfig{Provider: "openai", Model: "gpt-4", Endpoint: "http://localhost:11434", APIKey: "test-key"},
        DataDir: t.TempDir(),
    }
    client := llm.NewClient(cfg)
    app := fiber.New()
    SetupRoutes(app, store, client)
    return &TestSuite{App: app, Store: store, Client: client, Config: cfg}
}

// For tests that need a mock LLM server:
func newTestSuiteWithMock(t *testing.T, handler http.HandlerFunc) *TestSuiteWithMock {
    // ... creates httptest.NewServer(handler), wires it into config
}
```

**Impact**: Eliminates ~360 lines of duplicated boilerplate. Any setup change (e.g., new config field) affects one location.

### 2. [MEDIUM IMPACT] Add Direct Tests for `internal/interview` Package — Addresses Necessary (8 → 9), First (6 → 8)

**Problem**: The interview service has ~250 lines of business logic with zero direct tests. All coverage is indirect via API integration tests.

**Solution**: Write focused unit tests for the interview service:

```go
// internal/interview/service_test.go
func TestHandleMessage(t *testing.T) {
    t.Run("saves user message and assistant response", func(t *testing.T) {
        store := topic.NewStore()
        store.Create("t1", "Test", "Description")
        client := newMockClient("Assistant reply")
        svc := New(store, client)
        conv, updated, err := svc.HandleMessage(ctx, "t1", "Hello")
        require.NoError(t, err)
        // Verify messages saved
    })
    t.Run("extracts and saves document when delimiters present", func(t *testing.T) {
        // ...
    })
}
```

**Impact**: Faster test execution (no HTTP layer), more focused assertions, completes the TDD cycle for the refactoring.

### 3. [MEDIUM IMPACT] Assert on Structured Response Fields — Addresses Maintainability (5 → 7)

**Problem**: `strings.Contains` on error messages couples tests to implementation wording.

**Solution**: Decode JSON responses and assert on structured fields:

```go
// Instead of:
lowerResp := strings.ToLower(respStr)
if !strings.Contains(lowerResp, "name") || !strings.Contains(lowerResp, "required") { ... }

// Use:
var result struct {
    Message string `json:"message"`
}
json.NewDecoder(resp.Body).Decode(&result)
if result.Message == "" {
    t.Error("expected error message field in response")
}
```

**Impact**: Tests become resilient to wording changes while still verifying the error response structure.

### 4. [LOW IMPACT] Split Multi-Assertion Subtests — Addresses Granularity (7 → 8)

**Problem**: Some subtests assert 4+ fields, making failure diagnosis harder.

**Solution**: For the "topic creation success" case, consider splitting into two subtests:
- `"returns 201 status"` (one assertion)
- `"returns topic with correct fields"` (one assertion per field, or one assertion that the response matches expected)

This is lower priority since the current approach works and the tests pass. But for maximum granularity, each subtest should assert exactly one behavior.

---

## Score Summary Table

| Property | Before | Target | Gap |
|----------|--------|--------|-----|
| Understandable | 8 | 9 | Add test fixture helper to reduce noise |
| Maintainable | 5 | 8 | Extract shared fixtures, structured assertions |
| Repeatable | 10 | 10 | ✅ No action needed |
| Atomic | 10 | 10 | ✅ No action needed |
| Necessary | 8 | 9 | Add interview package tests |
| Granular | 7 | 8 | Split multi-assertion subtests |
| Fast | 8 | 9 | Shared test server via TestMain |
| First (TDD) | 6 | 8 | Write tests before refactoring changes |

**Projected post-improvement score**: ~8.4/10 (Excellent → Exemplary threshold)

---

## Reference

This review is based on Dave Farley's Properties of Good Tests:
https://www.linkedin.com/pulse/tdd-properties-good-tests-dave-farley-iexge/
