package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestFileReadTool(t *testing.T) {
	t.Run("reads existing file", func(t *testing.T) {
		dir := t.TempDir()
		testFile := filepath.Join(dir, "document.md")
		if err := os.WriteFile(testFile, []byte("# Test Document\n\nSome content."), 0644); err != nil {
			t.Fatal(err)
		}

		tool := FileReadTool(dir)
		args, _ := json.Marshal(map[string]string{"filename": "document.md"})
		result, err := tool.Execute(context.Background(), string(args))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "# Test Document\n\nSome content." {
			t.Errorf("unexpected result: %q", result)
		}
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		dir := t.TempDir()
		tool := FileReadTool(dir)
		args, _ := json.Marshal(map[string]string{"filename": "missing.md"})
		_, err := tool.Execute(context.Background(), string(args))
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("prevents directory traversal", func(t *testing.T) {
		dir := t.TempDir()
		outsideFile := filepath.Join(dir, "..", "outside.txt")
		if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(outsideFile)

		tool := FileReadTool(dir)
		args, _ := json.Marshal(map[string]string{"filename": "../outside.txt"})
		_, err := tool.Execute(context.Background(), string(args))
		if err == nil {
			t.Fatal("expected error for directory traversal")
		}
	})

	t.Run("truncates large files", func(t *testing.T) {
		dir := t.TempDir()
		testFile := filepath.Join(dir, "large.txt")
		largeContent := make([]byte, 40000)
		for i := range largeContent {
			largeContent[i] = 'x'
		}
		if err := os.WriteFile(testFile, largeContent, 0644); err != nil {
			t.Fatal(err)
		}

		tool := FileReadTool(dir)
		args, _ := json.Marshal(map[string]string{"filename": "large.txt"})
		result, err := tool.Execute(context.Background(), string(args))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) > 32500 {
			t.Errorf("expected truncated result, got %d chars", len(result))
		}
	})

	t.Run("returns error for empty filename", func(t *testing.T) {
		dir := t.TempDir()
		tool := FileReadTool(dir)
		args, _ := json.Marshal(map[string]string{"filename": ""})
		_, err := tool.Execute(context.Background(), string(args))
		if err == nil {
			t.Fatal("expected error for empty filename")
		}
	})
}

func TestExtractToolCalls(t *testing.T) {
	t.Run("returns nil for message without tool calls", func(t *testing.T) {
		msg := schema.UserMessage("hello")
		result := ExtractToolCalls(msg)
		if result != nil {
			t.Errorf("expected nil, got: %v", result)
		}
	})

	t.Run("returns nil for nil message", func(t *testing.T) {
		result := ExtractToolCalls(nil)
		if result != nil {
			t.Errorf("expected nil, got: %v", result)
		}
	})
}

func TestToSchemaTools(t *testing.T) {
	tools := []*Tool{
		{Info: &schema.ToolInfo{Name: "read_file", Desc: "Read a file"}},
		{Info: &schema.ToolInfo{Name: "write_file", Desc: "Write a file"}},
	}
	result := ToSchemaTools(tools)
	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}
	if result[0].Name != "read_file" {
		t.Errorf("expected 'read_file', got %q", result[0].Name)
	}
	if result[1].Name != "write_file" {
		t.Errorf("expected 'write_file', got %q", result[1].Name)
	}
}

func TestWriteDocumentTool(t *testing.T) {
	t.Run("writes document successfully", func(t *testing.T) {
		dir := t.TempDir()
		tool := WriteDocumentTool(dir, nil)
		content := "# Requirements\n\n## Overview\n\nTest document."
		args, _ := json.Marshal(map[string]string{"content": content})
		result, err := tool.Execute(context.Background(), string(args))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "saved successfully") {
			t.Errorf("expected success message, got: %q", result)
		}

		// Verify file was written
		data, err := os.ReadFile(filepath.Join(dir, "document.md"))
		if err != nil {
			t.Fatalf("file not written: %v", err)
		}
		if string(data) != content {
			t.Errorf("content mismatch: got %q", string(data))
		}
	})

	t.Run("returns error for empty content", func(t *testing.T) {
		dir := t.TempDir()
		tool := WriteDocumentTool(dir, nil)
		args, _ := json.Marshal(map[string]string{"content": ""})
		_, err := tool.Execute(context.Background(), string(args))
		if err == nil {
			t.Fatal("expected error for empty content")
		}
	})

	t.Run("rejects oversized documents", func(t *testing.T) {
		dir := t.TempDir()
		tool := WriteDocumentTool(dir, nil)
		largeContent := make([]byte, maxDocumentSize+1)
		for i := range largeContent {
			largeContent[i] = 'x'
		}
		args, _ := json.Marshal(map[string]string{"content": string(largeContent)})
		_, err := tool.Execute(context.Background(), string(args))
		if err == nil {
			t.Fatal("expected error for oversized document")
		}
	})

	t.Run("overwrites existing document", func(t *testing.T) {
		dir := t.TempDir()
		tool := WriteDocumentTool(dir, nil)

		// Write first version
		args1, _ := json.Marshal(map[string]string{"content": "# V1"})
		if _, err := tool.Execute(context.Background(), string(args1)); err != nil {
			t.Fatal(err)
		}

		// Write second version
		args2, _ := json.Marshal(map[string]string{"content": "# V2 - Updated"})
		if _, err := tool.Execute(context.Background(), string(args2)); err != nil {
			t.Fatal(err)
		}

		data, err := os.ReadFile(filepath.Join(dir, "document.md"))
		if err != nil {
			t.Fatalf("file not found: %v", err)
		}
		if string(data) != "# V2 - Updated" {
			t.Errorf("expected updated content, got: %q", string(data))
		}
	})

	t.Run("calls onWrite callback", func(t *testing.T) {
		dir := t.TempDir()
		var captured string
		tool := WriteDocumentTool(dir, func(content string) {
			captured = content
		})
		content := "# Callback Test"
		args, _ := json.Marshal(map[string]string{"content": content})
		if _, err := tool.Execute(context.Background(), string(args)); err != nil {
			t.Fatal(err)
		}
		if captured != content {
			t.Errorf("callback did not receive correct content: got %q", captured)
		}
	})

	t.Run("nil callback is safe", func(t *testing.T) {
		dir := t.TempDir()
		tool := WriteDocumentTool(dir, nil)
		args, _ := json.Marshal(map[string]string{"content": "test"})
		_, err := tool.Execute(context.Background(), string(args))
		if err != nil {
			t.Fatalf("unexpected error with nil callback: %v", err)
		}
	})
}

func TestBuildToolResponses(t *testing.T) {
	calls := []ToolCallRequest{
		{ID: "call_1", Name: "read_file", ArgsJSON: `{"filename":"test.md"}`},
		{ID: "call_2", Name: "read_file", ArgsJSON: `{"filename":"doc.md"}`},
	}
	results := []string{"content1", "content2"}
	msgs := BuildToolResponses(calls, results)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "content1" {
		t.Errorf("expected 'content1', got %q", msgs[0].Content)
	}
	if msgs[1].Content != "content2" {
		t.Errorf("expected 'content2', got %q", msgs[1].Content)
	}
}
