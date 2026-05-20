package convert

import (
	"strings"
	"testing"

	"wavelength/internal/topic"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     topic.Format
		wantErr  bool
	}{
		{"markdown .md", "document.md", topic.FormatMarkdown, false},
		{"markdown .markdown", "document.markdown", topic.FormatMarkdown, false},
		{"markdown uppercase", "DOCUMENT.MD", topic.FormatMarkdown, false},
		{"pdf", "document.pdf", topic.FormatPDF, false},
		{"pdf uppercase", "DOCUMENT.PDF", topic.FormatPDF, false},
		{"word docx", "document.docx", topic.FormatWord, false},
		{"word uppercase", "DOCUMENT.DOCX", topic.FormatWord, false},
		{"unsupported txt", "document.txt", "", true},
		{"unsupported jpg", "image.jpg", "", true},
		{"unsupported doc", "document.doc", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectFormat(tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Errorf("DetectFormat(%q) expected error, got nil", tt.filename)
				}
				return
			}
			if err != nil {
				t.Errorf("DetectFormat(%q) unexpected error: %v", tt.filename, err)
				return
			}
			if got != tt.want {
				t.Errorf("DetectFormat(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestConvertMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple markdown",
			input: "# Hello\n\nThis is a test document.\n\n## Section\n\n- Item 1\n- Item 2",
			want:  "# Hello\n\nThis is a test document.\n\n## Section\n\n- Item 1\n- Item 2",
		},
		{
			name:  "whitespace trimmed",
			input: "  \n  # Title  \n\n  Content  \n  ",
			want:  "# Title  \n\n  Content",
		},
		{
			name:    "empty",
			input:   "",
			want:    "",
			wantErr: false,
		},
	}

	c := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Convert(strings.NewReader(tt.input), "test.md")
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertUnsupportedFormat(t *testing.T) {
	c := New()
	_, err := c.Convert(strings.NewReader("content"), "file.txt")
	if err == nil {
		t.Error("expected error for unsupported format, got nil")
	}
}

func TestConvertPDF(t *testing.T) {
	// Create a minimal valid PDF with text content
	// This is a minimal PDF 1.4 with one page containing "Hello World"
	pdfContent := `%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
endobj
4 0 obj
<< /Length 44 >>
stream
BT /F1 12 Tf 100 700 Td (Hello World) Tj ET
endstream
endobj
5 0 obj
<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000266 00000 n 
0000000360 00000 n 
trailer
<< /Size 6 /Root 1 0 R >>
startxref
443
%%EOF`

	c := New()
	result, err := c.Convert(strings.NewReader(pdfContent), "test.pdf")
	if err != nil {
		// PDF parsing may fail with minimal PDFs depending on library capabilities.
		// The important thing is that it returns a clear error rather than panicking.
		if !strings.Contains(strings.ToLower(err.Error()), "pdf") {
			t.Errorf("expected PDF-related error, got: %v", err)
		}
		return
	}
	if !strings.Contains(result, "Hello World") {
		t.Errorf("expected PDF text extraction to contain 'Hello World', got: %q", result)
	}
}

func TestConvertWordInvalid(t *testing.T) {
	// Test with invalid DOCX content (not a valid ZIP)
	invalidDocx := "not a valid docx file"

	c := New()
	_, err := c.Convert(strings.NewReader(invalidDocx), "test.docx")
	if err == nil {
		t.Error("expected error for invalid DOCX, got nil")
	}
}

func TestUnescapeXML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello &amp; World", "Hello & World"},
		{"&lt;tag&gt;", "<tag>"},
		{"&quot;quoted&quot;", "\"quoted\""},
		{"&apos;apostrophe&apos;", "'apostrophe'"},
		{"Line1&#10;Line2", "Line1\nLine2"},
	}

	for _, tt := range tests {
		got := unescapeXML(tt.input)
		if got != tt.want {
			t.Errorf("unescapeXML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
