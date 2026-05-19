package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/schema"
)

// Tool defines an LLM function tool that can be called during a conversation.
type Tool struct {
	// Info describes the tool's name, description, and parameters.
	Info *schema.ToolInfo
	// Execute is called when the LLM invokes this tool.
	// It receives the JSON arguments string and returns the result string.
	Execute func(ctx context.Context, args string) (string, error)
}

// FileReadTool creates a tool that allows the LLM to read files from a topic's data directory.
// The tool can read attachment files and the requirement document.
func FileReadTool(topicDir string) *Tool {
	return &Tool{
		Info: &schema.ToolInfo{
			Name: "read_file",
			Desc: "Read the contents of a file. Use this to examine uploaded reference documents or the current requirement document. " +
				"Available files: 'document.md' (current requirement document), or any uploaded attachment file by its filename. " +
				"Pass the filename (e.g., 'document.md' or 'spec.pdf') as the argument.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"filename": {
					Type:     schema.String,
					Desc:     "The name of the file to read (e.g., 'document.md', 'requirements.pdf', 'design-spec.docx')",
					Required: true,
				},
			}),
		},
		Execute: func(ctx context.Context, args string) (string, error) {
			var req struct {
				Filename string `json:"filename"`
			}
			if err := json.Unmarshal([]byte(args), &req); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if req.Filename == "" {
				return "", fmt.Errorf("filename is required")
			}

			// Sanitize: prevent directory traversal
			filename := filepath.Base(req.Filename)
			filePath := filepath.Join(topicDir, filename)

			// Ensure the file is within the topic directory
			absTopicDir, err := filepath.Abs(topicDir)
			if err != nil {
				return "", fmt.Errorf("failed to resolve topic directory: %w", err)
			}
			absFilePath, err := filepath.Abs(filePath)
			if err != nil {
				return "", fmt.Errorf("failed to resolve file path: %w", err)
			}
			if !filepath.HasPrefix(absFilePath, absTopicDir) {
				return "", fmt.Errorf("access denied: file path escapes topic directory")
			}

			log.Printf("[TOOL:read_file] Reading file %q from %q", filename, topicDir)
			data, err := os.ReadFile(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					log.Printf("[TOOL:read_file] File not found: %q", filename)
					return "", fmt.Errorf("file not found: %q", filename)
				}
				log.Printf("[TOOL:read_file] Error reading file %q: %v", filename, err)
				return "", fmt.Errorf("failed to read file %q: %w", filename, err)
			}

			// Limit response size to avoid overwhelming the context
			maxLen := 32000
			if len(data) > maxLen {
				log.Printf("[TOOL:read_file] File %q read: %d bytes (truncated to %d)", filename, len(data), maxLen)
				return string(data[:maxLen]) + fmt.Sprintf("\n\n...(truncated, file is %d bytes total)", len(data)), nil
			}
			log.Printf("[TOOL:read_file] File %q read: %d bytes", filename, len(data))
			return string(data), nil
		},
	}
}

// maxDocumentSize is the maximum document size allowed for writing (64KB).
const maxDocumentSize = 64 * 1024

// WriteDocumentTool creates a tool that allows the LLM to save the requirement document.
// The document is written to 'document.md' in the topic directory.
// The onWrite callback is called after a successful write with the document content.
func WriteDocumentTool(topicDir string, onWrite func(content string)) *Tool {
	return &Tool{
		Info: &schema.ToolInfo{
			Name: "write_document",
			Desc: "Save or update the living requirements document. Use this to persist the complete " +
				"markdown document after creating or updating it. The document will be saved as 'document.md'. " +
				"Always provide the FULL document content, not just changes. " +
				"Use this tool after you have finalized the document content to save.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"content": {
					Type:     schema.String,
					Desc:     "The complete markdown content of the requirements document",
					Required: true,
				},
			}),
		},
		Execute: func(ctx context.Context, args string) (string, error) {
			var req struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(args), &req); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if req.Content == "" {
				return "", fmt.Errorf("document content is required")
			}
			if len(req.Content) > maxDocumentSize {
				log.Printf("[TOOL:write_document] Document too large: %d bytes (max %d bytes)", len(req.Content), maxDocumentSize)
				return "", fmt.Errorf("document too large: %d bytes (max %d bytes)", len(req.Content), maxDocumentSize)
			}

			// Write to document.md in the topic directory
			filePath := filepath.Join(topicDir, "document.md")
			log.Printf("[TOOL:write_document] Writing document to %q (%d bytes)", filePath, len(req.Content))
			if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
				log.Printf("[TOOL:write_document] Error writing document to %q: %v", filePath, err)
				return "", fmt.Errorf("failed to write document: %w", err)
			}

			// Notify caller via callback
			if onWrite != nil {
				onWrite(req.Content)
			}

			log.Printf("[TOOL:write_document] Document saved successfully (%d bytes)", len(req.Content))
			return fmt.Sprintf("Document saved successfully (%d characters, %d bytes).", len(req.Content), len(req.Content)), nil
		},
	}
}

// ToSchemaTools converts a slice of Tool pointers to schema.ToolInfo pointers for passing to the LLM.
func ToSchemaTools(tools []*Tool) []*schema.ToolInfo {
	result := make([]*schema.ToolInfo, len(tools))
	for i, t := range tools {
		result[i] = t.Info
	}
	return result
}

// ToolCallRequest represents a single tool call from the LLM response.
type ToolCallRequest struct {
	ID       string
	Name     string
	ArgsJSON string
}

// ExtractToolCalls parses an LLM response message and extracts any tool call requests.
// Returns nil if the response has no tool calls.
func ExtractToolCalls(msg *schema.Message) []ToolCallRequest {
	if msg == nil {
		log.Printf("[TOOL-EXTRACT] nil message")
		return nil
	}

	log.Printf("[TOOL-EXTRACT] Message: role=%q, content_len=%d, tool_calls_len=%d, reasoning_len=%d", msg.Role, len(msg.Content), len(msg.ToolCalls), len(msg.ReasoningContent))

	if len(msg.ToolCalls) == 0 {
		log.Printf("[TOOL-EXTRACT] No tool calls in message")
		return nil
	}

	var result []ToolCallRequest
	for _, tc := range msg.ToolCalls {
		log.Printf("[TOOL-EXTRACT] Tool call: id=%q, type=%q, function.name=%q, function.args_len=%d", tc.ID, tc.Type, tc.Function.Name, len(tc.Function.Arguments))
		if tc.Function != (schema.FunctionCall{}) {
			result = append(result, ToolCallRequest{
				ID:       tc.ID,
				Name:     tc.Function.Name,
				ArgsJSON: tc.Function.Arguments,
			})
		}
	}
	log.Printf("[TOOL-EXTRACT] Extracted %d tool call request(s)", len(result))
	return result
}

// BuildToolResponses creates tool result messages for the given tool call results.
func BuildToolResponses(calls []ToolCallRequest, results []string) []*schema.Message {
	msgs := make([]*schema.Message, len(calls))
	for i, call := range calls {
		content := ""
		if i < len(results) {
			content = results[i]
		}
		msgs[i] = schema.ToolMessage(content, call.ID)
	}
	return msgs
}
