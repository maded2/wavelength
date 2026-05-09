package export

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// Format represents a document export format.
type Format string

const (
	FormatMarkdown Format = "markdown"
	FormatPDF      Format = "pdf"
	FormatWord     Format = "word"
)

// Exporter converts a markdown document to the requested format.
type Exporter struct {
	content string
}

// New creates a new Exporter for the given markdown content.
func New(content string) *Exporter {
	return &Exporter{content: content}
}

// Export renders the document in the requested format and returns the bytes
// along with the appropriate MIME type and file extension.
func (e *Exporter) Export(format Format) (data []byte, mimeType string, ext string, err error) {
	switch format {
	case FormatMarkdown:
		return []byte(e.content), "text/markdown", ".md", nil
	case FormatPDF:
		return e.toPDF()
	case FormatWord:
		return e.toWord()
	default:
		return nil, "", "", fmt.Errorf("unsupported export format: %q (supported: markdown, pdf, word)", format)
	}
}

// toPDF converts the markdown content to a PDF document.
func (e *Exporter) toPDF() ([]byte, string, string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 20) // 20mm bottom margin
	pdf.AddPage()

	// Page dimensions for A4
	pageW, _ := pdf.GetPageSize()
	leftMargin := 20.0
	rightMargin := 20.0
	contentWidth := pageW - leftMargin - rightMargin

	// Parse markdown lines and render them
	lines := strings.Split(e.content, "\n")
	var paragraph strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Headers
		if strings.HasPrefix(trimmed, "#### ") {
			flushParagraph(&paragraph, pdf, "Helvetica", "", 10, leftMargin)
			pdf.SetFont("Helvetica", "B", 12)
			pdf.SetX(leftMargin)
			pdf.MultiCell(contentWidth, 7, cleanInlineMarkdown(trimmed[5:]), "", "L", false)
			pdf.Ln(3)
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			flushParagraph(&paragraph, pdf, "Helvetica", "", 10, leftMargin)
			pdf.SetFont("Helvetica", "B", 14)
			pdf.SetX(leftMargin)
			pdf.MultiCell(contentWidth, 8, cleanInlineMarkdown(trimmed[4:]), "", "L", false)
			pdf.Ln(3)
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			flushParagraph(&paragraph, pdf, "Helvetica", "", 10, leftMargin)
			pdf.SetFont("Helvetica", "B", 16)
			pdf.SetX(leftMargin)
			pdf.MultiCell(contentWidth, 9, cleanInlineMarkdown(trimmed[3:]), "", "L", false)
			pdf.Ln(4)
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			flushParagraph(&paragraph, pdf, "Helvetica", "", 10, leftMargin)
			pdf.SetFont("Helvetica", "B", 18)
			pdf.SetX(leftMargin)
			pdf.MultiCell(contentWidth, 10, cleanInlineMarkdown(trimmed[2:]), "", "L", false)
			pdf.Ln(5)
			continue
		}

		// Horizontal rule
		if trimmed == "---" {
			flushParagraph(&paragraph, pdf, "Helvetica", "", 10, leftMargin)
			pdf.SetY(pdf.GetY() + 2)
			pdf.Line(leftMargin, pdf.GetY(), pageW-rightMargin, pdf.GetY())
			pdf.Ln(4)
			continue
		}

		// Blockquote
		if strings.HasPrefix(trimmed, "> ") {
			flushParagraph(&paragraph, pdf, "Helvetica", "", 10, leftMargin)
			pdf.SetFont("Helvetica", "I", 10)
			pdf.SetX(leftMargin + 5)
			pdf.MultiCell(contentWidth-5, 5, cleanInlineMarkdown(trimmed[2:]), "", "L", false)
			pdf.Ln(3)
			continue
		}

		// List items
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			flushParagraph(&paragraph, pdf, "Helvetica", "", 10, leftMargin)
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetX(leftMargin + 5)
			pdf.MultiCell(contentWidth-5, 5, "• "+cleanInlineMarkdown(trimmed[2:]), "", "L", false)
			continue
		}

		// Numbered list items (e.g. "1. Something")
		if dotIdx := strings.Index(trimmed, ". "); dotIdx > 0 {
			// Verify the prefix is numeric
			prefix := trimmed[:dotIdx]
			isNumeric := true
			for _, r := range prefix {
				if r < '0' || r > '9' {
					isNumeric = false
					break
				}
			}
			if isNumeric {
				flushParagraph(&paragraph, pdf, "Helvetica", "", 10, leftMargin)
				pdf.SetFont("Helvetica", "", 10)
				pdf.SetX(leftMargin + 5)
				pdf.MultiCell(contentWidth-5, 5, cleanInlineMarkdown(trimmed[dotIdx+2:]), "", "L", false)
				continue
			}
		}

		// Empty line — flush paragraph
		if trimmed == "" {
			flushParagraph(&paragraph, pdf, "Helvetica", "", 10, leftMargin)
			pdf.Ln(2)
			continue
		}

		// Regular text — accumulate into paragraph
		paragraph.WriteString(trimmed + " ")
	}

	// Flush any remaining text
	flushParagraph(&paragraph, pdf, "Helvetica", "", 10, leftMargin)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", "", fmt.Errorf("failed to generate PDF: %w", err)
	}

	return buf.Bytes(), "application/pdf", ".pdf", nil
}

func flushParagraph(p *strings.Builder, pdf *gofpdf.Fpdf, font, style string, size float64, leftMargin float64) {
	text := strings.TrimSpace(p.String())
	if text == "" {
		return
	}
	pdf.SetFont(font, style, size)
	pdf.SetX(leftMargin)
	pageW, _ := pdf.GetPageSize()
	contentWidth := pageW - leftMargin - 20 // 20mm right margin
	pdf.MultiCell(contentWidth, 5, cleanInlineMarkdown(text), "", "L", false)
	p.Reset()
}

// cleanInlineMarkdown strips basic inline markdown syntax for plain text rendering.
func cleanInlineMarkdown(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "`", "")
	// Strip markdown links [text](url) → text
	for {
		open := strings.Index(s, "[")
		if open == -1 {
			break
		}
		closeBracket := strings.Index(s[open+1:], "]")
		if closeBracket == -1 {
			break
		}
		closeBracket += open + 1 // absolute position
		// Look for (url) after ]
		afterBracket := s[closeBracket+1:]
		parenOpen := strings.Index(afterBracket, "(")
		if parenOpen == -1 {
			break
		}
		parenClose := strings.Index(afterBracket[parenOpen+1:], ")")
		if parenClose == -1 {
			break
		}
		// Reconstruct: text before [ + link text + text after )
		linkText := s[open+1 : closeBracket]
		suffixStart := closeBracket + 1 + parenOpen + parenClose + 2
		if suffixStart > len(s) {
			suffixStart = len(s)
		}
		s = s[:open] + linkText + s[suffixStart:]
	}
	return s
}

// toWord converts the markdown content to a DOCX file.
func (e *Exporter) toWord() ([]byte, string, string, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// [Content_Types].xml
	if err := writeZipFile(w, "[Content_Types].xml", contentTypesXML()); err != nil {
		return nil, "", "", fmt.Errorf("failed to write Content_Types: %w", err)
	}

	// _rels/.rels
	if err := writeZipFile(w, "_rels/.rels", relsXML()); err != nil {
		return nil, "", "", fmt.Errorf("failed to write root rels: %w", err)
	}

	// word/_rels/document.xml.rels
	if err := writeZipFile(w, "word/_rels/document.xml.rels", docRelsXML()); err != nil {
		return nil, "", "", fmt.Errorf("failed to write doc rels: %w", err)
	}

	// word/document.xml
	docXML := buildDocumentXML(e.content)
	if err := writeZipFile(w, "word/document.xml", docXML); err != nil {
		return nil, "", "", fmt.Errorf("failed to write document.xml: %w", err)
	}

	// word/styles.xml
	if err := writeZipFile(w, "word/styles.xml", stylesXML()); err != nil {
		return nil, "", "", fmt.Errorf("failed to write styles.xml: %w", err)
	}

	// word/settings.xml
	if err := writeZipFile(w, "word/settings.xml", settingsXML()); err != nil {
		return nil, "", "", fmt.Errorf("failed to write settings.xml: %w", err)
	}

	// word/fontTable.xml
	if err := writeZipFile(w, "word/fontTable.xml", fontTableXML()); err != nil {
		return nil, "", "", fmt.Errorf("failed to write fontTable.xml: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, "", "", fmt.Errorf("failed to close DOCX zip: %w", err)
	}

	return buf.Bytes(), "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".docx", nil
}

func writeZipFile(w *zip.Writer, name, content string) error {
	f, err := w.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(content))
	return err
}

// buildDocumentXML converts markdown content to WordprocessingML.
func buildDocumentXML(md string) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
            xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
`)

	lines := strings.Split(md, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Headers
		if strings.HasPrefix(trimmed, "#### ") {
			sb.WriteString(fmt.Sprintf("    <w:p><w:pPr><w:pStyle w:val=\"Heading4\"/></w:pPr><w:r><w:t xml:space=\"preserve\">%s</w:t></w:r></w:p>\n", escapeXML(trimmed[5:])))
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			sb.WriteString(fmt.Sprintf("    <w:p><w:pPr><w:pStyle w:val=\"Heading3\"/></w:pPr><w:r><w:t xml:space=\"preserve\">%s</w:t></w:r></w:p>\n", escapeXML(trimmed[4:])))
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			sb.WriteString(fmt.Sprintf("    <w:p><w:pPr><w:pStyle w:val=\"Heading2\"/></w:pPr><w:r><w:t xml:space=\"preserve\">%s</w:t></w:r></w:p>\n", escapeXML(trimmed[3:])))
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			sb.WriteString(fmt.Sprintf("    <w:p><w:pPr><w:pStyle w:val=\"Heading1\"/></w:pPr><w:r><w:t xml:space=\"preserve\">%s</w:t></w:r></w:p>\n", escapeXML(trimmed[2:])))
			continue
		}

		// Bold text
		if strings.HasPrefix(trimmed, "**") && strings.HasSuffix(trimmed, "**") {
			sb.WriteString(fmt.Sprintf("    <w:p><w:r><w:rPr><w:b/></w:rPr><w:t xml:space=\"preserve\">%s</w:t></w:r></w:p>\n", escapeXML(trimmed[2:len(trimmed)-2])))
			continue
		}

		// List items
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			sb.WriteString(fmt.Sprintf("    <w:p><w:pPr><w:pStyle w:val=\"ListBullet\"/></w:pPr><w:r><w:t xml:space=\"preserve\">%s</w:t></w:r></w:p>\n", escapeXML(trimmed[2:])))
			continue
		}

		// Regular paragraph
		sb.WriteString(fmt.Sprintf("    <w:p><w:r><w:t xml:space=\"preserve\">%s</w:t></w:r></w:p>\n", escapeXML(trimmed)))
	}

	sb.WriteString(`  </w:body>
</w:document>`)
	return sb.String()
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// DOCX XML templates

func contentTypesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
  <Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>
  <Override PartName="/word/_rels/document.xml.rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
</Types>`
}

func relsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`
}

func docRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`
}

func stylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:rPr>
      <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/>
      <w:sz w:val="22"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="Heading 1"/>
    <w:basedOn w:val="Normal"/>
    <w:rPr>
      <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/>
      <w:color w:val="0F4761"/>
      <w:sz w:val="32"/>
      <w:b/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="Heading 2"/>
    <w:basedOn w:val="Normal"/>
    <w:rPr>
      <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/>
      <w:color w:val="0F4761"/>
      <w:sz w:val="26"/>
      <w:b/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading3">
    <w:name w:val="Heading 3"/>
    <w:basedOn w:val="Normal"/>
    <w:rPr>
      <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/>
      <w:color w:val="1F4E79"/>
      <w:sz w:val="22"/>
      <w:b/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading4">
    <w:name w:val="Heading 4"/>
    <w:basedOn w:val="Normal"/>
    <w:rPr>
      <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/>
      <w:color w:val="1F4E79"/>
      <w:sz w:val="20"/>
      <w:b/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="ListBullet">
    <w:name w:val="List Bullet"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:ind w:left="720" w:hanging="360"/>
    </w:pPr>
  </w:style>
</w:styles>`
}

func settingsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:zoom w:percent="100"/>
</w:settings>`
}

func fontTableXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:fonts xmlns:ev="http://schemas.openxmlformats.org/markup-compatibility/2006"
         xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
         xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
         xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"
         xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
         mc:Ignorable="w14 w15 w16se19 wp14"
         xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml"
         xmlns:w16se19="http://schemas.microsoft.com/office/word/2019/wordml"
         xmlns:wp14="http://schemas.microsoft.com/office/word/2010/wordprocessingDrawing">
  <w:font w:ascii="Calibri" w:hAnsi="Calibri" w:family="swiss" w:pitchNotation="variable"/>
</w:fonts>`
}
