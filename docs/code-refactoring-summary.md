# Code Refactoring Summary — Wavelength

**Date**: 2026-05-13
**Based on**: FARLEY Score Report (40/60)

---

## High-Level Overview

Six refactoring operations were applied to address the top code smells in the Wavelength codebase. The primary target was `api/routes.go` (500+ lines), which mixed HTTP handling, LLM orchestration, and business logic into a single monolithic function.

### What Changed

| Area | Before | After |
|------|--------|-------|
| **Business Logic** | Free functions in `api/routes.go` | Dedicated `internal/interview/` package with `Service` struct |
| **LLM Calls** | 2 near-identical ~95-line functions | Single `client.Call(ctx, messages)` method |
| **Route Handlers** | 500+ line `SetupRoutes` with inline closures | `Handler` struct with 14 named methods |
| **Format Type** | Defined in 2 packages (`export`, `convert`) | Single source: `internal/topic/types.go` |
| **topicMeta** | Duplicated in 3 FileStore methods | Single `topic.Meta()` method |
| **Dead Code** | `lockTopic()`, `RedactedLLMConfig()` | Removed |

### New Package

```
internal/interview/service.go
  └─ Service struct
       ├─ HandleMessage(ctx, topicID, userMessage) → (conversational, docUpdated, err)
       ├─ Reevaluate(ctx, topicID) → (conversational, docUpdated, err)
       ├─ BuildPrompt(topic, userMessage) → string
       ├─ BuildConversationContext(topic, userMessage) → string (exported for tests)
       ├─ SummarizeMessages(messages) → string (exported for tests)
       └─ ExtractDocument(response) → (conversational, document) (exported for tests)
```

---

## Priority Matrix

| # | Refactoring | Impact | Complexity | Risk |
|---|------------|--------|-----------|------|
| 1 | Extract `interview` package | 🔴 High | 🟡 Moderate | 🟡 Medium |
| 2 | Consolidate `client.Call()` | 🔴 High | 🟢 Low | 🟢 Low |
| 3 | Extract `Handler` struct | 🔴 High | 🟡 Moderate | 🟢 Low |
| 4 | Consolidate `Format` type | 🟡 Medium | 🟢 Low | 🟢 Low |
| 5 | Deduplicate `topicMeta` | 🟡 Medium | 🟢 Low | 🟢 Low |
| 6 | Delete dead code | 🟢 Low | 🟢 Low | 🟢 Low |

---

## Quick Reference: Techniques Applied

| Technique | Category | What It Fixed |
|-----------|----------|---------------|
| **Extract Class** | Moving Features | Split `routes.go` business logic into `interview` package |
| **Move Method** | Moving Features | Moved `buildConversationContext`, `summarizeMessages`, `extractDocument` to `interview` |
| **Parameterize Method** | Simplifying Calls | Merged `generateResponse` + `generateReevaluateResponse` → `client.Call()` |
| **Extract Method** | Composing Methods | Extracted `requireActiveTopic()`, `sanitizeFilename()`, `writeJSON()`, `topic.Meta()`, `buildMessagePayload()`, `parseResponse()` |
| **Compose Method** (Kerievsky) | Simplification Pattern | Decomposed 500-line `SetupRoutes` into `Handler` with named methods |
| **Decompose Conditional** | Simplifying Conditionals | Extracted guard clauses into `requireActiveTopic()` |
| **Delete Dead Code** | Dispensables | Removed `lockTopic()`, `topicLockFile()`, `RedactedLLMConfig()` |
| **Extract Class** | Organizing Data | Consolidated duplicate `Format` type into `internal/topic/types.go` |

---

## Key Benefits

1. **Testability**: Interview service can now be unit-tested independently of HTTP layer
2. **Maintainability**: Each handler method is < 30 lines (down from 500-line `SetupRoutes`)
3. **Reduced Duplication**: LLM HTTP call consolidated from 190 lines to 60
4. **Clearer Separation**: HTTP layer (api/) vs Business logic (interview/) vs Persistence (topic/) vs External (llm/)
5. **Single Source of Truth**: `Format` type and `topicMeta` marshalling in one place

---

## Implementation Sequence

```
Phase 1: Foundation (no dependencies)
  ├─ Consolidate Format type → internal/topic/types.go
  └─ Add topic.Meta() method

Phase 2: Consolidation
  ├─ Add client.Call(ctx, messages) → internal/llm/client.go
  └─ Deduplicate topicMeta marshalling → internal/topic/filestore.go

Phase 3: Business Logic Extraction
  └─ Create internal/interview package with Service

Phase 4: Route Decomposition
  └─ Extract Handler struct, replace inline closures

Phase 5: Dead Code Removal
  ├─ Remove lockTopic()/topicLockFile()
  └─ Remove RedactedLLMConfig()
```

---

## Verification

```bash
$ go build ./...     # ✅ Clean
$ go vet ./...       # ✅ Clean
$ go test -count=1 ./...  # ✅ 30+ tests pass
```

**Estimated FARLEY improvement**: ~40/60 → ~22/60 (37%)
- **Feature Envy**: 7 → 3 (business logic has a home)
- **Arrow Procedure**: 8 → 3 (interview service owns cross-cutting flow)
- **Long Method**: 8 → 4 (SetupRoutes decomposed, handlers < 30 lines each)
- **Entropy**: 6 → 3 (Format consolidated, Call() eliminates duplication)
