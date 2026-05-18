package export

import (
	"strings"
	"testing"

	"wavelength/internal/topic"
)

func TestExportMarkdown(t *testing.T) {
	content := "# Test Doc\n\nSome content here."
	exp := New(content)

	data, mimeType, ext, err := exp.Export(topic.FormatMarkdown)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mimeType != "text/markdown" {
		t.Errorf("expected mime type text/markdown, got %s", mimeType)
	}
	if ext != ".md" {
		t.Errorf("expected extension .md, got %s", ext)
	}
	if string(data) != content {
		t.Errorf("expected content to match, got: %s", string(data))
	}
}

func TestExportPDF(t *testing.T) {
	content := "# Requirements\n\n## Overview\n\nA test system.\n\n- Item 1\n- Item 2"
	exp := New(content)

	data, mimeType, ext, err := exp.Export(topic.FormatPDF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mimeType != "application/pdf" {
		t.Errorf("expected mime type application/pdf, got %s", mimeType)
	}
	if ext != ".pdf" {
		t.Errorf("expected extension .pdf, got %s", ext)
	}
	// PDF files start with %PDF
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Errorf("expected valid PDF header, got: %s", string(data[:min(len(data), 20)]))
	}
}

func TestExportWord(t *testing.T) {
	content := "# Requirements\n\n## Overview\n\nA test system."
	exp := New(content)

	data, mimeType, ext, err := exp.Export(topic.FormatWord)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mimeType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Errorf("expected word mime type, got %s", mimeType)
	}
	if ext != ".docx" {
		t.Errorf("expected extension .docx, got %s", ext)
	}
	// DOCX files are ZIP files, which start with PK
	if len(data) < 2 || string(data[:2]) != "PK" {
		t.Errorf("expected valid DOCX (ZIP) header, got: %s", string(data[:min(len(data), 20)]))
	}
}

func TestExportUnsupportedFormat(t *testing.T) {
	exp := New("content")
	_, _, _, err := exp.Export(topic.Format("rtf"))
	if err == nil {
		t.Error("expected error for unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected unsupported error message, got: %s", err.Error())
	}
}

func TestExportEmptyContent(t *testing.T) {
	exp := New("")

	// Markdown should return empty bytes
	data, _, _, err := exp.Export(topic.FormatMarkdown)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data for markdown, got %d bytes", len(data))
	}

	// PDF should still generate a valid (empty page) PDF
	data, _, _, err = exp.Export(topic.FormatPDF)
	if err != nil {
		t.Fatalf("unexpected error for empty PDF: %v", err)
	}
	if string(data[:5]) != "%PDF-" {
		t.Error("expected valid PDF even with empty content")
	}

	// Word should still generate a valid DOCX
	data, _, _, err = exp.Export(topic.FormatWord)
	if err != nil {
		t.Fatalf("unexpected error for empty Word: %v", err)
	}
	if string(data[:2]) != "PK" {
		t.Error("expected valid DOCX even with empty content")
	}
}

func TestExportComplexMarkdown(t *testing.T) {
	content := `# Title

## Section 1

Some **bold** and *italic* text.

- Bullet 1
- Bullet 2

1. First
2. Second

> A quote

---

### Subsection

Code: ` + "`inline`" + ` and blocks:

` + "```" + `
code block
` + "```" + `
`
	exp := New(content)

	// All formats should handle complex content without error
	for _, format := range []topic.Format{topic.FormatMarkdown, topic.FormatPDF, topic.FormatWord} {
		_, _, _, err := exp.Export(format)
		if err != nil {
			t.Errorf("format %s failed with complex content: %v", format, err)
		}
	}
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello <world>", "hello &lt;world&gt;"},
		{"a & b", "a &amp; b"},
		{`say "hi"`, `say &quot;hi&quot;`},
		{"it's", "it&apos;s"},
		{"plain", "plain"},
	}

	for _, tc := range tests {
		if got := escapeXML(tc.input); got != tc.expected {
			t.Errorf("escapeXML(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestCleanInlineMarkdown(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"**bold**", "bold"},
		{"*italic*", "italic"},
		{"`code`", "code"},
		{"[link](url)", "link"},
		{"plain text", "plain text"},
	}

	for _, tc := range tests {
		if got := cleanInlineMarkdown(tc.input); got != tc.expected {
			t.Errorf("cleanInlineMarkdown(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// min returns the smaller of two integers (for Go < 1.21 compatibility).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
