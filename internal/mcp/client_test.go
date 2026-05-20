package mcp

import (
	"testing"

	"wavelength/internal/config"
)

func TestExtractServerName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"mcp::filesystem::read_file", "filesystem"},
		{"mcp::web_search::search", "web_search"},
		{"mcp::my_server::my_tool", "my_server"},
		{"mcp::a::b", "a"},
		{"mcp::", ""},
		{"mcp", ""},
		{"no_prefix", ""},
	}

	for _, tc := range tests {
		result := extractServerName(tc.input)
		if result != tc.expected {
			t.Errorf("extractServerName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestExtractToolName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"mcp::filesystem::read_file", "read_file"},
		{"mcp::web_search::search", "search"},
		{"mcp::my_server::my_tool", "my_tool"},
		{"mcp::a::b", "b"},
		{"mcp::", ""},
		{"mcp", ""},
		{"no_prefix", ""},
	}

	for _, tc := range tests {
		result := extractToolName(tc.input)
		if result != tc.expected {
			t.Errorf("extractToolName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestNew(t *testing.T) {
	cfg := &config.MCPConfig{
		Servers: []config.MCPServerConfig{
			{Name: "test", Transport: "stdio", Command: "echo"},
		},
	}
	mgr := New(cfg)
	if mgr == nil {
		t.Fatal("New() returned nil")
	}
	if mgr.cfg != cfg {
		t.Error("Manager config not set")
	}
	if len(mgr.sessions) != 0 {
		t.Error("Expected empty sessions map")
	}
}

func TestNewEmptyConfig(t *testing.T) {
	mgr := New(&config.MCPConfig{})
	if mgr == nil {
		t.Fatal("New() returned nil")
	}
	if len(mgr.Tools()) != 0 {
		t.Error("Expected no tools from empty config")
	}
}

func TestHasMCP(t *testing.T) {
	cfg := &config.Config{
		MCP: config.MCPConfig{
			Servers: []config.MCPServerConfig{
				{Name: "test", Transport: "stdio", Command: "echo"},
			},
		},
	}
	if !cfg.HasMCP() {
		t.Error("Expected HasMCP() to return true")
	}

	emptyCfg := &config.Config{}
	if emptyCfg.HasMCP() {
		t.Error("Expected HasMCP() to return false for empty config")
	}
}
