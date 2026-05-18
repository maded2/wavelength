package topic

import "fmt"

// Format represents a supported document format for export and conversion.
type Format string

const (
	FormatMarkdown Format = "markdown"
	FormatPDF      Format = "pdf"
	FormatWord     Format = "word" // .docx
)

// Validate checks if the format is supported.
func (f Format) Validate() error {
	switch f {
	case FormatMarkdown, FormatPDF, FormatWord:
		return nil
	default:
		return fmt.Errorf("unsupported format: %q (supported: %s, %s, %s)",
			f, FormatMarkdown, FormatPDF, FormatWord)
	}
}

// Meta returns the serializable metadata for a topic (no messages, document, or attachments).
func (t *Topic) Meta() topicMeta {
	return topicMeta{
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Status:       t.Status,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
		MessageCount: t.MessageCount,
	}
}
