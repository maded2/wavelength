package api

import (
	"strings"
	"testing"
)

func TestExtractDocument(t *testing.T) {
	t.Run("no delimiters returns full response as conversational", func(t *testing.T) {
		response := "Sure, let me ask you about the user roles in this system."
		conv, doc := extractDocument(response)

		if doc != "" {
			t.Errorf("expected empty document, got: %q", doc)
		}
		if conv != "Sure, let me ask you about the user roles in this system." {
			t.Errorf("expected full response as conversational, got: %q", conv)
		}
	})

	t.Run("document between delimiters is extracted", func(t *testing.T) {
		response := `Thank you! I've updated the requirements document.

---
# Requirements: Test System

## Overview

A test system.
---

Let me know if you have any questions.`

		conv, doc := extractDocument(response)

		if doc == "" {
			t.Fatal("expected document to be extracted, got empty string")
		}
		if !strings.Contains(doc, "# Requirements: Test System") {
			t.Errorf("expected document to contain title, got: %s", doc)
		}
		if !strings.Contains(doc, "## Overview") {
			t.Errorf("expected document to contain overview section, got: %s", doc)
		}

		if !strings.Contains(conv, "Thank you") {
			t.Errorf("expected conversational part to contain greeting, got: %s", conv)
		}
		if !strings.Contains(conv, "Let me know") {
			t.Errorf("expected conversational part to contain closing, got: %s", conv)
		}
		// Document content should NOT appear in conversational part
		if strings.Contains(conv, "# Requirements: Test System") {
			t.Errorf("expected document content to NOT be in conversational part, got: %s", conv)
		}
	})

	t.Run("document only (no surrounding conversation) returns empty conversational", func(t *testing.T) {
		response := `---
# Requirements

## Overview

Some content.
---`

		conv, doc := extractDocument(response)

		if doc == "" {
			t.Fatal("expected document to be extracted, got empty string")
		}
		if !strings.Contains(doc, "# Requirements") {
			t.Errorf("expected document content, got: %s", doc)
		}
		if conv != "" {
			t.Errorf("expected empty conversational part, got: %q", conv)
		}
	})

	t.Run("only conversation after document", func(t *testing.T) {
		response := `---
# Requirements

## Overview

Some content.
---

Great, let's move on to the next section.`

		conv, doc := extractDocument(response)

		if doc == "" {
			t.Fatal("expected document to be extracted, got empty string")
		}
		if !strings.Contains(conv, "Great, let's move on") {
			t.Errorf("expected conversational part after document, got: %s", conv)
		}
	})

	t.Run("only conversation before document", func(t *testing.T) {
		response := `Based on what you've told me, I've drafted the initial requirements.

---
# Requirements

## Overview

Some content.
---`

		conv, doc := extractDocument(response)

		if doc == "" {
			t.Fatal("expected document to be extracted, got empty string")
		}
		if !strings.Contains(conv, "Based on what you've told me") {
			t.Errorf("expected conversational part before document, got: %s", conv)
		}
	})

	t.Run("complex document with nested markdown is preserved", func(t *testing.T) {
		response := `---
### 📄 Living Requirements Document: Staff Stock Trading Declaration System
*Version: 0.1 | Last Updated: [Current Date]*

#### 1. System Overview
- **Purpose:** Enable staff to formally declare stock trades.
- **Scope:** Limited to a specific subset of employees.

#### 2. User Roles & Access
| Role | Description | Access/Permissions |
|------|-------------|-------------------|
| Covered Staff | Employees subject to trading declaration | Can submit, view, and edit |

#### 3. Business Rules
- **BR-01:** The system shall only be available to "covered staff."
- **BR-02:** Definition of "covered staff" to be clarified.

#### 4. Data & Integrations
- **D-01:** User identity sourced from authoritative system (TBD).
- **D-02:** Handle status changes with effective dates.

#### 5. Assumptions & Open Questions
- [ ] How is "covered staff" officially defined?
- [ ] What is the current system/process?
- [ ] Are there temporary covered periods?
- [ ] Should non-covered staff see any part of the system?

*This document will be updated iteratively as we progress through the interview.*
---

Does this look accurate so far?`

		conv, doc := extractDocument(response)

		if doc == "" {
			t.Fatal("expected document to be extracted, got empty string")
		}

		// Verify document structure is preserved
		if !strings.Contains(doc, "### 📄 Living Requirements Document") {
			t.Errorf("expected document title with emoji, got: %s", doc)
		}
		if !strings.Contains(doc, "| Covered Staff |") {
			t.Errorf("expected table row, got: %s", doc)
		}
		if !strings.Contains(doc, "- **BR-01:**") {
			t.Errorf("expected business rule, got: %s", doc)
		}
		if !strings.Contains(doc, "- [ ] How is") {
			t.Errorf("expected checkbox item, got: %s", doc)
		}
		if !strings.Contains(doc, "*Version: 0.1") {
			t.Errorf("expected italic version, got: %s", doc)
		}

		if !strings.Contains(conv, "Does this look accurate") {
			t.Errorf("expected conversational follow-up, got: %s", conv)
		}
	})

	t.Run("single delimiter is treated as conversational content", func(t *testing.T) {
		response := `Here's the document:

---

# Requirements

No closing delimiter.`

		conv, doc := extractDocument(response)

		if doc != "" {
			t.Errorf("expected empty document with single delimiter, got: %q", doc)
		}
		if !strings.Contains(conv, "# Requirements") {
			t.Errorf("expected full content as conversational, got: %s", conv)
		}
	})

	t.Run("empty document between delimiters", func(t *testing.T) {
		response := `---
---`

		conv, doc := extractDocument(response)

		if doc != "" {
			t.Errorf("expected empty document, got: %q", doc)
		}
		if conv != "" {
			t.Errorf("expected empty conversational, got: %q", conv)
		}
	})

	t.Run("whitespace around delimiters is trimmed", func(t *testing.T) {
		response := `Some intro.

   ---

# Requirements

## Content

   ---

Some outro.`

		conv, doc := extractDocument(response)

		if doc == "" {
			t.Fatal("expected document to be extracted")
		}
		// Document should not have leading/trailing whitespace
		if doc != strings.TrimSpace(doc) {
			t.Errorf("expected trimmed document, got: %q", doc)
		}
		// Conversational parts should be trimmed
		if conv != strings.TrimSpace(conv) {
			t.Errorf("expected trimmed conversational, got: %q", conv)
		}
	})
}
