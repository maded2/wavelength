package mcp

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestJSONSchemaTypeToEinoType(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected schema.DataType
	}{
		{"string", schema.String},
		{"number", schema.Number},
		{"integer", schema.Integer},
		{"boolean", schema.Boolean},
		{"array", schema.Array},
		{"object", schema.Object},
		{"unknown", schema.String},
		{nil, schema.String},
		{123, schema.String},
	}

	for _, tc := range tests {
		result := jsonSchemaTypeToEinoType(tc.input)
		if result != tc.expected {
			t.Errorf("jsonSchemaTypeToEinoType(%v) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestBuildParamsOneOf(t *testing.T) {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"filename": map[string]interface{}{
				"type":        "string",
				"description": "The file to read",
			},
			"line": map[string]interface{}{
				"type":        "integer",
				"description": "Starting line number",
			},
			"format": map[string]interface{}{
				"type":  "string",
				"enum":  []interface{}{"json", "yaml", "toml"},
			},
		},
		"required": []interface{}{"filename"},
	}

	paramsOneOf := buildParamsOneOf(inputSchema)
	if paramsOneOf == nil {
		t.Fatal("buildParamsOneOf returned nil")
	}

	// Convert to JSON Schema to verify it is valid
	js, err := paramsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema failed: %v", err)
	}
	if js == nil {
		t.Fatal("ToJSONSchema returned nil")
	}

	// Verify basic structure
	if js.Type != "object" {
		t.Errorf("expected type 'object', got %q", js.Type)
	}

	// Verify properties count
	if js.Properties == nil {
		t.Fatal("expected properties in JSON schema")
	}
	if js.Properties.Len() != 3 {
		t.Errorf("expected 3 properties, got %d", js.Properties.Len())
	}

	// Verify required fields
	if len(js.Required) != 1 || js.Required[0] != "filename" {
		t.Errorf("expected required=[filename], got %v", js.Required)
	}
}

func TestBuildParamsOneOfEmpty(t *testing.T) {
	paramsOneOf := buildParamsOneOf(nil)
	if paramsOneOf == nil {
		t.Fatal("buildParamsOneOf(nil) returned nil")
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{nil, ""},
		{"hello", "hello"},
		{123, "123"},
		{true, "true"},
	}

	for _, tc := range tests {
		result := toString(tc.input)
		if result != tc.expected {
			t.Errorf("toString(%v) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		input    []string
		sep      string
		expected string
	}{
		{nil, ",", ""},
		{[]string{}, ",", ""},
		{[]string{"a"}, ",", "a"},
		{[]string{"a", "b"}, ",", "a,b"},
		{[]string{"a", "b", "c"}, ", ", "a, b, c"},
		{[]string{"one", "two"}, "\n", "one\ntwo"},
	}

	for _, tc := range tests {
		result := joinStrings(tc.input, tc.sep)
		if result != tc.expected {
			t.Errorf("joinStrings(%v, %q) = %q, want %q", tc.input, tc.sep, result, tc.expected)
		}
	}
}
