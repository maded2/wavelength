package convert

import (
	"archive/zip"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// MaxUploadSize is the maximum allowed upload size (10 MB).
const MaxUploadSize = 10 * 1024 * 1024

// Format represents a supported document format.
type Format string

const (
	FormatMarkdown Format = "markdown"
	FormatPDF      Format = "pdf"
	FormatWord     Format = "word" // .docx
)

// Converter converts various document formats to markdown.
type Converter struct{}

// New creates a new Converter.
func New() *Converter {
	return &Converter{}
}

// DetectFormat determines the document format from the file extension.
func DetectFormat(filename string) (Format, error) {
	ext := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(ext, ".md") || strings.HasSuffix(ext, ".markdown"):
		return FormatMarkdown, nil
	case strings.HasSuffix(ext, ".pdf"):
		return FormatPDF, nil
	case strings.HasSuffix(ext, ".docx"):
		return FormatWord, nil
	default:
		return "", fmt.Errorf("unsupported file format: %q (supported: .md, .pdf, .docx)", filename)
	}
}

// Convert reads the document from the reader and converts it to markdown.
func (c *Converter) Convert(r io.Reader, filename string) (string, error) {
	format, err := DetectFormat(filename)
	if err != nil {
		return "", err
	}

	switch format {
	case FormatMarkdown:
		return c.convertMarkdown(r)
	case FormatPDF:
		return c.convertPDF(r)
	case FormatWord:
		return c.convertWord(r)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// convertMarkdown reads markdown content directly.
func (c *Converter) convertMarkdown(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read markdown file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// convertPDF extracts text from a PDF and formats it as markdown.
func (c *Converter) convertPDF(r io.Reader) (string, error) {
	// Read all data into memory for PDF parsing
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF file: %w", err)
	}

	buf := bytes.NewReader(data)
	reader, err := pdf.NewReader(buf, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to parse PDF: %w", err)
	}

	var md strings.Builder
	md.WriteString("# PDF Document\n\n")

	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		content := page.Content()

		// Collect all text elements from the page
		var pageText strings.Builder
		for _, t := range content.Text {
			if pageText.Len() > 0 {
				pageText.WriteString(" ")
			}
			pageText.WriteString(t.S)
		}

		// Split into lines and format as markdown
		lines := strings.Split(pageText.String(), "\n")
		var paragraph strings.Builder

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				if paragraph.Len() > 0 {
					md.WriteString(paragraph.String())
					md.WriteString("\n\n")
					paragraph.Reset()
				}
			} else {
				if paragraph.Len() > 0 {
					paragraph.WriteString(" ")
				}
				paragraph.WriteString(trimmed)
			}
		}

		// Flush remaining paragraph
		if paragraph.Len() > 0 {
			md.WriteString(paragraph.String())
			md.WriteString("\n\n")
		}
	}

	result := strings.TrimSpace(md.String())
	if result == "# PDF Document" {
		return "", errors.New("PDF contains no extractable text (may be scanned images)")
	}
	return result, nil
}

// convertWord extracts text from a DOCX file (ZIP of XML) and formats it as markdown.
func (c *Converter) convertWord(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read DOCX file: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to parse DOCX (not a valid ZIP): %w", err)
	}

	// Find word/document.xml
	var docXML *zip.File
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			docXML = f
			break
		}
	}

	if docXML == nil {
		return "", errors.New("invalid DOCX file: missing word/document.xml")
	}

	rc, err := docXML.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open document.xml: %w", err)
	}
	defer rc.Close()

	return c.parseWordXML(rc)
}

// parseWordXML extracts text from WordprocessingML and converts to markdown.
func (c *Converter) parseWordXML(r io.Reader) (string, error) {
	var md strings.Builder
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	var paragraph strings.Builder
	var currentStyle string
	var isBold bool
	var isItalic bool
	var pendingText strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Detect paragraph style
		if idx := strings.Index(line, "w:val=\""); idx != -1 {
			valStart := idx + 7
			valEnd := strings.Index(line[valStart:], "\"")
			if valEnd != -1 {
				style := line[valStart : valStart+valEnd]
				if strings.HasPrefix(style, "Heading") {
					currentStyle = style
				}
			}
		}

		// Detect bold
		if strings.Contains(line, "<w:b/") || strings.Contains(line, "<w:b/>") {
			if strings.Contains(line, "<w:b>") || strings.Contains(line, "<w:b />") {
				isBold = true
			} else {
				isBold = false
			}
		}

		// Detect italic
		if strings.Contains(line, "<w:i/") || strings.Contains(line, "<w:i/>") {
			if strings.Contains(line, "<w:i>") || strings.Contains(line, "<w:i />") {
				isItalic = true
			} else {
				isItalic = false
			}
		}

		// Extract text content from <w:t> elements
		for {
			tStart := strings.Index(line, "<w:t")
			if tStart == -1 {
				break
			}

			// Find the closing > of <w:t...>
			gtIdx := strings.Index(line[tStart:], ">")
			if gtIdx == -1 {
				break
			}
			contentStart := tStart + gtIdx + 1

			// Find closing </w:t>
			tEnd := strings.Index(line[contentStart:], "</w:t>")
			if tEnd == -1 {
				break
			}

			textContent := line[contentStart : contentStart+tEnd]
			textContent = unescapeXML(textContent)

			if textContent == "" {
				line = line[contentStart+tEnd+6:]
				continue
			}

			// Apply formatting
			formatted := textContent
			if isBold && isItalic {
				formatted = "***" + formatted + "***"
			} else if isBold {
				formatted = "**" + formatted + "**"
			} else if isItalic {
				formatted = "*" + formatted + "*"
			}

			pendingText.WriteString(formatted)
			line = line[contentStart+tEnd+6:]
		}

		// Detect paragraph end
		if strings.Contains(line, "</w:p>") {
			text := strings.TrimSpace(pendingText.String())
			if text != "" {
				// Apply heading style
				switch {
				case currentStyle == "Heading1":
					if paragraph.Len() > 0 {
						md.WriteString(paragraph.String())
						md.WriteString("\n\n")
						paragraph.Reset()
					}
					md.WriteString("# ")
					md.WriteString(text)
					md.WriteString("\n\n")
					currentStyle = ""
				case currentStyle == "Heading2":
					if paragraph.Len() > 0 {
						md.WriteString(paragraph.String())
						md.WriteString("\n\n")
						paragraph.Reset()
					}
					md.WriteString("## ")
					md.WriteString(text)
					md.WriteString("\n\n")
					currentStyle = ""
				case currentStyle == "Heading3":
					if paragraph.Len() > 0 {
						md.WriteString(paragraph.String())
						md.WriteString("\n\n")
						paragraph.Reset()
					}
					md.WriteString("### ")
					md.WriteString(text)
					md.WriteString("\n\n")
					currentStyle = ""
				case currentStyle == "Heading4":
					if paragraph.Len() > 0 {
						md.WriteString(paragraph.String())
						md.WriteString("\n\n")
						paragraph.Reset()
					}
					md.WriteString("#### ")
					md.WriteString(text)
					md.WriteString("\n\n")
					currentStyle = ""
				default:
					// Regular paragraph
					if paragraph.Len() > 0 {
						paragraph.WriteString(" ")
					}
					paragraph.WriteString(text)
				}
			} else if paragraph.Len() > 0 {
				// Empty text but paragraph end — flush paragraph
				md.WriteString(paragraph.String())
				md.WriteString("\n\n")
				paragraph.Reset()
			}

			pendingText.Reset()
			isBold = false
			isItalic = false
		}
	}

	// Flush any remaining content
	if paragraph.Len() > 0 {
		md.WriteString(paragraph.String())
		md.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read document.xml: %w", err)
	}

	result := strings.TrimSpace(md.String())
	if result == "" {
		return "", errors.New("DOCX contains no extractable text")
	}
	return result, nil
}

// unescapeXML unescapes common XML entities.
func unescapeXML(s string) string {
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&#10;", "\n")
	s = strings.ReplaceAll(s, " xml:space=\"preserve\"", "")
	return s
}
